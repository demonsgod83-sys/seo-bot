package intake

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

//==========================================//
//              URL VALIDATOR               //
//==========================================//

// validator is the single authority for everything that can go wrong with a URL
// before we commit to storing it. Each method is individually testable — that is
// intentional; this is where bugs hide.
type validator struct {
	reachabilityTimeout time.Duration
}

func newValidator() *validator {
	return &validator{
		// 5s per attempt (HEAD + optional GET fallback) = 10s worst case,
		// which stays safely under the server's 10s WriteTimeout.
		reachabilityTimeout: 5 * time.Second,
	}
}

// Validate runs the full pipeline: parse → normalize → DNS → SSRF → reachability.
// Returns the normalized URL and extracted domain on success.
// ctx is the request context — cancellation propagates through DNS and the
// outbound reachability HTTP call so the handler never outlives the caller.
func (v *validator) Validate(ctx context.Context, rawURL string) (normalizedURL string, domain string, err error) {
	parsed, err := v.parseAndNormalize(rawURL)
	if err != nil {
		return "", "", err
	}

	domain = parsed.Hostname()

	ips, err := v.resolveHost(domain)
	if err != nil {
		return "", "", err
	}

	if err := v.checkSSRF(ips); err != nil {
		return "", "", err
	}

	if err := v.checkReachability(ctx, parsed.String()); err != nil {
		return "", "", err
	}

	return parsed.String(), domain, nil
}

//==========================================//
//         STEP 1 — PARSE & NORMALIZE       //
//==========================================//

// parseAndNormalize enforces scheme (http/https only), lowercases the host,
// strips default ports, trailing slashes, and any fragment (#...) so that
// https://Example.com/ and http://example.com end up at the same audit key.
func (v *validator) parseAndNormalize(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("URL must not be empty")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("malformed URL: %w", err)
	}

	// Must be absolute — scheme + host both required
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("URL must be absolute (must include scheme and host), got: %q", raw)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q — only http and https are allowed", parsed.Scheme)
	}

	// Lowercase the host
	host := strings.ToLower(parsed.Host)

	// Strip default ports (:80 for http, :443 for https)
	hostname, port, splitErr := net.SplitHostPort(host)
	if splitErr == nil {
		if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
			host = hostname
		}
	}

	// Strip trailing slash from path, normalise empty path
	path := strings.TrimRight(parsed.Path, "/")

	// Strip tracking fragment — fragments are client-side only and create
	// false uniqueness (example.com/#section vs example.com/)
	parsed.Scheme = scheme
	parsed.Host = host
	parsed.Path = path
	parsed.Fragment = ""
	parsed.RawFragment = ""

	return parsed, nil
}

//==========================================//
//         STEP 2 — DNS RESOLUTION          //
//==========================================//

func (v *validator) resolveHost(host string) ([]net.IP, error) {
	// SplitHostPort in case a non-default port survived normalization
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}

	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed for %q: %w", hostname, err)
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("DNS resolution returned no addresses for %q", hostname)
	}

	ips := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip != nil {
			ips = append(ips, ip)
		}
	}

	return ips, nil
}

//==========================================//
//         STEP 3 — SSRF CHECK              //
//==========================================//

// checkSSRF rejects any IP that falls in a range your crawler should never touch:
// loopback, private RFC 1918, link-local, and other reserved ranges.
// This is the check most people skip and regret — someone will submit
// http://169.254.169.254/latest/meta-data/ or an internal hostname.
func (v *validator) checkSSRF(ips []net.IP) error {
	for _, ip := range ips {
		if isPrivateOrReserved(ip) {
			return fmt.Errorf("URL resolves to a private or reserved IP address (%s) — not allowed", ip.String())
		}
	}
	return nil
}

func isPrivateOrReserved(ip net.IP) bool {
	private := []string{
		// Loopback
		"127.0.0.0/8",
		"::1/128",
		// Private RFC 1918
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		// Link-local (AWS/GCP metadata endpoint lives here)
		"169.254.0.0/16",
		"fe80::/10",
		// Unique local IPv6
		"fc00::/7",
		// Loopback IPv6
		"::1/128",
		// Unspecified
		"0.0.0.0/8",
		// Documentation ranges (should never be routable)
		"192.0.2.0/24",
		"198.51.100.0/24",
		"203.0.113.0/24",
		// Multicast
		"224.0.0.0/4",
		"ff00::/8",
		// Broadcast
		"255.255.255.255/32",
	}

	for _, cidr := range private {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

//==========================================//
//         STEP 4 — REACHABILITY CHECK      //
//==========================================//

// checkReachability fires a HEAD (falling back to GET on 405) with a short
// timeout using a safe transport that re-runs the SSRF check on every TCP
// dial — including every redirect hop. This closes the TOCTOU gap where
// legit-site.com could redirect to http://169.254.169.254/ and our earlier
// DNS-time check would never see it.
//
// ctx is the request context. If the caller cancels (client disconnect,
// server WriteTimeout), both outbound requests are aborted immediately.
func (v *validator) checkReachability(ctx context.Context, targetURL string) error {
	client := &http.Client{
		Timeout:   v.reachabilityTimeout,
		Transport: newSafeTransport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// Use NewRequestWithContext so the outbound call is bound to the
	// incoming request's lifetime — if Postman cancels or the server's
	// WriteTimeout fires, we stop immediately instead of hanging.
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		return fmt.Errorf("failed to build reachability request: %w", err)
	}

	resp, err := client.Do(headReq)
	if err != nil {
		// Some servers reject HEAD outright — fall back to a GET.
		getReq, err2 := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err2 != nil {
			return fmt.Errorf("failed to build reachability request: %w", err2)
		}
		resp, err = client.Do(getReq)
		if err != nil {
			return fmt.Errorf("site is unreachable (%s): %w", targetURL, err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("site returned server error status %d — treating as unreachable", resp.StatusCode)
	}

	return nil
}

//==========================================//
//         SAFE TRANSPORT (SSRF GUARD)      //
//==========================================//

// newSafeTransport returns an http.Transport whose DialContext resolves the
// target host and runs isPrivateOrReserved before opening any socket.
// Because this hook fires on every dial — including mid-redirect — no
// redirect chain can smuggle a private IP past our SSRF check.
func newSafeTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// addr arrives as "host:port"
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address %q: %w", addr, err)
			}

			// Resolve the host so we can inspect the concrete IPs before dialing.
			// This is the second layer of SSRF protection — the first ran at
			// DNS-resolution time in Validate(); this one runs per-dial so
			// redirect hops are covered too.
			resolved, err := net.DefaultResolver.LookupHost(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("DNS resolution failed for %q: %w", host, err)
			}

			for _, ipStr := range resolved {
				ip := net.ParseIP(ipStr)
				if ip != nil && isPrivateOrReserved(ip) {
					return nil, fmt.Errorf(
						"SSRF: connection to private/reserved IP %s (resolved from %q) is blocked",
						ipStr, host,
					)
				}
			}

			// All resolved IPs are public — dial the first one.
			return dialer.DialContext(ctx, network, net.JoinHostPort(resolved[0], port))
		},
	}
}
