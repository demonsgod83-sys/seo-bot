package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const (
	SubjectNotificationCompleted = "audit.events.notification.completed"
)

type Service interface {
	HandleReportGenerated(ctx context.Context, msg *nats.Msg) error
	HandleAuditFailed(ctx context.Context, msg *nats.Msg) error
	GetDeliveries(ctx context.Context, auditID uuid.UUID) (NotificationDeliveriesResponse, error)
	SetTenantConfig(ctx context.Context, req SetTenantConfigRequest) error
}

type service struct {
	repository         Repository
	emailSender        EmailSender
	webhookSender      WebhookSender
	js                 nats.JetStreamContext
	defaultNotifyEmail string
	logger             *zap.Logger
}

func NewService(
	repository Repository,
	emailSender EmailSender,
	webhookSender WebhookSender,
	js nats.JetStreamContext,
	defaultNotifyEmail string,
	logger *zap.Logger,
) Service {
	return &service{
		repository:         repository,
		emailSender:        emailSender,
		webhookSender:      webhookSender,
		js:                 js,
		defaultNotifyEmail: defaultNotifyEmail,
		logger:             logger,
	}
}

// HandleReportGenerated processes completion notification for successful audits.
func (s *service) HandleReportGenerated(ctx context.Context, msg *nats.Msg) error {
	var event ReportGeneratedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		s.logger.Error("report_generated: malformed payload — acking", zap.Error(err))
		return nil
	}

	s.logger.Info("report_generated received — processing notifications",
		zap.String("audit_id", event.AuditID.String()),
		zap.String("domain", event.Domain),
	)

	// 1. Fetch tenant config or fallback
	tenantCfg, _ := s.repository.GetTenantConfig(ctx, event.TenantID)

	targetEmail := s.defaultNotifyEmail
	emailEnabled := true
	webhookEnabled := false
	var webhookURL, webhookSecret string

	if tenantCfg != nil {
		if tenantCfg.Email != "" {
			targetEmail = tenantCfg.Email
		}
		emailEnabled = tenantCfg.IsEmailEnabled
		webhookEnabled = tenantCfg.IsWebhookEnabled && tenantCfg.WebhookURL != ""
		webhookURL = tenantCfg.WebhookURL
		webhookSecret = tenantCfg.WebhookSecret
	}

	// 2. Fetch score and top prioritized issues for the executive summary
	auditScore, _ := s.repository.GetAuditScore(ctx, event.AuditID)
	topIssues, _ := s.repository.GetTopPrioritizedIssues(ctx, event.AuditID, 3)

	var techScore, onpageScore, contentScore int
	if auditScore != nil {
		techScore = auditScore.TechnicalScore
		onpageScore = auditScore.OnPageScore
		contentScore = auditScore.ContentScore
	}

	emailStatus := "skipped"
	webhookStatus := "skipped"

	// 3. Email Delivery
	if emailEnabled && targetEmail != "" {
		emailStatus = s.deliverEmailWithRetry(ctx, event.AuditID, event.TenantID, targetEmail, event.Domain,
			event.OverallScore, techScore, onpageScore, contentScore, topIssues, event.ShareURL)
	}

	// 4. Webhook Delivery
	if webhookEnabled && webhookURL != "" {
		payload := WebhookPayload{
			Event:          "audit.completed",
			AuditID:        event.AuditID,
			TenantID:       event.TenantID,
			Domain:         event.Domain,
			Status:         "completed",
			OverallScore:   event.OverallScore,
			TechnicalScore: techScore,
			OnPageScore:    onpageScore,
			ContentScore:   contentScore,
			ReportURL:      event.ShareURL,
			PdfURL:         event.ShareURL + "/pdf",
			JsonURL:        event.ShareURL + "/json",
			Timestamp:      time.Now().UTC(),
		}
		if event.HasWarnings {
			payload.Status = "completed_with_warnings"
		}
		webhookStatus = s.deliverWebhookWithRetry(ctx, event.AuditID, event.TenantID, webhookURL, webhookSecret, payload)
	}

	// 5. Publish notification.completed event
	s.publishNotificationCompleted(ctx, event.AuditID, event.TenantID, event.Domain, emailStatus, webhookStatus)

	return nil
}

// HandleAuditFailed processes failure notification for aborted audits.
func (s *service) HandleAuditFailed(ctx context.Context, msg *nats.Msg) error {
	var event AuditFailedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return nil
	}

	s.logger.Warn("audit failed event received — processing failure notifications",
		zap.String("audit_id", event.AuditID.String()),
		zap.String("domain", event.Domain),
		zap.String("reason", event.Reason),
	)

	tenantCfg, _ := s.repository.GetTenantConfig(ctx, event.TenantID)

	targetEmail := s.defaultNotifyEmail
	emailEnabled := true
	webhookEnabled := false
	var webhookURL, webhookSecret string

	if tenantCfg != nil {
		if tenantCfg.Email != "" {
			targetEmail = tenantCfg.Email
		}
		emailEnabled = tenantCfg.IsEmailEnabled
		webhookEnabled = tenantCfg.IsWebhookEnabled && tenantCfg.WebhookURL != ""
		webhookURL = tenantCfg.WebhookURL
		webhookSecret = tenantCfg.WebhookSecret
	}

	emailStatus := "skipped"
	webhookStatus := "skipped"

	if emailEnabled && targetEmail != "" {
		emailStatus = s.deliverFailedEmailWithRetry(ctx, event.AuditID, event.TenantID, targetEmail, event.Domain, event.Reason)
	}

	if webhookEnabled && webhookURL != "" {
		payload := WebhookPayload{
			Event:         "audit.failed",
			AuditID:       event.AuditID,
			TenantID:      event.TenantID,
			Domain:        event.Domain,
			Status:        "failed",
			FailureReason: event.Reason,
			Timestamp:     time.Now().UTC(),
		}
		webhookStatus = s.deliverWebhookWithRetry(ctx, event.AuditID, event.TenantID, webhookURL, webhookSecret, payload)
	}

	s.publishNotificationCompleted(ctx, event.AuditID, event.TenantID, event.Domain, emailStatus, webhookStatus)
	return nil
}

// deliverEmailWithRetry delivers audit completed email with idempotency and retry.
func (s *service) deliverEmailWithRetry(
	ctx context.Context,
	auditID, tenantID uuid.UUID,
	email, domain string,
	overallScore, techScore, onpageScore, contentScore int,
	topIssues []PrioritizedIssue,
	shareURL string,
) string {
	// Idempotency check: if already delivered, do not duplicate
	existing, _ := s.repository.FindDelivery(ctx, auditID, "email")
	if existing != nil && existing.Status == "delivered" {
		s.logger.Info("email already delivered for audit (idempotent skip)", zap.String("audit_id", auditID.String()))
		return "delivered"
	}

	delivery := &NotificationDelivery{
		AuditID:       auditID,
		TenantID:      tenantID,
		Channel:       "email",
		Recipient:     email,
		Status:        "pending",
		AttemptsCount: 0,
		Payload: map[string]any{
			"domain":        domain,
			"overall_score": overallScore,
			"share_url":     shareURL,
		},
	}
	_ = s.repository.SaveDelivery(ctx, delivery)

	var lastErr error
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		delivery.AttemptsCount = attempt
		err := s.emailSender.SendAuditCompletedEmail(
			email, domain, overallScore, techScore, onpageScore, contentScore, topIssues, shareURL,
		)
		if err == nil {
			now := time.Now().UTC()
			delivery.Status = "delivered"
			delivery.DeliveredAt = &now
			delivery.LastError = ""
			_ = s.repository.UpdateDelivery(ctx, delivery)
			return "delivered"
		}

		lastErr = err
		s.logger.Warn("email delivery attempt failed",
			zap.Int("attempt", attempt),
			zap.String("to", email),
			zap.Error(err),
		)

		if attempt < maxRetries {
			select {
			case <-time.After(time.Duration(attempt) * time.Second):
			case <-ctx.Done():
				return "failed"
			}
		}
	}

	delivery.Status = "failed"
	if lastErr != nil {
		delivery.LastError = lastErr.Error()
	}
	_ = s.repository.UpdateDelivery(ctx, delivery)
	return "failed"
}

// deliverFailedEmailWithRetry delivers failure notification email.
func (s *service) deliverFailedEmailWithRetry(
	ctx context.Context,
	auditID, tenantID uuid.UUID,
	email, domain, reason string,
) string {
	existing, _ := s.repository.FindDelivery(ctx, auditID, "email")
	if existing != nil && existing.Status == "delivered" {
		return "delivered"
	}

	delivery := &NotificationDelivery{
		AuditID:       auditID,
		TenantID:      tenantID,
		Channel:       "email",
		Recipient:     email,
		Status:        "pending",
		AttemptsCount: 0,
		Payload: map[string]any{
			"domain": domain,
			"reason": reason,
		},
	}
	_ = s.repository.SaveDelivery(ctx, delivery)

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		delivery.AttemptsCount = attempt
		err := s.emailSender.SendAuditFailedEmail(email, domain, reason)
		if err == nil {
			now := time.Now().UTC()
			delivery.Status = "delivered"
			delivery.DeliveredAt = &now
			_ = s.repository.UpdateDelivery(ctx, delivery)
			return "delivered"
		}
		lastErr = err
	}

	delivery.Status = "failed"
	if lastErr != nil {
		delivery.LastError = lastErr.Error()
	}
	_ = s.repository.UpdateDelivery(ctx, delivery)
	return "failed"
}

// deliverWebhookWithRetry delivers webhook payload with HMAC signature and retry.
func (s *service) deliverWebhookWithRetry(
	ctx context.Context,
	auditID, tenantID uuid.UUID,
	webhookURL, webhookSecret string,
	payload WebhookPayload,
) string {
	existing, _ := s.repository.FindDelivery(ctx, auditID, "webhook")
	if existing != nil && existing.Status == "delivered" {
		s.logger.Info("webhook already delivered for audit (idempotent skip)", zap.String("audit_id", auditID.String()))
		return "delivered"
	}

	delivery := &NotificationDelivery{
		AuditID:       auditID,
		TenantID:      tenantID,
		Channel:       "webhook",
		Recipient:     webhookURL,
		Status:        "pending",
		AttemptsCount: 0,
		Payload: map[string]any{
			"event": payload.Event,
			"url":   webhookURL,
		},
	}
	_ = s.repository.SaveDelivery(ctx, delivery)

	var lastErr error
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		delivery.AttemptsCount = attempt
		err := s.webhookSender.SendWebhook(ctx, webhookURL, webhookSecret, payload)
		if err == nil {
			now := time.Now().UTC()
			delivery.Status = "delivered"
			delivery.DeliveredAt = &now
			delivery.LastError = ""
			_ = s.repository.UpdateDelivery(ctx, delivery)
			return "delivered"
		}

		lastErr = err
		s.logger.Warn("webhook delivery attempt failed",
			zap.Int("attempt", attempt),
			zap.String("url", webhookURL),
			zap.Error(err),
		)

		if attempt < maxRetries {
			select {
			case <-time.After(time.Duration(attempt) * time.Second):
			case <-ctx.Done():
				return "failed"
			}
		}
	}

	delivery.Status = "failed"
	if lastErr != nil {
		delivery.LastError = lastErr.Error()
	}
	_ = s.repository.UpdateDelivery(ctx, delivery)
	return "failed"
}

func (s *service) publishNotificationCompleted(ctx context.Context, auditID, tenantID uuid.UUID, domain, emailStatus, webhookStatus string) {
	evt := NotificationCompletedEvent{
		AuditID:       auditID,
		TenantID:      tenantID,
		Domain:        domain,
		EmailStatus:   emailStatus,
		WebhookStatus: webhookStatus,
		CompletedAt:   time.Now().UTC(),
	}
	payload, _ := json.Marshal(evt)
	msgID := fmt.Sprintf("audit:%s:notification_completed", auditID)
	_, _ = s.js.Publish(SubjectNotificationCompleted, payload, nats.MsgId(msgID), nats.Context(ctx))
}

func (s *service) GetDeliveries(ctx context.Context, auditID uuid.UUID) (NotificationDeliveriesResponse, error) {
	deliveries, err := s.repository.GetDeliveriesByAuditID(ctx, auditID)
	if err != nil {
		return NotificationDeliveriesResponse{}, err
	}
	return NotificationDeliveriesResponse{
		AuditID:    auditID,
		Deliveries: deliveries,
	}, nil
}

func (s *service) SetTenantConfig(ctx context.Context, req SetTenantConfigRequest) error {
	cfg := &TenantNotificationConfig{
		TenantID:         req.TenantID,
		Email:            req.Email,
		WebhookURL:       req.WebhookURL,
		WebhookSecret:    req.WebhookSecret,
		IsEmailEnabled:   req.IsEmailEnabled,
		IsWebhookEnabled: req.IsWebhookEnabled,
	}
	return s.repository.SaveTenantConfig(ctx, cfg)
}
