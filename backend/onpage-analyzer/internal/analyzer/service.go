package analyzer

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
	ExecuteAnalysis(ctx context.Context, event ParseCompletedEvent) error
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
//          ANALYSIS EXECUTION PIPELINE     //
//==========================================//

// ExecuteAnalysis reads parsed pages, runs the on-page rule suite, stores findings, and publishes completion event.
func (s *service) ExecuteAnalysis(ctx context.Context, event ParseCompletedEvent) error {
	startTime := time.Now()
	s.logger.Info("starting on-page analysis for audit",
		zap.String("audit_id", event.AuditID.String()),
		zap.String("domain", event.Domain),
	)

	// 1. Fetch parsed structured pages
	pages, err := s.repository.FindParsedPagesByAuditID(ctx, event.AuditID)
	if err != nil {
		return fmt.Errorf("failed to query parsed pages: %w", err)
	}

	if len(pages) == 0 {
		s.logger.Warn("no parsed pages found for audit — completing with 0 findings",
			zap.String("audit_id", event.AuditID.String()),
		)
		return s.publishAnalysisCompleted(ctx, event, 0, 0, 0, 0, 0, 0)
	}

	// 2. Run On-Page Rule Engine
	findings := AnalyzePages(pages)

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
		s.logger.Error("failed to save findings", zap.Error(err))
		return fmt.Errorf("failed to save findings: %w", err)
	}

	durationMs := time.Since(startTime).Milliseconds()

	s.logger.Info("on-page analysis finished",
		zap.String("audit_id", event.AuditID.String()),
		zap.Int("total_pages", len(pages)),
		zap.Int("total_findings", len(findings)),
		zap.Int("critical_count", criticalCount),
		zap.Int("warning_count", warningCount),
		zap.Int("info_count", infoCount),
		zap.Int64("duration_ms", durationMs),
	)

	// 5. Publish On-Page Analysis Completed Event to NATS
	return s.publishAnalysisCompleted(ctx, event, len(pages), len(findings), criticalCount, warningCount, infoCount, durationMs)
}

// GetFindingsByAuditID implements [Service].
func (s *service) GetFindingsByAuditID(ctx context.Context, auditID uuid.UUID) ([]Finding, error) {
	return s.repository.FindFindingsByAuditID(ctx, auditID)
}

//==========================================//
//             HELPER FUNCTIONS             //
//==========================================//

func (s *service) publishAnalysisCompleted(
	ctx context.Context,
	event ParseCompletedEvent,
	totalPages, totalFindings, critical, warning, info int,
	dur int64,
) error {
	evt := OnPageAnalysisCompletedEvent{
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
		return fmt.Errorf("marshal analysis completed event: %w", err)
	}

	msgID := fmt.Sprintf("audit:%s:analysis_onpage_completed", event.AuditID)
	_, err = s.js.Publish(
		"audit.events.analysis.onpage.completed",
		payload,
		nats.MsgId(msgID),
		nats.Context(ctx),
	)
	if err != nil {
		s.logger.Error("failed to publish on-page analysis completed event", zap.Error(err))
		return err
	}

	s.logger.Info("published analysis.onpage.completed event", zap.String("audit_id", event.AuditID.String()))
	return nil
}
