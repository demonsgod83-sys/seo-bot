package crawler

import (
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

// GetCrawlPagesHandler retrieves the list of crawled pages for a given audit ID.
func (h *Handler) GetCrawlPagesHandler(c *gin.Context) {
	auditIDStr := c.Param("id")
	auditID, err := uuid.Parse(auditIDStr)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid audit ID UUID")
		return
	}

	pages, err := h.svc.GetPagesByAuditID(c.Request.Context(), auditID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	success(c, http.StatusOK, pages)
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
