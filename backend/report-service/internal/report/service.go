package report

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/demonsgod83-sys/seo-bot/report-service/internal/platform/storage"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const (
	SubjectReportGenerated = "audit.events.report.generated"
)

type Service interface {
	HandleScoreComputed(ctx context.Context, msg *nats.Msg) error
	GetReportsByAuditID(ctx context.Context, auditID uuid.UUID) (AuditReportsListResponse, error)
	GetLatestReport(ctx context.Context, auditID uuid.UUID) (*AuditReport, error)
	GetReportContentByToken(ctx context.Context, shareToken, format string) ([]byte, string, error)
}

type service struct {
	repository    Repository
	storage       storage.Storage
	jsonRenderer  *JSONRenderer
	htmlRenderer  *HTMLRenderer
	pdfRenderer   *PDFRenderer
	js            nats.JetStreamContext
	publicBaseURL string
	logger        *zap.Logger
}

func NewService(
	repository Repository,
	storage storage.Storage,
	jsonRenderer *JSONRenderer,
	htmlRenderer *HTMLRenderer,
	pdfRenderer *PDFRenderer,
	js nats.JetStreamContext,
	publicBaseURL string,
	logger *zap.Logger,
) Service {
	return &service{
		repository:    repository,
		storage:       storage,
		jsonRenderer:  jsonRenderer,
		htmlRenderer:  htmlRenderer,
		pdfRenderer:   pdfRenderer,
		js:            js,
		publicBaseURL: publicBaseURL,
		logger:        logger,
	}
}

func (s *service) HandleScoreComputed(ctx context.Context, msg *nats.Msg) error {
	start := time.Now()

	var event ScoreComputedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		s.logger.Error("score_computed: malformed payload — acking", zap.Error(err))
		return nil
	}

	s.logger.Info("score_computed received — generating reports",
		zap.String("audit_id", event.AuditID.String()),
		zap.String("domain", event.Domain),
	)

	// 1. Gather all scoring and audit data
	auditRec, err := s.repository.GetAuditRecord(ctx, event.AuditID)
	if err != nil {
		return fmt.Errorf("get audit record: %w", err)
	}

	auditScore, err := s.repository.GetAuditScore(ctx, event.AuditID)
	if err != nil {
		return fmt.Errorf("get audit score: %w", err)
	}

	pageScores, err := s.repository.GetPageScores(ctx, event.AuditID)
	if err != nil {
		return fmt.Errorf("get page scores: %w", err)
	}

	issues, err := s.repository.GetPrioritizedIssues(ctx, event.AuditID)
	if err != nil {
		return fmt.Errorf("get prioritized issues: %w", err)
	}

	keywordSummary, _ := s.repository.GetSiteKeywordSummary(ctx, event.AuditID)

	// 2. Determine Version and Share Token
	version, err := s.repository.GetNextVersion(ctx, event.AuditID)
	if err != nil {
		version = 1
	}

	shareToken := generateShareToken()
	shareURL := fmt.Sprintf("%s/public/reports/%s", s.publicBaseURL, shareToken)

	// 3. Assemble Canonical ReportData model
	reportData := s.assembleReportData(auditRec, auditScore, pageScores, issues, keywordSummary, version, shareToken, shareURL)

	// 4. Render All Three Formats from the Single Data Model
	jsonBytes, err := s.jsonRenderer.Render(reportData)
	if err != nil {
		return fmt.Errorf("render json: %w", err)
	}

	htmlBytes, err := s.htmlRenderer.Render(reportData)
	if err != nil {
		return fmt.Errorf("render html: %w", err)
	}

	pdfBytes, err := s.pdfRenderer.Render(reportData)
	if err != nil {
		return fmt.Errorf("render pdf: %w", err)
	}

	// 5. Store Generated Artifacts
	jsonPath, err := s.storage.SaveReportFile(event.AuditID, version, "report.json", jsonBytes)
	if err != nil {
		return fmt.Errorf("save json artifact: %w", err)
	}

	htmlPath, err := s.storage.SaveReportFile(event.AuditID, version, "report.html", htmlBytes)
	if err != nil {
		return fmt.Errorf("save html artifact: %w", err)
	}

	pdfPath, err := s.storage.SaveReportFile(event.AuditID, version, "report.pdf", pdfBytes)
	if err != nil {
		return fmt.Errorf("save pdf artifact: %w", err)
	}

	// 6. Record in Database
	reportStatus := "completed"
	if reportData.HasWarnings {
		reportStatus = "completed_with_warnings"
	}

	reportModel := &AuditReport{
		AuditID:          event.AuditID,
		TenantID:         event.TenantID,
		Version:          version,
		Status:           reportStatus,
		ShareToken:       shareToken,
		JsonPath:         jsonPath,
		HtmlPath:         htmlPath,
		PdfPath:          pdfPath,
		OverallScore:     reportData.OverallScore,
		TechnicalScore:   reportData.TechnicalScore,
		OnPageScore:      reportData.OnPageScore,
		ContentScore:     reportData.ContentScore,
		TotalFindings:    reportData.TotalFindings,
		TotalIssues:      len(reportData.PrioritizedIssues),
		TotalPagesScored: reportData.TotalPagesScored,
		HasWarnings:      reportData.HasWarnings,
		Warnings:         reportData.Warnings,
	}

	if err := s.repository.SaveReport(ctx, reportModel); err != nil {
		return fmt.Errorf("save audit report: %w", err)
	}

	duration := time.Since(start)

	s.logger.Info("report generation completed successfully",
		zap.String("audit_id", event.AuditID.String()),
		zap.Int("version", version),
		zap.String("share_token", shareToken),
		zap.Int("overall_score", reportData.OverallScore),
		zap.Duration("duration", duration),
	)

	// 7. Publish report.generated event
	generatedEvent := ReportGeneratedEvent{
		AuditID:      event.AuditID,
		TenantID:     event.TenantID,
		Domain:       event.Domain,
		Version:      version,
		ShareToken:   shareToken,
		ShareURL:     shareURL,
		JsonPath:     jsonPath,
		HtmlPath:     htmlPath,
		PdfPath:      pdfPath,
		OverallScore: reportData.OverallScore,
		HasWarnings:  reportData.HasWarnings,
		GeneratedAt:  reportData.GeneratedAt,
		DurationMs:   duration.Milliseconds(),
	}

	payload, _ := json.Marshal(generatedEvent)
	msgID := fmt.Sprintf("audit:%s:report_generated:v%d", event.AuditID, version)
	_, err = s.js.Publish(
		SubjectReportGenerated,
		payload,
		nats.MsgId(msgID),
		nats.Context(ctx),
	)
	if err != nil {
		s.logger.Warn("failed to publish report.generated event", zap.Error(err))
	}

	return nil
}

func (s *service) assembleReportData(
	auditRec *AuditRecord,
	auditScore *AuditScore,
	pageScores []PageScore,
	issues []PrioritizedIssue,
	keywordSummary *SiteKeywordSummary,
	version int,
	shareToken, shareURL string,
) *ReportData {
	now := time.Now().UTC()

	var pIssues []ReportPrioritizedIssue
	for _, is := range issues {
		pIssues = append(pIssues, ReportPrioritizedIssue{
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

	var pBreakdowns []ReportPageBreakdown
	for _, ps := range pageScores {
		pBreakdowns = append(pBreakdowns, ReportPageBreakdown{
			PageURL:        ps.PageURL,
			OverallScore:   ps.OverallScore,
			TechnicalScore: ps.TechnicalScore,
			OnPageScore:    ps.OnPageScore,
			ContentScore:   ps.ContentScore,
			CriticalCount:  ps.CriticalCount,
			WarningCount:   ps.WarningCount,
			InfoCount:      ps.InfoCount,
		})
	}

	var kwItems []ReportKeywordItem
	totalKeywords := 0
	cannibalizations := 0

	if keywordSummary != nil {
		totalKeywords = keywordSummary.TotalKeywords
		cannibalizations = keywordSummary.Cannibalizations
		for kw, urls := range keywordSummary.KeywordMap {
			kwItems = append(kwItems, ReportKeywordItem{
				Keyword:     kw,
				TargetPages: urls,
				PageCount:   len(urls),
			})
		}
		sort.Slice(kwItems, func(i, j int) bool {
			if kwItems[i].PageCount != kwItems[j].PageCount {
				return kwItems[i].PageCount > kwItems[j].PageCount
			}
			return kwItems[i].Keyword < kwItems[j].Keyword
		})
	}

	hasWarnings := auditRec.Status == "completed_with_warnings"
	var warnings []string
	if hasWarnings {
		warnings = append(warnings, "Audit completed with partial analysis. Some non-critical audit modules were unavailable.")
	}

	return &ReportData{
		AuditID:              auditRec.ID,
		TenantID:             auditRec.TenantID,
		Domain:               auditRec.Domain,
		NormalizedURL:        auditRec.NormalizedURL,
		Status:               auditRec.Status,
		SubmittedAt:          auditRec.SubmittedAt,
		GeneratedAt:          now,
		Version:              version,
		ShareToken:           shareToken,
		ShareURL:             shareURL,
		HasWarnings:          hasWarnings,
		Warnings:             warnings,
		OverallScore:         auditScore.OverallScore,
		TechnicalScore:       auditScore.TechnicalScore,
		OnPageScore:          auditScore.OnPageScore,
		ContentScore:         auditScore.ContentScore,
		TotalFindings:        auditScore.TotalFindings,
		CriticalCount:        auditScore.CriticalCount,
		WarningCount:         auditScore.WarningCount,
		InfoCount:            auditScore.InfoCount,
		TotalPagesScored:     auditScore.TotalPagesScored,
		PrioritizedIssues:    pIssues,
		PageBreakdown:        pBreakdowns,
		KeywordMap:           kwItems,
		TotalKeywords:        totalKeywords,
		CannibalizationCount: cannibalizations,
	}
}

func (s *service) GetReportsByAuditID(ctx context.Context, auditID uuid.UUID) (AuditReportsListResponse, error) {
	reports, err := s.repository.GetReportsByAuditID(ctx, auditID)
	if err != nil {
		return AuditReportsListResponse{}, err
	}

	var briefs []AuditReportBrief
	for _, r := range reports {
		briefs = append(briefs, AuditReportBrief{
			Version:      r.Version,
			Status:       r.Status,
			ShareToken:   r.ShareToken,
			ShareURL:     fmt.Sprintf("%s/public/reports/%s", s.publicBaseURL, r.ShareToken),
			OverallScore: r.OverallScore,
			HasWarnings:  r.HasWarnings,
			CreatedAt:    r.CreatedAt,
		})
	}

	return AuditReportsListResponse{
		AuditID: auditID,
		Reports: briefs,
	}, nil
}

func (s *service) GetLatestReport(ctx context.Context, auditID uuid.UUID) (*AuditReport, error) {
	return s.repository.GetLatestReportByAuditID(ctx, auditID)
}

func (s *service) GetReportContentByToken(ctx context.Context, shareToken, format string) ([]byte, string, error) {
	report, err := s.repository.GetReportByShareToken(ctx, shareToken)
	if err != nil {
		return nil, "", err
	}

	switch format {
	case "json":
		data, err := s.storage.ReadReportFile(report.JsonPath)
		return data, "application/json", err
	case "pdf":
		data, err := s.storage.ReadReportFile(report.PdfPath)
		return data, "application/pdf", err
	default:
		data, err := s.storage.ReadReportFile(report.HtmlPath)
		return data, "text/html; charset=utf-8", err
	}
}

func generateShareToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
