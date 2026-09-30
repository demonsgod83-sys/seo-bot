package parser

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

type Repository interface {
	FindCrawlPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]CrawlPageReference, error)
	SaveParsedPage(ctx context.Context, page *ParsedPage) error
	FindParsedPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]ParsedPage, error)
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

// FindCrawlPagesByAuditID retrieves the list of raw crawl pages from the database.
func (r *repository) FindCrawlPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]CrawlPageReference, error) {
	var pages []CrawlPageReference
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&pages).
			Table("crawl_pages").
			Where("audit_id = ?", auditID).
			Order("fetched_at ASC").
			Scan(ctx)
	}, "Find Crawl Pages By Audit ID")

	if err != nil {
		return nil, fmt.Errorf("failed to query crawl pages for audit %s: %w", auditID, err)
	}
	return pages, nil
}

// SaveParsedPage inserts or updates the structured page facts for an audit.
func (r *repository) SaveParsedPage(ctx context.Context, page *ParsedPage) error {
	return r.executeWithRetry(ctx, func() error {
		_, err := r.db.NewInsert().
			Model(page).
			Returning("_id, parsed_at").
			Exec(ctx)
		return err
	}, "Save Parsed Page")
}

// FindParsedPagesByAuditID retrieves all parsed structured records for an audit.
func (r *repository) FindParsedPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]ParsedPage, error) {
	var pages []ParsedPage
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&pages).
			Where("pp.audit_id = ?", auditID).
			Order("pp.parsed_at ASC").
			Scan(ctx)
	}, "Find Parsed Pages By Audit ID")

	if err != nil {
		return nil, fmt.Errorf("failed to query parsed pages: %w", err)
	}
	return pages, nil
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
