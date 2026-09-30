package orchestrator

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
	FindAuditByID(ctx context.Context, auditID uuid.UUID) (*Audit, error)
	TransitionStatus(ctx context.Context, auditID uuid.UUID, from, to AuditStatus, idempotencyKey string) error
	RecordAnalyzerRun(ctx context.Context, run *AuditAnalyzerRun) error
	GetCompletedAnalyzers(ctx context.Context, auditID uuid.UUID) ([]string, error)
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
//        AUDIT QUERY OPERATIONS            //
//==========================================//

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
		return nil, fmt.Errorf("Error finding audit (%s): %w", auditID.String(), err)
	}

	return audit, nil
}

//==========================================//
//        STATE TRANSITION OPERATION        //
//==========================================//

// TransitionStatus implements [Repository].
func (r *repository) TransitionStatus(ctx context.Context, auditID uuid.UUID, from, to AuditStatus, idempotencyKey string) error {
	if err := Transition(from, to); err != nil {
		return err
	}

	now := time.Now().UTC()

	return r.executeWithRetry(ctx, func() error {
		return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			transition := &AuditTransition{
				AuditID:        auditID,
				FromStatus:     from,
				ToStatus:       to,
				IdempotencyKey: idempotencyKey,
				TransitionedAt: now,
			}
			if _, err := tx.NewInsert().Model(transition).Exec(ctx); err != nil {
				return fmt.Errorf("insert transition log: %w", err)
			}

			res, err := tx.NewUpdate().
				TableExpr("audits").
				Set("status = ?", to).
				Set("step_started_at = ?", now).
				Set("step_attempts = 0").
				Set("updated_at = ?", now).
				Where("_id = ?", auditID).
				Where("status = ?", from).
				Exec(ctx)

			if err != nil {
				return fmt.Errorf("update audit status: %w", err)
			}

			rows, err := res.RowsAffected()
			if err != nil {
				return fmt.Errorf("check rows affected: %w", err)
			}
			if rows == 0 {
				return ErrStatusMismatch
			}

			return nil
		})
	}, fmt.Sprintf("Transition %s → %s", from, to))
}

//==========================================//
//     PARALLEL JOIN ANALYZER TRACKING      //
//==========================================//

// RecordAnalyzerRun saves the completion status of a specific analyzer for an audit.
func (r *repository) RecordAnalyzerRun(ctx context.Context, run *AuditAnalyzerRun) error {
	return r.executeWithRetry(ctx, func() error {
		_, err := r.db.NewInsert().
			Model(run).
			On("CONFLICT (audit_id, analyzer_name) DO UPDATE").
			Set("status = EXCLUDED.status").
			Set("findings_count = EXCLUDED.findings_count").
			Set("duration_ms = EXCLUDED.duration_ms").
			Set("completed_at = EXCLUDED.completed_at").
			Exec(ctx)
		return err
	}, "Record Analyzer Run")
}

// GetCompletedAnalyzers returns the list of unique analyzer names that have completed for an audit.
func (r *repository) GetCompletedAnalyzers(ctx context.Context, auditID uuid.UUID) ([]string, error) {
	var runs []AuditAnalyzerRun
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&runs).
			Where("aar.audit_id = ?", auditID).
			Where("aar.status = ?", "completed").
			Scan(ctx)
	}, "Get Completed Analyzers")

	if err != nil {
		return nil, err
	}

	analyzers := make([]string, len(runs))
	for i, run := range runs {
		analyzers[i] = run.AnalyzerName
	}
	return analyzers, nil
}

//==========================================//
//             SENTINEL ERRORS              //
//==========================================//

var ErrAlreadyProcessed = fmt.Errorf("event already processed (idempotency key exists)")
var ErrStatusMismatch = fmt.Errorf("audit status mismatch — concurrent transition or audit not found")

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
			select {
			case <-time.After(r.retryDelay * time.Duration(attempt)):
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			break
		}
	}

	return lastErr
}

func (r *repository) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, ErrAlreadyProcessed) || errors.Is(err, ErrStatusMismatch) {
		return false
	}

	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	retryable := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"deadlock",
	}
	for _, e := range retryable {
		if strings.Contains(err.Error(), e) {
			return true
		}
	}

	return false
}
