package analyzer

import (
	"time"

	"github.com/google/uuid"
)

//==========================================//
//              INBOUND EVENTS              //
//==========================================//

// ParseCompletedEvent is consumed from NATS subject "audit.events.parse_completed".
type ParseCompletedEvent struct {
	AuditID          uuid.UUID `json:"audit_id"`
	TenantID         uuid.UUID `json:"tenant_id"`
	Domain           string    `json:"domain"`
	NormalizedURL    string    `json:"normalized_url"`
	TotalPagesParsed int       `json:"total_pages_parsed"`
	TotalPagesFailed int       `json:"total_pages_failed"`
	TotalPages       int       `json:"total_pages"`
	DurationMs       int64     `json:"duration_ms"`
	CompletedAt      time.Time `json:"completed_at"`
}

//==========================================//
//              OUTBOUND EVENTS             //
//==========================================//

// OnPageAnalysisCompletedEvent is published to NATS subject "audit.events.analysis.onpage.completed".
type OnPageAnalysisCompletedEvent struct {
	AuditID        uuid.UUID `json:"audit_id"`
	TenantID       uuid.UUID `json:"tenant_id"`
	Domain         string    `json:"domain"`
	TotalPages     int       `json:"total_pages"`
	TotalFindings  int       `json:"total_findings"`
	CriticalCount  int       `json:"critical_count"`
	WarningCount   int       `json:"warning_count"`
	InfoCount      int       `json:"info_count"`
	DurationMs     int64     `json:"duration_ms"`
	CompletedAt    time.Time `json:"completed_at"`
}
