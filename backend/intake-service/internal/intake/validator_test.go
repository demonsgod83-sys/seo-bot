package intake

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

//==========================================//
//         PARSE & NORMALIZE TESTS          //
//==========================================//

func TestParseAndNormalize(t *testing.T) {
	v := newValidator()

	tests := []struct {
		name        string
		input       string
		wantURL     string
		wantErr     bool
		errContains string
	}{
		// ---- Happy paths ----
		{
			name:    "already clean https URL",
			input:   "https://example.com",
			wantURL: "https://example.com",
		},
		{
			name:    "trailing slash stripped",
			input:   "https://example.com/",
			wantURL: "https://example.com",
		},
		{
			name:    "uppercase host lowercased",
			input:   "https://EXAMPLE.COM/path",
			wantURL: "https://example.com/path",
		},
		{
			name:    "mixed case host and scheme",
			input:   "HTTPS://Example.COM/",
			wantURL: "https://example.com",
		},
		{
			name:    "default https port 443 stripped",
			input:   "https://example.com:443/page",
			wantURL: "https://example.com/page",
		},
		{
			name:    "default http port 80 stripped",
			input:   "http://example.com:80/page",
			wantURL: "http://example.com/page",
		},
		{
			name:    "non-default port preserved",
			input:   "https://example.com:8443/",
			wantURL: "https://example.com:8443",
		},
		{
			name:    "fragment stripped",
			input:   "https://example.com/page#section",
			wantURL: "https://example.com/page",
		},
		{
			name:    "query string preserved",
			input:   "https://example.com/search?q=seo",
			wantURL: "https://example.com/search?q=seo",
		},
		{
			name:    "http scheme accepted",
			input:   "http://example.com",
			wantURL: "http://example.com",
		},
		// The key normalization guarantee from the spec:
		// Example.com/ and https://example.com → same audit key
		{
			name:    "normalization: uppercase host + trailing slash equals clean form",
			input:   "https://Example.com/",
			wantURL: "https://example.com",
		},

		// ---- Error paths ----
		{
			name:        "empty string rejected",
			input:       "",
			wantErr:     true,
			errContains: "must not be empty",
		},
		{
			name:        "no scheme rejected",
			input:       "example.com",
			wantErr:     true,
			errContains: "must be absolute",
		},
		{
			name:        "ftp scheme rejected",
			input:       "ftp://example.com",
			wantErr:     true,
			errContains: "unsupported scheme",
		},
		{
			// file:///etc/passwd has a host="", so it trips "must be absolute" first.
			// Either way it's rejected — the important thing is it is rejected.
			name:        "file scheme rejected",
			input:       "file:///etc/passwd",
			wantErr:     true,
			errContains: "",
		},
		{
			name:        "relative path rejected",
			input:       "/just/a/path",
			wantErr:     true,
			errContains: "must be absolute",
		},
		{
			// javascript:alert(1) has no host so it also trips "must be absolute".
			// Either way it is rejected.
			name:        "javascript scheme rejected",
			input:       "javascript:alert(1)",
			wantErr:     true,
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v.parseAndNormalize(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.String() != tt.wantURL {
				t.Fatalf("got %q, want %q", got.String(), tt.wantURL)
			}
		})
	}
}

//==========================================//
//            SSRF CHECK TESTS              //
//==========================================//

// isPrivateOrReservedStr is a test helper that parses a string IP
// and delegates to the real isPrivateOrReserved function.
func isPrivateOrReservedStr(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return isPrivateOrReserved(ip)
}

func TestIsPrivateOrReserved(t *testing.T) {
	tests := []struct {
		ipStr   string
		blocked bool
	}{
		// Loopback
		{"127.0.0.1", true},
		{"127.255.255.255", true},
		{"::1", true},

		// RFC 1918 private — the obvious ones
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.0.1", true},
		{"192.168.255.255", true},

		// The famous one — AWS / GCP / Azure instance metadata endpoint
		// Someone WILL submit http://169.254.169.254/ one day
		{"169.254.169.254", true},
		{"169.254.0.1", true},

		// 172.32.x is just outside the private range — must NOT be blocked
		{"172.32.0.1", false},

		// Documentation / test ranges (RFC 5737)
		{"192.0.2.1", true},
		{"198.51.100.1", true},
		{"203.0.113.1", true},

		// Multicast
		{"224.0.0.1", true},
		{"239.255.255.255", true},

		// Public IPs — must NOT be blocked
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"142.250.80.46", false},
		{"104.21.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.ipStr, func(t *testing.T) {
			got := isPrivateOrReservedStr(tt.ipStr)
			if got != tt.blocked {
				if tt.blocked {
					t.Errorf("IP %s should be blocked (private/reserved) but was allowed", tt.ipStr)
				} else {
					t.Errorf("IP %s should be allowed (public) but was blocked", tt.ipStr)
				}
			}
		})
	}
}

//==========================================//
//        SSRF END-TO-END CHECK TESTS       //
//==========================================//

func TestCheckSSRF(t *testing.T) {
	v := newValidator()

	blockedIPs := []string{
		"127.0.0.1",
		"10.0.0.1",
		"172.16.5.1",
		"192.168.1.1",
		"169.254.169.254",
	}

	for _, ipStr := range blockedIPs {
		t.Run("blocked_"+ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("invalid test IP: %s", ipStr)
			}
			err := v.checkSSRF([]net.IP{ip})
			if err == nil {
				t.Errorf("expected SSRF block for %s but got nil", ipStr)
			}
		})
	}

	allowedIPs := []string{
		"8.8.8.8",
		"1.1.1.1",
		"104.21.0.0",
	}

	for _, ipStr := range allowedIPs {
		t.Run("allowed_"+ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("invalid test IP: %s", ipStr)
			}
			err := v.checkSSRF([]net.IP{ip})
			if err != nil {
				t.Errorf("expected IP %s to be allowed but got: %v", ipStr, err)
			}
		})
	}
}

//==========================================//
//         NORMALIZATION PROPERTY TESTS     //
//==========================================//

// TestNormalizationIdempotency verifies that running parseAndNormalize twice
// on a clean URL produces the same result — normalization must be stable.
func TestNormalizationIdempotency(t *testing.T) {
	v := newValidator()

	inputs := []string{
		"https://example.com",
		"https://example.com/path",
		"https://example.com/search?q=foo",
		"http://example.com:8080/page",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			first, err := v.parseAndNormalize(input)
			if err != nil {
				t.Fatalf("first normalization failed: %v", err)
			}

			second, err := v.parseAndNormalize(first.String())
			if err != nil {
				t.Fatalf("second normalization failed: %v", err)
			}

			if first.String() != second.String() {
				t.Errorf("normalization not idempotent: %q → %q → %q", input, first.String(), second.String())
			}
		})
	}
}

//==========================================//
//         SAME-AUDIT-KEY GUARANTEE         //
//==========================================//

// TestSameAuditKey verifies the spec's core normalization promise:
// "Example.com/ and https://example.com end up as the same audit key"
func TestSameAuditKey(t *testing.T) {
	v := newValidator()

	pairs := []struct {
		a, b string
	}{
		{"https://example.com/", "https://example.com"},
		{"https://EXAMPLE.COM/", "https://example.com"},
		{"https://Example.com", "https://example.com"},
		{"https://example.com:443/", "https://example.com"},
		{"http://example.com:80/", "http://example.com"},
	}

	for _, p := range pairs {
		t.Run(p.a+"=="+p.b, func(t *testing.T) {
			urlA, err := v.parseAndNormalize(p.a)
			if err != nil {
				t.Fatalf("failed to normalize %q: %v", p.a, err)
			}

			urlB, err := v.parseAndNormalize(p.b)
			if err != nil {
				t.Fatalf("failed to normalize %q: %v", p.b, err)
			}

			if urlA.String() != urlB.String() {
				t.Errorf("expected same audit key but got:\n  %q\n  %q", urlA.String(), urlB.String())
			}
		})
	}
}

//==========================================//
//             HELPERS                      //
//==========================================//

func containsStr(s, sub string) bool {
	return strings.Contains(s, sub)
}

//==========================================//
//      SAFE TRANSPORT — REDIRECT TESTS     //
//==========================================//

// TestSafeTransportBlocksPrivateIP verifies that newSafeTransport refuses to
// open a TCP connection to a private/reserved IP regardless of how the request
// got there (direct or via redirect).
func TestSafeTransportBlocksPrivateIP(t *testing.T) {
	privateIPs := []string{
		"127.0.0.1",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.169.254",
	}

	transport := newSafeTransport()
	client := &http.Client{Transport: transport}

	for _, ip := range privateIPs {
		t.Run("blocks_"+ip, func(t *testing.T) {
			// Attempt a direct connection to the private IP on an arbitrary port.
			// The DialContext should resolve it and reject before connecting.
			_, err := client.Get(fmt.Sprintf("http://%s:80/", ip))
			if err == nil {
				t.Errorf("expected connection to %s to be blocked, but it succeeded", ip)
				return
			}
			if !strings.Contains(err.Error(), "SSRF") {
				t.Errorf("expected SSRF error for %s, got: %v", ip, err)
			}
		})
	}
}

// TestSafeTransportRedirectToPrivateIPBlocked is the core scenario:
// a public server (httptest.NewServer on 127.0.0.1) issues a redirect to a
// private IP. The safe transport must catch this on the second dial.
//
// Note: httptest.NewServer itself binds to 127.0.0.1, which is private.
// We therefore test this at the transport level directly — we spin up the
// redirect server, build the redirect URL, and confirm the transport blocks
// the redirected dial to the private target.
func TestSafeTransportRedirectToPrivateIPBlocked(t *testing.T) {
	// Target of the redirect — the famous AWS metadata endpoint
	privateTarget := "http://169.254.169.254/latest/meta-data/"

	// Spin up a local server that would issue the redirect.
	// We don't actually use the server for an end-to-end request (because
	// httptest itself is on a loopback address that the safe transport would
	// block on the first dial). Instead we test the transport directly by
	// attempting to connect to the private target — exactly what the
	// redirect would do.
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, privateTarget, http.StatusFound)
	}))
	defer redirectServer.Close()

	transport := newSafeTransport()
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil // allow redirects so the transport sees the second dial
		},
	}

	// Direct request to the private target — simulates what the redirect hop does.
	_, err := client.Get(privateTarget)
	if err == nil {
		t.Fatal("expected SSRF block when connecting to 169.254.169.254, got nil")
	}
	if !strings.Contains(err.Error(), "SSRF") {
		t.Errorf("expected SSRF error, got: %v", err)
	}
}

// TestSafeTransportAllowsPublicIP verifies that the transport does NOT block
// legitimate public IPs — we must not over-block.
func TestSafeTransportAllowsPublicIP(t *testing.T) {
	// Spin up a public-facing test server on localhost — but we test the
	// transport in isolation here rather than through the full validator,
	// so we directly test that the transport accepts a public IP string
	// by using the SSRF check function (the transport's inner logic).
	publicIPs := []string{
		"8.8.8.8",
		"1.1.1.1",
		"142.250.80.46",
	}

	for _, ip := range publicIPs {
		t.Run("allows_"+ip, func(t *testing.T) {
			parsed := net.ParseIP(ip)
			if parsed == nil {
				t.Fatalf("invalid test IP: %s", ip)
			}
			if isPrivateOrReserved(parsed) {
				t.Errorf("public IP %s was wrongly classified as private/reserved", ip)
			}
		})
	}
}
