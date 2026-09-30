package intake

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

//==========================================//
//              EVENT PUBLISHER             //
//==========================================//

// EventPublisher is the interface the service uses to publish lifecycle events.
// The concrete implementation wraps nats.JetStreamContext.
// Keeping it as an interface means tests don't need a real NATS server.
type EventPublisher interface {
	PublishAuditValidated(ctx context.Context, event AuditValidatedEvent) error
}

// AuditValidatedEvent is the payload published to "audit.validated" after a
// URL passes all validation checks and the audit record is persisted.
// The Orchestrator subscribes to this event to drive the state machine forward.
type AuditValidatedEvent struct {
	AuditID       uuid.UUID `json:"audit_id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	NormalizedURL string    `json:"normalized_url"`
	Domain        string    `json:"domain"`
}

// natsPublisher is the production EventPublisher backed by NATS JetStream.
type natsPublisher struct {
	js nats.JetStreamContext
}

func NewNatsPublisher(js nats.JetStreamContext) EventPublisher {
	return &natsPublisher{js: js}
}

func (p *natsPublisher) PublishAuditValidated(ctx context.Context, event AuditValidatedEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit.validated event: %w", err)
	}

	// Deterministic Msg-Id: if Intake crashes after publishing but before
	// committing, it will retry with the same ID — JetStream deduplicates it.
	msgID := fmt.Sprintf("audit:%s:validated", event.AuditID)

	_, err = p.js.Publish(
		"audit.validated",
		payload,
		nats.MsgId(msgID),
		nats.Context(ctx),
	)
	if err != nil {
		return fmt.Errorf("publish audit.validated: %w", err)
	}

	return nil
}

//==========================================//
//              SERVICE                     //
//==========================================//

type Service interface {
	SubmitAudit(ctx context.Context, req SubmitAuditRequest) (uuid.UUID, error)
	GetAudit(ctx context.Context, auditID uuid.UUID) (GetAuditResponse, error)
}

type service struct {
	repository Repository
	validator  *validator
	publisher  EventPublisher // nil-safe — NATS is optional at startup
	logger     *zap.Logger
}

func NewService(repository Repository, publisher EventPublisher, logger *zap.Logger) Service {
	return &service{
		repository: repository,
		validator:  newValidator(),
		publisher:  publisher,
		logger:     logger,
	}
}

//==========================================//
//             AUDIT FUNCTIONS              //
//==========================================//

// SubmitAudit implements [Service].
// Runs the full intake pipeline: validate → normalize → SSRF → reachability →
// duplicate check → persist → publish event.
func (s *service) SubmitAudit(ctx context.Context, req SubmitAuditRequest) (uuid.UUID, error) {

	// Step 1–4: format validation, normalization, DNS, SSRF, reachability
	normalizedURL, domain, err := s.validator.Validate(ctx, req.URL)
	if err != nil {
		return uuid.Nil, fmt.Errorf("URL validation failed: %w", err)
	}

	s.logger.Info("URL validated",
		zap.String("raw", req.URL),
		zap.String("normalized", normalizedURL),
		zap.String("domain", domain),
	)

	// Step 5: Duplicate / active-audit check
	existing, err := s.repository.FindActiveAuditByDomain(ctx, domain, req.TenantID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("Error checking for existing audit: %w", err)
	}

	if existing != nil {
		return uuid.Nil, fmt.Errorf(
			"An audit for domain (%s) is already %s (audit_id: %s) — wait for it to finish before submitting a new one",
			domain, existing.Status, existing.ID.String(),
		)
	}

	// Step 6: Create the audit record
	audit := &Audit{
		TenantID:      req.TenantID,
		RawURL:        req.URL,
		NormalizedURL: normalizedURL,
		Domain:        domain,
		Status:        AuditStatusQueued,
	}

	auditID, err := s.repository.CreateAudit(ctx, audit)
	if err != nil {
		return uuid.Nil, fmt.Errorf("Failed to create audit: %w", err)
	}

	s.logger.Info("Audit created",
		zap.String("audit_id", auditID.String()),
		zap.String("domain", domain),
		zap.String("tenant_id", req.TenantID.String()),
	)

	// Step 7: Publish audit.validated so the Orchestrator drives the state machine.
	// Non-fatal: the audit record exists. A future reconciler can catch missed events.
	if s.publisher != nil {
		event := AuditValidatedEvent{
			AuditID:       auditID,
			TenantID:      req.TenantID,
			NormalizedURL: normalizedURL,
			Domain:        domain,
		}
		if err := s.publisher.PublishAuditValidated(ctx, event); err != nil {
			s.logger.Warn("Failed to publish audit.validated — audit created but Orchestrator not notified",
				zap.String("audit_id", auditID.String()),
				zap.Error(err),
			)
		}
	}

	// Step 8: Return only the audit ID — caller does not need anything else
	return auditID, nil
}

// GetAudit implements [Service].
func (s *service) GetAudit(ctx context.Context, auditID uuid.UUID) (GetAuditResponse, error) {
	audit, err := s.repository.FindAuditByID(ctx, auditID)
	if err != nil {
		return GetAuditResponse{}, err
	}

	return GetAuditResponse{
		ID:            audit.ID,
		TenantID:      audit.TenantID,
		RawURL:        audit.RawURL,
		NormalizedURL: audit.NormalizedURL,
		Domain:        audit.Domain,
		Status:        audit.Status,
		SubmittedAt:   audit.SubmittedAt,
		UpdatedAt:     audit.UpdatedAt,
	}, nil
}

