package notification

import (
	"time"

	"github.com/google/uuid"
)

//==========================================//
//              NATS EVENTS                 //
//==========================================//

type ReportGeneratedEvent struct {
	AuditID      uuid.UUID `json:"audit_id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	Domain       string    `json:"domain"`
	Version      int       `json:"version"`
	ShareToken   string    `json:"share_token"`
	ShareURL     string    `json:"share_url"`
	JsonPath     string    `json:"json_path"`
	HtmlPath     string    `json:"html_path"`
	PdfPath      string    `json:"pdf_path"`
	OverallScore int       `json:"overall_score"`
	HasWarnings  bool      `json:"has_warnings"`
	GeneratedAt  time.Time `json:"generated_at"`
	DurationMs   int64     `json:"duration_ms"`
}

type AuditFailedEvent struct {
	AuditID  uuid.UUID `json:"audit_id"`
	TenantID uuid.UUID `json:"tenant_id"`
	Domain   string    `json:"domain"`
	Reason   string    `json:"reason"`
}

type NotificationCompletedEvent struct {
	AuditID       uuid.UUID `json:"audit_id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	Domain        string    `json:"domain"`
	EmailStatus   string    `json:"email_status"`
	WebhookStatus string    `json:"webhook_status"`
	CompletedAt   time.Time `json:"completed_at"`
}

//==========================================//
//              WEBHOOK PAYLOAD             //
//==========================================//

type WebhookPayload struct {
	Event          string    `json:"event"` // "audit.completed", "audit.failed"
	AuditID        uuid.UUID `json:"audit_id"`
	TenantID       uuid.UUID `json:"tenant_id"`
	Domain         string    `json:"domain"`
	Status         string    `json:"status"`
	OverallScore   int       `json:"overall_score,omitempty"`
	TechnicalScore int       `json:"technical_score,omitempty"`
	OnPageScore    int       `json:"onpage_score,omitempty"`
	ContentScore   int       `json:"content_score,omitempty"`
	ReportURL      string    `json:"report_url,omitempty"`
	PdfURL         string    `json:"pdf_url,omitempty"`
	JsonURL        string    `json:"json_url,omitempty"`
	FailureReason  string    `json:"failure_reason,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

//==========================================//
//              HTTP RESPONSES              //
//==========================================//

type NotificationDeliveriesResponse struct {
	AuditID    uuid.UUID              `json:"audit_id"`
	Deliveries []NotificationDelivery `json:"deliveries"`
}

type SetTenantConfigRequest struct {
	TenantID         uuid.UUID `json:"tenant_id" binding:"required"`
	Email            string    `json:"email"`
	WebhookURL       string    `json:"webhook_url"`
	WebhookSecret    string    `json:"webhook_secret"`
	IsEmailEnabled   bool      `json:"is_email_enabled"`
	IsWebhookEnabled bool      `json:"is_webhook_enabled"`
}
