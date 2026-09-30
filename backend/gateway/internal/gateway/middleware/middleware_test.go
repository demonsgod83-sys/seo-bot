package gatewaymiddleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger, _ := zap.NewDevelopment()

	tenantID := uuid.New()
	validKey := "sk_test_12345"
	apiKeys := map[string]uuid.UUID{
		validKey: tenantID,
	}

	tests := []struct {
		name           string
		headerKey      string
		headerVal      string
		expectedStatus int
	}{
		{
			name:           "Missing key",
			headerKey:      "",
			headerVal:      "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid key in X-API-Key",
			headerKey:      "X-API-Key",
			headerVal:      "sk_invalid",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Valid key in X-API-Key",
			headerKey:      "X-API-Key",
			headerVal:      validKey,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Valid key in Authorization Bearer",
			headerKey:      "Authorization",
			headerVal:      "Bearer " + validKey,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(Auth(apiKeys, logger))
			r.GET("/test", func(c *gin.Context) {
				gotTenant := c.MustGet("tenant_id").(uuid.UUID)
				if gotTenant != tenantID {
					t.Errorf("expected tenant %s, got %s", tenantID, gotTenant)
				}
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.headerKey != "" {
				req.Header.Set(tt.headerKey, tt.headerVal)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestRateLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 2 requests per second, burst of 2
	limiter := NewRateLimiter(2, 2)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("api_key", "test_key")
		c.Next()
	})
	r.Use(RateLimit(limiter))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// First 2 requests should succeed (burst)
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request immediately should hit rate limit
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("request 3: expected 429, got %d", w.Code)
	}

	// Wait for token replenishment
	time.Sleep(600 * time.Millisecond)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("request after replenish: expected 200, got %d", w.Code)
	}
}
