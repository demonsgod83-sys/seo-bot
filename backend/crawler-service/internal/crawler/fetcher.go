package crawler

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

//==========================================//
//          SSRF-PROTECTED FETCHER          //
//==========================================//

// 17 RFC-standard private, loopback, link-local, and reserved CIDR ranges.
var privateCIDRs = []string{
	"0.0.0.0/8",          // Current network (RFC 1122)
	"10.0.0.0/8",         // Private-Use (RFC 1918)
	"100.64.0.0/10",      // Shared Address Space (RFC 6598)
	"127.0.0.0/8",        // Loopback (RFC 1122)
	"169.254.0.0/16",     // Link Local (RFC 3927) - includes AWS/cloud metadata 169.254.169.254
	"172.16.0.0/12",      // Private-Use (RFC 1918)
	"192.0.0.0/24",       // IETF Protocol Assignments (RFC 6890)
	"192.0.2.0/24",       // Documentation / TEST-NET-1 (RFC 5737)
	"192.88.99.0/24",     // 6to4 Relay Anycast (RFC 7526)
	"192.168.0.0/16",     // Private-Use (RFC 1918)
	"198.18.0.0/15",      // Benchmarking (RFC 2544)
	"198.51.100.0/24",    // Documentation / TEST-NET-2 (RFC 5737)
	"203.0.113.0/24",     // Documentation / TEST-NET-3 (RFC 5737)
	"224.0.0.0/4",        // Multicast (RFC 5771)
	"240.0.0.0/4",        // Reserved (RFC 1112)
	"255.255.255.255/32", // Limited Broadcast (RFC 919)
	"::1/128",            // IPv6 Loopback (RFC 4291)
	"fc00::/7",           // IPv6 Unique Local (RFC 4193)
	"fe80::/10",          // IPv6 Link-Local (RFC 4291)
}

var parsedPrivateCIDRs []*net.IPNet

func init() {
	for _, cidr := range privateCIDRs {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			parsedPrivateCIDRs = append(parsedPrivateCIDRs, block)
		}
	}
}

// isPrivateOrReserved checks if an IP belongs to private/reserved ranges.
func isPrivateOrReserved(ip net.IP) bool {
	for _, block := range parsedPrivateCIDRs {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// Fetcher defines the interface for safely fetching remote web resources.
type Fetcher interface {
	Fetch(ctx context.Context, targetURL string) (*FetchResponse, error)
}

type FetchResponse struct {
	StatusCode   int
	ContentType  string
	Body         []byte
	DurationMs   int64
	FinalURL     string
}

type safeFetcher struct {
	httpClient *http.Client
	userAgent  string
	maxBodyBytes int64
}

// NewSafeFetcher creates an HTTP fetcher with DialContext-level SSRF guards and redirect protection.
func NewSafeFetcher(userAgent string, timeout time.Duration) Fetcher {
	safeTransport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid dial address: %w", err)
			}

			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("DNS resolution failed for %s: %w", host, err)
			}

			for _, ip := range ips {
				if isPrivateOrReserved(ip) {
					return nil, fmt.Errorf("SSRF blocked: host %s resolved to private/reserved IP %s", host, ip.String())
				}
			}

			dialer := &net.Dialer{
				Timeout:   timeout,
				KeepAlive: 30 * time.Second,
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     60 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
	}

	client := &http.Client{
		Transport: safeTransport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("stopped after 5 redirects")
			}
			return nil
		},
	}

	return &safeFetcher{
		httpClient:   client,
		userAgent:    userAgent,
		maxBodyBytes: 10 * 1024 * 1024, // 10 MB maximum
	}
}

func (f *safeFetcher) Fetch(ctx context.Context, targetURL string) (*FetchResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	start := time.Now()
	resp, err := f.httpClient.Do(req)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Limit reader to avoid memory exhaustion from huge files
	limitedReader := io.LimitReader(resp.Body, f.maxBodyBytes)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	// Clean charset suffix (e.g., "text/html; charset=utf-8" -> "text/html")
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	return &FetchResponse{
		StatusCode:  resp.StatusCode,
		ContentType: contentType,
		Body:        body,
		DurationMs:  duration,
		FinalURL:    resp.Request.URL.String(),
	}, nil
}
