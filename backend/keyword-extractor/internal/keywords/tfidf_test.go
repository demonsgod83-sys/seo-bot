package keywords

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestExtractSiteKeywords_TFIDF_And_Cannibalization(t *testing.T) {
	auditID := uuid.New()
	tenantID := uuid.New()

	// Page 1: Topic is "Email Marketing Automation"
	page1 := ParsedPageRecord{
		ID:          uuid.New(),
		AuditID:     auditID,
		TenantID:    tenantID,
		URL:         "https://example.com/email-marketing-automation",
		StatusCode:  200,
		ParseStatus: "success",
		Title:       "Best Email Marketing Automation Platform 2026",
		Headings: []Heading{
			{Level: 1, Text: "Email Marketing Automation Software"},
			{Level: 2, Text: "Automated Workflows & Drip Campaigns"},
		},
		BodyText: `Acme Corporation provides leading email marketing automation software for enterprise teams.
Our email marketing automation tools help companies build powerful automated campaigns, segment subscribers,
and deliver targeted email marketing automation at scale. With Acme Corporation, your email marketing campaigns succeed.`,
		WordCount: 50,
	}

	// Page 2: Competing page for "Email Marketing Automation" (to trigger cannibalization)
	page2 := ParsedPageRecord{
		ID:          uuid.New(),
		AuditID:     auditID,
		TenantID:    tenantID,
		URL:         "https://example.com/features/email-marketing-automation",
		StatusCode:  200,
		ParseStatus: "success",
		Title:       "Email Marketing Automation Features & Pricing",
		Headings: []Heading{
			{Level: 1, Text: "Email Marketing Automation Guide"},
		},
		BodyText: `Explore our advanced email marketing automation features. We offer scalable email marketing automation
workflows with custom webhooks and email marketing automation integrations for growth marketers.`,
		WordCount: 40,
	}

	// Page 3: Topic is "CRM Sales Pipeline"
	page3 := ParsedPageRecord{
		ID:          uuid.New(),
		AuditID:     auditID,
		TenantID:    tenantID,
		URL:         "https://example.com/crm-sales-pipeline",
		StatusCode:  200,
		ParseStatus: "success",
		Title:       "Cloud CRM Sales Pipeline Management",
		Headings: []Heading{
			{Level: 1, Text: "CRM Sales Pipeline System"},
		},
		BodyText: `Manage deals seamlessly with our CRM sales pipeline. Track lead conversion stages, forecast quarterly revenue,
and empower sales reps with our CRM sales pipeline tracker. Acme Corporation guarantees CRM efficiency.`,
		WordCount: 45,
	}

	pages := []ParsedPageRecord{page1, page2, page3}
	result := ExtractSiteKeywords(pages)

	if len(result.PageKeywords) != 3 {
		t.Fatalf("expected 3 page keyword profiles, got %d", len(result.PageKeywords))
	}

	// Check 1: Primary keyword extraction for Page 1 & 2
	pk1 := result.PageKeywords[0]
	if !strings.Contains(pk1.PrimaryKeyword, "email marketing") {
		t.Errorf("expected page 1 primary keyword to contain 'email marketing', got %q", pk1.PrimaryKeyword)
	}

	pk3 := result.PageKeywords[2]
	if !strings.Contains(pk3.PrimaryKeyword, "crm") && !strings.Contains(pk3.PrimaryKeyword, "sales") && !strings.Contains(pk3.PrimaryKeyword, "pipeline") {
		t.Errorf("expected page 3 primary keyword to contain 'crm', 'sales', or 'pipeline', got %q", pk3.PrimaryKeyword)
	}

	// Check 2: Named Entity Recognition
	hasAcmeCorp := false
	for _, ent := range pk1.NamedEntities {
		if strings.Contains(ent, "Acme Corporation") {
			hasAcmeCorp = true
			break
		}
	}
	if !hasAcmeCorp {
		t.Errorf("expected Named Entity 'Acme Corporation' to be detected, got %v", pk1.NamedEntities)
	}

	// Check 3: Cannibalization Detection between Page 1 and Page 2
	if result.Cannibalizations == 0 {
		t.Errorf("expected cannibalization to be detected between page 1 and page 2")
	}

	if len(result.Findings) == 0 {
		t.Errorf("expected at least 1 finding for keywords.cannibalization")
	} else {
		f := result.Findings[0]
		if f.RuleID != "keywords.cannibalization" {
			t.Errorf("expected RuleID keywords.cannibalization, got %s", f.RuleID)
		}
		if f.Category != "keywords" {
			t.Errorf("expected Category keywords, got %s", f.Category)
		}
		if f.Severity != SeverityWarning {
			t.Errorf("expected Severity warning, got %s", f.Severity)
		}
	}

	// Check 4: Site-Wide Summary Map
	if len(result.SiteSummary.KeywordMap) == 0 {
		t.Errorf("expected non-empty SiteSummary.KeywordMap")
	}
}
