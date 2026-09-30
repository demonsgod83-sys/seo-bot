package crawler

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

//==========================================//
//              CRAWL PAGE MODEL            //
//==========================================//

// CrawlPage represents an individual page fetched during a crawl job.
// Raw snapshot content is stored in object storage (StoragePath);
// metadata and fetch metrics are indexed here in PostgreSQL.
type CrawlPage struct {
	bun.BaseModel `bun:"table:crawl_pages,alias:cp"`

	ID              uuid.UUID `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	AuditID         uuid.UUID `bun:"audit_id,notnull,type:uuid"`
	TenantID        uuid.UUID `bun:"tenant_id,notnull,type:uuid"`
	URL             string    `bun:"url,notnull"`
	StatusCode      int       `bun:"status_code,notnull"`
	ContentType     string    `bun:"content_type"`
	ByteSize        int64     `bun:"byte_size,notnull"`
	StoragePath     string    `bun:"storage_path"`
	FetchDurationMs int64     `bun:"fetch_duration_ms,notnull"`
	ErrorMessage    string    `bun:"error_message"`
	FetchedAt       time.Time `bun:"fetched_at,nullzero,notnull,default:current_timestamp"`
}

//==========================================//
//              CRAWL SUMMARY MODEL         //
//==========================================//

// CrawlSummary records the overall stats of a completed crawl job.
type CrawlSummary struct {
	bun.BaseModel `bun:"table:crawl_summaries,alias:cs"`

	ID                uuid.UUID `bun:"_id,pk,type:uuid,default:gen_random_uuid()"`
	AuditID           uuid.UUID `bun:"audit_id,notnull,unique,type:uuid"`
	TenantID          uuid.UUID `bun:"tenant_id,notnull,type:uuid"`
	Domain            string    `bun:"domain,notnull"`
	BaseURL           string    `bun:"base_url,notnull"`
	RobotsAllowed     bool      `bun:"robots_allowed,notnull"`
	SitemapFound      bool      `bun:"sitemap_found,notnull"`
	TotalDiscovered   int       `bun:"total_discovered,notnull"`
	TotalFetched      int       `bun:"total_fetched,notnull"`
	TotalErrors       int       `bun:"total_errors,notnull"`
	DurationMs        int64     `bun:"duration_ms,notnull"`
	CompletedAt       time.Time `bun:"completed_at,nullzero,notnull,default:current_timestamp"`
}
