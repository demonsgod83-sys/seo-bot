package crawler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

type Repository interface {
	SaveCrawlPage(ctx context.Context, page *CrawlPage) error
	SaveCrawlSummary(ctx context.Context, summary *CrawlSummary) error
	FindPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]CrawlPage, error)
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

// SaveCrawlPage implements [Repository].
func (r *repository) SaveCrawlPage(ctx context.Context, page *CrawlPage) error {
	return r.executeWithRetry(ctx, func() error {
		_, err := r.db.NewInsert().
			Model(page).
			Returning("_id, fetched_at").
			Exec(ctx)
		return err
	}, "Save Crawl Page")
}

// SaveCrawlSummary implements [Repository].
func (r *repository) SaveCrawlSummary(ctx context.Context, summary *CrawlSummary) error {
	return r.executeWithRetry(ctx, func() error {
		_, err := r.db.NewInsert().
			Model(summary).
			On("CONFLICT (audit_id) DO UPDATE").
			Set("total_discovered = EXCLUDED.total_discovered").
			Set("total_fetched = EXCLUDED.total_fetched").
			Set("total_errors = EXCLUDED.total_errors").
			Set("duration_ms = EXCLUDED.duration_ms").
			Set("completed_at = EXCLUDED.completed_at").
			Exec(ctx)
		return err
	}, "Save Crawl Summary")
}

// FindPagesByAuditID implements [Repository].
func (r *repository) FindPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]CrawlPage, error) {
	var pages []CrawlPage
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&pages).
			Where("cp.audit_id = ?", auditID).
			Order("cp.fetched_at ASC").
			Scan(ctx)
	}, "Find Pages By Audit ID")

	if err != nil {
		return nil, fmt.Errorf("failed to query crawl pages: %w", err)
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
