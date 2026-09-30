package scoring

import (
	"testing"

	"github.com/google/uuid"
)

func TestEngine_DeductionsAndFlooring(t *testing.T) {
	cfg := DefaultEngineConfig()
	engine := NewEngine(cfg)

	auditID := uuid.New()
	tenantID := uuid.New()
	domain := "example.com"
	pageURL := "https://example.com/page1"

	pages := []ParsedPageRecord{
		{
			AuditID:     auditID,
			TenantID:    tenantID,
			URL:         pageURL,
			StatusCode:  200,
			ParseStatus: "success",
		},
	}

	// 10 critical technical findings -> 10 * 15 = 150 points deduction -> floored at 0
	var findings []Finding
	for i := 0; i < 10; i++ {
		findings = append(findings, Finding{
			AuditID:  auditID,
			TenantID: tenantID,
			PageURL:  pageURL,
			Category: "technical",
			RuleID:   "technical.status_5xx",
			Severity: SeverityCritical,
			Message:  "5xx error",
		})
	}

	res := engine.Compute(auditID, tenantID, domain, pages, findings)

	if res.AuditScore.TechnicalScore != 0 {
		t.Errorf("expected technical score floored at 0, got %d", res.AuditScore.TechnicalScore)
	}
	if res.AuditScore.OnPageScore != 100 {
		t.Errorf("expected onpage score 100 (no findings), got %d", res.AuditScore.OnPageScore)
	}
	if res.AuditScore.ContentScore != 100 {
		t.Errorf("expected content score 100 (no findings), got %d", res.AuditScore.ContentScore)
	}

	// Overall: 0.40 * 0 + 0.35 * 100 + 0.25 * 100 = 60
	expectedOverall := 60
	if res.AuditScore.OverallScore != expectedOverall {
		t.Errorf("expected overall score %d, got %d", expectedOverall, res.AuditScore.OverallScore)
	}

	if res.AuditScore.CriticalCount != 10 {
		t.Errorf("expected critical count 10, got %d", res.AuditScore.CriticalCount)
	}
}

func TestEngine_WeightedOverallScore(t *testing.T) {
	cfg := DefaultEngineConfig()
	engine := NewEngine(cfg)

	auditID := uuid.New()
	tenantID := uuid.New()
	domain := "example.com"
	pageURL := "https://example.com"

	pages := []ParsedPageRecord{
		{
			AuditID:     auditID,
			TenantID:    tenantID,
			URL:         pageURL,
			StatusCode:  200,
			ParseStatus: "success",
		},
	}

	// 1 warning in technical (-5 -> 95)
	// 2 warnings in onpage (-10 -> 90)
	// 1 critical in keywords (-15 -> 85)
	findings := []Finding{
		{
			AuditID:  auditID,
			TenantID: tenantID,
			PageURL:  pageURL,
			Category: "technical",
			RuleID:   "technical.canonical_missing",
			Severity: SeverityWarning,
			Message:  "missing canonical",
		},
		{
			AuditID:  auditID,
			TenantID: tenantID,
			PageURL:  pageURL,
			Category: "onpage",
			RuleID:   "onpage.title_too_short",
			Severity: SeverityWarning,
			Message:  "title too short",
		},
		{
			AuditID:  auditID,
			TenantID: tenantID,
			PageURL:  pageURL,
			Category: "onpage",
			RuleID:   "onpage.meta_description_missing",
			Severity: SeverityWarning,
			Message:  "meta desc missing",
		},
		{
			AuditID:  auditID,
			TenantID: tenantID,
			PageURL:  pageURL,
			Category: "keywords",
			RuleID:   "keywords.cannibalization",
			Severity: SeverityCritical,
			Message:  "cannibalization",
		},
	}

	res := engine.Compute(auditID, tenantID, domain, pages, findings)

	if res.AuditScore.TechnicalScore != 95 {
		t.Errorf("expected tech score 95, got %d", res.AuditScore.TechnicalScore)
	}
	if res.AuditScore.OnPageScore != 90 {
		t.Errorf("expected onpage score 90, got %d", res.AuditScore.OnPageScore)
	}
	if res.AuditScore.ContentScore != 85 {
		t.Errorf("expected content score 85, got %d", res.AuditScore.ContentScore)
	}

	// Overall: 0.40 * 95 (38) + 0.35 * 90 (31.5) + 0.25 * 85 (21.25) = 90.75 -> 91
	if res.AuditScore.OverallScore != 91 {
		t.Errorf("expected overall score 91, got %d", res.AuditScore.OverallScore)
	}
}

func TestEngine_IssueRollupAndPrioritization(t *testing.T) {
	cfg := DefaultEngineConfig()
	engine := NewEngine(cfg)

	auditID := uuid.New()
	tenantID := uuid.New()
	domain := "example.com"

	pages := []ParsedPageRecord{
		{AuditID: auditID, TenantID: tenantID, URL: "https://example.com/", StatusCode: 200, ParseStatus: "success"},
		{AuditID: auditID, TenantID: tenantID, URL: "https://example.com/blog", StatusCode: 200, ParseStatus: "success"},
		{AuditID: auditID, TenantID: tenantID, URL: "https://example.com/blog/post-1", StatusCode: 200, ParseStatus: "success"},
	}

	findings := []Finding{
		// Easy fix on homepage (High Impact, Low Effort -> High Priority)
		{
			AuditID:  auditID,
			TenantID: tenantID,
			PageURL:  "https://example.com/",
			Category: "onpage",
			RuleID:   "onpage.title_missing",
			Severity: SeverityCritical,
			Message:  "Missing Title Tag on Homepage",
		},
		// Hard fix on deep page (Lower Impact, High Effort -> Lower Priority)
		{
			AuditID:  auditID,
			TenantID: tenantID,
			PageURL:  "https://example.com/blog/post-1",
			Category: "onpage",
			RuleID:   "onpage.thin_content",
			Severity: SeverityWarning,
			Message:  "Thin content on post-1",
		},
		// Repeated issue across multiple pages to test rollup
		{
			AuditID:  auditID,
			TenantID: tenantID,
			PageURL:  "https://example.com/",
			Category: "onpage",
			RuleID:   "onpage.image_missing_alt",
			Severity: SeverityWarning,
			Message:  "Images missing alt on homepage",
		},
		{
			AuditID:  auditID,
			TenantID: tenantID,
			PageURL:  "https://example.com/blog",
			Category: "onpage",
			RuleID:   "onpage.image_missing_alt",
			Severity: SeverityWarning,
			Message:  "Images missing alt on blog",
		},
	}

	res := engine.Compute(auditID, tenantID, domain, pages, findings)

	// Verify Rollup
	var altIssue *PrioritizedIssue
	for i, issue := range res.PrioritizedIssues {
		if issue.RuleID == "onpage.image_missing_alt" {
			altIssue = &res.PrioritizedIssues[i]
			break
		}
	}

	if altIssue == nil {
		t.Fatalf("expected rolled up issue for onpage.image_missing_alt")
	}

	if altIssue.AffectedPagesCount != 2 {
		t.Errorf("expected 2 affected pages for image_missing_alt, got %d", altIssue.AffectedPagesCount)
	}
	if len(altIssue.AffectedURLs) != 2 {
		t.Errorf("expected 2 affected URLs, got %d", len(altIssue.AffectedURLs))
	}

	// Verify Priority Ordering: First issue should have higher priority score than last issue
	if len(res.PrioritizedIssues) < 3 {
		t.Fatalf("expected at least 3 prioritized issues, got %d", len(res.PrioritizedIssues))
	}

	for i := 0; i < len(res.PrioritizedIssues)-1; i++ {
		if res.PrioritizedIssues[i].PriorityScore < res.PrioritizedIssues[i+1].PriorityScore {
			t.Errorf("issues not sorted descending by priority: [%d]=%f, [%d]=%f",
				i, res.PrioritizedIssues[i].PriorityScore,
				i+1, res.PrioritizedIssues[i+1].PriorityScore,
			)
		}
	}
}

func TestEngine_PageImportanceMultiplier(t *testing.T) {
	cfg := DefaultEngineConfig()
	engine := NewEngine(cfg)

	homeMult := engine.calculatePageImportance("https://example.com/", "example.com", 0)
	if homeMult != 2.0 {
		t.Errorf("expected homepage multiplier 2.0, got %f", homeMult)
	}

	depth1Mult := engine.calculatePageImportance("https://example.com/pricing", "example.com", 0)
	if depth1Mult != 1.5 {
		t.Errorf("expected depth 1 multiplier 1.5, got %f", depth1Mult)
	}

	depth2Mult := engine.calculatePageImportance("https://example.com/blog/post-1", "example.com", 0)
	if depth2Mult != 1.2 {
		t.Errorf("expected depth 2 multiplier 1.2, got %f", depth2Mult)
	}

	// With inbound links boost
	boostedMult := engine.calculatePageImportance("https://example.com/pricing", "example.com", 5)
	if boostedMult <= depth1Mult {
		t.Errorf("expected boosted multiplier > base depth multiplier, got %f vs %f", boostedMult, depth1Mult)
	}
}
