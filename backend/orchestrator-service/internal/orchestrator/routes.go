package orchestrator

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Routes interface {
	SetupPublicRoutes()
	SetupPrivateRoutes()
}

type routes struct {
	ginEngine *gin.Engine
	handler   *Handler
}

func NewRoutes(ginEngine *gin.Engine, handler *Handler) Routes {
	return &routes{
		ginEngine: ginEngine,
		handler:   handler,
	}
}

// SetupPrivateRoutes implements [Routes].
func (r *routes) SetupPrivateRoutes() {
	panic("unimplemented")
}

// SetupPublicRoutes implements [Routes].
func (r *routes) SetupPublicRoutes() {
	r.ginEngine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "OK",
			"time":   time.Now().UTC(),
		})
	})

	// Internal status endpoint — the API Gateway will call this to serve
	// the client's polling requests. Not meant to be publicly exposed.
	r.ginEngine.GET("/internal/audits/:id/status", r.handler.GetAuditStatusHandler)
}
