package analyzer

import (
	"testing"

	"github.com/google/uuid"
)

func TestAnalyzePages_RuleSuite(t *testing.T) {
	auditID := uuid.New()
	tenantID := uuid.New()

	// Page 1: Broken Page (missing H1, missing alt, thin content, duplicate title with Page 2)
	page1 := ParsedPageRecord{
		ID:          uuid.New(),
		AuditID:     auditID,
		TenantID:    tenantID,
		URL:         "https://example.com/products/blue-widget",
		StatusCode:  200,
		ParseStatus: "success",
		Title:       "Best Widgets 2026", // 17 chars (too short + duplicate)
		MetaDescription: "Shop our collection of high quality products with free worldwide shipping on orders.", // 85 chars (valid)
		Headings: []Heading{
			{Level: 2, Text: "Product Overview"},
			{Level: 4, Text: "Specifications"}, // Skipped H3!
		},
		Images: []PageImage{
			{Src: "https://example.com/img1.jpg", HasAlt: false},
			{Src: "https://example.com/img2.jpg", Alt: "img2", HasAlt: true}, // repeats filename!
		},
		Links: []PageLink{
			{Href: "https://example.com/orphan-subpage", Text: "click here", IsInternal: true},
		},
		BodyText:  "This is a very short text with under twenty words.",
		WordCount: 10, // Thin content!
	}

	// Page 2: Second page sharing the same title (to trigger title duplicate)
	page2 := ParsedPageRecord{
		ID:          uuid.New(),
		AuditID:     auditID,
		TenantID:    tenantID,
		URL:         "https://example.com/products/red-widget",
		StatusCode:  200,
		ParseStatus: "success",
		Title:       "Best Widgets 2026", // Duplicate title with page1
		MetaDescription: "",               // Missing meta desc
		Headings: []Heading{
			{Level: 1, Text: "Red Widget 1"},
			{Level: 1, Text: "Red Widget 2"}, // Multiple H1!
		},
		BodyText:  "Another short page text.",
		WordCount: 4,
	}

	// Page 3: Orphan page (nobody links to it)
	page3 := ParsedPageRecord{
		ID:          uuid.New(),
		AuditID:     auditID,
		TenantID:    tenantID,
		URL:         "https://example.com/secret-landing",
		StatusCode:  200,
		ParseStatus: "success",
		Title:       "Secret Landing Page For Special Campaigns", // 42 chars (valid)
		MetaDescription: "Valid meta description text exceeding seventy characters for testing purpose.",
		Headings: []Heading{
			{Level: 1, Text: "Special Campaigns"},
		},
		BodyText:  "Substantive page content with enough words to avoid thin content...",
		WordCount: 300,
	}

	pages := []ParsedPageRecord{page1, page2, page3}
	findings := AnalyzePages(pages)

	// Map findings by RuleID
	findingsByRule := make(map[string][]Finding)
	for _, f := range findings {
		findingsByRule[f.RuleID] = append(findingsByRule[f.RuleID], f)
	}

	// Check 1: Missing H1 (Page 1)
	if len(findingsByRule[RuleH1Missing]) == 0 {
		t.Errorf("expected finding for RuleH1Missing")
	} else if findingsByRule[RuleH1Missing][0].Severity != SeverityCritical {
		t.Errorf("expected RuleH1Missing severity critical, got %s", findingsByRule[RuleH1Missing][0].Severity)
	}

	// Check 2: Multiple H1 (Page 2)
	if len(findingsByRule[RuleH1Multiple]) == 0 {
		t.Errorf("expected finding for RuleH1Multiple")
	} else if findingsByRule[RuleH1Multiple][0].Severity != SeverityWarning {
		t.Errorf("expected RuleH1Multiple severity warning, got %s", findingsByRule[RuleH1Multiple][0].Severity)
	}

	// Check 3: Heading Skipped Level H2 -> H4 (Page 1)
	if len(findingsByRule[RuleHeadingSkippedLevel]) == 0 {
		t.Errorf("expected finding for RuleHeadingSkippedLevel")
	}

	// Check 4: Missing Image Alt & Filename Alt (Page 1)
	if len(findingsByRule[RuleImageAltMissing]) == 0 {
		t.Errorf("expected finding for RuleImageAltMissing")
	}
	if len(findingsByRule[RuleImageAltIsFilename]) == 0 {
		t.Errorf("expected finding for RuleImageAltIsFilename")
	}

	// Check 5: Title Too Short & Title Duplicate (Page 1 & 2)
	if len(findingsByRule[RuleTitleTooShort]) == 0 {
		t.Errorf("expected finding for RuleTitleTooShort")
	}
	if len(findingsByRule[RuleTitleDuplicate]) < 2 {
		t.Errorf("expected duplicate title findings on both pages, got %d", len(findingsByRule[RuleTitleDuplicate]))
	}

	// Check 6: Meta Description Missing (Page 2)
	if len(findingsByRule[RuleMetaDescMissing]) == 0 {
		t.Errorf("expected finding for RuleMetaDescMissing")
	}

	// Check 7: Thin Content (Page 1 & Page 2)
	if len(findingsByRule[RuleThinContent]) != 2 {
		t.Errorf("expected 2 thin content findings, got %d", len(findingsByRule[RuleThinContent]))
	}

	// Check 8: Generic Anchor Text (Page 1)
	if len(findingsByRule[RuleGenericAnchorText]) == 0 {
		t.Errorf("expected finding for RuleGenericAnchorText")
	}

	// Check 9: Orphan Page (Page 3 has no inbound links)
	if len(findingsByRule[RuleOrphanPage]) == 0 {
		t.Errorf("expected finding for RuleOrphanPage")
	}

	// Validate Finding Schema cleanliness
	for _, f := range findings {
		if f.AuditID != auditID {
			t.Errorf("finding audit ID mismatch")
		}
		if f.TenantID != tenantID {
			t.Errorf("finding tenant ID mismatch")
		}
		if f.Category != "onpage" {
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
