package keywords

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

//==========================================//
//          KEYWORD & FINDING MODELS        //
//==========================================//

type FindingSeverity string

const (
	SeverityCritical FindingSeverity = "critical"
	SeverityWarning  FindingSeverity = "warning"
	SeverityInfo     FindingSeverity = "info"
)

// Finding maps into the shared audit_findings table with category="keywords".
type Finding struct {
	bun.BaseModel `bun:"table:audit_findings,alias:af"`

	ID        uuid.UUID       `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	AuditID   uuid.UUID       `bun:"audit_id,notnull,type:uuid"`
	TenantID  uuid.UUID       `bun:"tenant_id,notnull,type:uuid"`
	PageURL   string          `bun:"page_url,notnull"`
	Category  string          `bun:"category,notnull"` // "keywords"
	RuleID    string          `bun:"rule_id,notnull"`  // e.g. "keywords.cannibalization"
	Severity  FindingSeverity `bun:"severity,notnull"`
	Message   string          `bun:"message,notnull"`
	Evidence  map[string]any  `bun:"evidence,type:jsonb"`
	CreatedAt time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

// ScoredTerm represents a candidate keyword with its TF-IDF + structural weight score.
type ScoredTerm struct {
	Term      string  `json:"term"`
	Score     float64 `json:"score"`
	NGramType int     `json:"n_gram_type"` // 1 (unigram), 2 (bigram), 3 (trigram)
	Occurrences int   `json:"occurrences"`
}

// PageKeyword stores the per-page target keyword profile extracted from the content.
type PageKeyword struct {
	bun.BaseModel `bun:"table:page_keywords,alias:pk"`

	ID                uuid.UUID    `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	AuditID           uuid.UUID    `bun:"audit_id,notnull,type:uuid"`
	TenantID          uuid.UUID    `bun:"tenant_id,notnull,type:uuid"`
	PageURL           string       `bun:"page_url,notnull"`
	PrimaryKeyword    string       `bun:"primary_keyword,notnull"`
	SecondaryKeywords []string     `bun:"secondary_keywords,type:jsonb"`
	TopTerms          []ScoredTerm `bun:"top_terms,type:jsonb"`
	NamedEntities     []string     `bun:"named_entities,type:jsonb"`
	CreatedAt         time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

// SiteKeywordSummary records site-wide aggregated keyword mappings.
type SiteKeywordSummary struct {
	bun.BaseModel `bun:"table:site_keyword_summaries,alias:sks"`

	ID              uuid.UUID        `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	AuditID         uuid.UUID        `bun:"audit_id,notnull,unique,type:uuid"`
	TenantID        uuid.UUID        `bun:"tenant_id,notnull,type:uuid"`
	TotalKeywords   int              `bun:"total_keywords,notnull"`
	KeywordMap      map[string][]string `bun:"keyword_map,type:jsonb"` // keyword -> []pageURLs
	Cannibalizations int             `bun:"cannibalizations,notnull"`
	CompletedAt     time.Time        `bun:"completed_at,nullzero,notnull,default:current_timestamp"`
}

// ParsedPageRecord is the input read from parsed_pages table.
type ParsedPageRecord struct {
	bun.BaseModel `bun:"table:parsed_pages,alias:pp"`

	ID              uuid.UUID         `bun:"_id,pk,type:uuid"`
	AuditID         uuid.UUID         `bun:"audit_id,notnull,type:uuid"`
	TenantID        uuid.UUID         `bun:"tenant_id,notnull,type:uuid"`
	URL             string            `bun:"url,notnull"`
	StatusCode      int               `bun:"status_code,notnull"`
	Title           string            `bun:"title"`
	MetaDescription string            `bun:"meta_description"`
	Headings        []Heading         `bun:"headings,type:jsonb"`
	BodyText        string            `bun:"body_text"`
	WordCount       int               `bun:"word_count,notnull"`
	ParseStatus     string            `bun:"parse_status,notnull"`
}

type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}
