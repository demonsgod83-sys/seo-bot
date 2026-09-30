package scoring

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
	SubjectScoreComputed = "audit.events.score.computed"
)

type Service interface {
	HandleStartScoring(ctx context.Context, msg *nats.Msg) error
	GetAuditScores(ctx context.Context, auditID uuid.UUID) (AuditScoresResponse, error)
}

type service struct {
	repository Repository
	engine     *Engine
	js         nats.JetStreamContext
	logger     *zap.Logger
}

func NewService(repository Repository, engine *Engine, js nats.JetStreamContext, logger *zap.Logger) Service {
	return &service{
		repository: repository,
		engine:     engine,
		js:         js,
		logger:     logger,
	}
}

// HandleStartScoring executes scoring and impact prioritization after all analyzers complete.
func (s *service) HandleStartScoring(ctx context.Context, msg *nats.Msg) error {
	start := time.Now()

	var event StartScoringEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		s.logger.Error("start_scoring: malformed payload — acking", zap.Error(err))
		return nil
	}

	s.logger.Info("start scoring request received",
		zap.String("audit_id", event.AuditID.String()),
		zap.String("domain", event.Domain),
	)

	// 1. Fetch all findings across all analyzers
	findings, err := s.repository.FindFindingsByAuditID(ctx, event.AuditID)
	if err != nil {
		return fmt.Errorf("fetch findings: %w", err)
	}

	// 2. Fetch parsed pages inventory
	pages, err := s.repository.FindParsedPagesByAuditID(ctx, event.AuditID)
	if err != nil {
		return fmt.Errorf("fetch parsed pages: %w", err)
	}

	// 3. Execute Scoring Engine
	result := s.engine.Compute(event.AuditID, event.TenantID, event.Domain, pages, findings)

	// 4. Save calculation results to database
	if err := s.repository.SaveCalculationResult(ctx, result); err != nil {
		return fmt.Errorf("save calculation result: %w", err)
	}

	duration := time.Since(start)

	s.logger.Info("scoring completed successfully",
		zap.String("audit_id", event.AuditID.String()),
		zap.Int("overall_score", result.AuditScore.OverallScore),
		zap.Int("technical_score", result.AuditScore.TechnicalScore),
		zap.Int("onpage_score", result.AuditScore.OnPageScore),
		zap.Int("content_score", result.AuditScore.ContentScore),
		zap.Int("total_findings", result.AuditScore.TotalFindings),
		zap.Int("prioritized_issues", len(result.PrioritizedIssues)),
		zap.Duration("duration", duration),
	)

	// 5. Publish score.computed event
	completedEvent := ScoringCompletedEvent{
		AuditID:        event.AuditID,
		TenantID:       event.TenantID,
		Domain:         event.Domain,
		OverallScore:   result.AuditScore.OverallScore,
		TechnicalScore: result.AuditScore.TechnicalScore,
		OnPageScore:    result.AuditScore.OnPageScore,
		ContentScore:   result.AuditScore.ContentScore,
		TotalFindings:  result.AuditScore.TotalFindings,
		CriticalCount:  result.AuditScore.CriticalCount,
		WarningCount:   result.AuditScore.WarningCount,
		InfoCount:      result.AuditScore.InfoCount,
		IssuesCount:    len(result.PrioritizedIssues),
		DurationMs:     duration.Milliseconds(),
	}

	payload, err := json.Marshal(completedEvent)
	if err != nil {
		s.logger.Error("failed to marshal score.computed event", zap.Error(err))
		return nil
	}

	msgID := fmt.Sprintf("audit:%s:score_computed", event.AuditID)
	_, err = s.js.Publish(
		SubjectScoreComputed,
		payload,
		nats.MsgId(msgID),
		nats.Context(ctx),
	)
	if err != nil {
		s.logger.Warn("failed to publish score.computed event", zap.Error(err))
	}

	return nil
}

// GetAuditScores returns comprehensive scoring report for an audit.
func (s *service) GetAuditScores(ctx context.Context, auditID uuid.UUID) (AuditScoresResponse, error) {
	score, pageScores, issues, err := s.repository.GetAuditScores(ctx, auditID)
	if err != nil {
		return AuditScoresResponse{}, err
	}

	var pScoresResp []PageScoreResponse
	for _, ps := range pageScores {
		pScoresResp = append(pScoresResp, PageScoreResponse{
			PageURL:        ps.PageURL,
			OverallScore:   ps.OverallScore,
			TechnicalScore: ps.TechnicalScore,
			OnPageScore:    ps.OnPageScore,
			ContentScore:   ps.ContentScore,
			CriticalCount:  ps.CriticalCount,
			WarningCount:   ps.WarningCount,
			InfoCount:      ps.InfoCount,
			CalculatedAt:   ps.CalculatedAt,
		})
	}

	var issuesResp []PrioritizedIssueResponse
	for _, is := range issues {
		issuesResp = append(issuesResp, PrioritizedIssueResponse{
			RuleID:             is.RuleID,
			Category:           is.Category,
			Severity:           is.Severity,
			Title:              is.Title,
			Message:            is.Message,
			EffortTier:         is.EffortTier,
			ImpactScore:        is.ImpactScore,
			PriorityScore:      is.PriorityScore,
			AffectedPagesCount: is.AffectedPagesCount,
			AffectedURLs:       is.AffectedURLs,
		})
	}

	return AuditScoresResponse{
		AuditID:           score.AuditID,
		TenantID:          score.TenantID,
		OverallScore:      score.OverallScore,
		TechnicalScore:    score.TechnicalScore,
		OnPageScore:       score.OnPageScore,
		ContentScore:      score.ContentScore,
		TotalFindings:     score.TotalFindings,
		CriticalCount:     score.CriticalCount,
		WarningCount:      score.WarningCount,
		InfoCount:         score.InfoCount,
		TotalPagesScored:  score.TotalPagesScored,
		CalculatedAt:      score.CalculatedAt,
		PageScores:        pScoresResp,
		PrioritizedIssues: issuesResp,
	}, nil
}
