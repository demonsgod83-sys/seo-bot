package keywords

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
	ExecuteExtraction(ctx context.Context, event ParseCompletedEvent) error
	GetKeywordsByAuditID(ctx context.Context, auditID uuid.UUID) ([]PageKeyword, *SiteKeywordSummary, error)
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
//          EXTRACTION PIPELINE             //
//==========================================//

// ExecuteExtraction extracts keywords, computes TF-IDF, builds site keyword map, stores results, and publishes event.
func (s *service) ExecuteExtraction(ctx context.Context, event ParseCompletedEvent) error {
	startTime := time.Now()
	s.logger.Info("starting keyword extraction for audit",
		zap.String("audit_id", event.AuditID.String()),
		zap.String("domain", event.Domain),
	)

	// 1. Fetch parsed pages
	pages, err := s.repository.FindParsedPagesByAuditID(ctx, event.AuditID)
	if err != nil {
		return fmt.Errorf("failed to query parsed pages: %w", err)
	}

	if len(pages) == 0 {
		s.logger.Warn("no parsed pages found for keyword extraction", zap.String("audit_id", event.AuditID.String()))
		return s.publishKeywordsExtracted(ctx, event, 0, 0, 0, 0)
	}

	// 2. Run TF-IDF & Keyword Extraction Engine
	result := ExtractSiteKeywords(pages)

	// 3. Save Per-Page Keyword Profiles
	if err := s.repository.SavePageKeywords(ctx, result.PageKeywords); err != nil {
		s.logger.Error("failed to save page keywords", zap.Error(err))
		return fmt.Errorf("failed to save page keywords: %w", err)
	}

	// 4. Save Site-Wide Summary Map
	if err := s.repository.SaveSiteSummary(ctx, &result.SiteSummary); err != nil {
		s.logger.Error("failed to save site summary", zap.Error(err))
	}

	// 5. Save Keyword Cannibalization Findings into audit_findings
	if len(result.Findings) > 0 {
		if err := s.repository.SaveFindings(ctx, result.Findings); err != nil {
			s.logger.Error("failed to save keyword findings", zap.Error(err))
		}
	}

	durationMs := time.Since(startTime).Milliseconds()

	s.logger.Info("keyword extraction finished",
		zap.String("audit_id", event.AuditID.String()),
		zap.Int("pages_profiled", len(result.PageKeywords)),
		zap.Int("unique_keywords", result.UniqueKeywords),
		zap.Int("cannibalizations", result.Cannibalizations),
		zap.Int64("duration_ms", durationMs),
	)

	// 6. Publish Keywords Extracted Event to NATS
	return s.publishKeywordsExtracted(ctx, event, len(pages), result.UniqueKeywords, result.Cannibalizations, durationMs)
}

// GetKeywordsByAuditID implements [Service].
func (s *service) GetKeywordsByAuditID(ctx context.Context, auditID uuid.UUID) ([]PageKeyword, *SiteKeywordSummary, error) {
	pageKws, err := s.repository.GetPageKeywordsByAuditID(ctx, auditID)
	if err != nil {
		return nil, nil, err
	}
	summary, err := s.repository.GetSiteKeywordSummary(ctx, auditID)
	if err != nil {
		return nil, nil, err
	}
	return pageKws, summary, nil
}

//==========================================//
//             HELPER FUNCTIONS             //
//==========================================//

func (s *service) publishKeywordsExtracted(
	ctx context.Context,
	event ParseCompletedEvent,
	totalPages, uniqueKws, cannibalizations int,
	dur int64,
) error {
	evt := KeywordsExtractedEvent{
		AuditID:          event.AuditID,
		TenantID:         event.TenantID,
		Domain:           event.Domain,
		TotalPages:       totalPages,
		UniqueKeywords:   uniqueKws,
		Cannibalizations: cannibalizations,
		DurationMs:       dur,
		CompletedAt:      time.Now().UTC(),
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal keywords extracted event: %w", err)
	}

	msgID := fmt.Sprintf("audit:%s:keywords_extracted", event.AuditID)
	_, err = s.js.Publish(
		"audit.events.keywords.extracted",
		payload,
		nats.MsgId(msgID),
		nats.Context(ctx),
	)
	if err != nil {
		s.logger.Error("failed to publish keywords.extracted event", zap.Error(err))
		return err
	}

	s.logger.Info("published keywords.extracted event", zap.String("audit_id", event.AuditID.String()))
	return nil
}
