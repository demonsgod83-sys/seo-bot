package notification

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
	GetTenantConfig(ctx context.Context, tenantID uuid.UUID) (*TenantNotificationConfig, error)
	SaveTenantConfig(ctx context.Context, cfg *TenantNotificationConfig) error
	FindDelivery(ctx context.Context, auditID uuid.UUID, channel string) (*NotificationDelivery, error)
	SaveDelivery(ctx context.Context, delivery *NotificationDelivery) error
	UpdateDelivery(ctx context.Context, delivery *NotificationDelivery) error
	GetDeliveriesByAuditID(ctx context.Context, auditID uuid.UUID) ([]NotificationDelivery, error)
	GetAuditScore(ctx context.Context, auditID uuid.UUID) (*AuditScore, error)
	GetTopPrioritizedIssues(ctx context.Context, auditID uuid.UUID, limit int) ([]PrioritizedIssue, error)
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

func (r *repository) GetTenantConfig(ctx context.Context, tenantID uuid.UUID) (*TenantNotificationConfig, error) {
	var cfg TenantNotificationConfig
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&cfg).
			Where("tnc.tenant_id = ?", tenantID).
			Scan(ctx)
	}, "Get Tenant Notification Config")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No custom config set for tenant
		}
		return nil, fmt.Errorf("get tenant config: %w", err)
	}
	return &cfg, nil
}

func (r *repository) SaveTenantConfig(ctx context.Context, cfg *TenantNotificationConfig) error {
	return r.executeWithRetry(ctx, func() error {
		_, err := r.db.NewInsert().
			Model(cfg).
			On("CONFLICT (tenant_id) DO UPDATE").
			Set("email = EXCLUDED.email").
			Set("webhook_url = EXCLUDED.webhook_url").
			Set("webhook_secret = EXCLUDED.webhook_secret").
			Set("is_email_enabled = EXCLUDED.is_email_enabled").
			Set("is_webhook_enabled = EXCLUDED.is_webhook_enabled").
			Set("updated_at = current_timestamp").
			Exec(ctx)
		return err
	}, "Save Tenant Notification Config")
}

func (r *repository) FindDelivery(ctx context.Context, auditID uuid.UUID, channel string) (*NotificationDelivery, error) {
	var delivery NotificationDelivery
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&delivery).
			Where("nd.audit_id = ? AND nd.channel = ?", auditID, channel).
			Scan(ctx)
	}, "Find Notification Delivery")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find delivery: %w", err)
	}
	return &delivery, nil
}

func (r *repository) SaveDelivery(ctx context.Context, delivery *NotificationDelivery) error {
	return r.executeWithRetry(ctx, func() error {
		_, err := r.db.NewInsert().
			Model(delivery).
			Exec(ctx)
		return err
	}, "Save Notification Delivery")
}

func (r *repository) UpdateDelivery(ctx context.Context, delivery *NotificationDelivery) error {
	return r.executeWithRetry(ctx, func() error {
		_, err := r.db.NewUpdate().
			Model(delivery).
			WherePK().
			Exec(ctx)
		return err
	}, "Update Notification Delivery")
}

func (r *repository) GetDeliveriesByAuditID(ctx context.Context, auditID uuid.UUID) ([]NotificationDelivery, error) {
	var deliveries []NotificationDelivery
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&deliveries).
			Where("nd.audit_id = ?", auditID).
			Order("nd.created_at ASC").
			Scan(ctx)
	}, "Get Deliveries By Audit ID")
	if err != nil {
		return nil, fmt.Errorf("get deliveries: %w", err)
	}
	return deliveries, nil
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get audit score: %w", err)
	}
	return &score, nil
}

func (r *repository) GetTopPrioritizedIssues(ctx context.Context, auditID uuid.UUID, limit int) ([]PrioritizedIssue, error) {
	var issues []PrioritizedIssue
	err := r.executeWithRetry(ctx, func() error {
		return r.db.NewSelect().
			Model(&issues).
			Where("pi.audit_id = ?", auditID).
			Order("pi.priority_score DESC", "pi.impact_score DESC").
			Limit(limit).
			Scan(ctx)
	}, "Get Top Prioritized Issues")
	if err != nil {
		return nil, fmt.Errorf("get top issues: %w", err)
	}
	return issues, nil
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
