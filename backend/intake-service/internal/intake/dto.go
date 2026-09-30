package intake

import (
	"time"

	"github.com/google/uuid"
)

//==========================================//
//               REQUESTS                   //
//==========================================//

type SubmitAuditRequest struct {
	URL      string    `json:"url"       validate:"required"`
	TenantID uuid.UUID `json:"tenant_id" validate:"required"`
}

//==========================================//
//               RESPONSES                  //
//==========================================//

type SubmitAuditResponse struct {
	AuditID uuid.UUID `json:"audit_id"`
}

type GetAuditResponse struct {
	ID            uuid.UUID   `json:"id"`
	TenantID      uuid.UUID   `json:"tenant_id"`
	RawURL        string      `json:"raw_url"`
	NormalizedURL string      `json:"normalized_url"`
	Domain        string      `json:"domain"`
	Status        AuditStatus `json:"status"`
	SubmittedAt   time.Time   `json:"submitted_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}
