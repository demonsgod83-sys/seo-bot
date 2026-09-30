package scoring

import (
	"strings"
)

// EffortTier defines how difficult an issue is to fix.
type EffortTier string

const (
	EffortLow    EffortTier = "low"
	EffortMedium EffortTier = "medium"
	EffortHigh   EffortTier = "high"
)

// RuleMetadata holds descriptive title and effort classification for a rule_id.
type RuleMetadata struct {
	Title      string
	EffortTier EffortTier
	EffortCost float64 // Low = 1.0, Medium = 2.0, High = 3.0
}

var ruleRegistry = map[string]RuleMetadata{
	// On-Page Rules
	"onpage.title_missing":              {Title: "Missing Title Tag", EffortTier: EffortLow, EffortCost: 1.0},
	"onpage.title_too_short":             {Title: "Title Tag Too Short", EffortTier: EffortLow, EffortCost: 1.0},
	"onpage.title_too_long":              {Title: "Title Tag Too Long", EffortTier: EffortLow, EffortCost: 1.0},
	"onpage.title_duplicate":             {Title: "Duplicate Title Tags", EffortTier: EffortMedium, EffortCost: 2.0},
	"onpage.meta_description_missing":   {Title: "Missing Meta Description", EffortTier: EffortLow, EffortCost: 1.0},
	"onpage.meta_description_too_short":  {Title: "Meta Description Too Short", EffortTier: EffortLow, EffortCost: 1.0},
	"onpage.meta_description_too_long":   {Title: "Meta Description Too Long", EffortTier: EffortLow, EffortCost: 1.0},
	"onpage.meta_description_duplicate":  {Title: "Duplicate Meta Description", EffortTier: EffortMedium, EffortCost: 2.0},
	"onpage.h1_missing":                 {Title: "Missing H1 Heading", EffortTier: EffortLow, EffortCost: 1.0},
	"onpage.h1_multiple":                {Title: "Multiple H1 Headings", EffortTier: EffortMedium, EffortCost: 2.0},
	"onpage.heading_skipped_level":      {Title: "Skipped Heading Levels", EffortTier: EffortLow, EffortCost: 1.0},
	"onpage.image_missing_alt":          {Title: "Missing Image Alt Attributes", EffortTier: EffortLow, EffortCost: 1.0},
	"onpage.image_filename_as_alt":      {Title: "Image Filename Used as Alt Text", EffortTier: EffortLow, EffortCost: 1.0},
	"onpage.thin_content":               {Title: "Thin Content (<250 words)", EffortTier: EffortHigh, EffortCost: 3.0},
	"onpage.flesch_reading_difficult":   {Title: "Low Readability Score", EffortTier: EffortMedium, EffortCost: 2.0},
	"onpage.generic_anchor_text":        {Title: "Generic Link Anchor Text", EffortTier: EffortLow, EffortCost: 1.0},
	"onpage.orphan_page":                {Title: "Orphan Page (No Inbound Internal Links)", EffortTier: EffortMedium, EffortCost: 2.0},
	"onpage.duplicate_body":             {Title: "Duplicate Body Content Across Pages", EffortTier: EffortHigh, EffortCost: 3.0},

	// Technical Rules
	"technical.status_4xx":                 {Title: "Broken Page (4xx Client Error)", EffortTier: EffortMedium, EffortCost: 2.0},
	"technical.status_5xx":                 {Title: "Server Error (5xx Internal Error)", EffortTier: EffortHigh, EffortCost: 3.0},
	"technical.unreachable":                {Title: "Unreachable Page", EffortTier: EffortHigh, EffortCost: 3.0},
	"technical.canonical_missing":          {Title: "Missing Canonical Tag", EffortTier: EffortLow, EffortCost: 1.0},
	"technical.canonical_relative":         {Title: "Relative Canonical URL", EffortTier: EffortLow, EffortCost: 1.0},
	"technical.canonical_external":         {Title: "External Canonical Tag", EffortTier: EffortMedium, EffortCost: 2.0},
	"technical.canonical_chain":            {Title: "Canonical Chain / Conflict", EffortTier: EffortHigh, EffortCost: 3.0},
	"technical.mixed_content":              {Title: "Insecure Mixed Content on HTTPS", EffortTier: EffortMedium, EffortCost: 2.0},
	"technical.viewport_missing":           {Title: "Missing Mobile Viewport Tag", EffortTier: EffortLow, EffortCost: 1.0},
	"technical.viewport_invalid":           {Title: "Invalid Mobile Viewport Tag", EffortTier: EffortLow, EffortCost: 1.0},
	"technical.hreflang_invalid_code":      {Title: "Invalid Hreflang Language Code", EffortTier: EffortLow, EffortCost: 1.0},
	"technical.hreflang_missing_reciprocal":{Title: "Missing Reciprocal Hreflang Tag", EffortTier: EffortMedium, EffortCost: 2.0},
	"technical.url_too_long":               {Title: "Excessively Long URL (>200 chars)", EffortTier: EffortMedium, EffortCost: 2.0},
	"technical.excessive_query_params":     {Title: "Excessive URL Query Parameters (>3)", EffortTier: EffortMedium, EffortCost: 2.0},

	// Keywords & Content Rules
	"keywords.cannibalization": {Title: "Keyword Cannibalization (Multiple Pages Competing)", EffortTier: EffortHigh, EffortCost: 3.0},
	"keywords.missing_primary": {Title: "Missing Primary Keyword Focus", EffortTier: EffortMedium, EffortCost: 2.0},
}

// GetRuleMetadata returns metadata for a given rule_id, with intelligent fallback.
func GetRuleMetadata(ruleID string) RuleMetadata {
	if meta, exists := ruleRegistry[ruleID]; exists {
		return meta
	}

	// Fallback title generator from ruleID (e.g. "category.something_bad" -> "Something Bad")
	parts := strings.Split(ruleID, ".")
	titlePart := ruleID
	if len(parts) > 1 {
		titlePart = parts[1]
	}
	words := strings.Split(titlePart, "_")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	generatedTitle := strings.Join(words, " ")

	return RuleMetadata{
		Title:      generatedTitle,
		EffortTier: EffortMedium,
		EffortCost: 2.0,
	}
}
