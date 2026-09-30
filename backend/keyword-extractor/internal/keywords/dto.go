package keywords

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

// KeywordsExtractedEvent is published to NATS subject "audit.events.keywords.extracted".
type KeywordsExtractedEvent struct {
	AuditID          uuid.UUID `json:"audit_id"`
	TenantID         uuid.UUID `json:"tenant_id"`
	Domain           string    `json:"domain"`
	TotalPages       int       `json:"total_pages"`
	UniqueKeywords   int       `json:"unique_keywords"`
	Cannibalizations int       `json:"cannibalizations"`
	DurationMs       int64     `json:"duration_ms"`
	CompletedAt      time.Time `json:"completed_at"`
}
