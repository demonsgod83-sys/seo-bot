package report

import (
	"time"

	"github.com/google/uuid"
)

//==========================================//
//          CANONICAL REPORT DATA MODEL     //
//==========================================//

// ReportData is the single canonical data structure from which JSON, HTML, and PDF formats are generated.
type ReportData struct {
	AuditID              uuid.UUID                `json:"audit_id"`
	TenantID             uuid.UUID                `json:"tenant_id"`
	Domain               string                   `json:"domain"`
	NormalizedURL        string                   `json:"normalized_url"`
	Status               string                   `json:"status"`
	SubmittedAt          time.Time                `json:"submitted_at"`
	GeneratedAt          time.Time                `json:"generated_at"`
	Version              int                      `json:"version"`
	ShareToken           string                   `json:"share_token"`
	ShareURL             string                   `json:"share_url"`
	HasWarnings          bool                     `json:"has_warnings"`
	Warnings             []string                 `json:"warnings"`
	OverallScore         int                      `json:"overall_score"`
	TechnicalScore       int                      `json:"technical_score"`
	OnPageScore          int                      `json:"onpage_score"`
	ContentScore         int                      `json:"content_score"`
	TotalFindings        int                      `json:"total_findings"`
	CriticalCount        int                      `json:"critical_count"`
	WarningCount         int                      `json:"warning_count"`
	InfoCount            int                      `json:"info_count"`
	TotalPagesScored     int                      `json:"total_pages_scored"`
	PrioritizedIssues    []ReportPrioritizedIssue `json:"prioritized_issues"`
	PageBreakdown        []ReportPageBreakdown    `json:"page_breakdown"`
	KeywordMap           []ReportKeywordItem      `json:"keyword_map"`
	TotalKeywords        int                      `json:"total_keywords"`
	CannibalizationCount int                      `json:"cannibalization_count"`
}

type ReportPrioritizedIssue struct {
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

type ReportPageBreakdown struct {
	PageURL        string `json:"page_url"`
	OverallScore   int    `json:"overall_score"`
	TechnicalScore int    `json:"technical_score"`
	OnPageScore    int    `json:"onpage_score"`
	ContentScore   int    `json:"content_score"`
	CriticalCount  int    `json:"critical_count"`
	WarningCount   int    `json:"warning_count"`
	InfoCount      int    `json:"info_count"`
}

type ReportKeywordItem struct {
	Keyword     string   `json:"keyword"`
	TargetPages []string `json:"target_pages"`
	PageCount   int      `json:"page_count"`
}

//==========================================//
//              NATS EVENTS                 //
//==========================================//

// ScoreComputedEvent is consumed from audit.events.score.computed.
type ScoreComputedEvent struct {
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

// ReportGeneratedEvent is published to audit.events.report.generated.
type ReportGeneratedEvent struct {
	AuditID      uuid.UUID `json:"audit_id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	Domain       string    `json:"domain"`
	Version      int       `json:"version"`
	ShareToken   string    `json:"share_token"`
	ShareURL     string    `json:"share_url"`
	JsonPath     string    `json:"json_path"`
	HtmlPath     string    `json:"html_path"`
	PdfPath      string    `json:"pdf_path"`
	OverallScore int       `json:"overall_score"`
	HasWarnings  bool      `json:"has_warnings"`
	GeneratedAt  time.Time `json:"generated_at"`
	DurationMs   int64     `json:"duration_ms"`
}

//==========================================//
//              HTTP RESPONSES              //
//==========================================//

type AuditReportsListResponse struct {
	AuditID uuid.UUID          `json:"audit_id"`
	Reports []AuditReportBrief `json:"reports"`
}

type AuditReportBrief struct {
	Version      int       `json:"version"`
	Status       string    `json:"status"`
	ShareToken   string    `json:"share_token"`
	ShareURL     string    `json:"share_url"`
	OverallScore int       `json:"overall_score"`
	HasWarnings  bool      `json:"has_warnings"`
	CreatedAt    time.Time `json:"created_at"`
}
