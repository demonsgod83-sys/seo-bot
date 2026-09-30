package keywords

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
	FindParsedPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]ParsedPageRecord, error)
	SavePageKeywords(ctx context.Context, pageKeywords []PageKeyword) error
	SaveSiteSummary(ctx context.Context, summary *SiteKeywordSummary) error
	SaveFindings(ctx context.Context, findings []Finding) error
	GetPageKeywordsByAuditID(ctx context.Context, auditID uuid.UUID) ([]PageKeyword, error)
	GetSiteKeywordSummary(ctx context.Context, auditID uuid.UUID) (*SiteKeywordSummary, error)
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

// FindParsedPagesByAuditID queries structured page records from parsed_pages table.
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
		return nil, fmt.Errorf("failed to query parsed pages: %w", err)
	}
	return pages, nil
}

// SavePageKeywords batch inserts the per-page target keyword profiles.
func (r *repository) SavePageKeywords(ctx context.Context, pageKeywords []PageKeyword) error {
	if len(pageKeywords) == 0 {
		return nil
	}

	return r.executeWithRetry(ctx, func() error {
		_, err := r.db.NewInsert().
			Model(&pageKeywords).
			Exec(ctx)
		return err
	}, "Save Page Keywords")
}

// SaveSiteSummary stores the site-wide keyword aggregation map.
func (r *repository) SaveSiteSummary(ctx context.Context, summary *SiteKeywordSummary) error {
	return r.executeWithRetry(ctx, func() error {
		_, err := r.db.NewInsert().
			Model(summary).
			On("CONFLICT (audit_id) DO UPDATE").
			Set("total_keywords = EXCLUDED.total_keywords").
			Set("keyword_map = EXCLUDED.keyword_map").
			Set("cannibalizations = EXCLUDED.cannibalizations").
			Set("completed_at = EXCLUDED.completed_at").
			Exec(ctx)
		return err
	}, "Save Site Keyword Summary")
}

// SaveFindings batch inserts keyword findings (e.g. cannibalization) into audit_findings.
func (r *repository) SaveFindings(ctx context.Context, findings []Finding) error {
	if len(findings) == 0 {
		return nil
	}

	return r.executeWithRetry(ctx, func() error {
		_, err := r.db.NewInsert().
			Model(&findings).
			Exec(ctx)
		return err
	}, "Save Keyword Findings")
}

// GetPageKeywordsByAuditID queries keyword profiles for an audit.
func (r *repository) GetPageKeywordsByAuditID(ctx context.Context, auditID uuid.UUID) ([]PageKeyword, error) {
	var pageKeywords []PageKeyword
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&pageKeywords).
			Where("pk.audit_id = ?", auditID).
			Order("pk.created_at ASC").
			Scan(ctx)
	}, "Get Page Keywords")

	if err != nil {
		return nil, err
	}
	return pageKeywords, nil
}

// GetSiteKeywordSummary queries the aggregated keyword map.
func (r *repository) GetSiteKeywordSummary(ctx context.Context, auditID uuid.UUID) (*SiteKeywordSummary, error) {
	summary := new(SiteKeywordSummary)
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(summary).
			Where("sks.audit_id = ?", auditID).
			Scan(ctx)
	}, "Get Site Keyword Summary")

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return summary, nil
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
