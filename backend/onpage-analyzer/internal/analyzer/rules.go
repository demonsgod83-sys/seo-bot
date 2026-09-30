package analyzer

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

//==========================================//
//          RULE CATALOG DEFINITION         //
//==========================================//

const (
	RuleTitleMissing           = "onpage.title_missing"
	RuleTitleEmpty             = "onpage.title_empty"
	RuleTitleTooShort          = "onpage.title_too_short"
	RuleTitleTooLong           = "onpage.title_too_long"
	RuleTitleDuplicate         = "onpage.title_duplicate"
	RuleMetaDescMissing        = "onpage.meta_description_missing"
	RuleMetaDescEmpty          = "onpage.meta_description_empty"
	RuleMetaDescTooShort       = "onpage.meta_description_too_short"
	RuleMetaDescTooLong        = "onpage.meta_description_too_long"
	RuleMetaDescDuplicate      = "onpage.meta_description_duplicate"
	RuleH1Missing              = "onpage.h1_missing"
	RuleH1Multiple             = "onpage.h1_multiple"
	RuleHeadingSkippedLevel    = "onpage.heading_skipped_level"
	RuleHeadingEmpty           = "onpage.heading_empty"
	RuleSlugKeywordMismatch    = "onpage.url_slug_keyword_mismatch"
	RuleImageAltMissing        = "onpage.image_alt_missing"
	RuleImageAltIsFilename     = "onpage.image_alt_is_filename"
	RuleOrphanPage             = "onpage.orphan_page"
	RuleGenericAnchorText      = "onpage.generic_anchor_text"
	RuleThinContent            = "onpage.thin_content"
	RuleReadabilityDifficult   = "onpage.readability_difficult"
	RuleDuplicateContent       = "onpage.duplicate_content"
)

// Static Rule Severity Map
var ruleSeverities = map[string]FindingSeverity{
	RuleTitleMissing:         SeverityCritical,
	RuleTitleEmpty:           SeverityCritical,
	RuleTitleTooShort:        SeverityWarning,
	RuleTitleTooLong:         SeverityWarning,
	RuleTitleDuplicate:       SeverityWarning,
	RuleMetaDescMissing:      SeverityWarning,
	RuleMetaDescEmpty:        SeverityWarning,
	RuleMetaDescTooShort:     SeverityInfo,
	RuleMetaDescTooLong:      SeverityWarning,
	RuleMetaDescDuplicate:    SeverityWarning,
	RuleH1Missing:            SeverityCritical,
	RuleH1Multiple:           SeverityWarning,
	RuleHeadingSkippedLevel:  SeverityWarning,
	RuleHeadingEmpty:         SeverityInfo,
	RuleSlugKeywordMismatch:  SeverityInfo,
	RuleImageAltMissing:      SeverityWarning,
	RuleImageAltIsFilename:   SeverityInfo,
	RuleOrphanPage:           SeverityWarning,
	RuleGenericAnchorText:    SeverityInfo,
	RuleThinContent:          SeverityWarning,
	RuleReadabilityDifficult: SeverityInfo,
	RuleDuplicateContent:     SeverityWarning,
}

var genericAnchorPhrases = map[string]bool{
	"click here": true, "read more": true, "learn more": true,
	"here": true, "link": true, "more": true, "details": true,
	"view more": true, "continue reading": true, "website": true,
}

//==========================================//
//          ON-PAGE RULE ENGINE             //
//==========================================//

// AnalyzePages runs the complete suite of per-page and site-graph on-page SEO checks.
func AnalyzePages(pages []ParsedPageRecord) []Finding {
	var findings []Finding

	titleMap := make(map[string][]string)
	metaDescMap := make(map[string][]string)
	contentHashMap := make(map[string][]string)
	inboundLinkCount := make(map[string]int)

	for _, page := range pages {
		inboundLinkCount[normalizeForMatching(page.URL)] = 0
	}

	// 1. First Pass: Per-page evaluations and graph building
	for _, page := range pages {
		if page.ParseStatus != "success" || page.StatusCode != 200 {
			continue
		}

		pageFindings := analyzeSinglePage(page)
		findings = append(findings, pageFindings...)

		// Track graph mappings
		cleanTitle := strings.TrimSpace(page.Title)
		if cleanTitle != "" {
			titleMap[cleanTitle] = append(titleMap[cleanTitle], page.URL)
		}

		cleanDesc := strings.TrimSpace(page.MetaDescription)
		if cleanDesc != "" {
			metaDescMap[cleanDesc] = append(metaDescMap[cleanDesc], page.URL)
		}

		// Content hash for duplicate body detection
		if page.WordCount >= 50 {
			bodyHash := hashBodyText(page.BodyText)
			contentHashMap[bodyHash] = append(contentHashMap[bodyHash], page.URL)
		}

		// Inbound link graph calculation
		for _, link := range page.Links {
			if link.IsInternal {
				normLink := normalizeForMatching(link.Href)
				if _, exists := inboundLinkCount[normLink]; exists {
					inboundLinkCount[normLink]++
				}
			}
		}
	}

	// 2. Second Pass: Cross-page (site-wide graph) evaluations
	crossPageFindings := analyzeCrossPage(pages, titleMap, metaDescMap, contentHashMap, inboundLinkCount)
	findings = append(findings, crossPageFindings...)

	return findings
}

// analyzeSinglePage executes all checks for a single parsed page.
func analyzeSinglePage(page ParsedPageRecord) []Finding {
	var f []Finding

	// A. Title Tag Checks
	trimmedTitle := strings.TrimSpace(page.Title)
	if page.Title == "" {
		f = append(f, newFinding(page, RuleTitleMissing, "Page is missing a <title> tag", nil))
	} else if trimmedTitle == "" {
		f = append(f, newFinding(page, RuleTitleEmpty, "Page <title> tag is empty", nil))
	} else {
		titleLen := len([]rune(trimmedTitle))
		if titleLen < 30 {
			f = append(f, newFinding(page, RuleTitleTooShort,
				fmt.Sprintf("Title tag is too short (%d characters). Optimal length is 30–60 characters.", titleLen),
				map[string]any{"length": titleLen, "title": trimmedTitle}))
		} else if titleLen > 60 {
			f = append(f, newFinding(page, RuleTitleTooLong,
				fmt.Sprintf("Title tag is too long (%d characters) and may be truncated in search results.", titleLen),
				map[string]any{"length": titleLen, "title": trimmedTitle}))
		}
	}

	// B. Meta Description Checks
	trimmedDesc := strings.TrimSpace(page.MetaDescription)
	if page.MetaDescription == "" {
		f = append(f, newFinding(page, RuleMetaDescMissing, "Page is missing a meta description tag", nil))
	} else if trimmedDesc == "" {
		f = append(f, newFinding(page, RuleMetaDescEmpty, "Meta description tag is empty", nil))
	} else {
		descLen := len([]rune(trimmedDesc))
		if descLen < 70 {
			f = append(f, newFinding(page, RuleMetaDescTooShort,
				fmt.Sprintf("Meta description is very short (%d characters). Recommended is 70–160 characters.", descLen),
				map[string]any{"length": descLen, "description": trimmedDesc}))
		} else if descLen > 160 {
			f = append(f, newFinding(page, RuleMetaDescTooLong,
				fmt.Sprintf("Meta description is too long (%d characters) and will likely be truncated.", descLen),
				map[string]any{"length": descLen, "description": trimmedDesc}))
		}
	}

	// C. Heading Checks (H1..H6)
	h1Count := 0
	lastLevel := 0
	var h1Texts []string

	for _, h := range page.Headings {
		trimmedHeadingText := strings.TrimSpace(h.Text)
		if trimmedHeadingText == "" {
			f = append(f, newFinding(page, RuleHeadingEmpty,
				fmt.Sprintf("H%d heading tag is empty", h.Level),
				map[string]any{"heading_level": h.Level}))
		}

		if h.Level == 1 {
			h1Count++
			if trimmedHeadingText != "" {
				h1Texts = append(h1Texts, trimmedHeadingText)
			}
		}

		// Check skipped heading levels (e.g. H1 -> H3)
		if lastLevel > 0 && h.Level > lastLevel+1 {
			f = append(f, newFinding(page, RuleHeadingSkippedLevel,
				fmt.Sprintf("Heading hierarchy skipped levels from H%d directly to H%d", lastLevel, h.Level),
				map[string]any{"from_level": lastLevel, "to_level": h.Level, "heading": h.Text}))
		}
		lastLevel = h.Level
	}

	if h1Count == 0 {
		f = append(f, newFinding(page, RuleH1Missing, "Page is missing an <h1> heading tag", nil))
	} else if h1Count > 1 {
		f = append(f, newFinding(page, RuleH1Multiple,
			fmt.Sprintf("Page has %d <h1> tags. Pages should generally have a single primary <h1>.", h1Count),
			map[string]any{"h1_count": h1Count, "h1_headings": h1Texts}))
	}

	// D. URL Slug Keyword Overlap
	slugKeywords := extractSlugKeywords(page.URL)
	if len(slugKeywords) > 0 {
		searchCorpus := strings.ToLower(trimmedTitle + " " + strings.Join(h1Texts, " "))
		foundMatch := false
		for _, kw := range slugKeywords {
			if strings.Contains(searchCorpus, kw) {
				foundMatch = true
				break
			}
		}
		if !foundMatch {
			f = append(f, newFinding(page, RuleSlugKeywordMismatch,
				"Keywords from URL slug are not present in either the Title tag or primary H1 heading.",
				map[string]any{"slug_keywords": slugKeywords, "title": trimmedTitle, "h1": h1Texts}))
		}
	}

	// E. Image Alt Text Checks
	for _, img := range page.Images {
		if !img.HasAlt || strings.TrimSpace(img.Alt) == "" {
			f = append(f, newFinding(page, RuleImageAltMissing,
				"Image is missing an alt attribute",
				map[string]any{"src": img.Src}))
		} else {
			// Check if alt text simply repeats the filename
			imgBase := filepath.Base(img.Src)
			cleanBase := strings.TrimSuffix(imgBase, filepath.Ext(imgBase))
			if strings.EqualFold(strings.TrimSpace(img.Alt), cleanBase) || strings.EqualFold(strings.TrimSpace(img.Alt), imgBase) {
				f = append(f, newFinding(page, RuleImageAltIsFilename,
					"Image alt text simply repeats the file name instead of describing the image",
					map[string]any{"src": img.Src, "alt": img.Alt}))
			}
		}
	}

	// F. Internal Generic Anchor Text Checks
	for _, link := range page.Links {
		if link.IsInternal {
			anchor := strings.ToLower(strings.TrimSpace(link.Text))
			if genericAnchorPhrases[anchor] {
				f = append(f, newFinding(page, RuleGenericAnchorText,
					fmt.Sprintf("Internal link uses generic anchor text (%q) instead of descriptive keywords", link.Text),
					map[string]any{"href": link.Href, "anchor_text": link.Text}))
			}
		}
	}

	// G. Content Depth / Thin Content (<250 words)
	if page.WordCount < 250 {
		f = append(f, newFinding(page, RuleThinContent,
			fmt.Sprintf("Page has thin content (%d words). Aim for at least 250+ substantive words.", page.WordCount),
			map[string]any{"word_count": page.WordCount}))
	}

	// H. Readability (Flesch Reading Ease)
	if page.WordCount >= 100 {
		score := calculateFleschReadingEase(page.BodyText, page.WordCount)
		if score < 30.0 {
			f = append(f, newFinding(page, RuleReadabilityDifficult,
				fmt.Sprintf("Content has a very difficult Flesch Reading Ease score (%.1f/100). Consider simplifying sentence structure.", score),
				map[string]any{"flesch_score": score, "word_count": page.WordCount}))
		}
	}

	return f
}

// analyzeCrossPage evaluates site-graph rules: duplicate titles, duplicate descriptions, duplicate content, and orphan pages.
func analyzeCrossPage(
	pages []ParsedPageRecord,
	titleMap map[string][]string,
	metaDescMap map[string][]string,
	contentHashMap map[string][]string,
	inboundLinkCount map[string]int,
) []Finding {
	var f []Finding

	for _, page := range pages {
		if page.ParseStatus != "success" || page.StatusCode != 200 {
			continue
		}

		// 1. Duplicate Titles
		cleanTitle := strings.TrimSpace(page.Title)
		if cleanTitle != "" && len(titleMap[cleanTitle]) > 1 {
			f = append(f, newFinding(page, RuleTitleDuplicate,
				fmt.Sprintf("Title tag is identical to %d other page(s) on the site", len(titleMap[cleanTitle])-1),
				map[string]any{"title": cleanTitle, "duplicate_urls": titleMap[cleanTitle]}))
		}

		// 2. Duplicate Meta Descriptions
		cleanDesc := strings.TrimSpace(page.MetaDescription)
		if cleanDesc != "" && len(metaDescMap[cleanDesc]) > 1 {
			f = append(f, newFinding(page, RuleMetaDescDuplicate,
				fmt.Sprintf("Meta description is identical to %d other page(s) on the site", len(metaDescMap[cleanDesc])-1),
				map[string]any{"description": cleanDesc, "duplicate_urls": metaDescMap[cleanDesc]}))
		}

		// 3. Duplicate Body Content
		if page.WordCount >= 50 {
			bodyHash := hashBodyText(page.BodyText)
			if len(contentHashMap[bodyHash]) > 1 {
				f = append(f, newFinding(page, RuleDuplicateContent,
					fmt.Sprintf("Page body content is near-identical to %d other page(s)", len(contentHashMap[bodyHash])-1),
					map[string]any{"duplicate_urls": contentHashMap[bodyHash]}))
			}
		}

		// 4. Orphan Page Check (0 internal inbound links across crawl, excluding home root)
		normURL := normalizeForMatching(page.URL)
		parsedURL, _ := url.Parse(page.URL)
		isRoot := parsedURL != nil && (parsedURL.Path == "" || parsedURL.Path == "/")
		if !isRoot && inboundLinkCount[normURL] == 0 {
			f = append(f, newFinding(page, RuleOrphanPage,
				"Page is an orphan page with 0 internal inbound links discovered from other pages on the site",
				map[string]any{"inbound_links_count": 0}))
		}
	}

	return f
}

//==========================================//
//             HELPER UTILITIES             //
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
		Category: "onpage",
		RuleID:   ruleID,
		Severity: severity,
		Message:  message,
		Evidence: evidence,
	}
}

func normalizeForMatching(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return strings.ToLower(strings.TrimRight(raw, "/"))
	}
	u.Fragment = ""
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String()
}

func hashBodyText(text string) string {
	words := strings.Fields(strings.ToLower(text))
	var sample []string
	step := len(words) / 30
	if step < 1 {
		step = 1
	}
	for i := 0; i < len(words); i += step {
		sample = append(sample, words[i])
	}
	h := md5.Sum([]byte(strings.Join(sample, " ")))
	return hex.EncodeToString(h[:])
}

func extractSlugKeywords(pageURL string) []string {
	u, err := url.Parse(pageURL)
	if err != nil || u.Path == "" || u.Path == "/" {
		return nil
	}

	cleanPath := strings.Trim(u.Path, "/")
	parts := strings.Split(cleanPath, "/")
	lastSegment := parts[len(parts)-1]
	lastSegment = strings.TrimSuffix(lastSegment, ".html")

	tokens := regexp.MustCompile(`[-_]+`).Split(lastSegment, -1)
	var keywords []string
	stopWords := map[string]bool{"the": true, "a": true, "an": true, "and": true, "of": true, "in": true, "to": true, "for": true}

	for _, t := range tokens {
		t = strings.ToLower(strings.TrimSpace(t))
		if len(t) >= 3 && !stopWords[t] {
			keywords = append(keywords, t)
		}
	}
	return keywords
}

// calculateFleschReadingEase returns standard Flesch Reading Ease score (0–100 scale).
func calculateFleschReadingEase(text string, totalWords int) float64 {
	if totalWords == 0 {
		return 100.0
	}

	sentences := countSentences(text)
	if sentences == 0 {
		sentences = 1
	}

	syllables := countSyllablesInText(text)
	if syllables == 0 {
		syllables = totalWords
	}

	score := 206.835 - (1.015 * (float64(totalWords) / float64(sentences))) - (84.6 * (float64(syllables) / float64(totalWords)))
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func countSentences(text string) int {
	count := 0
	for _, r := range text {
		if r == '.' || r == '!' || r == '?' {
			count++
		}
	}
	if count == 0 {
		return 1
	}
	return count
}

func countSyllablesInText(text string) int {
	words := strings.Fields(strings.ToLower(text))
	total := 0
	for _, w := range words {
		total += countSyllablesInWord(w)
	}
	return total
}

func countSyllablesInWord(word string) int {
	vowels := "aeiouy"
	count := 0
	prevIsVowel := false

	runes := []rune(word)
	for i, r := range runes {
		if !unicode.IsLetter(r) {
			continue
		}
		isVowel := strings.ContainsRune(vowels, r)
		if isVowel && !prevIsVowel {
			count++
		}
		prevIsVowel = isVowel

		// Silent 'e' at the end
		if i == len(runes)-1 && r == 'e' && count > 1 {
			count--
		}
	}

	if count == 0 {
		return 1
	}
	return count
}
