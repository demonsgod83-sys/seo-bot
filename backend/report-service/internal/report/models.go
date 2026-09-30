package report

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

//==========================================//
//          AUDIT REPORT MODEL              //
//==========================================//

type AuditReport struct {
	bun.BaseModel `bun:"table:audit_reports,alias:ar"`

	ID               uuid.UUID `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	AuditID          uuid.UUID `bun:"audit_id,notnull,type:uuid"`
	TenantID         uuid.UUID `bun:"tenant_id,notnull,type:uuid"`
	Version          int       `bun:"version,notnull"`
	Status           string    `bun:"status,notnull"` // "completed", "completed_with_warnings"
	ShareToken       string    `bun:"share_token,notnull,unique"`
	JsonPath         string    `bun:"json_path,notnull"`
	HtmlPath         string    `bun:"html_path,notnull"`
	PdfPath          string    `bun:"pdf_path,notnull"`
	OverallScore     int       `bun:"overall_score,notnull"`
	TechnicalScore   int       `bun:"technical_score,notnull"`
	OnPageScore      int       `bun:"onpage_score,notnull"`
	ContentScore     int       `bun:"content_score,notnull"`
	TotalFindings    int       `bun:"total_findings,notnull"`
	TotalIssues      int       `bun:"total_issues,notnull"`
	TotalPagesScored int       `bun:"total_pages_scored,notnull"`
	HasWarnings      bool      `bun:"has_warnings,notnull,default:false"`
	Warnings         []string  `bun:"warnings,type:jsonb"`
	CreatedAt        time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

//==========================================//
//          REFERENCE READ MODELS           //
//==========================================//

type AuditRecord struct {
	bun.BaseModel `bun:"table:audits,alias:a"`

	ID            uuid.UUID  `bun:"_id,pk,type:uuid"`
	TenantID      uuid.UUID  `bun:"tenant_id,notnull,type:uuid"`
	NormalizedURL string     `bun:"normalized_url,notnull"`
	Domain        string     `bun:"domain,notnull"`
	Status        string     `bun:"status,notnull"`
	StepStartedAt *time.Time `bun:"step_started_at"`
	SubmittedAt   time.Time  `bun:"submitted_at"`
	UpdatedAt     time.Time  `bun:"updated_at"`
}

type AuditScore struct {
	bun.BaseModel `bun:"table:audit_scores,alias:asc"`

	ID               uuid.UUID `bun:"_id,pk,type:uuid"`
	AuditID          uuid.UUID `bun:"audit_id,notnull,type:uuid"`
	TenantID         uuid.UUID `bun:"tenant_id,notnull,type:uuid"`
	OverallScore     int       `bun:"overall_score,notnull"`
	TechnicalScore   int       `bun:"technical_score,notnull"`
	OnPageScore      int       `bun:"onpage_score,notnull"`
	ContentScore     int       `bun:"content_score,notnull"`
	TotalFindings    int       `bun:"total_findings,notnull"`
	CriticalCount    int       `bun:"critical_count,notnull"`
	WarningCount     int       `bun:"warning_count,notnull"`
	InfoCount        int       `bun:"info_count,notnull"`
	TotalPagesScored int       `bun:"total_pages_scored,notnull"`
	CalculatedAt     time.Time `bun:"calculated_at,nullzero,notnull"`
}

type PageScore struct {
	bun.BaseModel `bun:"table:page_scores,alias:psc"`

	ID             uuid.UUID `bun:"_id,pk,type:uuid"`
	AuditID        uuid.UUID `bun:"audit_id,notnull,type:uuid"`
	TenantID       uuid.UUID `bun:"tenant_id,notnull,type:uuid"`
	PageURL        string    `bun:"page_url,notnull"`
	OverallScore   int       `bun:"overall_score,notnull"`
	TechnicalScore int       `bun:"technical_score,notnull"`
	OnPageScore    int       `bun:"onpage_score,notnull"`
	ContentScore   int       `bun:"content_score,notnull"`
	CriticalCount  int       `bun:"critical_count,notnull"`
	WarningCount   int       `bun:"warning_count,notnull"`
	InfoCount      int       `bun:"info_count,notnull"`
	CalculatedAt   time.Time `bun:"calculated_at,nullzero,notnull"`
}

type PrioritizedIssue struct {
	bun.BaseModel `bun:"table:prioritized_issues,alias:pi"`

	ID                 uuid.UUID `bun:"_id,pk,type:uuid"`
	AuditID            uuid.UUID `bun:"audit_id,notnull,type:uuid"`
	TenantID           uuid.UUID `bun:"tenant_id,notnull,type:uuid"`
	RuleID             string    `bun:"rule_id,notnull"`
	Category           string    `bun:"category,notnull"`
	Severity           string    `bun:"severity,notnull"`
	Title              string    `bun:"title,notnull"`
	Message            string    `bun:"message,notnull"`
	EffortTier         string    `bun:"effort_tier,notnull"`
	ImpactScore        float64   `bun:"impact_score,notnull"`
	PriorityScore      float64   `bun:"priority_score,notnull"`
	AffectedPagesCount int       `bun:"affected_pages_count,notnull"`
	AffectedURLs       []string  `bun:"affected_urls,type:jsonb"`
	CreatedAt          time.Time `bun:"created_at,nullzero,notnull"`
}

type SiteKeywordSummary struct {
	bun.BaseModel `bun:"table:site_keyword_summaries,alias:sks"`

	ID               uuid.UUID           `bun:"_id,pk,type:uuid"`
	AuditID          uuid.UUID           `bun:"audit_id,notnull,type:uuid"`
	TenantID         uuid.UUID           `bun:"tenant_id,notnull,type:uuid"`
	TotalKeywords    int                 `bun:"total_keywords,notnull"`
	KeywordMap       map[string][]string `bun:"keyword_map,type:jsonb"`
	Cannibalizations int                 `bun:"cannibalizations,notnull"`
	CompletedAt      time.Time           `bun:"completed_at,nullzero,notnull"`
}
