package auditor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

//==========================================//
//              SERVICE INTERFACE           //
//==========================================//

type Service interface {
	ExecuteAudit(ctx context.Context, event ParseCompletedEvent) error
	GetFindingsByAuditID(ctx context.Context, auditID uuid.UUID) ([]Finding, error)
}

type service struct {
	repository Repository
	js         nats.JetStreamContext
	logger     *zap.Logger
}

func NewService(repository Repository, js nats.JetStreamContext, logger *zap.Logger) Service {
	return &service{
		repository: repository,
		js:         js,
		logger:     logger,
	}
}

//==========================================//
//          AUDIT EXECUTION PIPELINE        //
//==========================================//

// ExecuteAudit reads parsed pages, runs technical SEO checks, stores findings, and publishes completion event.
func (s *service) ExecuteAudit(ctx context.Context, event ParseCompletedEvent) error {
	startTime := time.Now()
	s.logger.Info("starting technical audit",
		zap.String("audit_id", event.AuditID.String()),
		zap.String("domain", event.Domain),
	)

	// 1. Fetch parsed pages
	pages, err := s.repository.FindParsedPagesByAuditID(ctx, event.AuditID)
	if err != nil {
		return fmt.Errorf("failed to query parsed pages: %w", err)
	}

	if len(pages) == 0 {
		s.logger.Warn("no parsed pages found — completing technical audit with 0 findings",
			zap.String("audit_id", event.AuditID.String()),
		)
		return s.publishAuditCompleted(ctx, event, 0, 0, 0, 0, 0, 0)
	}

	// 2. Run Technical Audit Rules
	findings := AuditPages(pages)

	// 3. Count Severities
	var criticalCount, warningCount, infoCount int
	for _, finding := range findings {
		switch finding.Severity {
		case SeverityCritical:
			criticalCount++
		case SeverityWarning:
			warningCount++
		case SeverityInfo:
			infoCount++
		}
	}

	// 4. Save findings to database
	if err := s.repository.SaveFindings(ctx, findings); err != nil {
		s.logger.Error("failed to save technical findings", zap.Error(err))
		return fmt.Errorf("failed to save technical findings: %w", err)
	}

	durationMs := time.Since(startTime).Milliseconds()

	s.logger.Info("technical audit finished",
		zap.String("audit_id", event.AuditID.String()),
		zap.Int("total_pages", len(pages)),
		zap.Int("total_findings", len(findings)),
		zap.Int("critical_count", criticalCount),
		zap.Int("warning_count", warningCount),
		zap.Int("info_count", infoCount),
		zap.Int64("duration_ms", durationMs),
	)

	// 5. Publish Technical Analysis Completed Event to NATS
	return s.publishAuditCompleted(ctx, event, len(pages), len(findings), criticalCount, warningCount, infoCount, durationMs)
}

// GetFindingsByAuditID implements [Service].
func (s *service) GetFindingsByAuditID(ctx context.Context, auditID uuid.UUID) ([]Finding, error) {
	return s.repository.FindFindingsByAuditID(ctx, auditID)
}

//==========================================//
//             HELPER FUNCTIONS             //
//==========================================//

func (s *service) publishAuditCompleted(
	ctx context.Context,
	event ParseCompletedEvent,
	totalPages, totalFindings, critical, warning, info int,
	dur int64,
) error {
	evt := TechnicalAnalysisCompletedEvent{
		AuditID:       event.AuditID,
		TenantID:      event.TenantID,
		Domain:        event.Domain,
		TotalPages:    totalPages,
		TotalFindings: totalFindings,
		CriticalCount: critical,
		WarningCount:  warning,
		InfoCount:     info,
		DurationMs:    dur,
		CompletedAt:   time.Now().UTC(),
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal technical audit completed event: %w", err)
	}

	msgID := fmt.Sprintf("audit:%s:analysis_technical_completed", event.AuditID)
	_, err = s.js.Publish(
		"audit.events.analysis.technical.completed",
		payload,
		nats.MsgId(msgID),
		nats.Context(ctx),
	)
	if err != nil {
		s.logger.Error("failed to publish technical audit completed event", zap.Error(err))
		return err
	}

	s.logger.Info("published analysis.technical.completed event", zap.String("audit_id", event.AuditID.String()))
	return nil
}
