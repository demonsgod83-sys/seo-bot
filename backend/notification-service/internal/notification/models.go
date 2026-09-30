package notification

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

//==========================================//
//          NOTIFICATION CONFIG MODEL       //
//==========================================//

type TenantNotificationConfig struct {
	bun.BaseModel `bun:"table:tenant_notification_configs,alias:tnc"`

	ID               uuid.UUID `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	TenantID         uuid.UUID `bun:"tenant_id,notnull,unique,type:uuid"`
	Email            string    `bun:"email"`
	WebhookURL       string    `bun:"webhook_url"`
	WebhookSecret    string    `bun:"webhook_secret"`
	IsEmailEnabled   bool      `bun:"is_email_enabled,notnull,default:true"`
	IsWebhookEnabled bool      `bun:"is_webhook_enabled,notnull,default:false"`
	CreatedAt        time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

//==========================================//
//          NOTIFICATION DELIVERY MODEL     //
//==========================================//

type NotificationDelivery struct {
	bun.BaseModel `bun:"table:notification_deliveries,alias:nd"`

	ID            uuid.UUID      `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	AuditID       uuid.UUID      `bun:"audit_id,notnull,type:uuid"`
	TenantID      uuid.UUID      `bun:"tenant_id,notnull,type:uuid"`
	Channel       string         `bun:"channel,notnull"`   // "email", "webhook"
	Recipient     string         `bun:"recipient,notnull"` // email address or webhook URL
	Status        string         `bun:"status,notnull"`    // "delivered", "failed", "pending"
	AttemptsCount int            `bun:"attempts_count,notnull,default:1"`
	LastError     string         `bun:"last_error"`
	Payload       map[string]any `bun:"payload,type:jsonb"`
	DeliveredAt   *time.Time     `bun:"delivered_at"`
	CreatedAt     time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

//==========================================//
//          REFERENCE READ MODELS           //
//==========================================//

type AuditRecord struct {
	bun.BaseModel `bun:"table:audits,alias:a"`

	ID            uuid.UUID  `bun:"_id,pk,type:uuid"`
	TenantID      uuid.UUID  `bun:"tenant_id,notnull,type:uuid"`
	NormalizedURL string     `bun:"normalized_url,notnull"`
	Domain        string     `bun:"domain,notnull"`
	Status        string     `bun:"status,notnull"`
	StepStartedAt *time.Time `bun:"step_started_at"`
	SubmittedAt   time.Time  `bun:"submitted_at"`
	UpdatedAt     time.Time  `bun:"updated_at"`
}

type AuditScore struct {
	bun.BaseModel `bun:"table:audit_scores,alias:asc"`

	ID               uuid.UUID `bun:"_id,pk,type:uuid"`
	AuditID          uuid.UUID `bun:"audit_id,notnull,type:uuid"`
	TenantID         uuid.UUID `bun:"tenant_id,notnull,type:uuid"`
	OverallScore     int       `bun:"overall_score,notnull"`
	TechnicalScore   int       `bun:"technical_score,notnull"`
	OnPageScore      int       `bun:"onpage_score,notnull"`
	ContentScore     int       `bun:"content_score,notnull"`
	TotalFindings    int       `bun:"total_findings,notnull"`
	CriticalCount    int       `bun:"critical_count,notnull"`
	WarningCount     int       `bun:"warning_count,notnull"`
	InfoCount        int       `bun:"info_count,notnull"`
	TotalPagesScored int       `bun:"total_pages_scored,notnull"`
}

type PrioritizedIssue struct {
	bun.BaseModel `bun:"table:prioritized_issues,alias:pi"`

	ID                 uuid.UUID `bun:"_id,pk,type:uuid"`
	AuditID            uuid.UUID `bun:"audit_id,notnull,type:uuid"`
	RuleID             string    `bun:"rule_id,notnull"`
	Category           string    `bun:"category,notnull"`
	Severity           string    `bun:"severity,notnull"`
	Title              string    `bun:"title,notnull"`
	Message            string    `bun:"message,notnull"`
	EffortTier         string    `bun:"effort_tier,notnull"`
	ImpactScore        float64   `bun:"impact_score,notnull"`
	PriorityScore      float64   `bun:"priority_score,notnull"`
	AffectedPagesCount int       `bun:"affected_pages_count,notnull"`
}
