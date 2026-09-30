package auditor

import (
	"testing"

	"github.com/google/uuid"
)

func TestAuditPages_RuleSuite(t *testing.T) {
	auditID := uuid.New()
	tenantID := uuid.New()

	// Page 1: Broken Technical Page (HTTP 404)
	page1 := ParsedPageRecord{
		ID:         uuid.New(),
		AuditID:    auditID,
		TenantID:   tenantID,
		URL:        "https://example.com/broken-link",
		StatusCode: 404,
	}

	// Page 2: HTTPS page with mixed content, relative canonical, missing viewport
	page2 := ParsedPageRecord{
		ID:           uuid.New(),
		AuditID:      auditID,
		TenantID:     tenantID,
		URL:          "https://example.com/page-mixed",
		StatusCode:   200,
		CanonicalURL: "/relative/canonical", // Relative!
		Viewport:     "",                    // Missing viewport!
		MetaRobots:   "noindex, follow",     // Noindex!
		Images: []PageImage{
			{Src: "http://insecure-cdn.com/image.jpg", HasAlt: true}, // Mixed content!
		},
		Hreflangs: []HreflangTag{
			{Hreflang: "es-MX", Href: "https://example.com/page-spanish"},
			{Hreflang: "INVALID_LANG_CODE_1234", Href: "https://example.com/page-invalid"},
		},
	}

	// Page 3: Target of hreflang (does NOT link back to Page 2)
	page3 := ParsedPageRecord{
		ID:           uuid.New(),
		AuditID:      auditID,
		TenantID:     tenantID,
		URL:          "https://example.com/page-spanish",
		StatusCode:   200,
		CanonicalURL: "https://example.com/page-spanish",
		Viewport:     "width=device-width, initial-scale=1.0",
		Hreflangs:    []HreflangTag{}, // Missing reciprocal link back to Page 2!
	}

	// Page 4 & 5: Canonical Chain Conflict: Page 4 -> Canonical 5, Page 5 -> Canonical external
	page4 := ParsedPageRecord{
		ID:           uuid.New(),
		AuditID:      auditID,
		TenantID:     tenantID,
		URL:          "https://example.com/page-chain-a",
		StatusCode:   200,
		CanonicalURL: "https://example.com/page-chain-b",
		Viewport:     "width=device-width, initial-scale=1.0",
	}

	page5 := ParsedPageRecord{
		ID:           uuid.New(),
		AuditID:      auditID,
		TenantID:     tenantID,
		URL:          "https://example.com/page-chain-b",
		StatusCode:   200,
		CanonicalURL: "https://otherdomain.com/final-canonical", // External domain + chain conflict!
		Viewport:     "width=device-width, initial-scale=1.0",
	}

	// Page 6: Overly long URL with excessive parameters
	page6 := ParsedPageRecord{
		ID:           uuid.New(),
		AuditID:      auditID,
		TenantID:     tenantID,
		URL:          "https://example.com/products/items/category/subcategory/details?id=992&filter=active&sort=desc&page=2&ref=banner_campaign_promo_code_summer_sale",
		StatusCode:   200,
		CanonicalURL: "https://example.com/products/items",
		Viewport:     "width=device-width, initial-scale=1.0",
	}

	pages := []ParsedPageRecord{page1, page2, page3, page4, page5, page6}
	findings := AuditPages(pages)

	findingsByRule := make(map[string][]Finding)
	for _, f := range findings {
		findingsByRule[f.RuleID] = append(findingsByRule[f.RuleID], f)
	}

	// Check 1: Status 4xx (Page 1)
	if len(findingsByRule[RuleStatus4xx]) == 0 {
		t.Errorf("expected finding for RuleStatus4xx")
	} else if findingsByRule[RuleStatus4xx][0].Severity != SeverityCritical {
		t.Errorf("expected RuleStatus4xx severity critical, got %s", findingsByRule[RuleStatus4xx][0].Severity)
	}

	// Check 2: Mixed Content (Page 2)
	if len(findingsByRule[RuleMixedContent]) == 0 {
		t.Errorf("expected finding for RuleMixedContent")
	} else if findingsByRule[RuleMixedContent][0].Severity != SeverityCritical {
		t.Errorf("expected RuleMixedContent severity critical, got %s", findingsByRule[RuleMixedContent][0].Severity)
	}

	// Check 3: Relative Canonical (Page 2)
	if len(findingsByRule[RuleCanonicalRelative]) == 0 {
		t.Errorf("expected finding for RuleCanonicalRelative")
	}

	// Check 4: Missing Viewport (Page 2)
	if len(findingsByRule[RuleViewportMissing]) == 0 {
		t.Errorf("expected finding for RuleViewportMissing")
	}

	// Check 5: Meta Noindex (Page 2)
	if len(findingsByRule[RuleMetaNoindex]) == 0 {
		t.Errorf("expected finding for RuleMetaNoindex")
	}

	// Check 6: hreflang Invalid Code & Missing Reciprocal (Page 2 & 3)
	if len(findingsByRule[RuleHreflangInvalidCode]) == 0 {
		t.Errorf("expected finding for RuleHreflangInvalidCode")
	}
	if len(findingsByRule[RuleHreflangMissingRecip]) == 0 {
		t.Errorf("expected finding for RuleHreflangMissingRecip")
	}

	// Check 7: Canonical Chain Conflict & External Domain Canonical (Page 4 & 5)
	if len(findingsByRule[RuleCanonicalChainConflict]) == 0 {
		t.Errorf("expected finding for RuleCanonicalChainConflict")
	}
	if len(findingsByRule[RuleCanonicalExternalDomain]) == 0 {
		t.Errorf("expected finding for RuleCanonicalExternalDomain")
	}

	// Check 8: URL Too Long & Excessive Parameters (Page 6)
	if len(findingsByRule[RuleURLTooLong]) == 0 {
		t.Errorf("expected finding for RuleURLTooLong")
	}
	if len(findingsByRule[RuleURLExcessiveParams]) == 0 {
		t.Errorf("expected finding for RuleURLExcessiveParams")
	}
	if len(findingsByRule[RuleURLNonDescriptiveID]) == 0 {
		t.Errorf("expected finding for RuleURLNonDescriptiveID")
	}

	// Validate Finding Schema cleanliness
	for _, f := range findings {
		if f.AuditID != auditID {
			t.Errorf("finding audit ID mismatch")
		}
		if f.TenantID != tenantID {
			t.Errorf("finding tenant ID mismatch")
		}
		if f.Category != "technical" {
			t.Errorf("finding category mismatch: %s", f.Category)
		}
		if f.Severity != SeverityCritical && f.Severity != SeverityWarning && f.Severity != SeverityInfo {
			t.Errorf("invalid severity: %s", f.Severity)
		}
		if f.Message == "" {
			t.Errorf("finding message is empty for rule: %s", f.RuleID)
		}
	}
}
