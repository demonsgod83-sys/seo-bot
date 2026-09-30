package scoring

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type mockRepository struct {
	findings   []Finding
	pages      []ParsedPageRecord
	savedRes   CalculationResult
	auditScore *AuditScore
	pageScores []PageScore
	issues     []PrioritizedIssue
}

func (m *mockRepository) FindFindingsByAuditID(ctx context.Context, auditID uuid.UUID) ([]Finding, error) {
	return m.findings, nil
}

func (m *mockRepository) FindParsedPagesByAuditID(ctx context.Context, auditID uuid.UUID) ([]ParsedPageRecord, error) {
	return m.pages, nil
}

func (m *mockRepository) SaveCalculationResult(ctx context.Context, result CalculationResult) error {
	m.savedRes = result
	return nil
}

func (m *mockRepository) GetAuditScores(ctx context.Context, auditID uuid.UUID) (*AuditScore, []PageScore, []PrioritizedIssue, error) {
	return m.auditScore, m.pageScores, m.issues, nil
}

func TestService_GetAuditScores(t *testing.T) {
	auditID := uuid.New()
	tenantID := uuid.New()

	mockRepo := &mockRepository{
		auditScore: &AuditScore{
			AuditID:          auditID,
			TenantID:         tenantID,
			OverallScore:     88,
			TechnicalScore:   92,
			OnPageScore:      85,
			ContentScore:     84,
			TotalFindings:    5,
			CriticalCount:    1,
			WarningCount:     3,
			InfoCount:        1,
			TotalPagesScored: 2,
			CalculatedAt:     time.Now().UTC(),
		},
		pageScores: []PageScore{
			{
				AuditID:        auditID,
				TenantID:       tenantID,
				PageURL:        "https://example.com",
				OverallScore:   90,
				TechnicalScore: 95,
				OnPageScore:    88,
				ContentScore:   85,
			},
		},
		issues: []PrioritizedIssue{
			{
				AuditID:            auditID,
				TenantID:           tenantID,
				RuleID:             "onpage.title_missing",
				Category:           "onpage",
				Severity:           "critical",
				Title:              "Missing Title Tag",
				Message:            "Missing title",
				EffortTier:         "low",
				ImpactScore:        30.0,
				PriorityScore:      30.0,
				AffectedPagesCount: 1,
				AffectedURLs:       []string{"https://example.com"},
			},
		},
	}

	logger := zap.NewNop()
	engine := NewEngine(DefaultEngineConfig())
	svc := NewService(mockRepo, engine, nil, logger)

	resp, err := svc.GetAuditScores(context.Background(), auditID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.OverallScore != 88 {
		t.Errorf("expected overall score 88, got %d", resp.OverallScore)
	}
	if len(resp.PageScores) != 1 {
		t.Errorf("expected 1 page score, got %d", len(resp.PageScores))
	}
	if len(resp.PrioritizedIssues) != 1 {
		t.Errorf("expected 1 prioritized issue, got %d", len(resp.PrioritizedIssues))
	}
}
