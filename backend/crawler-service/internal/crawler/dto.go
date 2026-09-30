package crawler

import (
	"time"

	"github.com/google/uuid"
)

//==========================================//
//              INBOUND COMMANDS            //
//==========================================//

// StartCrawlCommand is received from NATS subject "audit.commands.start_crawl".
type StartCrawlCommand struct {
	AuditID       uuid.UUID `json:"audit_id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	NormalizedURL string    `json:"normalized_url"`
	Domain        string    `json:"domain"`
}

//==========================================//
//              OUTBOUND EVENTS             //
//==========================================//

// CrawlCompletedEvent is published to NATS subject "audit.events.crawl_completed".
type CrawlCompletedEvent struct {
	AuditID       uuid.UUID `json:"audit_id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	Domain        string    `json:"domain"`
	NormalizedURL string    `json:"normalized_url"`
	TotalFetched  int       `json:"total_fetched"`
	TotalErrors   int       `json:"total_errors"`
	DurationMs    int64     `json:"duration_ms"`
	CompletedAt   time.Time `json:"completed_at"`
}

// CrawlFailedEvent is published to NATS subject "audit.events.crawl_failed".
type CrawlFailedEvent struct {
	AuditID       uuid.UUID `json:"audit_id"`
	TenantID      uuid.UUID `json:"tenant_id"`
	Domain        string    `json:"domain"`
	NormalizedURL string    `json:"normalized_url"`
	Reason        string    `json:"reason"`
	FailedAt      time.Time `json:"failed_at"`
}

//==========================================//
//              INTERNAL STRUCTS            //
//==========================================//

// PageFetchResult carries the raw outcome of fetching a single URL.
type PageFetchResult struct {
	URL             string
	StatusCode      int
	ContentType     string
	HTML            []byte
	ByteSize        int64
	StoragePath     string
	FetchDurationMs int64
	ErrorMessage    string
	DiscoveredLinks []string
}
