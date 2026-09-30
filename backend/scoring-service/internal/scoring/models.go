package scoring

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

//==========================================//
//          CANONICAL FINDING MODEL         //
//==========================================//

type FindingSeverity string

const (
	SeverityCritical FindingSeverity = "critical"
	SeverityWarning  FindingSeverity = "warning"
	SeverityInfo     FindingSeverity = "info"
)

type Finding struct {
	bun.BaseModel `bun:"table:audit_findings,alias:af"`

	ID        uuid.UUID       `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	AuditID   uuid.UUID       `bun:"audit_id,notnull,type:uuid"`
	TenantID  uuid.UUID       `bun:"tenant_id,notnull,type:uuid"`
	PageURL   string          `bun:"page_url,notnull"`
	Category  string          `bun:"category,notnull"` // "onpage", "technical", "keywords"
	RuleID    string          `bun:"rule_id,notnull"`  // e.g. "onpage.title_missing"
	Severity  FindingSeverity `bun:"severity,notnull"`
	Message   string          `bun:"message,notnull"`
	Evidence  map[string]any  `bun:"evidence,type:jsonb"`
	CreatedAt time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

//==========================================//
//          PARSED PAGE DATA REFERENCE      //
//==========================================//

type PageLink struct {
	Href        string `json:"href"`
	Text        string `json:"text"`
	IsInternal  bool   `json:"is_internal"`
	IsNoFollow  bool   `json:"is_nofollow"`
	IsSponsored bool   `json:"is_sponsored"`
	IsUgc       bool   `json:"is_ugc"`
}

type ParsedPageRecord struct {
	bun.BaseModel `bun:"table:parsed_pages,alias:pp"`

	ID              uuid.UUID  `bun:"_id,pk,type:uuid"`
	AuditID         uuid.UUID  `bun:"audit_id,notnull,type:uuid"`
	TenantID        uuid.UUID  `bun:"tenant_id,notnull,type:uuid"`
	URL             string     `bun:"url,notnull"`
	StatusCode      int        `bun:"status_code,notnull"`
	Title           string     `bun:"title"`
	MetaDescription string     `bun:"meta_description"`
	Links           []PageLink `bun:"links,type:jsonb"`
	WordCount       int        `bun:"word_count,notnull"`
	ParseStatus     string     `bun:"parse_status,notnull"`
}

//==========================================//
//          SCORING OUTPUT MODELS           //
//==========================================//

// AuditScore stores the overall and category scores for an audit.
type AuditScore struct {
	bun.BaseModel `bun:"table:audit_scores,alias:asc"`

	ID               uuid.UUID `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	AuditID          uuid.UUID `bun:"audit_id,notnull,unique,type:uuid"`
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
	CalculatedAt     time.Time `bun:"calculated_at,nullzero,notnull,default:current_timestamp"`
}

// PageScore stores per-page scores broken down by category.
type PageScore struct {
	bun.BaseModel `bun:"table:page_scores,alias:psc"`

	ID             uuid.UUID `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
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
	CalculatedAt   time.Time `bun:"calculated_at,nullzero,notnull,default:current_timestamp"`
}

// PrioritizedIssue represents a rolled-up issue ranked by impact-to-effort ROI.
type PrioritizedIssue struct {
	bun.BaseModel `bun:"table:prioritized_issues,alias:pi"`

	ID                 uuid.UUID `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	AuditID            uuid.UUID `bun:"audit_id,notnull,type:uuid"`
	TenantID           uuid.UUID `bun:"tenant_id,notnull,type:uuid"`
	RuleID             string    `bun:"rule_id,notnull"`
	Category           string    `bun:"category,notnull"` // "technical", "onpage", "keywords"
	Severity           string    `bun:"severity,notnull"` // "critical", "warning", "info"
	Title              string    `bun:"title,notnull"`
	Message            string    `bun:"message,notnull"`
	EffortTier         string    `bun:"effort_tier,notnull"` // "low", "medium", "high"
	ImpactScore        float64   `bun:"impact_score,notnull"`
	PriorityScore      float64   `bun:"priority_score,notnull"`
	AffectedPagesCount int       `bun:"affected_pages_count,notnull"`
	AffectedURLs       []string  `bun:"affected_urls,type:jsonb"`
	CreatedAt          time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}
