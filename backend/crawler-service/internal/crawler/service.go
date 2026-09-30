package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/demonsgod83-sys/seo-bot/crawler-service/internal/platform/config"
	"github.com/demonsgod83-sys/seo-bot/crawler-service/internal/platform/storage"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

//==========================================//
//              SERVICE INTERFACE           //
//==========================================//

type Service interface {
	ExecuteCrawl(ctx context.Context, cmd StartCrawlCommand) error
	GetPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]CrawlPage, error)
}

type service struct {
	cfg           *config.Config
	repository    Repository
	storage       storage.StorageEngine
	fetcher       Fetcher
	robotsHandler *RobotsHandler
	js            nats.JetStreamContext
	logger        *zap.Logger
}

func NewService(
	cfg *config.Config,
	repository Repository,
	storage storage.StorageEngine,
	js nats.JetStreamContext,
	logger *zap.Logger,
) Service {
	fetcher := NewSafeFetcher(cfg.UserAgent, time.Duration(cfg.PageTimeoutSec)*time.Second)
	robotsHandler := NewRobotsHandler(fetcher, logger)

	return &service{
		cfg:           cfg,
		repository:    repository,
		storage:       storage,
		fetcher:       fetcher,
		robotsHandler: robotsHandler,
		js:            js,
		logger:        logger,
	}
}

//==========================================//
//          CRAWL EXECUTION LOGIC           //
//==========================================//

// ExecuteCrawl runs the full discovery, fetch, snapshot, and event publishing pipeline.
func (s *service) ExecuteCrawl(ctx context.Context, cmd StartCrawlCommand) error {
	startTime := time.Now()
	s.logger.Info("starting crawl job",
		zap.String("audit_id", cmd.AuditID.String()),
		zap.String("domain", cmd.Domain),
		zap.String("normalized_url", cmd.NormalizedURL),
	)

	baseParsedURL, err := url.Parse(cmd.NormalizedURL)
	if err != nil {
		return s.failCrawl(ctx, cmd, fmt.Sprintf("invalid normalized URL: %v", err))
	}

	// 1. Fetch & Evaluate robots.txt
	robotsRes, err := s.robotsHandler.FetchAndParseRobots(ctx, baseParsedURL, s.cfg.UserAgent)
	if err != nil {
		s.logger.Warn("error parsing robots.txt — proceeding with polite defaults", zap.Error(err))
	}

	if robotsRes != nil && !robotsRes.Allowed {
		s.logger.Warn("crawling disallowed by robots.txt",
			zap.String("domain", cmd.Domain),
		)
		return s.failCrawl(ctx, cmd, "crawling disallowed by site robots.txt policy")
	}

	// 2. Setup polite domain rate limiter
	rps := s.cfg.DefaultRPS
	if robotsRes != nil && robotsRes.CrawlDelay > 0 {
		rps = 1.0 / robotsRes.CrawlDelay.Seconds()
		if rps > 5.0 {
			rps = 5.0
		}
	}
	limiter := rate.NewLimiter(rate.Limit(rps), 1)

	// 3. Discovery: Sitemaps & Initial seed
	frontier := make([]string, 0)
	var frontierMu sync.Mutex

	visited := make(map[string]bool)
	var visitedMu sync.Mutex

	// Seed with root normalized URL
	cleanRoot := NormalizeURL(baseParsedURL)
	frontier = append(frontier, cleanRoot)
	visited[cleanRoot] = true

	// Discover and seed sitemap URLs
	if robotsRes != nil {
		sitemapURLs := s.robotsHandler.DiscoverSitemapURLs(ctx, baseParsedURL, robotsRes.Sitemaps)
		for _, smURL := range sitemapURLs {
			if !visited[smURL] {
				visited[smURL] = true
				frontier = append(frontier, smURL)
			}
		}
	}

	totalDiscovered := len(frontier)
	var totalFetched int
	var totalErrors int

	// 4. Crawl Loop with bounded concurrency worker pool
	concurrency := s.cfg.MaxConcurrency
	if concurrency < 1 {
		concurrency = 1
	}

	workChan := make(chan string, s.cfg.MaxPagesPerJob*2)
	var wg sync.WaitGroup

	// Worker routine
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for targetURL := range workChan {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Enforce per-domain rate limit
				_ = limiter.Wait(ctx)

				// Respect robots.txt rule for individual page path
				if robotsRes != nil && robotsRes.RobotsData != nil {
					pURL, parseErr := url.Parse(targetURL)
					if parseErr == nil {
						group := robotsRes.RobotsData.FindGroup(s.cfg.UserAgent)
						if group == nil {
							group = robotsRes.RobotsData.FindGroup("*")
						}
						if group != nil && !group.Test(pURL.Path) {
							s.logger.Debug("skipping path disallowed by robots.txt", zap.String("url", targetURL))
							continue
						}
					}
				}

				// Safe Fetch
				fetchRes, fetchErr := s.fetcher.Fetch(ctx, targetURL)
				page := &CrawlPage{
					AuditID:  cmd.AuditID,
					TenantID: cmd.TenantID,
					URL:      targetURL,
				}

				if fetchErr != nil {
					page.ErrorMessage = fetchErr.Error()
					page.StatusCode = 0
					s.logger.Warn("page fetch failed", zap.String("url", targetURL), zap.Error(fetchErr))
				} else {
					page.StatusCode = fetchRes.StatusCode
					page.ContentType = fetchRes.ContentType
					page.ByteSize = int64(len(fetchRes.Body))
					page.FetchDurationMs = fetchRes.DurationMs

					// If 200 OK and text/html, save raw snapshot to storage
					if fetchRes.StatusCode == 200 && strings.Contains(strings.ToLower(fetchRes.ContentType), "html") {
						storagePath, sErr := s.storage.SaveSnapshot(ctx, cmd.AuditID, targetURL, fetchRes.Body)
						if sErr != nil {
							s.logger.Error("failed to save snapshot", zap.Error(sErr))
						} else {
							page.StoragePath = storagePath
						}

						// Link discovery: extract internal links if still under page cap
						parsedPageURL, _ := url.Parse(targetURL)
						if parsedPageURL != nil {
							links := ExtractInternalLinks(parsedPageURL, fetchRes.Body)
							frontierMu.Lock()
							visitedMu.Lock()
							for _, link := range links {
								if !visited[link] && (len(visited) < s.cfg.MaxPagesPerJob) {
									visited[link] = true
									frontier = append(frontier, link)
									totalDiscovered++
									select {
									case workChan <- link:
									default:
									}
								}
							}
							visitedMu.Unlock()
							frontierMu.Unlock()
						}
					}
				}

				// Persist page metadata
				if err := s.repository.SaveCrawlPage(ctx, page); err != nil {
					s.logger.Error("failed to save crawl page record", zap.Error(err))
				}

				frontierMu.Lock()
				if page.StatusCode == 200 {
					totalFetched++
				} else {
					totalErrors++
				}
				frontierMu.Unlock()
			}
		}()
	}

	// Feed initial frontier into work channel
	frontierMu.Lock()
	initialCount := len(frontier)
	if initialCount > s.cfg.MaxPagesPerJob {
		initialCount = s.cfg.MaxPagesPerJob
	}
	for i := 0; i < initialCount; i++ {
		workChan <- frontier[i]
	}
	frontierMu.Unlock()

	// Close work channel and wait for workers to finish
	close(workChan)
	wg.Wait()

	durationMs := time.Since(startTime).Milliseconds()

	// 5. Record Crawl Summary in DB
	summary := &CrawlSummary{
		AuditID:         cmd.AuditID,
		TenantID:        cmd.TenantID,
		Domain:          cmd.Domain,
		BaseURL:         cmd.NormalizedURL,
		RobotsAllowed:   true,
		SitemapFound:    robotsRes != nil && len(robotsRes.Sitemaps) > 0,
		TotalDiscovered: totalDiscovered,
		TotalFetched:    totalFetched,
		TotalErrors:     totalErrors,
		DurationMs:      durationMs,
		CompletedAt:     time.Now().UTC(),
	}
	_ = s.repository.SaveCrawlSummary(ctx, summary)

	s.logger.Info("crawl job finished",
		zap.String("audit_id", cmd.AuditID.String()),
		zap.Int("total_fetched", totalFetched),
		zap.Int("total_errors", totalErrors),
		zap.Int64("duration_ms", durationMs),
	)

	// 6. If 0 pages were successfully fetched, publish failure event
	if totalFetched == 0 {
		return s.failCrawl(ctx, cmd, "zero pages could be fetched successfully from target domain")
	}

	// 7. Publish Crawl Completed Event to NATS
	return s.publishCrawlCompleted(ctx, cmd, totalFetched, totalErrors, durationMs)
}

// GetPagesByAuditID implements [Service].
func (s *service) GetPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]CrawlPage, error) {
	return s.repository.FindPagesByAuditID(ctx, auditID)
}

//==========================================//
//             HELPER FUNCTIONS             //
//==========================================//

func (s *service) publishCrawlCompleted(ctx context.Context, cmd StartCrawlCommand, fetched, errs int, dur int64) error {
	evt := CrawlCompletedEvent{
		AuditID:       cmd.AuditID,
		TenantID:      cmd.TenantID,
		Domain:        cmd.Domain,
		NormalizedURL: cmd.NormalizedURL,
		TotalFetched:  fetched,
		TotalErrors:   errs,
		DurationMs:    dur,
		CompletedAt:   time.Now().UTC(),
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal crawl completed event: %w", err)
	}

	msgID := fmt.Sprintf("audit:%s:crawl_completed", cmd.AuditID)
	_, err = s.js.Publish(
		"audit.events.crawl_completed",
		payload,
		nats.MsgId(msgID),
		nats.Context(ctx),
	)
	if err != nil {
		s.logger.Error("failed to publish crawl completed event", zap.Error(err))
		return err
	}

	s.logger.Info("published crawl.completed event", zap.String("audit_id", cmd.AuditID.String()))
	return nil
}

func (s *service) failCrawl(ctx context.Context, cmd StartCrawlCommand, reason string) error {
	evt := CrawlFailedEvent{
		AuditID:       cmd.AuditID,
		TenantID:      cmd.TenantID,
		Domain:        cmd.Domain,
		NormalizedURL: cmd.NormalizedURL,
		Reason:        reason,
		FailedAt:      time.Now().UTC(),
	}

	payload, err := json.Marshal(evt)
	if err == nil {
		msgID := fmt.Sprintf("audit:%s:crawl_failed", cmd.AuditID)
		_, _ = s.js.Publish("audit.events.crawl_failed", payload, nats.MsgId(msgID), nats.Context(ctx))
	}

	return fmt.Errorf("crawl failed: %s", reason)
}
