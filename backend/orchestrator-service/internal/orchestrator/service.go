package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// Required parallel analyzers for the analyzing stage
var requiredAnalyzers = []string{"onpage", "technical", "keywords"}

type Service interface {
	HandleAuditValidated(ctx context.Context, msg *nats.Msg) error
	HandleCrawlCompleted(ctx context.Context, msg *nats.Msg) error
	HandleCrawlFailed(ctx context.Context, msg *nats.Msg) error
	HandleParseCompleted(ctx context.Context, msg *nats.Msg) error
	HandleParseFailed(ctx context.Context, msg *nats.Msg) error
	HandleOnPageAnalysisCompleted(ctx context.Context, msg *nats.Msg) error
	HandleTechnicalAnalysisCompleted(ctx context.Context, msg *nats.Msg) error
	HandleKeywordsExtracted(ctx context.Context, msg *nats.Msg) error
	HandleScoreComputed(ctx context.Context, msg *nats.Msg) error
	HandleReportGenerated(ctx context.Context, msg *nats.Msg) error
	GetAuditStatus(ctx context.Context, auditID uuid.UUID) (AuditStatusResponse, error)
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
//             LIFECYCLE HANDLERS           //
//==========================================//

// HandleAuditValidated handles "audit.validated", transitioning queued → crawling.
func (s *service) HandleAuditValidated(ctx context.Context, msg *nats.Msg) error {
	var event AuditValidatedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		s.logger.Error("audit.validated: malformed payload — acking", zap.Error(err))
		return nil
	}

	idempotencyKey := msg.Header.Get(nats.MsgIdHdr)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("audit:%s:validated", event.AuditID)
	}

	s.logger.Info("audit.validated received",
		zap.String("audit_id", event.AuditID.String()),
		zap.String("domain", event.Domain),
	)

	err := s.repository.TransitionStatus(ctx, event.AuditID, StatusQueued, StatusCrawling, idempotencyKey)
	if err != nil {
		if errors.Is(err, ErrAlreadyProcessed) {
			return nil
		}
		if errors.Is(err, ErrStatusMismatch) {
			s.logger.Warn("audit.validated: status mismatch", zap.String("audit_id", event.AuditID.String()))
			return nil
		}
		return fmt.Errorf("HandleAuditValidated: transition failed: %w", err)
	}

	s.logger.Info("audit transitioned queued → crawling", zap.String("audit_id", event.AuditID.String()))
	_ = s.publishStartCrawl(ctx, event)
	return nil
}

// HandleCrawlCompleted handles "audit.events.crawl_completed", transitioning crawling → parsing.
func (s *service) HandleCrawlCompleted(ctx context.Context, msg *nats.Msg) error {
	var event struct {
		AuditID      uuid.UUID `json:"audit_id"`
		TenantID     uuid.UUID `json:"tenant_id"`
		Domain       string    `json:"domain"`
		TotalFetched int       `json:"total_fetched"`
	}

	if err := json.Unmarshal(msg.Data, &event); err != nil {
		s.logger.Error("crawl_completed: malformed payload — acking", zap.Error(err))
		return nil
	}

	idempotencyKey := msg.Header.Get(nats.MsgIdHdr)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("audit:%s:crawl_completed", event.AuditID)
	}

	s.logger.Info("audit.events.crawl_completed received",
		zap.String("audit_id", event.AuditID.String()),
		zap.Int("pages_fetched", event.TotalFetched),
	)

	err := s.repository.TransitionStatus(ctx, event.AuditID, StatusCrawling, StatusParsing, idempotencyKey)
	if err != nil {
		if errors.Is(err, ErrAlreadyProcessed) {
			return nil
		}
		if errors.Is(err, ErrStatusMismatch) {
			s.logger.Warn("crawl_completed: status mismatch", zap.String("audit_id", event.AuditID.String()))
			return nil
		}
		return fmt.Errorf("HandleCrawlCompleted: transition failed: %w", err)
	}

	s.logger.Info("audit transitioned crawling → parsing", zap.String("audit_id", event.AuditID.String()))
	_ = s.publishStartParse(ctx, event.AuditID, event.TenantID, event.Domain)
	return nil
}

// HandleCrawlFailed handles "audit.events.crawl_failed", transitioning crawling → failed.
func (s *service) HandleCrawlFailed(ctx context.Context, msg *nats.Msg) error {
	var event struct {
		AuditID uuid.UUID `json:"audit_id"`
		Domain  string    `json:"domain"`
		Reason  string    `json:"reason"`
	}

	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return nil
	}

	idempotencyKey := msg.Header.Get(nats.MsgIdHdr)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("audit:%s:crawl_failed", event.AuditID)
	}

	s.logger.Warn("audit.events.crawl_failed received", zap.String("audit_id", event.AuditID.String()), zap.String("reason", event.Reason))
	_ = s.repository.TransitionStatus(ctx, event.AuditID, StatusCrawling, StatusFailed, idempotencyKey)
	return nil
}

// HandleParseCompleted handles "audit.events.parse_completed", transitioning parsing → analyzing.
func (s *service) HandleParseCompleted(ctx context.Context, msg *nats.Msg) error {
	var event struct {
		AuditID          uuid.UUID `json:"audit_id"`
		TenantID         uuid.UUID `json:"tenant_id"`
		Domain           string    `json:"domain"`
		TotalPagesParsed int       `json:"total_pages_parsed"`
	}

	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return nil
	}

	idempotencyKey := msg.Header.Get(nats.MsgIdHdr)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("audit:%s:parse_completed", event.AuditID)
	}

	s.logger.Info("audit.events.parse_completed received",
		zap.String("audit_id", event.AuditID.String()),
		zap.Int("pages_parsed", event.TotalPagesParsed),
	)

	err := s.repository.TransitionStatus(ctx, event.AuditID, StatusParsing, StatusAnalyzing, idempotencyKey)
	if err != nil {
		if errors.Is(err, ErrAlreadyProcessed) {
			return nil
		}
		if errors.Is(err, ErrStatusMismatch) {
			s.logger.Warn("parse_completed: status mismatch", zap.String("audit_id", event.AuditID.String()))
			return nil
		}
		return fmt.Errorf("HandleParseCompleted: transition failed: %w", err)
	}

	s.logger.Info("audit transitioned parsing → analyzing (parallel analyzers active)", zap.String("audit_id", event.AuditID.String()))
	return nil
}

// HandleParseFailed handles "audit.events.parse_failed", transitioning parsing → failed.
func (s *service) HandleParseFailed(ctx context.Context, msg *nats.Msg) error {
	var event struct {
		AuditID uuid.UUID `json:"audit_id"`
		Domain  string    `json:"domain"`
		Reason  string    `json:"reason"`
	}

	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return nil
	}

	idempotencyKey := msg.Header.Get(nats.MsgIdHdr)
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("audit:%s:parse_failed", event.AuditID)
	}

	_ = s.repository.TransitionStatus(ctx, event.AuditID, StatusParsing, StatusFailed, idempotencyKey)
	return nil
}

//==========================================//
//          PARALLEL JOIN LOGIC             //
//==========================================//

// HandleOnPageAnalysisCompleted records onpage analyzer completion and checks for parallel join completion.
func (s *service) HandleOnPageAnalysisCompleted(ctx context.Context, msg *nats.Msg) error {
	var event struct {
		AuditID       uuid.UUID `json:"audit_id"`
		TenantID      uuid.UUID `json:"tenant_id"`
		Domain        string    `json:"domain"`
		TotalFindings int       `json:"total_findings"`
		DurationMs    int64     `json:"duration_ms"`
	}

	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return nil
	}

	run := &AuditAnalyzerRun{
		AuditID:       event.AuditID,
		AnalyzerName:  "onpage",
		Status:        "completed",
		FindingsCount: event.TotalFindings,
		DurationMs:    event.DurationMs,
		CompletedAt:   time.Now().UTC(),
	}
	_ = s.repository.RecordAnalyzerRun(ctx, run)

	s.logger.Info("parallel analyzer completed",
		zap.String("analyzer", "onpage"),
		zap.String("audit_id", event.AuditID.String()),
		zap.Int("findings", event.TotalFindings),
	)

	return s.checkAndAdvanceAnalyzing(ctx, event.AuditID, event.TenantID, event.Domain)
}

// HandleTechnicalAnalysisCompleted records technical auditor completion and checks for parallel join completion.
func (s *service) HandleTechnicalAnalysisCompleted(ctx context.Context, msg *nats.Msg) error {
	var event struct {
		AuditID       uuid.UUID `json:"audit_id"`
		TenantID      uuid.UUID `json:"tenant_id"`
		Domain        string    `json:"domain"`
		TotalFindings int       `json:"total_findings"`
		DurationMs    int64     `json:"duration_ms"`
	}

	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return nil
	}

	run := &AuditAnalyzerRun{
		AuditID:       event.AuditID,
		AnalyzerName:  "technical",
		Status:        "completed",
		FindingsCount: event.TotalFindings,
		DurationMs:    event.DurationMs,
		CompletedAt:   time.Now().UTC(),
	}
	_ = s.repository.RecordAnalyzerRun(ctx, run)

	s.logger.Info("parallel analyzer completed",
		zap.String("analyzer", "technical"),
		zap.String("audit_id", event.AuditID.String()),
		zap.Int("findings", event.TotalFindings),
	)

	return s.checkAndAdvanceAnalyzing(ctx, event.AuditID, event.TenantID, event.Domain)
}

// HandleKeywordsExtracted records keyword extraction completion and checks for parallel join completion.
func (s *service) HandleKeywordsExtracted(ctx context.Context, msg *nats.Msg) error {
	var event struct {
		AuditID        uuid.UUID `json:"audit_id"`
		TenantID       uuid.UUID `json:"tenant_id"`
		Domain         string    `json:"domain"`
		UniqueKeywords int       `json:"unique_keywords"`
		DurationMs     int64     `json:"duration_ms"`
	}

	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return nil
	}

	run := &AuditAnalyzerRun{
		AuditID:       event.AuditID,
		AnalyzerName:  "keywords",
		Status:        "completed",
		FindingsCount: event.UniqueKeywords,
		DurationMs:    event.DurationMs,
		CompletedAt:   time.Now().UTC(),
	}
	_ = s.repository.RecordAnalyzerRun(ctx, run)

	s.logger.Info("parallel analyzer completed",
		zap.String("analyzer", "keywords"),
		zap.String("audit_id", event.AuditID.String()),
		zap.Int("unique_keywords", event.UniqueKeywords),
	)

	return s.checkAndAdvanceAnalyzing(ctx, event.AuditID, event.TenantID, event.Domain)
}

// checkAndAdvanceAnalyzing checks if all required parallel analyzers have completed, and if so, advances analyzing → advising.
func (s *service) checkAndAdvanceAnalyzing(ctx context.Context, auditID, tenantID uuid.UUID, domain string) error {
	completedList, err := s.repository.GetCompletedAnalyzers(ctx, auditID)
	if err != nil {
		return err
	}

	completedMap := make(map[string]bool)
	for _, name := range completedList {
		completedMap[name] = true
	}

	// Check if all required analyzers are finished
	allDone := true
	for _, required := range requiredAnalyzers {
		if !completedMap[required] {
			allDone = false
			break
		}
	}

	if !allDone {
		s.logger.Info("parallel join waiting for remaining analyzers",
			zap.String("audit_id", auditID.String()),
			zap.Strings("completed", completedList),
			zap.Strings("required", requiredAnalyzers),
		)
		return nil
	}

	s.logger.Info("all required parallel analyzers finished — advancing state machine",
		zap.String("audit_id", auditID.String()),
		zap.Strings("analyzers", requiredAnalyzers),
	)

	idempotencyKey := fmt.Sprintf("audit:%s:all_analyzers_completed", auditID)
	err = s.repository.TransitionStatus(ctx, auditID, StatusAnalyzing, StatusAdvising, idempotencyKey)
	if err != nil {
		if errors.Is(err, ErrAlreadyProcessed) || errors.Is(err, ErrStatusMismatch) {
			return nil
		}
		return fmt.Errorf("transition analyzing → advising failed: %w", err)
	}

	s.logger.Info("audit transitioned analyzing → advising", zap.String("audit_id", auditID.String()))
	_ = s.publishAnalyzingCompleted(ctx, auditID, tenantID, domain)
	return nil
}

// HandleScoreComputed handles "audit.events.score.computed", transitioning advising → reporting.
func (s *service) HandleScoreComputed(ctx context.Context, msg *nats.Msg) error {
	var event struct {
		AuditID  uuid.UUID `json:"audit_id"`
		TenantID uuid.UUID `json:"tenant_id"`
		Domain   string    `json:"domain"`
	}
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return nil
	}

	idempotencyKey := fmt.Sprintf("audit:%s:score_computed", event.AuditID)
	err := s.repository.TransitionStatus(ctx, event.AuditID, StatusAdvising, StatusReporting, idempotencyKey)
	if err != nil {
		if errors.Is(err, ErrAlreadyProcessed) || errors.Is(err, ErrStatusMismatch) {
			return nil
		}
		return fmt.Errorf("transition advising → reporting failed: %w", err)
	}

	s.logger.Info("audit transitioned advising → reporting", zap.String("audit_id", event.AuditID.String()))
	return nil
}

// HandleReportGenerated handles "audit.events.report.generated", transitioning reporting → completed.
func (s *service) HandleReportGenerated(ctx context.Context, msg *nats.Msg) error {
	var event struct {
		AuditID  uuid.UUID `json:"audit_id"`
		TenantID uuid.UUID `json:"tenant_id"`
		Domain   string    `json:"domain"`
	}
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return nil
	}

	idempotencyKey := fmt.Sprintf("audit:%s:report_generated", event.AuditID)
	err := s.repository.TransitionStatus(ctx, event.AuditID, StatusReporting, StatusCompleted, idempotencyKey)
	if err != nil {
		if errors.Is(err, ErrAlreadyProcessed) || errors.Is(err, ErrStatusMismatch) {
			return nil
		}
		return fmt.Errorf("transition reporting → completed failed: %w", err)
	}

	s.logger.Info("audit transitioned reporting → completed (pipeline fully finished)", zap.String("audit_id", event.AuditID.String()))
	return nil
}

// GetAuditStatus implements [Service].
func (s *service) GetAuditStatus(ctx context.Context, auditID uuid.UUID) (AuditStatusResponse, error) {
	audit, err := s.repository.FindAuditByID(ctx, auditID)
	if err != nil {
		return AuditStatusResponse{}, err
	}

	return AuditStatusResponse{
		ID:            audit.ID,
		TenantID:      audit.TenantID,
		NormalizedURL: audit.NormalizedURL,
		Domain:        audit.Domain,
		Status:        audit.Status,
		StepStartedAt: audit.StepStartedAt,
		StepAttempts:  audit.StepAttempts,
		SubmittedAt:   audit.SubmittedAt,
		UpdatedAt:     audit.UpdatedAt,
	}, nil
}

//==========================================//
//             HELPER FUNCTIONS             //
//==========================================//

func (s *service) publishStartCrawl(ctx context.Context, event AuditValidatedEvent) error {
	cmd := StartCrawlCommand{
		AuditID:       event.AuditID,
		TenantID:      event.TenantID,
		NormalizedURL: event.NormalizedURL,
		Domain:        event.Domain,
	}

	payload, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("marshal start_crawl command: %w", err)
	}

	msgID := fmt.Sprintf("audit:%s:start_crawl", event.AuditID)
	_, err = s.js.Publish(
		SubjectStartCrawl,
		payload,
		nats.MsgId(msgID),
		nats.Context(ctx),
	)
	if err != nil {
		if strings.Contains(err.Error(), "context") {
			return ctx.Err()
		}
		return fmt.Errorf("publish start_crawl: %w", err)
	}

	return nil
}

func (s *service) publishStartParse(ctx context.Context, auditID, tenantID uuid.UUID, domain string) error {
	cmd := map[string]interface{}{
		"audit_id":  auditID,
		"tenant_id": tenantID,
		"domain":    domain,
	}
	payload, _ := json.Marshal(cmd)
	msgID := fmt.Sprintf("audit:%s:start_parse", auditID)
	_, err := s.js.Publish("audit.commands.start_parse", payload, nats.MsgId(msgID), nats.Context(ctx))
	return err
}

func (s *service) publishAnalyzingCompleted(ctx context.Context, auditID, tenantID uuid.UUID, domain string) error {
	evt := map[string]interface{}{
		"audit_id":  auditID,
		"tenant_id": tenantID,
		"domain":    domain,
	}
	payload, _ := json.Marshal(evt)
	msgID := fmt.Sprintf("audit:%s:analyzing_completed", auditID)
	_, err := s.js.Publish("audit.events.analyzing.completed", payload, nats.MsgId(msgID), nats.Context(ctx))
	return err
}
