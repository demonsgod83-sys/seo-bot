package gatewaymiddleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

//==========================================//
//           RATE LIMIT MIDDLEWARE          //
//==========================================//

// RateLimiter is a per-API-key in-memory token bucket.
//
// Each key gets its own *rate.Limiter. Entries are garbage-collected after
// inactivity to prevent unbounded memory growth.
//
// This is intentionally simple — good enough to protect the skeleton.
// Swap for a Redis-backed distributed limiter once you have multiple
// gateway replicas (a horizontal scaling pass, not a day-1 concern).
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rps     rate.Limit
	burst   int
}

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*bucket),
		rps:     rate.Limit(rps),
		burst:   burst,
	}
	// Background cleanup — evict buckets idle for > 5 minutes to prevent leaks.
	go rl.cleanup(5 * time.Minute)
	return rl
}

// Allow returns true if the key is within its rate limit.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(rl.rps, rl.burst)}
		rl.buckets[key] = b
	}
	b.lastSeen = time.Now()
	allow := b.limiter.Allow()
	rl.mu.Unlock()
	return allow
}

func (rl *RateLimiter) cleanup(interval time.Duration) {
	for range time.Tick(interval) {
		rl.mu.Lock()
		cutoff := time.Now().Add(-interval)
		for key, b := range rl.buckets {
			if b.lastSeen.Before(cutoff) {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

//==========================================//
//           MIDDLEWARE CONSTRUCTOR         //
//==========================================//

// RateLimit returns a Gin middleware that enforces per-API-key request rates.
// It reads the api_key value set by the Auth middleware, so Auth must run first.
func RateLimit(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetString("api_key")
		if key == "" {
			// No key = Auth middleware rejected or wasn't applied.
			// Fail safe: block the request.
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "too many requests — please slow down",
				},
				"request_id": c.GetString("request_id"),
			})
			return
		}

		if !rl.Allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "too many requests — please slow down",
				},
				"request_id": c.GetString("request_id"),
			})
			return
		}

		c.Next()
	}
}
