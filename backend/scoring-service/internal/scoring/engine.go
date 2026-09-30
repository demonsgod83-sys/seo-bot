package scoring

import (
	"math"
	"net/url"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// EngineConfig holds configuration parameters for the scoring calculations.
type EngineConfig struct {
	CriticalDeduction float64
	WarningDeduction  float64
	InfoDeduction     float64
	WeightTechnical   float64
	WeightOnPage      float64
	WeightContent     float64
}

// DefaultEngineConfig returns sensible defaults matching the SEO audit guidelines.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		CriticalDeduction: 15.0,
		WarningDeduction:  5.0,
		InfoDeduction:     1.0,
		WeightTechnical:   0.40,
		WeightOnPage:      0.35,
		WeightContent:     0.25,
	}
}

// Engine encapsulates scoring algorithms, issue rollup, and impact-to-effort prioritization.
type Engine struct {
	cfg EngineConfig
}

func NewEngine(cfg EngineConfig) *Engine {
	// Normalize weights if they do not sum to 1.0
	totalWeight := cfg.WeightTechnical + cfg.WeightOnPage + cfg.WeightContent
	if totalWeight > 0 && math.Abs(totalWeight-1.0) > 0.001 {
		cfg.WeightTechnical /= totalWeight
		cfg.WeightOnPage /= totalWeight
		cfg.WeightContent /= totalWeight
	}
	return &Engine{cfg: cfg}
}

// CalculationResult bundles all computed scoring artifacts for persistence.
type CalculationResult struct {
	AuditScore        AuditScore
	PageScores        []PageScore
	PrioritizedIssues []PrioritizedIssue
}

// Compute processes all findings and parsed pages to produce scores and prioritized issues.
func (e *Engine) Compute(
	auditID, tenantID uuid.UUID,
	domain string,
	pages []ParsedPageRecord,
	findings []Finding,
) CalculationResult {
	// 1. Build Inbound Links Map for Page Importance
	inboundCounts := e.buildInboundLinksMap(pages)

	// 2. Map pages by URL to ensure all crawled/parsed pages are accounted for
	pageMap := make(map[string]bool)
	for _, p := range pages {
		pageMap[p.URL] = true
	}
	// Also ensure any finding page_urls are known
	for _, f := range findings {
		if f.PageURL != "" {
			pageMap[f.PageURL] = true
		}
	}

	// 3. Group findings by page and calculate per-page scores
	pageFindings := make(map[string][]Finding)
	for _, f := range findings {
		pageFindings[f.PageURL] = append(pageFindings[f.PageURL], f)
	}

	var pageScores []PageScore
	var totalTechScore, totalOnpageScore, totalContentScore float64

	criticalCount := 0
	warningCount := 0
	infoCount := 0

	for pageURL := range pageMap {
		ff := pageFindings[pageURL]

		var techDeduction, onpageDeduction, contentDeduction float64
		var pCrit, pWarn, pInfo int

		for _, f := range ff {
			deduction := e.getSeverityDeduction(f.Severity)
			switch f.Severity {
			case SeverityCritical:
				pCrit++
				criticalCount++
			case SeverityWarning:
				pWarn++
				warningCount++
			case SeverityInfo:
				pInfo++
				infoCount++
			}

			cat := strings.ToLower(f.Category)
			switch cat {
			case "technical":
				techDeduction += deduction
			case "onpage":
				onpageDeduction += deduction
			case "keywords", "content":
				contentDeduction += deduction
			default:
				onpageDeduction += deduction
			}
		}

		pTech := int(math.Max(0, math.Round(100.0-techDeduction)))
		pOnpage := int(math.Max(0, math.Round(100.0-onpageDeduction)))
		pContent := int(math.Max(0, math.Round(100.0-contentDeduction)))

		pOverall := int(math.Round(
			e.cfg.WeightTechnical*float64(pTech) +
				e.cfg.WeightOnPage*float64(pOnpage) +
				e.cfg.WeightContent*float64(pContent),
		))
		if pOverall > 100 {
			pOverall = 100
		} else if pOverall < 0 {
			pOverall = 0
		}

		totalTechScore += float64(pTech)
		totalOnpageScore += float64(pOnpage)
		totalContentScore += float64(pContent)

		pageScores = append(pageScores, PageScore{
			AuditID:        auditID,
			TenantID:       tenantID,
			PageURL:        pageURL,
			OverallScore:   pOverall,
			TechnicalScore: pTech,
			OnPageScore:    pOnpage,
			ContentScore:   pContent,
			CriticalCount:  pCrit,
			WarningCount:   pWarn,
			InfoCount:      pInfo,
		})
	}

	// 4. Compute Site-wide Category & Overall Scores
	numPages := float64(len(pageMap))
	siteTech := 100
	siteOnpage := 100
	siteContent := 100
	siteOverall := 100

	if numPages > 0 {
		siteTech = int(math.Round(totalTechScore / numPages))
		siteOnpage = int(math.Round(totalOnpageScore / numPages))
		siteContent = int(math.Round(totalContentScore / numPages))

		siteOverall = int(math.Round(
			e.cfg.WeightTechnical*float64(siteTech) +
				e.cfg.WeightOnPage*float64(siteOnpage) +
				e.cfg.WeightContent*float64(siteContent),
		))
		if siteOverall > 100 {
			siteOverall = 100
		} else if siteOverall < 0 {
			siteOverall = 0
		}
	}

	auditScore := AuditScore{
		AuditID:          auditID,
		TenantID:         tenantID,
		OverallScore:     siteOverall,
		TechnicalScore:   siteTech,
		OnPageScore:      siteOnpage,
		ContentScore:     siteContent,
		TotalFindings:    len(findings),
		CriticalCount:    criticalCount,
		WarningCount:     warningCount,
		InfoCount:        infoCount,
		TotalPagesScored: len(pageMap),
	}

	// 5. Issue Rollup, Impact Calculation & Prioritization
	prioritizedIssues := e.prioritizeIssues(auditID, tenantID, domain, findings, inboundCounts)

	return CalculationResult{
		AuditScore:        auditScore,
		PageScores:        pageScores,
		PrioritizedIssues: prioritizedIssues,
	}
}

// prioritizeIssues groups findings by rule_id, calculates aggregate impact, and ranks by ROI (Impact / Effort).
func (e *Engine) prioritizeIssues(
	auditID, tenantID uuid.UUID,
	domain string,
	findings []Finding,
	inboundCounts map[string]int,
) []PrioritizedIssue {
	type ruleGroup struct {
		ruleID        string
		category      string
		severity      FindingSeverity
		sampleMessage string
		pagesSet      map[string]bool
		totalImpact   float64
	}

	groups := make(map[string]*ruleGroup)

	for _, f := range findings {
		rg, exists := groups[f.RuleID]
		if !exists {
			rg = &ruleGroup{
				ruleID:        f.RuleID,
				category:      f.Category,
				severity:      f.Severity,
				sampleMessage: f.Message,
				pagesSet:      make(map[string]bool),
			}
			groups[f.RuleID] = rg
		}

		// Elevate severity if higher severity encountered
		if e.severityRank(f.Severity) > e.severityRank(rg.severity) {
			rg.severity = f.Severity
			rg.sampleMessage = f.Message
		}

		if f.PageURL != "" {
			rg.pagesSet[f.PageURL] = true
		}

		// Calculate impact for this finding
		pageMultiplier := e.calculatePageImportance(f.PageURL, domain, inboundCounts[f.PageURL])
		baseWeight := e.getSeverityImpactWeight(f.Severity)
		findingImpact := baseWeight * pageMultiplier

		rg.totalImpact += findingImpact
	}

	var issues []PrioritizedIssue
	for ruleID, rg := range groups {
		meta := GetRuleMetadata(ruleID)

		var affectedURLs []string
		for u := range rg.pagesSet {
			affectedURLs = append(affectedURLs, u)
		}
		sort.Strings(affectedURLs)

		priorityScore := rg.totalImpact / meta.EffortCost

		issues = append(issues, PrioritizedIssue{
			AuditID:            auditID,
			TenantID:           tenantID,
			RuleID:             ruleID,
			Category:           rg.category,
			Severity:           string(rg.severity),
			Title:              meta.Title,
			Message:            rg.sampleMessage,
			EffortTier:         string(meta.EffortTier),
			ImpactScore:        math.Round(rg.totalImpact*100) / 100,
			PriorityScore:      math.Round(priorityScore*100) / 100,
			AffectedPagesCount: len(affectedURLs),
			AffectedURLs:       affectedURLs,
		})
	}

	// Sort prioritized issues descending by PriorityScore
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].PriorityScore != issues[j].PriorityScore {
			return issues[i].PriorityScore > issues[j].PriorityScore
		}
		if issues[i].ImpactScore != issues[j].ImpactScore {
			return issues[i].ImpactScore > issues[j].ImpactScore
		}
		return issues[i].RuleID < issues[j].RuleID
	})

	return issues
}

// calculatePageImportance determines weight multiplier based on depth and inbound link popularity.
func (e *Engine) calculatePageImportance(pageURL, domain string, inboundLinks int) float64 {
	if pageURL == "" {
		return 1.0
	}

	parsed, err := url.Parse(pageURL)
	if err != nil {
		return 1.0
	}

	path := strings.Trim(parsed.Path, "/")

	// 1. Base Depth Multiplier
	var depthMultiplier float64
	if path == "" {
		// Homepage / Root
		depthMultiplier = 2.0
	} else {
		segments := strings.Split(path, "/")
		depth := len(segments)
		switch depth {
		case 1:
			depthMultiplier = 1.5
		case 2:
			depthMultiplier = 1.2
		default:
			depthMultiplier = 1.0
		}
	}

	// 2. Inbound links boost factor (up to +0.5)
	inboundBoost := math.Min(0.5, float64(inboundLinks)*0.05)

	totalMultiplier := depthMultiplier * (1.0 + inboundBoost)
	return math.Min(3.0, totalMultiplier)
}

func (e *Engine) buildInboundLinksMap(pages []ParsedPageRecord) map[string]int {
	inbound := make(map[string]int)
	for _, page := range pages {
		seenOnPage := make(map[string]bool)
		for _, link := range page.Links {
			if link.IsInternal && link.Href != "" && !seenOnPage[link.Href] {
				seenOnPage[link.Href] = true
				inbound[link.Href]++
			}
		}
	}
	return inbound
}

func (e *Engine) getSeverityDeduction(sev FindingSeverity) float64 {
	switch sev {
	case SeverityCritical:
		return e.cfg.CriticalDeduction
	case SeverityWarning:
		return e.cfg.WarningDeduction
	case SeverityInfo:
		return e.cfg.InfoDeduction
	default:
		return e.cfg.InfoDeduction
	}
}

func (e *Engine) getSeverityImpactWeight(sev FindingSeverity) float64 {
	switch sev {
	case SeverityCritical:
		return 15.0
	case SeverityWarning:
		return 5.0
	case SeverityInfo:
		return 1.0
	default:
		return 1.0
	}
}

func (e *Engine) severityRank(sev FindingSeverity) int {
	switch sev {
	case SeverityCritical:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}
