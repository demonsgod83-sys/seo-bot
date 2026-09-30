package scoring

import (
	"time"

	"github.com/google/uuid"
)

//==========================================//
//              NATS EVENTS                 //
//==========================================//

// StartScoringEvent is received when the 3 parallel analyzers complete.
type StartScoringEvent struct {
	AuditID  uuid.UUID `json:"audit_id"`
	TenantID uuid.UUID `json:"tenant_id"`
	Domain   string    `json:"domain"`
}

// ScoringCompletedEvent is published when score calculation and issue prioritization finish.
type ScoringCompletedEvent struct {
	AuditID        uuid.UUID `json:"audit_id"`
	TenantID       uuid.UUID `json:"tenant_id"`
	Domain         string    `json:"domain"`
	OverallScore   int       `json:"overall_score"`
	TechnicalScore int       `json:"technical_score"`
	OnPageScore    int       `json:"onpage_score"`
	ContentScore   int       `json:"content_score"`
	TotalFindings  int       `json:"total_findings"`
	CriticalCount  int       `json:"critical_count"`
	WarningCount   int       `json:"warning_count"`
	InfoCount      int       `json:"info_count"`
	IssuesCount    int       `json:"issues_count"`
	DurationMs     int64     `json:"duration_ms"`
}

//==========================================//
//              HTTP RESPONSES              //
//==========================================//

type AuditScoresResponse struct {
	AuditID          uuid.UUID                  `json:"audit_id"`
	TenantID         uuid.UUID                  `json:"tenant_id"`
	OverallScore     int                        `json:"overall_score"`
	TechnicalScore   int                        `json:"technical_score"`
	OnPageScore      int                        `json:"onpage_score"`
	ContentScore     int                        `json:"content_score"`
	TotalFindings    int                        `json:"total_findings"`
	CriticalCount    int                        `json:"critical_count"`
	WarningCount     int                        `json:"warning_count"`
	InfoCount        int                        `json:"info_count"`
	TotalPagesScored int                        `json:"total_pages_scored"`
	CalculatedAt     time.Time                  `json:"calculated_at"`
	PageScores       []PageScoreResponse        `json:"page_scores,omitempty"`
	PrioritizedIssues []PrioritizedIssueResponse `json:"prioritized_issues,omitempty"`
}

type PageScoreResponse struct {
	PageURL        string    `json:"page_url"`
	OverallScore   int       `json:"overall_score"`
	TechnicalScore int       `json:"technical_score"`
	OnPageScore    int       `json:"onpage_score"`
	ContentScore   int       `json:"content_score"`
	CriticalCount  int       `json:"critical_count"`
	WarningCount   int       `json:"warning_count"`
	InfoCount      int       `json:"info_count"`
	CalculatedAt   time.Time `json:"calculated_at"`
}

type PrioritizedIssueResponse struct {
	RuleID             string   `json:"rule_id"`
	Category           string   `json:"category"`
	Severity           string   `json:"severity"`
	Title              string   `json:"title"`
	Message            string   `json:"message"`
	EffortTier         string   `json:"effort_tier"`
	ImpactScore        float64  `json:"impact_score"`
	PriorityScore      float64  `json:"priority_score"`
	AffectedPagesCount int      `json:"affected_pages_count"`
	AffectedURLs       []string `json:"affected_urls"`
}
