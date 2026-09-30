package scoring

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

type Repository interface {
	FindFindingsByAuditID(ctx context.Context, auditID uuid.UUID) ([]Finding, error)
	FindParsedPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]ParsedPageRecord, error)
	SaveCalculationResult(ctx context.Context, result CalculationResult) error
	GetAuditScores(ctx context.Context, auditID uuid.UUID) (*AuditScore, []PageScore, []PrioritizedIssue, error)
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

// FindFindingsByAuditID queries all stored findings across all analyzers for an audit.
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
		return nil, fmt.Errorf("failed to query findings for audit %s: %w", auditID, err)
	}
	return findings, nil
}

// FindParsedPagesByAuditID reads all structured page records for an audit.
func (r *repository) FindParsedPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]ParsedPageRecord, error) {
	var pages []ParsedPageRecord
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&pages).
			Table("parsed_pages").
			Where("audit_id = ?", auditID).
			Scan(ctx)
	}, "Find Parsed Pages By Audit ID")

	if err != nil {
		return nil, fmt.Errorf("failed to query parsed pages for audit %s: %w", auditID, err)
	}
	return pages, nil
}

// SaveCalculationResult idempotently persists audit score, page scores, and prioritized issues in a transaction.
func (r *repository) SaveCalculationResult(ctx context.Context, res CalculationResult) error {
	return r.executeWithRetry(ctx, func() error {
		return r.db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			// 1. Delete previous calculations if re-running
			_, err := tx.NewDelete().
				Model((*AuditScore)(nil)).
				Where("audit_id = ?", res.AuditScore.AuditID).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("delete old audit score: %w", err)
			}

			_, err = tx.NewDelete().
				Model((*PageScore)(nil)).
				Where("audit_id = ?", res.AuditScore.AuditID).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("delete old page scores: %w", err)
			}

			_, err = tx.NewDelete().
				Model((*PrioritizedIssue)(nil)).
				Where("audit_id = ?", res.AuditScore.AuditID).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("delete old prioritized issues: %w", err)
			}

			// 2. Insert AuditScore
			_, err = tx.NewInsert().
				Model(&res.AuditScore).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("insert audit score: %w", err)
			}

			// 3. Insert PageScores
			if len(res.PageScores) > 0 {
				_, err = tx.NewInsert().
					Model(&res.PageScores).
					Exec(ctx)
				if err != nil {
					return fmt.Errorf("insert page scores: %w", err)
				}
			}

			// 4. Insert PrioritizedIssues
			if len(res.PrioritizedIssues) > 0 {
				_, err = tx.NewInsert().
					Model(&res.PrioritizedIssues).
					Exec(ctx)
				if err != nil {
					return fmt.Errorf("insert prioritized issues: %w", err)
				}
			}

			return nil
		})
	}, "Save Scoring Calculation Result")
}

// GetAuditScores retrieves audit scores, page scores, and prioritized issues for the API.
func (r *repository) GetAuditScores(ctx context.Context, auditID uuid.UUID) (*AuditScore, []PageScore, []PrioritizedIssue, error) {
	var auditScore AuditScore
	var pageScores []PageScore
	var issues []PrioritizedIssue

	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&auditScore).
			Where("asc.audit_id = ?", auditID).
			Scan(ctx)
	}, "Get Audit Score")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil, fmt.Errorf("audit scores not found: %w", err)
		}
		return nil, nil, nil, err
	}

	_ = r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&pageScores).
			Where("psc.audit_id = ?", auditID).
			Order("psc.overall_score ASC").
			Scan(ctx)
	}, "Get Page Scores")

	_ = r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&issues).
			Where("pi.audit_id = ?", auditID).
			Order("pi.priority_score DESC", "pi.impact_score DESC").
			Scan(ctx)
	}, "Get Prioritized Issues")

	return &auditScore, pageScores, issues, nil
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
