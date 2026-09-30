package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/demonsgod83-sys/seo-bot/parser-service/internal/platform/storage"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

//==========================================//
//              SERVICE INTERFACE           //
//==========================================//

type Service interface {
	ExecuteParse(ctx context.Context, event CrawlCompletedEvent) error
	GetParsedPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]ParsedPage, error)
}

type service struct {
	repository Repository
	storage    storage.StorageEngine
	js         nats.JetStreamContext
	logger     *zap.Logger
}

func NewService(
	repository Repository,
	storage storage.StorageEngine,
	js nats.JetStreamContext,
	logger *zap.Logger,
) Service {
	return &service{
		repository: repository,
		storage:    storage,
		js:         js,
		logger:     logger,
	}
}

//==========================================//
//          PARSE EXECUTION PIPELINE        //
//==========================================//

// ExecuteParse processes all raw snapshots for an audit into structured database records.
func (s *service) ExecuteParse(ctx context.Context, event CrawlCompletedEvent) error {
	startTime := time.Now()
	s.logger.Info("starting parse job for audit",
		zap.String("audit_id", event.AuditID.String()),
		zap.String("domain", event.Domain),
	)

	// 1. Fetch raw crawl pages from database
	crawlPages, err := s.repository.FindCrawlPagesByAuditID(ctx, event.AuditID)
	if err != nil {
		return s.failParse(ctx, event, fmt.Sprintf("failed to query crawl pages: %v", err))
	}

	if len(crawlPages) == 0 {
		return s.failParse(ctx, event, "no crawl pages found to parse for audit")
	}

	var totalParsed int
	var totalFailed int

	// 2. Process each page snapshot with per-page error isolation
	for _, cp := range crawlPages {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		parsedPage := &ParsedPage{
			AuditID:         event.AuditID,
			TenantID:        event.TenantID,
			URL:             cp.URL,
			StatusCode:      cp.StatusCode,
			ContentType:     cp.ContentType,
			ByteSize:        cp.ByteSize,
			FetchDurationMs: cp.FetchDurationMs,
			OpenGraph:       make(map[string]string),
			TwitterCard:     make(map[string]string),
			Hreflangs:       make([]HreflangTag, 0),
			Headings:        make([]Heading, 0),
			Links:           make([]PageLink, 0),
			Images:          make([]PageImage, 0),
			StructuredData:  make([]string, 0),
		}

		// If no snapshot stored (e.g. 404, DNS error, timeout during crawl)
		if cp.StoragePath == "" {
			parsedPage.ParseStatus = "failed"
			parsedPage.ErrorMessage = cp.ErrorMessage
			if parsedPage.ErrorMessage == "" {
				parsedPage.ErrorMessage = fmt.Sprintf("no snapshot available (HTTP %d)", cp.StatusCode)
			}
			totalFailed++
		} else {
			// Read raw snapshot from storage engine
			snapshotHTML, sErr := s.storage.GetSnapshot(ctx, cp.StoragePath)
			if sErr != nil {
				parsedPage.ParseStatus = "failed"
				parsedPage.ErrorMessage = fmt.Sprintf("failed to read snapshot: %v", sErr)
				totalFailed++
				s.logger.Warn("snapshot read failed", zap.String("url", cp.URL), zap.Error(sErr))
			} else {
				// Extract facts from HTML
				facts, pErr := ExtractPageFacts(cp.URL, snapshotHTML)
				if pErr != nil {
					parsedPage.ParseStatus = "failed"
					parsedPage.ErrorMessage = fmt.Sprintf("HTML parsing failed: %v", pErr)
					totalFailed++
					s.logger.Warn("page extraction failed", zap.String("url", cp.URL), zap.Error(pErr))
				} else {
					// Populate all extracted structured facts
					parsedPage.ParseStatus = "success"
					parsedPage.Title = facts.Title
					parsedPage.MetaDescription = facts.MetaDescription
					parsedPage.MetaRobots = facts.MetaRobots
					parsedPage.CanonicalURL = facts.CanonicalURL
					parsedPage.Viewport = facts.Viewport
					parsedPage.Charset = facts.Charset
					parsedPage.DeclaredLang = facts.DeclaredLang
					parsedPage.Hreflangs = facts.Hreflangs
					parsedPage.OpenGraph = facts.OpenGraph
					parsedPage.TwitterCard = facts.TwitterCard
					parsedPage.Headings = facts.Headings
					parsedPage.Links = facts.Links
					parsedPage.Images = facts.Images
					parsedPage.StructuredData = facts.StructuredData
					parsedPage.BodyText = facts.BodyText
					parsedPage.WordCount = facts.WordCount
					parsedPage.CharCount = facts.CharCount
					totalParsed++
				}
			}
		}

		// Save structured record to database
		if err := s.repository.SaveParsedPage(ctx, parsedPage); err != nil {
			s.logger.Error("failed to save parsed page record",
				zap.String("url", cp.URL),
				zap.Error(err),
			)
		}
	}

	durationMs := time.Since(startTime).Milliseconds()

	s.logger.Info("parse job finished",
		zap.String("audit_id", event.AuditID.String()),
		zap.Int("total_parsed", totalParsed),
		zap.Int("total_failed", totalFailed),
		zap.Int("total_pages", len(crawlPages)),
		zap.Int64("duration_ms", durationMs),
	)

	// 3. Publish Parse Completed Event to NATS (fan-out trigger for analyzers)
	return s.publishParseCompleted(ctx, event, totalParsed, totalFailed, len(crawlPages), durationMs)
}

// GetParsedPagesByAuditID implements [Service].
func (s *service) GetParsedPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]ParsedPage, error) {
	return s.repository.FindParsedPagesByAuditID(ctx, auditID)
}

//==========================================//
//             HELPER FUNCTIONS             //
//==========================================//

func (s *service) publishParseCompleted(
	ctx context.Context,
	event CrawlCompletedEvent,
	parsed, failed, total int,
	dur int64,
) error {
	evt := ParseCompletedEvent{
		AuditID:          event.AuditID,
		TenantID:         event.TenantID,
		Domain:           event.Domain,
		NormalizedURL:    event.NormalizedURL,
		TotalPagesParsed: parsed,
		TotalPagesFailed: failed,
		TotalPages:       total,
		DurationMs:       dur,
		CompletedAt:      time.Now().UTC(),
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal parse completed event: %w", err)
	}

	msgID := fmt.Sprintf("audit:%s:parse_completed", event.AuditID)
	_, err = s.js.Publish(
		"audit.events.parse_completed",
		payload,
		nats.MsgId(msgID),
		nats.Context(ctx),
	)
	if err != nil {
		s.logger.Error("failed to publish parse completed event", zap.Error(err))
		return err
	}

	s.logger.Info("published parse.completed event", zap.String("audit_id", event.AuditID.String()))
	return nil
}

func (s *service) failParse(ctx context.Context, event CrawlCompletedEvent, reason string) error {
	evt := ParseFailedEvent{
		AuditID:       event.AuditID,
		TenantID:      event.TenantID,
		Domain:        event.Domain,
		Reason:        reason,
		FailedAt:      time.Now().UTC(),
	}

	payload, err := json.Marshal(evt)
	if err == nil {
		msgID := fmt.Sprintf("audit:%s:parse_failed", event.AuditID)
		_, _ = s.js.Publish("audit.events.parse_failed", payload, nats.MsgId(msgID), nats.Context(ctx))
	}

	return fmt.Errorf("parse job failed: %s", reason)
}
