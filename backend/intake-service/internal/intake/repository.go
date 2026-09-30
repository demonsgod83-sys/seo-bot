package intake

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

type Repository interface {
	CreateAudit(ctx context.Context, audit *Audit) (uuid.UUID, error)
	FindAuditByID(ctx context.Context, auditID uuid.UUID) (*Audit, error)
	FindActiveAuditByDomain(ctx context.Context, domain string, tenantID uuid.UUID) (*Audit, error)
}

type repository struct {
	db         *bun.DB
	logger     *zap.Logger
	maxRetries int
	retryDelay time.Duration
}

func NewRepository(db *bun.DB, logger *zap.Logger) Repository {
	return &repository{
		db:         db,
		logger:     logger,
		maxRetries: 3,
		retryDelay: 1 * time.Second,
	}
}

//==========================================//
//        AUDIT CRUD OPERATIONS             //
//==========================================//

// CreateAudit implements [Repository].
func (r *repository) CreateAudit(ctx context.Context, audit *Audit) (uuid.UUID, error) {
	err := r.executeWithRetry(ctx, func() error {
		_, err := r.db.NewInsert().
			Model(audit).
			Returning("*").
			Exec(ctx)

		return err
	}, "Create Audit")

	if err != nil {
		return uuid.Nil, fmt.Errorf("Error creating audit: %w", err)
	}

	return audit.ID, nil
}

// FindAuditByID implements [Repository].
func (r *repository) FindAuditByID(ctx context.Context, auditID uuid.UUID) (*Audit, error) {
	audit := new(Audit)

	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(audit).
			Where("a._id = ?", auditID).
			Scan(ctx)
	}, "Find Audit By ID")

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("Audit (%s) not found!", auditID.String())
		}
		return nil, fmt.Errorf("Error finding audit with ID (%s): %w", auditID.String(), err)
	}

	return audit, nil
}

// FindActiveAuditByDomain implements [Repository].
// Returns a non-terminal audit for the given domain+tenant, or nil if none exists.
func (r *repository) FindActiveAuditByDomain(ctx context.Context, domain string, tenantID uuid.UUID) (*Audit, error) {
	audit := new(Audit)

	// Non-terminal statuses — queued or in_progress
	activeStatuses := []string{
		string(AuditStatusQueued),
		string(AuditStatusInProgress),
	}

	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(audit).
			Where("a.domain = ?", domain).
			Where("a.tenant_id = ?", tenantID).
			Where("a.status IN (?)", bun.In(activeStatuses)).
			Limit(1).
			Scan(ctx)
	}, "Find Active Audit By Domain")

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("Error checking for active audit for domain (%s): %w", domain, err)
	}

	return audit, nil
}

//==========================================//
//             HELPER FUNCTIONS             //
//==========================================//

func (r *repository) executeWithRetry(ctx context.Context, operation func() error, name string) error {

	var lastErr error
	for attempt := 1; attempt <= r.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		r.logger.Warn(name+" failed",
			zap.Int("attempt", attempt),
			zap.Error(err),
		)

		if attempt == r.maxRetries {
			break
		}

		if r.shouldRetry(err) {
			// Either cancel or wait
			select {
			case <-time.After(r.retryDelay * time.Duration(attempt)):
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			break
		}
	}

	r.logger.Error(name+" failed after retries", zap.Error(lastErr))
	return lastErr
}

func (r *repository) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	retryable := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"deadlock",
	}

	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	for _, e := range retryable {
		if strings.Contains(err.Error(), e) {
			return true
		}
	}

	return false
}
