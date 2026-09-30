package report

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
	GetAuditRecord(ctx context.Context, auditID uuid.UUID) (*AuditRecord, error)
	GetAuditScore(ctx context.Context, auditID uuid.UUID) (*AuditScore, error)
	GetPageScores(ctx context.Context, auditID uuid.UUID) ([]PageScore, error)
	GetPrioritizedIssues(ctx context.Context, auditID uuid.UUID) ([]PrioritizedIssue, error)
	GetSiteKeywordSummary(ctx context.Context, auditID uuid.UUID) (*SiteKeywordSummary, error)
	GetNextVersion(ctx context.Context, auditID uuid.UUID) (int, error)
	SaveReport(ctx context.Context, report *AuditReport) error
	GetReportsByAuditID(ctx context.Context, auditID uuid.UUID) ([]AuditReport, error)
	GetLatestReportByAuditID(ctx context.Context, auditID uuid.UUID) (*AuditReport, error)
	GetReportByShareToken(ctx context.Context, shareToken string) (*AuditReport, error)
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

func (r *repository) GetAuditRecord(ctx context.Context, auditID uuid.UUID) (*AuditRecord, error) {
	var rec AuditRecord
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&rec).
			Where("a._id = ?", auditID).
			Scan(ctx)
	}, "Get Audit Record")
	if err != nil {
		return nil, fmt.Errorf("get audit record: %w", err)
	}
	return &rec, nil
}

func (r *repository) GetAuditScore(ctx context.Context, auditID uuid.UUID) (*AuditScore, error) {
	var score AuditScore
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&score).
			Where("asc.audit_id = ?", auditID).
			Scan(ctx)
	}, "Get Audit Score")
	if err != nil {
		return nil, fmt.Errorf("get audit score: %w", err)
	}
	return &score, nil
}

func (r *repository) GetPageScores(ctx context.Context, auditID uuid.UUID) ([]PageScore, error) {
	var pageScores []PageScore
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&pageScores).
			Where("psc.audit_id = ?", auditID).
			Order("psc.overall_score ASC").
			Scan(ctx)
	}, "Get Page Scores")
	if err != nil {
		return nil, fmt.Errorf("get page scores: %w", err)
	}
	return pageScores, nil
}

func (r *repository) GetPrioritizedIssues(ctx context.Context, auditID uuid.UUID) ([]PrioritizedIssue, error) {
	var issues []PrioritizedIssue
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&issues).
			Where("pi.audit_id = ?", auditID).
			Order("pi.priority_score DESC", "pi.impact_score DESC").
			Scan(ctx)
	}, "Get Prioritized Issues")
	if err != nil {
		return nil, fmt.Errorf("get prioritized issues: %w", err)
	}
	return issues, nil
}

func (r *repository) GetSiteKeywordSummary(ctx context.Context, auditID uuid.UUID) (*SiteKeywordSummary, error) {
	var sks SiteKeywordSummary
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&sks).
			Where("sks.audit_id = ?", auditID).
			Scan(ctx)
	}, "Get Site Keyword Summary")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No keywords summary is fine
		}
		return nil, fmt.Errorf("get site keyword summary: %w", err)
	}
	return &sks, nil
}

func (r *repository) GetNextVersion(ctx context.Context, auditID uuid.UUID) (int, error) {
	var maxVer sql.NullInt32
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model((*AuditReport)(nil)).
			ColumnExpr("MAX(version)").
			Where("audit_id = ?", auditID).
			Scan(ctx, &maxVer)
	}, "Get Next Report Version")
	if err != nil {
		return 1, nil
	}
	if !maxVer.Valid {
		return 1, nil
	}
	return int(maxVer.Int32) + 1, nil
}

func (r *repository) SaveReport(ctx context.Context, report *AuditReport) error {
	return r.executeWithRetry(ctx, func() error {
		_, err := r.db.NewInsert().
			Model(report).
			Exec(ctx)
		return err
	}, "Save Audit Report")
}

func (r *repository) GetReportsByAuditID(ctx context.Context, auditID uuid.UUID) ([]AuditReport, error) {
	var reports []AuditReport
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&reports).
			Where("ar.audit_id = ?", auditID).
			Order("ar.version DESC").
			Scan(ctx)
	}, "Get Reports By Audit ID")
	if err != nil {
		return nil, fmt.Errorf("get reports: %w", err)
	}
	return reports, nil
}

func (r *repository) GetLatestReportByAuditID(ctx context.Context, auditID uuid.UUID) (*AuditReport, error) {
	var report AuditReport
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&report).
			Where("ar.audit_id = ?", auditID).
			Order("ar.version DESC").
			Limit(1).
			Scan(ctx)
	}, "Get Latest Report By Audit ID")
	if err != nil {
		return nil, fmt.Errorf("get latest report: %w", err)
	}
	return &report, nil
}

func (r *repository) GetReportByShareToken(ctx context.Context, shareToken string) (*AuditReport, error) {
	var report AuditReport
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&report).
			Where("ar.share_token = ?", shareToken).
			Scan(ctx)
	}, "Get Report By Share Token")
	if err != nil {
		return nil, fmt.Errorf("get report by token: %w", err)
	}
	return &report, nil
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
