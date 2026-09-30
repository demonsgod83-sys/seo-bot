package gateway

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	gatewaymiddleware "github.com/demonsgod83-sys/seo-bot/gateway/internal/gateway/middleware"
	"github.com/google/uuid"
)

type Routes interface {
	SetupPublicRoutes()
	SetupPrivateRoutes()
}

type routes struct {
	engine      *gin.Engine
	handler     *Handler
	auth        gin.HandlerFunc
	rateLimit   gin.HandlerFunc
}

func NewRoutes(
	engine *gin.Engine,
	handler *Handler,
	apiKeys map[string]uuid.UUID,
	rateLimiter *gatewaymiddleware.RateLimiter,
) Routes {
	return &routes{
		engine:    engine,
		handler:   handler,
		auth:      gatewaymiddleware.Auth(apiKeys, handler.logger),
		rateLimit: gatewaymiddleware.RateLimit(rateLimiter),
	}
}

// SetupPublicRoutes implements [Routes].
func (r *routes) SetupPublicRoutes() {
	r.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "OK",
			"time":   time.Now().UTC(),
		})
	})

	// Version 1 API — all routes require auth + rate limiting.
	// The middleware chain is applied at the group level so it's impossible
	// to accidentally add an authenticated route outside the group.
	v1 := r.engine.Group("/v1", r.auth, r.rateLimit)
	{
		audits := v1.Group("/audits")
		{
			// POST /v1/audits — submit a URL for auditing
			audits.POST("", r.handler.SubmitAudit)

			// GET /v1/audits/:id — poll audit status
			audits.GET("/:id", r.handler.GetAuditStatus)

			// GET /v1/audits/:id/report — stub: not implemented until Report service exists
			audits.GET("/:id/report", func(c *gin.Context) {
				c.JSON(http.StatusNotImplemented, gin.H{
					"error": gin.H{
						"code":    "NOT_IMPLEMENTED",
						"message": "report retrieval is not yet available",
					},
					"request_id": c.GetString("request_id"),
				})
			})
		}
	}
}

// SetupPrivateRoutes implements [Routes].
func (r *routes) SetupPrivateRoutes() {
	// No private routes yet — reserved for internal admin endpoints
	// (force-fail an audit, clear rate limit, etc.) behind a different auth mechanism.
}
