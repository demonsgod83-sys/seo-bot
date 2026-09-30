package report

import (
	"bytes"
	"fmt"

	"github.com/go-pdf/fpdf"
)

type PDFRenderer struct{}

func NewPDFRenderer() *PDFRenderer {
	return &PDFRenderer{}
}

func (r *PDFRenderer) Render(data *ReportData) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Title & Domain
	pdf.SetFont("Arial", "B", 18)
	pdf.SetTextColor(30, 41, 59)
	pdf.Cell(0, 10, fmt.Sprintf("SEO Audit Report: %s", data.Domain))
	pdf.Ln(8)

	// Metadata Line
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(100, 116, 139)
	metaStr := fmt.Sprintf("Target: %s | Date: %s | Version: v%d | Pages: %d",
		data.NormalizedURL, data.GeneratedAt.Format("Jan 02, 2006 15:04 UTC"), data.Version, data.TotalPagesScored)
	pdf.Cell(0, 6, metaStr)
	pdf.Ln(10)

	// Warnings Banner (if any)
	if data.HasWarnings {
		pdf.SetFillColor(254, 243, 199)
		pdf.SetDrawColor(245, 158, 11)
		pdf.SetTextColor(180, 83, 9)
		pdf.SetFont("Arial", "B", 10)
		pdf.CellFormat(0, 7, "  Audit Completed With Warnings", "1", 1, "L", true, 0, "")
		pdf.SetFont("Arial", "", 8)
		for _, w := range data.Warnings {
			pdf.CellFormat(0, 5, fmt.Sprintf("   * %s", w), "LR", 1, "L", true, 0, "")
		}
		pdf.CellFormat(0, 2, "", "LBR", 1, "L", true, 0, "")
		pdf.Ln(4)
	}

	// Score Overview Box
	pdf.SetFillColor(241, 245, 249)
	pdf.SetDrawColor(203, 213, 225)
	pdf.SetTextColor(30, 41, 59)
	pdf.Rect(15, pdf.GetY(), 180, 25, "DF")

	currY := pdf.GetY()
	// Overall Score
	pdf.SetXY(20, currY+3)
	pdf.SetFont("Arial", "B", 8)
	pdf.SetTextColor(100, 116, 139)
	pdf.Cell(35, 4, "OVERALL SCORE")
	pdf.SetXY(20, currY+8)
	pdf.SetFont("Arial", "B", 20)
	if data.OverallScore >= 80 {
		pdf.SetTextColor(16, 185, 129)
	} else if data.OverallScore >= 50 {
		pdf.SetTextColor(245, 158, 11)
	} else {
		pdf.SetTextColor(239, 68, 68)
	}
	pdf.Cell(35, 12, fmt.Sprintf("%d/100", data.OverallScore))

	// Technical Score
	pdf.SetXY(65, currY+4)
	pdf.SetFont("Arial", "B", 8)
	pdf.SetTextColor(100, 116, 139)
	pdf.Cell(35, 4, "TECHNICAL HEALTH")
	pdf.SetXY(65, currY+10)
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(30, 41, 59)
	pdf.Cell(35, 8, fmt.Sprintf("%d/100", data.TechnicalScore))

	// On-Page Score
	pdf.SetXY(110, currY+4)
	pdf.SetFont("Arial", "B", 8)
	pdf.SetTextColor(100, 116, 139)
	pdf.Cell(35, 4, "ON-PAGE QUALITY")
	pdf.SetXY(110, currY+10)
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(30, 41, 59)
	pdf.Cell(35, 8, fmt.Sprintf("%d/100", data.OnPageScore))

	// Content Score
	pdf.SetXY(155, currY+4)
	pdf.SetFont("Arial", "B", 8)
	pdf.SetTextColor(100, 116, 139)
	pdf.Cell(35, 4, "CONTENT & KEYWORDS")
	pdf.SetXY(155, currY+10)
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(30, 41, 59)
	pdf.Cell(35, 8, fmt.Sprintf("%d/100", data.ContentScore))

	pdf.SetXY(15, currY+30)

	// Summary Statistics Line
	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(71, 85, 105)
	statsStr := fmt.Sprintf("Findings: %d Total | %d Critical | %d Warnings | %d Notices | Target Keywords: %d",
		data.TotalFindings, data.CriticalCount, data.WarningCount, data.InfoCount, data.TotalKeywords)
	pdf.Cell(0, 6, statsStr)
	pdf.Ln(8)

	// Section: Prioritized Issues
	pdf.SetFont("Arial", "B", 12)
	pdf.SetTextColor(30, 41, 59)
	pdf.Cell(0, 8, fmt.Sprintf("Prioritized Issues (%d Actionable Items)", len(data.PrioritizedIssues)))
	pdf.Ln(6)

	// Table Header
	pdf.SetFillColor(30, 41, 59)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 8)
	pdf.CellFormat(75, 6, " Issue / Rule", "1", 0, "L", true, 0, "")
	pdf.CellFormat(25, 6, "Severity", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 6, "Effort", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 6, "Priority", "1", 0, "C", true, 0, "")
	pdf.CellFormat(30, 6, "Affected Pages", "1", 1, "C", true, 0, "")

	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(30, 41, 59)
	fill := false
	for _, issue := range data.PrioritizedIssues {
		if fill {
			pdf.SetFillColor(248, 250, 252)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}

		title := issue.Title
		if len(title) > 42 {
			title = title[:39] + "..."
		}

		pdf.CellFormat(75, 6, " "+title, "1", 0, "L", fill, 0, "")
		pdf.CellFormat(25, 6, issue.Severity, "1", 0, "C", fill, 0, "")
		pdf.CellFormat(25, 6, issue.EffortTier, "1", 0, "C", fill, 0, "")
		pdf.CellFormat(25, 6, fmt.Sprintf("%.2f", issue.PriorityScore), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(30, 6, fmt.Sprintf("%d pages", issue.AffectedPagesCount), "1", 1, "C", fill, 0, "")
		fill = !fill
	}
	pdf.Ln(6)

	// Section: Per-Page Scores Breakdown
	if len(data.PageBreakdown) > 0 {
		pdf.SetFont("Arial", "B", 12)
		pdf.SetTextColor(30, 41, 59)
		pdf.Cell(0, 8, "Per-Page Score Breakdown")
		pdf.Ln(6)

		pdf.SetFillColor(30, 41, 59)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 8)
		pdf.CellFormat(80, 6, " Page URL", "1", 0, "L", true, 0, "")
		pdf.CellFormat(20, 6, "Overall", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 6, "Technical", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 6, "On-Page", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 6, "Content", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 6, "Crit/Warn", "1", 1, "C", true, 0, "")

		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(30, 41, 59)
		fill = false
		for _, page := range data.PageBreakdown {
			if fill {
				pdf.SetFillColor(248, 250, 252)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}

			u := page.PageURL
			if len(u) > 46 {
				u = u[:43] + "..."
			}

			pdf.CellFormat(80, 6, " "+u, "1", 0, "L", fill, 0, "")
			pdf.CellFormat(20, 6, fmt.Sprintf("%d", page.OverallScore), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(20, 6, fmt.Sprintf("%d", page.TechnicalScore), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(20, 6, fmt.Sprintf("%d", page.OnPageScore), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(20, 6, fmt.Sprintf("%d", page.ContentScore), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(20, 6, fmt.Sprintf("%d / %d", page.CriticalCount, page.WarningCount), "1", 1, "C", fill, 0, "")
			fill = !fill
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to output PDF: %w", err)
	}

	return buf.Bytes(), nil
}
