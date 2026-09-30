package orchestrator

import (
	"time"

	"github.com/google/uuid"
)

//==========================================//
//              EVENTS (inbound)            //
//==========================================//

// AuditValidatedEvent is the JSON payload published by the Intake service
// to the "audit.validated" NATS subject after a URL passes all validation checks.
// The Orchestrator subscribes to this event and drives the state machine forward.
type AuditValidatedEvent struct {
	AuditID       uuid.UUID `json:"audit_id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	NormalizedURL string    `json:"normalized_url"`
	Domain        string    `json:"domain"`
}

//==========================================//
//              COMMANDS (outbound)         //
//==========================================//

// StartCrawlCommand is published to "audit.commands.start_crawl" after the
// Orchestrator transitions an audit to crawling.
// The Crawler doesn't exist yet — this publishes into the void for now.
// The shape is defined early so the Crawler can be built against a stable contract.
type StartCrawlCommand struct {
	AuditID       uuid.UUID `json:"audit_id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	NormalizedURL string    `json:"normalized_url"`
	Domain        string    `json:"domain"`
}

//==========================================//
//              RESPONSES (HTTP)            //
//==========================================//

// AuditStatusResponse is returned by the internal status endpoint.
// The API Gateway will call this; nothing else should need the raw DB row.
type AuditStatusResponse struct {
	ID            uuid.UUID    `json:"id"`
	TenantID      uuid.UUID    `json:"tenant_id"`
	NormalizedURL string       `json:"normalized_url"`
	Domain        string       `json:"domain"`
	Status        AuditStatus  `json:"status"`
	StepStartedAt *time.Time   `json:"step_started_at,omitempty"`
	StepAttempts  int          `json:"step_attempts"`
	SubmittedAt   time.Time    `json:"submitted_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}
