package report

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// GetAuditReportsHandler lists all report versions for an audit.
func (h *Handler) GetAuditReportsHandler(c *gin.Context) {
	auditIDStr := c.Param("id")
	auditID, err := uuid.Parse(auditIDStr)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid audit ID UUID")
		return
	}

	reports, err := h.svc.GetReportsByAuditID(c.Request.Context(), auditID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	success(c, http.StatusOK, reports)
}

// GetLatestAuditReportHandler returns latest report metadata for an audit.
func (h *Handler) GetLatestAuditReportHandler(c *gin.Context) {
	auditIDStr := c.Param("id")
	auditID, err := uuid.Parse(auditIDStr)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid audit ID UUID")
		return
	}

	report, err := h.svc.GetLatestReport(c.Request.Context(), auditID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	success(c, http.StatusOK, report)
}

// PublicViewReportHandler renders public HTML report or handles ?format=json/pdf query parameters.
func (h *Handler) PublicViewReportHandler(c *gin.Context) {
	token := c.Param("token")
	format := c.DefaultQuery("format", "html")

	data, contentType, err := h.svc.GetReportContentByToken(c.Request.Context(), token, format)
	if err != nil {
		fail(c, http.StatusNotFound, "report not found")
		return
	}

	if format == "json" {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"audit-report-%s.json\"", token))
	} else if format == "pdf" {
		c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"audit-report-%s.pdf\"", token))
	}

	c.Data(http.StatusOK, contentType, data)
}

// PublicDownloadJSONHandler downloads JSON report directly.
func (h *Handler) PublicDownloadJSONHandler(c *gin.Context) {
	token := c.Param("token")
	data, contentType, err := h.svc.GetReportContentByToken(c.Request.Context(), token, "json")
	if err != nil {
		fail(c, http.StatusNotFound, "report not found")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"audit-report-%s.json\"", token))
	c.Data(http.StatusOK, contentType, data)
}

// PublicDownloadPDFHandler downloads PDF report directly.
func (h *Handler) PublicDownloadPDFHandler(c *gin.Context) {
	token := c.Param("token")
	data, contentType, err := h.svc.GetReportContentByToken(c.Request.Context(), token, "pdf")
	if err != nil {
		fail(c, http.StatusNotFound, "report not found")
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"audit-report-%s.pdf\"", token))
	c.Data(http.StatusOK, contentType, data)
}

//==========================================//
//             HELPER FUNCTIONS             //
//==========================================//

func success(c *gin.Context, code int, data any) {
	c.JSON(code, gin.H{"data": data, "request_id": c.GetString("request_id")})
}

func fail(c *gin.Context, code int, msg string) {
	c.AbortWithStatusJSON(code, gin.H{"error": msg, "request_id": c.GetString("request_id")})
}
