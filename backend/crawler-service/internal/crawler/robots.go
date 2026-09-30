package crawler

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/temoto/robotstxt"
	"go.uber.org/zap"
)

//==========================================//
//              ROBOTS & SITEMAP            //
//==========================================//

type RobotsResult struct {
	Allowed    bool
	CrawlDelay time.Duration
	Sitemaps   []string
	RobotsData *robotstxt.RobotsData
}

type XMLSitemapIndex struct {
	XMLName  xml.Name     `xml:"sitemapindex"`
	Sitemaps []XMLSitemap `xml:"sitemap"`
}

type XMLSitemap struct {
	Loc string `xml:"loc"`
}

type XMLURLSet struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []XMLURL `xml:"url"`
}

type XMLURL struct {
	Loc string `xml:"loc"`
}

type RobotsHandler struct {
	fetcher Fetcher
	logger  *zap.Logger
}

func NewRobotsHandler(fetcher Fetcher, logger *zap.Logger) *RobotsHandler {
	return &RobotsHandler{
		fetcher: fetcher,
		logger:  logger,
	}
}

// FetchAndParseRobots retrieves /robots.txt and evaluates crawl permissions for the base URL.
func (h *RobotsHandler) FetchAndParseRobots(ctx context.Context, baseParsedURL *url.URL, userAgent string) (*RobotsResult, error) {
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", baseParsedURL.Scheme, baseParsedURL.Host)

	result := &RobotsResult{
		Allowed:    true,
		CrawlDelay: 0,
		Sitemaps:   make([]string, 0),
	}

	resp, err := h.fetcher.Fetch(ctx, robotsURL)
	if err != nil || resp.StatusCode >= 400 || len(resp.Body) == 0 {
		// If robots.txt is not found (404) or unreachable, standard crawling convention is full access
		h.logger.Info("robots.txt not found or unavailable — assuming full access",
			zap.String("url", robotsURL),
		)
		return result, nil
	}

	robotsData, err := robotstxt.FromBytes(resp.Body)
	if err != nil {
		h.logger.Warn("failed to parse robots.txt — allowing crawl",
			zap.String("url", robotsURL),
			zap.Error(err),
		)
		return result, nil
	}

	result.RobotsData = robotsData
	result.Sitemaps = robotsData.Sitemaps

	// Check permissions for user agent (e.g., "SEO-Bot") and fallback to "*"
	group := robotsData.FindGroup(userAgent)
	if group == nil {
		group = robotsData.FindGroup("*")
	}

	if group != nil {
		result.Allowed = group.Test(baseParsedURL.Path)
		result.CrawlDelay = group.CrawlDelay
	}

	h.logger.Info("robots.txt evaluated",
		zap.Bool("allowed", result.Allowed),
		zap.Duration("crawl_delay", result.CrawlDelay),
		zap.Int("sitemaps_count", len(result.Sitemaps)),
	)

	return result, nil
}

// DiscoverSitemapURLs fetches sitemaps (both robots.txt declared and standard /sitemap.xml)
// and returns unique discovered page URLs belonging to the target domain.
func (h *RobotsHandler) DiscoverSitemapURLs(ctx context.Context, baseParsedURL *url.URL, robotsSitemaps []string) []string {
	sitemapQueue := make([]string, 0)
	sitemapQueue = append(sitemapQueue, robotsSitemaps...)

	// Also try standard /sitemap.xml if none specified in robots.txt
	defaultSitemap := fmt.Sprintf("%s://%s/sitemap.xml", baseParsedURL.Scheme, baseParsedURL.Host)
	if len(sitemapQueue) == 0 {
		sitemapQueue = append(sitemapQueue, defaultSitemap)
	}

	visitedSitemaps := make(map[string]bool)
	discoveredPages := make(map[string]bool)
	var results []string

	targetHost := strings.ToLower(baseParsedURL.Host)

	for len(sitemapQueue) > 0 {
		currentSitemap := sitemapQueue[0]
		sitemapQueue = sitemapQueue[1:]

		if visitedSitemaps[currentSitemap] {
			continue
		}
		visitedSitemaps[currentSitemap] = true

		resp, err := h.fetcher.Fetch(ctx, currentSitemap)
		if err != nil || resp.StatusCode != 200 || len(resp.Body) == 0 {
			continue
		}

		// 1. Try parsing as sitemap index
		var sitemapIndex XMLSitemapIndex
		if err := xml.Unmarshal(resp.Body, &sitemapIndex); err == nil && len(sitemapIndex.Sitemaps) > 0 {
			for _, sm := range sitemapIndex.Sitemaps {
				loc := strings.TrimSpace(sm.Loc)
				if loc != "" && !visitedSitemaps[loc] {
					sitemapQueue = append(sitemapQueue, loc)
				}
			}
			continue
		}

		// 2. Try parsing as urlset
		var urlSet XMLURLSet
		decoder := xml.NewDecoder(bytes.NewReader(resp.Body))
		if err := decoder.Decode(&urlSet); err == nil && len(urlSet.URLs) > 0 {
			for _, u := range urlSet.URLs {
				pageLoc := strings.TrimSpace(u.Loc)
				if pageLoc == "" {
					continue
				}

				parsedPageURL, err := url.Parse(pageLoc)
				if err != nil {
					continue
				}

				// Must be HTTP/HTTPS and same domain
				if (parsedPageURL.Scheme == "http" || parsedPageURL.Scheme == "https") &&
					strings.ToLower(parsedPageURL.Host) == targetHost {
					cleanURL := NormalizeURL(parsedPageURL)
					if !discoveredPages[cleanURL] {
						discoveredPages[cleanURL] = true
						results = append(results, cleanURL)
					}
				}
			}
		}
	}

	h.logger.Info("sitemap discovery complete",
		zap.Int("urls_found", len(results)),
	)
	return results
}
