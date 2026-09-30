package parser

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

//==========================================//
//              PARSED PAGE MODEL           //
//==========================================//

// Heading represents an HTML heading tag (H1–H6) in document order.
type Heading struct {
	Level int    `json:"level"` // 1 for H1, 2 for H2, etc.
	Text  string `json:"text"`  // Clean inner text
}

// PageLink represents an <a> anchor tag extracted from the page.
type PageLink struct {
	Href         string `json:"href"`          // Absolute resolved URL
	Text         string `json:"text"`          // Anchor text
	IsInternal   bool   `json:"is_internal"`   // Same domain as target audit
	IsNoFollow   bool   `json:"is_nofollow"`   // rel contains "nofollow"
	IsSponsored  bool   `json:"is_sponsored"`  // rel contains "sponsored"
	IsUgc        bool   `json:"is_ugc"`        // rel contains "ugc"
}

// PageImage represents an <img> tag extracted from the markup.
type PageImage struct {
	Src     string `json:"src"`               // Absolute resolved URL
	Alt     string `json:"alt"`               // Alt text (empty string if missing)
	HasAlt  bool   `json:"has_alt"`           // Explicit flag: false if alt attribute omitted
	Width   string `json:"width,omitempty"`   // Width attribute if present
	Height  string `json:"height,omitempty"`  // Height attribute if present
	Loading string `json:"loading,omitempty"` // e.g. "lazy"
}

// HreflangTag represents <link rel="alternate" hreflang="..." href="...">.
type HreflangTag struct {
	Hreflang string `json:"hreflang"`
	Href     string `json:"href"`
}

// ParsedPage is the canonical structured data record for an individual crawled page.
// All downstream analyzer services read from this model — never raw HTML.
type ParsedPage struct {
	bun.BaseModel `bun:"table:parsed_pages,alias:pp"`

	ID              uuid.UUID `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	AuditID         uuid.UUID `bun:"audit_id,notnull,type:uuid"`
	TenantID        uuid.UUID `bun:"tenant_id,notnull,type:uuid"`
	URL             string    `bun:"url,notnull"`
	StatusCode      int       `bun:"status_code,notnull"`
	ContentType     string    `bun:"content_type"`
	ByteSize        int64     `bun:"byte_size,notnull"`
	FetchDurationMs int64     `bun:"fetch_duration_ms,notnull"`

	// Extracted HTML Metadata
	Title           string            `bun:"title"`
	MetaDescription string            `bun:"meta_description"`
	MetaRobots      string            `bun:"meta_robots"`
	CanonicalURL    string            `bun:"canonical_url"`
	Viewport        string            `bun:"viewport"`
	Charset         string            `bun:"charset"`
	DeclaredLang    string            `bun:"declared_lang"`

	// Complex Rich Structures (JSONB)
	Hreflangs       []HreflangTag     `bun:"hreflangs,type:jsonb"`
	OpenGraph       map[string]string `bun:"open_graph,type:jsonb"`
	TwitterCard     map[string]string `bun:"twitter_card,type:jsonb"`
	Headings        []Heading         `bun:"headings,type:jsonb"`
	Links           []PageLink        `bun:"links,type:jsonb"`
	Images          []PageImage       `bun:"images,type:jsonb"`
	StructuredData  []string          `bun:"structured_data,type:jsonb"` // Raw JSON-LD blocks

	// Extracted Main Text (Boilerplate Stripped) & Metrics
	BodyText        string            `bun:"body_text"`
	WordCount       int               `bun:"word_count,notnull"`
	CharCount       int               `bun:"char_count,notnull"`

	// Parsing Execution Status
	ParseStatus     string            `bun:"parse_status,notnull"` // "success", "failed"
	ErrorMessage    string            `bun:"error_message"`
	ParsedAt        time.Time         `bun:"parsed_at,nullzero,notnull,default:current_timestamp"`
}

// CrawlPageReference mirrors the crawl_pages table row to read crawl inputs.
type CrawlPageReference struct {
	bun.BaseModel `bun:"table:crawl_pages,alias:cp"`

	ID              uuid.UUID `bun:"_id,pk,type:uuid"`
	AuditID         uuid.UUID `bun:"audit_id,notnull,type:uuid"`
	TenantID        uuid.UUID `bun:"tenant_id,notnull,type:uuid"`
	URL             string    `bun:"url,notnull"`
	StatusCode      int       `bun:"status_code,notnull"`
	ContentType     string    `bun:"content_type"`
	ByteSize        int64     `bun:"byte_size,notnull"`
	StoragePath     string    `bun:"storage_path"`
	FetchDurationMs int64     `bun:"fetch_duration_ms,notnull"`
	ErrorMessage    string    `bun:"error_message"`
}
