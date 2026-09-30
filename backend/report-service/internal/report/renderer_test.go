package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func sampleReportData(hasWarnings bool) *ReportData {
	auditID := uuid.New()
	tenantID := uuid.New()

	var warnings []string
	if hasWarnings {
		warnings = append(warnings, "Backlink analysis was unavailable for this audit (module timed out).")
	}

	var urls []string
	for i := 1; i <= 200; i++ {
		urls = append(urls, "https://example.com/item-"+string(rune('a'+(i%26))))
	}

	return &ReportData{
		AuditID:          auditID,
		TenantID:         tenantID,
		Domain:           "example.com",
		NormalizedURL:    "https://example.com",
		Status:           "completed",
		SubmittedAt:      time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC),
		GeneratedAt:      time.Date(2026, 9, 27, 8, 15, 0, 0, time.UTC),
		Version:          1,
		ShareToken:       "abc123token456",
		ShareURL:         "http://localhost:8105/public/reports/abc123token456",
		HasWarnings:      hasWarnings,
		Warnings:         warnings,
		OverallScore:     87,
		TechnicalScore:   95,
		OnPageScore:      80,
		ContentScore:     85,
		TotalFindings:    12,
		CriticalCount:    1,
		WarningCount:     8,
		InfoCount:        3,
		TotalPagesScored: 200,
		PrioritizedIssues: []ReportPrioritizedIssue{
			{
				RuleID:             "onpage.image_missing_alt",
				Category:           "onpage",
				Severity:           "warning",
				Title:              "Missing Image Alt Attributes",
				Message:            "Images are missing alt descriptions",
				EffortTier:         "low",
				ImpactScore:        240.0,
				PriorityScore:      240.0,
				AffectedPagesCount: 200,
				AffectedURLs:       urls,
			},
			{
				RuleID:             "technical.status_5xx",
				Category:           "technical",
				Severity:           "critical",
				Title:              "Server Error (5xx Internal Error)",
				Message:            "500 Internal Server Error returned",
				EffortTier:         "high",
				ImpactScore:        30.0,
				PriorityScore:      10.0,
				AffectedPagesCount: 1,
				AffectedURLs:       []string{"https://example.com/broken"},
			},
		},
		PageBreakdown: []ReportPageBreakdown{
			{
				PageURL:        "https://example.com",
				OverallScore:   92,
				TechnicalScore: 100,
				OnPageScore:    85,
				ContentScore:   90,
				CriticalCount:  0,
				WarningCount:   2,
				InfoCount:      1,
			},
			{
				PageURL:        "https://example.com/broken",
				OverallScore:   40,
				TechnicalScore: 0,
				OnPageScore:    80,
				ContentScore:   70,
				CriticalCount:  1,
				WarningCount:   1,
				InfoCount:      0,
			},
		},
		KeywordMap: []ReportKeywordItem{
			{
				Keyword:     "seo audit tool",
				TargetPages: []string{"https://example.com", "https://example.com/features"},
				PageCount:   2,
			},
		},
		TotalKeywords:        15,
		CannibalizationCount: 0,
	}
}

func TestRenderer_Consistency(t *testing.T) {
	data := sampleReportData(false)

	// 1. Test JSON Renderer
	jsonRenderer := NewJSONRenderer()
	jsonBytes, err := jsonRenderer.Render(data)
	if err != nil {
		t.Fatalf("JSON rendering failed: %v", err)
	}

	var jsonOutput ReportData
	if err := json.Unmarshal(jsonBytes, &jsonOutput); err != nil {
		t.Fatalf("JSON output failed to unmarshal: %v", err)
	}

	if jsonOutput.OverallScore != 87 || jsonOutput.TechnicalScore != 95 || jsonOutput.OnPageScore != 80 || jsonOutput.ContentScore != 85 {
		t.Errorf("JSON scores mismatch: %+v", jsonOutput)
	}

	// 2. Test HTML Renderer
	htmlRenderer := NewHTMLRenderer()
	htmlBytes, err := htmlRenderer.Render(data)
	if err != nil {
		t.Fatalf("HTML rendering failed: %v", err)
	}

	htmlStr := string(htmlBytes)
	if !strings.Contains(htmlStr, "example.com") {
		t.Errorf("HTML missing domain: %s", htmlStr)
	}
	if !strings.Contains(htmlStr, ">87<") || !strings.Contains(htmlStr, ">95<") {
		t.Errorf("HTML missing expected scores (87, 95)")
	}
	if !strings.Contains(htmlStr, "Missing Image Alt Attributes") {
		t.Errorf("HTML missing prioritized issue title")
	}
	if !strings.Contains(htmlStr, "Affected Pages (200 pages)") {
		t.Errorf("HTML missing rolled up 200 pages summary tag")
	}

	// 3. Test PDF Renderer
	pdfRenderer := NewPDFRenderer()
	pdfBytes, err := pdfRenderer.Render(data)
	if err != nil {
		t.Fatalf("PDF rendering failed: %v", err)
	}

	if len(pdfBytes) < 500 {
		t.Errorf("PDF output unexpectedly small (%d bytes)", len(pdfBytes))
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Errorf("PDF output does not have standard PDF header")
	}
}

func TestRenderer_CompletedWithWarnings(t *testing.T) {
	data := sampleReportData(true)

	// JSON
	jsonRenderer := NewJSONRenderer()
	jsonBytes, err := jsonRenderer.Render(data)
	if err != nil {
		t.Fatalf("JSON rendering failed: %v", err)
	}

	var jsonOutput ReportData
	_ = json.Unmarshal(jsonBytes, &jsonOutput)
	if !jsonOutput.HasWarnings || len(jsonOutput.Warnings) == 0 {
		t.Errorf("expected HasWarnings true in JSON")
	}

	// HTML
	htmlRenderer := NewHTMLRenderer()
	htmlBytes, err := htmlRenderer.Render(data)
	if err != nil {
		t.Fatalf("HTML rendering failed: %v", err)
	}

	htmlStr := string(htmlBytes)
	if !strings.Contains(htmlStr, "Audit Completed With Warnings") {
		t.Errorf("HTML missing warnings banner")
	}
	if !strings.Contains(htmlStr, "Backlink analysis was unavailable") {
		t.Errorf("HTML missing specific warning text")
	}

	// PDF
	pdfRenderer := NewPDFRenderer()
	pdfBytes, err := pdfRenderer.Render(data)
	if err != nil {
		t.Fatalf("PDF rendering failed with warnings: %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Errorf("PDF empty")
	}
}
