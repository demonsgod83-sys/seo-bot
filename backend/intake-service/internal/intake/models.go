package intake

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// AuditStatus represents the lifecycle state of a URL audit.
type AuditStatus string

const (
	AuditStatusQueued     AuditStatus = "queued"
	AuditStatusInProgress AuditStatus = "in_progress"
	AuditStatusCompleted  AuditStatus = "completed"
	AuditStatusFailed     AuditStatus = "failed"
	AuditStatusCancelled  AuditStatus = "cancelled"
)

// terminalStatuses are states where no further work will happen — a new audit
// for the same domain is allowed once the existing one reaches one of these.
var terminalStatuses = map[AuditStatus]bool{
	AuditStatusCompleted: true,
	AuditStatusFailed:    true,
	AuditStatusCancelled: true,
}

func (s AuditStatus) IsTerminal() bool {
	return terminalStatuses[s]
}

//==========================================//
//               AUDIT MODEL               //
//==========================================//

type Audit struct {
	bun.BaseModel `bun:"table:audits,alias:a"`

	ID            uuid.UUID   `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	TenantID      uuid.UUID   `bun:"tenant_id,notnull,type:uuid"`
	RawURL        string      `bun:"raw_url,notnull"`
	NormalizedURL string      `bun:"normalized_url,notnull"`
	Domain        string      `bun:"domain,notnull"`
	Status        AuditStatus `bun:"status,notnull,default:'queued'"`

	// step_started_at and step_attempts are managed by the Orchestrator,
	// but mapped here so bun can scan the full row.
	StepStartedAt *time.Time `bun:"step_started_at,nullzero"`
	StepAttempts  int        `bun:"step_attempts,notnull,default:0"`

	SubmittedAt time.Time `bun:"submitted_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt   time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
