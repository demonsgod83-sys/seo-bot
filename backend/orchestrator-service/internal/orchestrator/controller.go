package orchestrator

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

//==========================================//
//             STATUS FUNCTIONS             //
//==========================================//

func (h *Handler) GetAuditStatusHandler(c *gin.Context) {
	auditIDStr := c.Param("id")

	auditID, err := uuid.Parse(auditIDStr)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}

	status, err := h.svc.GetAuditStatus(c.Request.Context(), auditID)
	if err != nil {
		fail(c, http.StatusNotFound, err.Error())
		return
	}

	success(c, http.StatusOK, status)
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
