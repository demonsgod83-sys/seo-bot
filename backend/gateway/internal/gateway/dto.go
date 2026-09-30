package gateway

import (
	"github.com/google/uuid"
)

//==========================================//
//           CANONICAL ERROR SHAPE          //
//==========================================//

// ErrorBody is the single error envelope all gateway responses use.
// Decided here because every service will eventually speak this shape
// back through the Gateway to the client.
//
// Shape: { "error": { "code": "SNAKE_CASE_CODE", "message": "human readable" }, "request_id": "..." }
type ErrorBody struct {
	Error     ErrorDetail `json:"error"`
	RequestID string      `json:"request_id"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

//==========================================//
//           INBOUND REQUEST TYPES          //
//==========================================//

// SubmitAuditRequest is what the external client sends to POST /v1/audits.
// The Gateway deliberately does NOT validate the URL deeply — that is Intake's job.
// All the gateway checks: is url present and is it a non-empty string.
type SubmitAuditRequest struct {
	URL string `json:"url" binding:"required"`
}

//==========================================//
//           OUTBOUND RESPONSE TYPES        //
//==========================================//

// SubmitAuditResponse is 202 Accepted for POST /v1/audits.
type SubmitAuditResponse struct {
	Data      SubmitAuditData `json:"data"`
	RequestID string          `json:"request_id"`
}

type SubmitAuditData struct {
	AuditID uuid.UUID `json:"audit_id"`
}

// AuditStatusResponse is 200 OK for GET /v1/audits/:id.
// Fields are a subset of what the Orchestrator returns — we expose what the
// client needs and nothing more.
type AuditStatusResponse struct {
	Data      AuditStatusData `json:"data"`
	RequestID string          `json:"request_id"`
}

type AuditStatusData struct {
	AuditID string `json:"audit_id"`
	Status  string `json:"status"`
	Domain  string `json:"domain"`
	// StepStartedAt and StepAttempts are intentionally exposed — clients
	// can use them to render progress UI or diagnose stuck audits.
	StepStartedAt *string `json:"step_started_at,omitempty"`
	StepAttempts  int     `json:"step_attempts"`
	SubmittedAt   string  `json:"submitted_at"`
	UpdatedAt     string  `json:"updated_at"`
}
