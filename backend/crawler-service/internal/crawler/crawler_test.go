package crawler

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/demonsgod83-sys/seo-bot/crawler-service/internal/platform/storage"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

//==========================================//
//          URL NORMALIZATION TESTS         //
//==========================================//

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		raw      string
		expected string
	}{
		{"https://EXAMPLE.COM", "https://example.com"},
		{"https://example.com/", "https://example.com"},
		{"https://example.com/about/", "https://example.com/about"},
		{"https://example.com:443/page", "https://example.com/page"},
		{"http://example.com:80/page", "http://example.com/page"},
		{"http://example.com:8080/page", "http://example.com:8080/page"},
		{"https://example.com/page?query=1&b=2", "https://example.com/page?query=1&b=2"},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			u, err := url.Parse(tt.raw)
			if err != nil {
				t.Fatalf("failed to parse test URL %s: %v", tt.raw, err)
			}
			got := NormalizeURL(u)
			if got != tt.expected {
				t.Errorf("NormalizeURL(%s) = %s, expected %s", tt.raw, got, tt.expected)
			}
		})
	}
}

//==========================================//
//          LINK EXTRACTION TESTS           //
//==========================================//

func TestExtractInternalLinks(t *testing.T) {
	baseURL, _ := url.Parse("https://example.com/blog/article-1")

	htmlContent := `
	<!DOCTYPE html>
	<html>
	<head><title>Test Page</title></head>
	<body>
		<a href="/about">About Us</a>
		<a href="https://example.com/contact">Contact</a>
		<a href="/pricing/">Pricing (trailing slash)</a>
		<a href="../team">Our Team</a>
		<!-- External links to ignore -->
		<a href="https://google.com">Google</a>
		<a href="https://subdomain.example.com">Subdomain</a>
		<a href="javascript:void(0)">JS link</a>
		<a href="mailto:info@example.com">Email</a>
		<a href="#section-1">Fragment only</a>
	</body>
	</html>
	`

	links := ExtractInternalLinks(baseURL, []byte(htmlContent))

	expected := map[string]bool{
		"https://example.com/about":   true,
		"https://example.com/contact": true,
		"https://example.com/pricing": true,
		"https://example.com/team":    true,
	}

	if len(links) != len(expected) {
		t.Fatalf("expected %d links, got %d: %v", len(expected), len(links), links)
	}

	for _, link := range links {
		if !expected[link] {
			t.Errorf("unexpected extracted link: %s", link)
		}
	}
}

//==========================================//
//          ROBOTS.TXT & SITEMAP TESTS      //
//==========================================//

type mockFetcher struct {
	responses map[string]*FetchResponse
}

func (m *mockFetcher) Fetch(ctx context.Context, targetURL string) (*FetchResponse, error) {
	if resp, ok := m.responses[targetURL]; ok {
		return resp, nil
	}
	return &FetchResponse{StatusCode: 404}, nil
}

func TestRobotsTxtParser(t *testing.T) {
	robotsContent := `
User-agent: *
Disallow: /admin/
Disallow: /private
Crawl-delay: 5
Sitemap: https://example.com/sitemap.xml
`
	fetcher := &mockFetcher{
		responses: map[string]*FetchResponse{
			"https://example.com/robots.txt": {
				StatusCode: 200,
				Body:       []byte(robotsContent),
			},
		},
	}

	logger, _ := zap.NewDevelopment()
	handler := NewRobotsHandler(fetcher, logger)

	baseURL, _ := url.Parse("https://example.com/admin/dashboard")
	res, err := handler.FetchAndParseRobots(context.Background(), baseURL, "SEO-Bot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Allowed {
		t.Errorf("expected /admin/dashboard to be disallowed")
	}

	if res.CrawlDelay != 5*time.Second {
		t.Errorf("expected crawl delay 5s, got %v", res.CrawlDelay)
	}

	if len(res.Sitemaps) != 1 || res.Sitemaps[0] != "https://example.com/sitemap.xml" {
		t.Errorf("expected sitemap https://example.com/sitemap.xml, got %v", res.Sitemaps)
	}
}

func TestSitemapDiscovery(t *testing.T) {
	sitemapIndexContent := `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<sitemap>
		<loc>https://example.com/sub-sitemap.xml</loc>
	</sitemap>
</sitemapindex>`

	subSitemapContent := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url>
		<loc>https://example.com/page1</loc>
	</url>
	<url>
		<loc>https://example.com/page2/</loc>
	</url>
	<!-- External URL to filter out -->
	<url>
		<loc>https://otherdomain.com/page3</loc>
	</url>
</urlset>`

	fetcher := &mockFetcher{
		responses: map[string]*FetchResponse{
			"https://example.com/sitemap.xml": {
				StatusCode: 200,
				Body:       []byte(sitemapIndexContent),
			},
			"https://example.com/sub-sitemap.xml": {
				StatusCode: 200,
				Body:       []byte(subSitemapContent),
			},
		},
	}

	logger, _ := zap.NewDevelopment()
	handler := NewRobotsHandler(fetcher, logger)

	baseURL, _ := url.Parse("https://example.com")
	urls := handler.DiscoverSitemapURLs(context.Background(), baseURL, []string{"https://example.com/sitemap.xml"})

	expected := map[string]bool{
		"https://example.com/page1": true,
		"https://example.com/page2": true,
	}

	if len(urls) != len(expected) {
		t.Fatalf("expected %d urls, got %d: %v", len(expected), len(urls), urls)
	}

	for _, u := range urls {
		if !expected[u] {
			t.Errorf("unexpected sitemap url: %s", u)
		}
	}
}

//==========================================//
//          SNAPSHOT STORAGE TESTS          //
//==========================================//

func TestLocalStorageSnapshot(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "seobot-test-storage-"+uuid.NewString())
	defer os.RemoveAll(tmpDir)

	logger, _ := zap.NewDevelopment()
	store, err := storage.NewLocalStorage(tmpDir, logger)
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	auditID := uuid.New()
	targetURL := "https://example.com/page-1"
	originalHTML := []byte("<html><body><h1>Hello World</h1></body></html>")

	relPath, err := store.SaveSnapshot(context.Background(), auditID, targetURL, originalHTML)
	if err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	decompressed, err := store.GetSnapshot(context.Background(), relPath)
	if err != nil {
		t.Fatalf("failed to get snapshot: %v", err)
	}

	if string(decompressed) != string(originalHTML) {
		t.Errorf("expected HTML %s, got %s", string(originalHTML), string(decompressed))
	}
}
