package auditor

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

// Finding reuses the canonical schema established in Step 6 with Category = "technical".
type Finding struct {
	bun.BaseModel `bun:"table:audit_findings,alias:af"`

	ID        uuid.UUID       `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	AuditID   uuid.UUID       `bun:"audit_id,notnull,type:uuid"`
	TenantID  uuid.UUID       `bun:"tenant_id,notnull,type:uuid"`
	PageURL   string          `bun:"page_url,notnull"`
	Category  string          `bun:"category,notnull"` // "technical"
	RuleID    string          `bun:"rule_id,notnull"`  // e.g. "technical.status_4xx"
	Severity  FindingSeverity `bun:"severity,notnull"`
	Message   string          `bun:"message,notnull"`
	Evidence  map[string]any  `bun:"evidence,type:jsonb"`
	CreatedAt time.Time       `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

//==========================================//
//          PARSED PAGE DATA REFERENCE      //
//==========================================//

type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}

type PageLink struct {
	Href        string `json:"href"`
	Text        string `json:"text"`
	IsInternal  bool   `json:"is_internal"`
	IsNoFollow  bool   `json:"is_nofollow"`
	IsSponsored bool   `json:"is_sponsored"`
	IsUgc       bool   `json:"is_ugc"`
}

type PageImage struct {
	Src     string `json:"src"`
	Alt     string `json:"alt"`
	HasAlt  bool   `json:"has_alt"`
	Width   string `json:"width,omitempty"`
	Height  string `json:"height,omitempty"`
	Loading string `json:"loading,omitempty"`
}

type HreflangTag struct {
	Hreflang string `json:"hreflang"`
	Href     string `json:"href"`
}

// ParsedPageRecord maps the input from parsed_pages table.
type ParsedPageRecord struct {
	bun.BaseModel `bun:"table:parsed_pages,alias:pp"`

	ID              uuid.UUID         `bun:"_id,pk,type:uuid"`
	AuditID         uuid.UUID         `bun:"audit_id,notnull,type:uuid"`
	TenantID        uuid.UUID         `bun:"tenant_id,notnull,type:uuid"`
	URL             string            `bun:"url,notnull"`
	StatusCode      int               `bun:"status_code,notnull"`
	ContentType     string            `bun:"content_type"`
	ByteSize        int64             `bun:"byte_size,notnull"`
	FetchDurationMs int64             `bun:"fetch_duration_ms,notnull"`
	Title           string            `bun:"title"`
	MetaDescription string            `bun:"meta_description"`
	MetaRobots      string            `bun:"meta_robots"`
	CanonicalURL    string            `bun:"canonical_url"`
	Viewport        string            `bun:"viewport"`
	Charset         string            `bun:"charset"`
	DeclaredLang    string            `bun:"declared_lang"`
	Hreflangs       []HreflangTag     `bun:"hreflangs,type:jsonb"`
	OpenGraph       map[string]string `bun:"open_graph,type:jsonb"`
	TwitterCard     map[string]string `bun:"twitter_card,type:jsonb"`
	Headings        []Heading         `bun:"headings,type:jsonb"`
	Links           []PageLink        `bun:"links,type:jsonb"`
	Images          []PageImage       `bun:"images,type:jsonb"`
	StructuredData  []string          `bun:"structured_data,type:jsonb"`
	BodyText        string            `bun:"body_text"`
	WordCount       int               `bun:"word_count,notnull"`
	CharCount       int               `bun:"char_count,notnull"`
	ParseStatus     string            `bun:"parse_status,notnull"`
	ErrorMessage    string            `bun:"error_message"`
}
