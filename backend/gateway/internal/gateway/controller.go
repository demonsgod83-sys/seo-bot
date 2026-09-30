package gateway

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	intakeclient "github.com/demonsgod83-sys/seo-bot/gateway/internal/clients/intake"
	orchestratorclient "github.com/demonsgod83-sys/seo-bot/gateway/internal/clients/orchestrator"
	"go.uber.org/zap"
)

type Handler struct {
	intake       intakeclient.Client
	orchestrator orchestratorclient.Client
	logger       *zap.Logger
}

func NewHandler(
	intake intakeclient.Client,
	orchestrator orchestratorclient.Client,
	logger *zap.Logger,
) *Handler {
	return &Handler{
		intake:       intake,
		orchestrator: orchestrator,
		logger:       logger,
	}
}

//==========================================//
//           POST /v1/audits                //
//==========================================//

// SubmitAudit accepts a URL from the client, does lightweight shape validation,
// and forwards to Intake. It deliberately does NOT duplicate Intake's deep
// validation (DNS, SSRF, reachability) — those live in Intake and nowhere else.
func (h *Handler) SubmitAudit(c *gin.Context) {
	var req SubmitAuditRequest

	// ShouldBindJSON handles: malformed JSON, missing required fields.
	// binding:"required" on the URL field means an empty/missing url returns 400
	// before we even call Intake.
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Extra guard: whitespace-only URL should not reach Intake.
	if strings.TrimSpace(req.URL) == "" {
		fail(c, http.StatusBadRequest, "MISSING_FIELD", "url must be a non-empty string")
		return
	}

	// Tenant ID is injected by the Auth middleware — never trust client-supplied value.
	tenantID := c.MustGet("tenant_id").(uuid.UUID)

	result, err := h.intake.SubmitAudit(c.Request.Context(), intakeclient.SubmitAuditRequest{
		URL:      req.URL,
		TenantID: tenantID,
	})

	if err != nil {
		// If Intake explicitly rejected the request (validation failure, duplicate),
		// propagate a 422 to the client rather than 502 Bad Gateway.
		var intakeErr *intakeclient.IntakeError
		if errors.As(err, &intakeErr) {
			h.logger.Info("intake rejected audit submission",
				zap.Int("intake_status", intakeErr.StatusCode),
				zap.String("request_id", c.GetString("request_id")),
			)
			fail(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", extractIntakeMessage(intakeErr.Body))
			return
		}

		h.logger.Error("intake call failed",
			zap.Error(err),
			zap.String("request_id", c.GetString("request_id")),
		)
		fail(c, http.StatusBadGateway, "UPSTREAM_ERROR", "intake service is unavailable — try again shortly")
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"data":       gin.H{"audit_id": result.AuditID},
		"request_id": c.GetString("request_id"),
	})
}

//==========================================//
//           GET /v1/audits/:id             //
//==========================================//

// GetAuditStatus proxies the status query to the Orchestrator.
// The Orchestrator is the single source of truth for audit state — the Gateway
// never queries the database directly.
func (h *Handler) GetAuditStatus(c *gin.Context) {
	auditIDStr := c.Param("id")

	auditID, err := uuid.Parse(auditIDStr)
	if err != nil {
		fail(c, http.StatusBadRequest, "INVALID_ID", "audit ID must be a valid UUID")
		return
	}

	requestingTenant := c.MustGet("tenant_id").(uuid.UUID)

	status, err := h.orchestrator.GetAuditStatus(c.Request.Context(), auditID)
	if err != nil {
		if errors.Is(err, orchestratorclient.ErrNotFound) {
			fail(c, http.StatusNotFound, "NOT_FOUND", "audit not found")
			return
		}
		h.logger.Error("orchestrator call failed",
			zap.Error(err),
			zap.String("request_id", c.GetString("request_id")),
		)
		fail(c, http.StatusBadGateway, "UPSTREAM_ERROR", "orchestrator is unavailable — try again shortly")
		return
	}

	// Tenant isolation: return 404 (not 403) if the audit belongs to a different
	// tenant — don't reveal that the audit exists at all.
	if status.TenantID != requestingTenant {
		fail(c, http.StatusNotFound, "NOT_FOUND", "audit not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": buildStatusData(status),
		"request_id": c.GetString("request_id"),
	})
}

//==========================================//
//           HELPER FUNCTIONS               //
//==========================================//

func fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
		"request_id": c.GetString("request_id"),
	})
}

func buildStatusData(s orchestratorclient.AuditStatusResponse) gin.H {
	data := gin.H{
		"audit_id":      s.ID,
		"status":        s.Status,
		"domain":        s.Domain,
		"step_attempts": s.StepAttempts,
		"submitted_at":  s.SubmittedAt,
		"updated_at":    s.UpdatedAt,
	}
	if s.StepStartedAt != nil {
		data["step_started_at"] = s.StepStartedAt
	}
	return data
}

// extractIntakeMessage tries to pull the human-readable message out of the
// intake error body so we don't expose internal JSON structure to clients.
func extractIntakeMessage(body string) string {
	if strings.Contains(body, "URL validation failed") {
		// Trim the "URL validation failed: " prefix for cleaner client messages
		if idx := strings.Index(body, "URL validation failed: "); idx >= 0 {
			return body[idx+len("URL validation failed: "):]
		}
	}
	if body == "" {
		return "the submitted URL was rejected by the validation service"
	}
	return body
}
