package analyzer

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

type Repository interface {
	FindParsedPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]ParsedPageRecord, error)
	SaveFindings(ctx context.Context, findings []Finding) error
	FindFindingsByAuditID(ctx context.Context, auditID uuid.UUID) ([]Finding, error)
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

// FindParsedPagesByAuditID reads all structured page records for an audit.
func (r *repository) FindParsedPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]ParsedPageRecord, error) {
	var pages []ParsedPageRecord
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&pages).
			Table("parsed_pages").
			Where("audit_id = ?", auditID).
			Order("parsed_at ASC").
			Scan(ctx)
	}, "Find Parsed Pages By Audit ID")

	if err != nil {
		return nil, fmt.Errorf("failed to query parsed pages for audit %s: %w", auditID, err)
	}
	return pages, nil
}

// SaveFindings batch inserts generated SEO findings into the audit_findings table.
func (r *repository) SaveFindings(ctx context.Context, findings []Finding) error {
	if len(findings) == 0 {
		return nil
	}

	return r.executeWithRetry(ctx, func() error {
		_, err := r.db.NewInsert().
			Model(&findings).
			Exec(ctx)
		return err
	}, "Save Audit Findings")
}

// FindFindingsByAuditID queries all stored findings for a given audit.
func (r *repository) FindFindingsByAuditID(ctx context.Context, auditID uuid.UUID) ([]Finding, error) {
	var findings []Finding
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&findings).
			Where("af.audit_id = ?", auditID).
			Order("af.created_at ASC").
			Scan(ctx)
	}, "Find Findings By Audit ID")

	if err != nil {
		return nil, fmt.Errorf("failed to query findings: %w", err)
	}
	return findings, nil
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

		select {
		case <-time.After(r.retryDelay * time.Duration(attempt)):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return lastErr
}
