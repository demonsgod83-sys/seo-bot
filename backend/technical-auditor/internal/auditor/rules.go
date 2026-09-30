package auditor

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

//==========================================//
//          RULE CATALOG DEFINITION         //
//==========================================//

const (
	RuleStatus4xx               = "technical.status_4xx"
	RuleStatus5xx               = "technical.status_5xx"
	RulePageUnreachable         = "technical.page_unreachable"
	RuleCanonicalMissing        = "technical.canonical_missing"
	RuleCanonicalExternalDomain = "technical.canonical_external_domain"
	RuleCanonicalChainConflict  = "technical.canonical_chain_conflict"
	RuleCanonicalRelative       = "technical.canonical_relative"
	RuleMetaNoindex             = "technical.meta_noindex"
	RuleHTTPUnencrypted         = "technical.http_unencrypted"
	RuleMixedContent            = "technical.mixed_content"
	RuleViewportMissing         = "technical.viewport_missing"
	RuleViewportInvalid         = "technical.viewport_invalid"
	RuleHreflangMissingRecip    = "technical.hreflang_missing_reciprocal"
	RuleHreflangInvalidCode     = "technical.hreflang_invalid_code"
	RuleURLTooLong              = "technical.url_too_long"
	RuleURLExcessiveParams      = "technical.url_excessive_parameters"
	RuleURLNonDescriptiveID     = "technical.url_non_descriptive_id"
)

// Static Rule Severity Map
var ruleSeverities = map[string]FindingSeverity{
	RuleStatus4xx:               SeverityCritical,
	RuleStatus5xx:               SeverityCritical,
	RulePageUnreachable:         SeverityCritical,
	RuleCanonicalMissing:        SeverityWarning,
	RuleCanonicalExternalDomain: SeverityInfo,
	RuleCanonicalChainConflict:  SeverityWarning,
	RuleCanonicalRelative:       SeverityWarning,
	RuleMetaNoindex:             SeverityInfo,
	RuleHTTPUnencrypted:         SeverityWarning,
	RuleMixedContent:            SeverityCritical,
	RuleViewportMissing:         SeverityCritical,
	RuleViewportInvalid:         SeverityWarning,
	RuleHreflangMissingRecip:    SeverityWarning,
	RuleHreflangInvalidCode:     SeverityWarning,
	RuleURLTooLong:              SeverityInfo,
	RuleURLExcessiveParams:      SeverityInfo,
	RuleURLNonDescriptiveID:     SeverityInfo,
}

var isoLangRegex = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})?$`)

//==========================================//
//          TECHNICAL AUDIT ENGINE          //
//==========================================//

// AuditPages executes all Technical SEO rules against the crawled & parsed page dataset.
func AuditPages(pages []ParsedPageRecord) []Finding {
	var findings []Finding

	pageByNormURL := make(map[string]ParsedPageRecord)
	canonicalTargetMap := make(map[string]string)
	hreflangTargetMap := make(map[string]map[string]bool) // pageURL -> targetURL -> true

	for _, page := range pages {
		normURL := normalizeURL(page.URL)
		pageByNormURL[normURL] = page

		// Track canonical map
		if page.CanonicalURL != "" {
			canonicalTargetMap[normURL] = normalizeURL(page.CanonicalURL)
		}

		// Track hreflang graph
		for _, h := range page.Hreflangs {
			if h.Href != "" {
				targetNorm := normalizeURL(h.Href)
				if hreflangTargetMap[normURL] == nil {
					hreflangTargetMap[normURL] = make(map[string]bool)
				}
				hreflangTargetMap[normURL][targetNorm] = true
			}
		}
	}

	// 1. Per-page technical evaluations
	for _, page := range pages {
		pageFindings := auditSinglePage(page)
		findings = append(findings, pageFindings...)
	}

	// 2. Cross-page graph technical evaluations
	crossFindings := auditCrossPage(pages, pageByNormURL, canonicalTargetMap, hreflangTargetMap)
	findings = append(findings, crossFindings...)

	return findings
}

func auditSinglePage(page ParsedPageRecord) []Finding {
	var f []Finding

	// A. HTTP Status Codes & Reachability
	if page.StatusCode >= 400 && page.StatusCode < 500 {
		f = append(f, newFinding(page, RuleStatus4xx,
			fmt.Sprintf("Page returned a client error HTTP status code (%d)", page.StatusCode),
			map[string]any{"status_code": page.StatusCode}))
		return f // Don't run on-page technical rules on dead pages
	} else if page.StatusCode >= 500 {
		f = append(f, newFinding(page, RuleStatus5xx,
			fmt.Sprintf("Page returned a server error HTTP status code (%d)", page.StatusCode),
			map[string]any{"status_code": page.StatusCode}))
		return f
	} else if page.StatusCode == 0 || page.ParseStatus == "failed" {
		msg := "Page was unreachable during crawl"
		if page.ErrorMessage != "" {
			msg = fmt.Sprintf("Page unreachable: %s", page.ErrorMessage)
		}
		f = append(f, newFinding(page, RulePageUnreachable, msg, map[string]any{"error": page.ErrorMessage}))
		return f
	}

	// B. HTTPS & Mixed Content
	parsedPageURL, err := url.Parse(page.URL)
	if err == nil {
		if strings.EqualFold(parsedPageURL.Scheme, "http") {
			f = append(f, newFinding(page, RuleHTTPUnencrypted,
				"Page is served over insecure plaintext HTTP instead of encrypted HTTPS",
				map[string]any{"url": page.URL}))
		} else if strings.EqualFold(parsedPageURL.Scheme, "https") {
			// Check for mixed content resources
			var insecureImages []string
			for _, img := range page.Images {
				if strings.HasPrefix(strings.ToLower(img.Src), "http://") {
					insecureImages = append(insecureImages, img.Src)
				}
			}
			if len(insecureImages) > 0 {
				f = append(f, newFinding(page, RuleMixedContent,
					fmt.Sprintf("HTTPS page loads %d insecure HTTP image(s) (Mixed Content)", len(insecureImages)),
					map[string]any{"insecure_resources": insecureImages}))
			}
		}
	}

	// C. Canonical Tag Checks
	if page.CanonicalURL == "" {
		f = append(f, newFinding(page, RuleCanonicalMissing,
			"Page is missing a rel=\"canonical\" link tag to specify its preferred indexing URL",
			nil))
	} else {
		parsedCanonical, cErr := url.Parse(page.CanonicalURL)
		if cErr == nil {
			if !parsedCanonical.IsAbs() {
				f = append(f, newFinding(page, RuleCanonicalRelative,
					"Canonical URL should be an absolute URL, not a relative path",
					map[string]any{"canonical_url": page.CanonicalURL}))
			} else if parsedPageURL != nil && !strings.EqualFold(parsedCanonical.Host, parsedPageURL.Host) {
				f = append(f, newFinding(page, RuleCanonicalExternalDomain,
					fmt.Sprintf("Canonical tag points to an external domain (%s)", parsedCanonical.Host),
					map[string]any{"canonical_url": page.CanonicalURL, "canonical_host": parsedCanonical.Host}))
			}
		}
	}

	// D. Meta Robots Directives
	robotsLower := strings.ToLower(page.MetaRobots)
	if strings.Contains(robotsLower, "noindex") {
		f = append(f, newFinding(page, RuleMetaNoindex,
			"Page has a meta robots 'noindex' directive instructing search engines not to index it",
			map[string]any{"meta_robots": page.MetaRobots}))
	}

	// E. Viewport & Mobile-Friendliness
	viewport := strings.ToLower(strings.TrimSpace(page.Viewport))
	if viewport == "" {
		f = append(f, newFinding(page, RuleViewportMissing,
			"Page is missing a <meta name=\"viewport\"> tag required for mobile-responsive rendering",
			nil))
	} else if !strings.Contains(viewport, "width=device-width") && !strings.Contains(viewport, "initial-scale=1") {
		f = append(f, newFinding(page, RuleViewportInvalid,
			"Viewport meta tag is present but missing standard responsive directives (width=device-width, initial-scale=1)",
			map[string]any{"viewport": page.Viewport}))
	}

	// F. hreflang Tag Format
	for _, h := range page.Hreflangs {
		cleanLang := strings.TrimSpace(h.Hreflang)
		if cleanLang != "" && !strings.EqualFold(cleanLang, "x-default") {
			if !isoLangRegex.MatchString(cleanLang) {
				f = append(f, newFinding(page, RuleHreflangInvalidCode,
					fmt.Sprintf("Invalid hreflang language/region code format (%q)", h.Hreflang),
					map[string]any{"hreflang": h.Hreflang, "href": h.Href}))
			}
		}
	}

	// G. URL Structure & Parameter Hygiene
	if parsedPageURL != nil {
		rawURL := page.URL
		if len(rawURL) > 100 {
			f = append(f, newFinding(page, RuleURLTooLong,
				fmt.Sprintf("URL is overly long (%d characters). Concise URLs are easier for search engines to crawl.", len(rawURL)),
				map[string]any{"length": len(rawURL), "url": rawURL}))
		}

		queryParams := parsedPageURL.Query()
		if len(queryParams) > 3 {
			f = append(f, newFinding(page, RuleURLExcessiveParams,
				fmt.Sprintf("URL contains %d query parameters. Excessive URL parameters create crawl inefficiency.", len(queryParams)),
				map[string]any{"params_count": len(queryParams), "query": parsedPageURL.RawQuery}))
		}

		// Check for non-descriptive numeric ID parameters
		if idVal := queryParams.Get("id"); idVal != "" {
			f = append(f, newFinding(page, RuleURLNonDescriptiveID,
				"URL uses a non-descriptive parameter (?id=...) instead of human-readable keyword slugs",
				map[string]any{"url": rawURL}))
		}
	}

	return f
}

func auditCrossPage(
	pages []ParsedPageRecord,
	pageByNormURL map[string]ParsedPageRecord,
	canonicalTargetMap map[string]string,
	hreflangTargetMap map[string]map[string]bool,
) []Finding {
	var f []Finding

	for _, page := range pages {
		if page.StatusCode != 200 {
			continue
		}

		normURL := normalizeURL(page.URL)

		// 1. Canonical Chain Conflicts: Page A -> Canonical B, but Page B -> Canonical C
		if targetB, hasTargetB := canonicalTargetMap[normURL]; hasTargetB && targetB != normURL {
			if targetC, hasTargetC := canonicalTargetMap[targetB]; hasTargetC && targetC != targetB {
				f = append(f, newFinding(page, RuleCanonicalChainConflict,
					fmt.Sprintf("Canonical chain conflict detected: page points to %s, which canonicals to %s", targetB, targetC),
					map[string]any{"target_1": targetB, "target_2": targetC}))
			}
		}

		// 2. hreflang Reciprocal Verification: Page A specifies hreflang to Page B, but Page B lacks reciprocal link
		for _, h := range page.Hreflangs {
			if h.Href != "" {
				targetNorm := normalizeURL(h.Href)
				if targetPage, exists := pageByNormURL[targetNorm]; exists && targetPage.StatusCode == 200 {
					targetHreflangs := hreflangTargetMap[targetNorm]
					if targetHreflangs == nil || !targetHreflangs[normURL] {
						f = append(f, newFinding(page, RuleHreflangMissingRecip,
							fmt.Sprintf("hreflang tag pointing to %s is missing a reciprocal link back to this page", h.Href),
							map[string]any{"target_page": h.Href, "hreflang": h.Hreflang}))
					}
				}
			}
		}
	}

	return f
}

//==========================================//
//             HELPER FUNCTIONS             //
//==========================================//

func newFinding(page ParsedPageRecord, ruleID string, message string, evidence map[string]any) Finding {
	severity := ruleSeverities[ruleID]
	if severity == "" {
		severity = SeverityInfo
	}
	if evidence == nil {
		evidence = make(map[string]any)
	}

	return Finding{
		AuditID:  page.AuditID,
		TenantID: page.TenantID,
		PageURL:  page.URL,
		Category: "technical",
		RuleID:   ruleID,
		Severity: severity,
		Message:  message,
		Evidence: evidence,
	}
}

func normalizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return strings.ToLower(strings.TrimRight(raw, "/"))
	}
	u.Fragment = ""
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String()
}
