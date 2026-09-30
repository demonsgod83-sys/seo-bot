package report

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

func (r *routes) SetupPrivateRoutes() {
	panic("unimplemented")
}

func (r *routes) SetupPublicRoutes() {
	r.ginEngine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "OK",
			"time":   time.Now().UTC(),
		})
	})

	// Internal Microservice endpoints
	r.ginEngine.GET("/internal/audits/:id/reports", r.handler.GetAuditReportsHandler)
	r.ginEngine.GET("/internal/audits/:id/reports/latest", r.handler.GetLatestAuditReportHandler)

	// Public shareable endpoints
	r.ginEngine.GET("/public/reports/:token", r.handler.PublicViewReportHandler)
	r.ginEngine.GET("/public/reports/:token/json", r.handler.PublicDownloadJSONHandler)
	r.ginEngine.GET("/public/reports/:token/pdf", r.handler.PublicDownloadPDFHandler)
}
