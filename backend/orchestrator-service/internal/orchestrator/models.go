package orchestrator

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

//==========================================//
//              AUDIT MODEL                 //
//==========================================//

// Audit mirrors the row created by the Intake service.
// The Orchestrator is the only service allowed to change status after
// Intake creates the row — this is enforced by convention, not DB constraint,
// and written here as an explicit rule:
//
//   RULE: Intake creates the audit row (status=queued).
//         The Orchestrator is the sole writer of the status column thereafter.
//         No other service may UPDATE audits.status directly — they publish
//         an event and let the Orchestrator do the state transition.
type Audit struct {
	bun.BaseModel `bun:"table:audits,alias:a"`

	ID            uuid.UUID   `bun:"_id,pk,type:uuid"`
	TenantID      uuid.UUID   `bun:"tenant_id,notnull,type:uuid"`
	NormalizedURL string      `bun:"normalized_url,notnull"`
	Domain        string      `bun:"domain,notnull"`
	Status        AuditStatus `bun:"status,notnull"`

	// step_started_at records when the current step began.
	StepStartedAt *time.Time `bun:"step_started_at,nullzero"`

	// step_attempts is the retry counter for the current step.
	StepAttempts int `bun:"step_attempts,notnull,default:0"`

	SubmittedAt time.Time `bun:"submitted_at,nullzero,notnull"`
	UpdatedAt   time.Time `bun:"updated_at,nullzero,notnull"`
}

//==========================================//
//         AUDIT TRANSITION MODEL           //
//==========================================//

type AuditTransition struct {
	bun.BaseModel `bun:"table:audit_transitions,alias:at"`

	ID             uuid.UUID   `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	AuditID        uuid.UUID   `bun:"audit_id,notnull,type:uuid"`
	FromStatus     AuditStatus `bun:"from_status,notnull"`
	ToStatus       AuditStatus `bun:"to_status,notnull"`
	IdempotencyKey string      `bun:"idempotency_key,notnull,unique"`
	TransitionedAt time.Time   `bun:"transitioned_at,nullzero,notnull,default:current_timestamp"`
}

//==========================================//
//       AUDIT ANALYZER RUN MODEL           //
//==========================================//

// AuditAnalyzerRun tracks which parallel analyzers have reported completion for an audit.
// The Orchestrator performs a parallel join: it only advances from analyzing to the next step
// once all required analyzers (On-Page, Technical, etc.) have completed.
type AuditAnalyzerRun struct {
	bun.BaseModel `bun:"table:audit_analyzer_runs,alias:aar"`

	ID            uuid.UUID `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	AuditID       uuid.UUID `bun:"audit_id,notnull,type:uuid"`
	AnalyzerName  string    `bun:"analyzer_name,notnull"` // "onpage", "technical"
	Status        string    `bun:"status,notnull"`        // "completed", "failed"
	FindingsCount int       `bun:"findings_count,notnull"`
	DurationMs    int64     `bun:"duration_ms,notnull"`
	CompletedAt   time.Time `bun:"completed_at,nullzero,notnull,default:current_timestamp"`
}
