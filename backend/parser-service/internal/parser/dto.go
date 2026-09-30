package parser

import (
	"time"

	"github.com/google/uuid"
)

//==========================================//
//              INBOUND COMMANDS/EVENTS     //
//==========================================//

// CrawlCompletedEvent is consumed from "audit.events.crawl_completed" or "audit.commands.start_parse".
type CrawlCompletedEvent struct {
	AuditID       uuid.UUID `json:"audit_id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	Domain        string    `json:"domain"`
	NormalizedURL string    `json:"normalized_url"`
	TotalFetched  int       `json:"total_fetched"`
	TotalErrors   int       `json:"total_errors"`
	DurationMs    int64     `json:"duration_ms"`
	CompletedAt   time.Time `json:"completed_at"`
}

//==========================================//
//              OUTBOUND EVENTS             //
//==========================================//

// ParseCompletedEvent is published to NATS subject "audit.events.parse_completed".
// Downstream Analyzer services listen on this event to fan out parallel analysis jobs.
type ParseCompletedEvent struct {
	AuditID           uuid.UUID `json:"audit_id"`
	TenantID          uuid.UUID `json:"tenant_id"`
	Domain            string    `json:"domain"`
	NormalizedURL     string    `json:"normalized_url"`
	TotalPagesParsed  int       `json:"total_pages_parsed"`
	TotalPagesFailed  int       `json:"total_pages_failed"`
	TotalPages        int       `json:"total_pages"`
	DurationMs        int64     `json:"duration_ms"`
	CompletedAt       time.Time `json:"completed_at"`
}

// ParseFailedEvent is published to NATS subject "audit.events.parse_failed".
type ParseFailedEvent struct {
	AuditID       uuid.UUID `json:"audit_id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	Domain        string    `json:"domain"`
	Reason        string    `json:"reason"`
	FailedAt      time.Time `json:"failed_at"`
}
