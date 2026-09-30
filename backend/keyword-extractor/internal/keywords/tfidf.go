package keywords

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

//==========================================//
//          STOPWORDS DICTIONARY            //
//==========================================//

var englishStopwords = map[string]bool{
	"a": true, "about": true, "above": true, "after": true, "again": true, "against": true,
	"all": true, "am": true, "an": true, "and": true, "any": true, "are": true, "aren't": true,
	"as": true, "at": true, "be": true, "because": true, "been": true, "before": true,
	"being": true, "below": true, "between": true, "both": true, "but": true, "by": true,
	"can't": true, "cannot": true, "could": true, "couldn't": true, "did": true, "didn't": true,
	"do": true, "does": true, "doesn't": true, "doing": true, "don't": true, "down": true,
	"during": true, "each": true, "few": true, "for": true, "from": true, "further": true,
	"had": true, "hadn't": true, "has": true, "hasn't": true, "have": true, "haven't": true,
	"having": true, "he": true, "he'd": true, "he'll": true, "he's": true, "her": true,
	"here": true, "here's": true, "hers": true, "herself": true, "him": true, "himself": true,
	"his": true, "how": true, "how's": true, "i": true, "i'd": true, "i'll": true,
	"i'm": true, "i've": true, "if": true, "in": true, "into": true, "is": true,
	"isn't": true, "it": true, "it's": true, "its": true, "itself": true, "let's": true,
	"me": true, "more": true, "most": true, "mustn't": true, "my": true, "myself": true,
	"no": true, "nor": true, "not": true, "of": true, "off": true, "on": true,
	"once": true, "only": true, "or": true, "other": true, "ought": true, "our": true,
	"ours": true, "ourselves": true, "out": true, "over": true, "own": true, "same": true,
	"shan't": true, "she": true, "she'd": true, "she'll": true, "she's": true, "should": true,
	"shouldn't": true, "so": true, "some": true, "such": true, "than": true, "that": true,
	"that's": true, "the": true, "their": true, "theirs": true, "them": true, "themselves": true,
	"then": true, "there": true, "there's": true, "these": true, "they": true, "they'd": true,
	"they'll": true, "they're": true, "they've": true, "this": true, "those": true, "through": true,
	"to": true, "too": true, "under": true, "until": true, "up": true, "very": true,
	"was": true, "wasn't": true, "we": true, "we'd": true, "we'll": true, "we're": true,
	"we've": true, "were": true, "weren't": true, "what": true, "what's": true, "when": true,
	"when's": true, "where": true, "where's": true, "which": true, "while": true, "who": true,
	"who's": true, "whom": true, "why": true, "why's": true, "with": true, "won't": true,
	"would": true, "wouldn't": true, "you": true, "you'd": true, "you'll": true, "you're": true,
	"you've": true, "your": true, "yours": true, "yourself": true, "yourselves": true,
	"click": true, "read": true, "learn": true, "view": true, "page": true, "home": true,
}

//==========================================//
//          KEYWORD EXTRACTION ENGINE       //
//==========================================//

type ExtractedSiteKeywords struct {
	PageKeywords    []PageKeyword
	Findings        []Finding
	SiteSummary     SiteKeywordSummary
	UniqueKeywords  int
	Cannibalizations int
}

// ExtractSiteKeywords processes all pages of an audit to calculate TF-IDF, assign target keywords, and flag cannibalization.
func ExtractSiteKeywords(pages []ParsedPageRecord) ExtractedSiteKeywords {
	var validPages []ParsedPageRecord
	for _, p := range pages {
		if p.StatusCode == 200 && p.ParseStatus == "success" && p.WordCount >= 5 {
			validPages = append(validPages, p)
		}
	}

	numDocs := float64(len(validPages))
	if numDocs == 0 {
		return ExtractedSiteKeywords{}
	}

	// 1. Tokenize each document and build n-grams
	docTokens := make([]map[string]int, len(validPages)) // term -> count per doc
	docTermTotals := make([]int, len(validPages))
	docFrequency := make(map[string]int)                // term -> doc count
	docEntities := make([][]string, len(validPages))

	for i, p := range validPages {
		tokens := tokenizeText(p.BodyText)
		docEntities[i] = extractNamedEntities(p.BodyText)

		ngrams := generateNGrams(tokens)
		docTokens[i] = ngrams

		totalNgrams := 0
		for term, count := range ngrams {
			totalNgrams += count
			docFrequency[term]++
		}
		docTermTotals[i] = totalNgrams
	}

	// 2. Score candidate keywords using TF-IDF + Structural Placement Weights
	pageKeywordProfiles := make([]PageKeyword, len(validPages))
	siteKeywordMap := make(map[string][]string) // keyword -> []pageURLs

	for i, p := range validPages {
		var candidateScores []ScoredTerm

		// Structural texts for position boosting
		titleLower := strings.ToLower(p.Title)
		h1Lower := strings.ToLower(extractHeadingsText(p.Headings, 1))
		h2h3Lower := strings.ToLower(extractHeadingsText(p.Headings, 2) + " " + extractHeadingsText(p.Headings, 3))
		slugLower := strings.ToLower(extractSlug(p.URL))

		for term, count := range docTokens[i] {
			tf := float64(count) / float64(docTermTotals[i]+1)
			df := float64(docFrequency[term])
			idf := math.Log((1.0+numDocs)/(1.0+df)) + 1.0

			baseScore := tf * idf

			// Position Multipliers
			positionMultiplier := 1.0
			if strings.Contains(titleLower, term) {
				positionMultiplier += 3.0
			}
			if strings.Contains(h1Lower, term) {
				positionMultiplier += 2.5
			}
			if strings.Contains(h2h3Lower, term) {
				positionMultiplier += 1.5
			}
			if strings.Contains(slugLower, term) {
				positionMultiplier += 2.0
			}

			// Phrase bonus: Multi-word phrases (2-grams and 3-grams) represent target search queries much better than single isolated words
			wordsInTerm := len(strings.Fields(term))
			if wordsInTerm == 2 {
				positionMultiplier *= 2.2
			} else if wordsInTerm == 3 {
				positionMultiplier *= 2.8
			}

			finalScore := baseScore * positionMultiplier
			candidateScores = append(candidateScores, ScoredTerm{
				Term:        term,
				Score:       finalScore,
				NGramType:   wordsInTerm,
				Occurrences: count,
			})
		}

		// Sort candidate terms by score descending
		sort.Slice(candidateScores, func(a, b int) bool {
			return candidateScores[a].Score > candidateScores[b].Score
		})

		// Select primary and secondary keywords
		primary := "general"
		var secondaries []string
		var topTerms []ScoredTerm

		if len(candidateScores) > 0 {
			primary = candidateScores[0].Term
		}

		// Pick up to 5 distinctive secondary keywords (avoiding sub-strings of primary)
		for j := 1; j < len(candidateScores) && len(secondaries) < 5; j++ {
			cand := candidateScores[j].Term
			if !strings.Contains(primary, cand) && !strings.Contains(cand, primary) {
				secondaries = append(secondaries, cand)
			}
		}

		if len(candidateScores) > 10 {
			topTerms = candidateScores[:10]
		} else {
			topTerms = candidateScores
		}

		pageKeywordProfiles[i] = PageKeyword{
			AuditID:           p.AuditID,
			TenantID:          p.TenantID,
			PageURL:           p.URL,
			PrimaryKeyword:    primary,
			SecondaryKeywords: secondaries,
			TopTerms:          topTerms,
			NamedEntities:     docEntities[i],
		}

		// Record in site-wide keyword map
		siteKeywordMap[primary] = append(siteKeywordMap[primary], p.URL)
	}

	// 3. Keyword Cannibalization Detection
	var findings []Finding
	cannibalizationCount := 0
	cannibalizedPairs := make(map[string]bool)

	// Check exact matches in siteKeywordMap
	for kw, urls := range siteKeywordMap {
		if len(urls) > 1 && kw != "general" {
			cannibalizationCount++
			firstPage := validPages[0]
			for _, page := range validPages {
				if page.URL == urls[0] {
					firstPage = page
					break
				}
			}

			findings = append(findings, Finding{
				AuditID:  firstPage.AuditID,
				TenantID: firstPage.TenantID,
				PageURL:  urls[0],
				Category: "keywords",
				RuleID:   "keywords.cannibalization",
				Severity: SeverityWarning,
				Message: fmt.Sprintf("Keyword cannibalization detected: %d pages are competing for the exact same primary keyword (%q)",
					len(urls), kw),
				Evidence: map[string]any{
					"target_keyword": kw,
					"competing_urls": urls,
				},
			})
		}
	}

	// Also check cross-page phrase overlap between distinct pages (e.g. "email marketing automation" vs "email marketing")
	for i := 0; i < len(pageKeywordProfiles); i++ {
		for j := i + 1; j < len(pageKeywordProfiles); j++ {
			p1 := pageKeywordProfiles[i]
			p2 := pageKeywordProfiles[j]
			if p1.PrimaryKeyword == p2.PrimaryKeyword || p1.PrimaryKeyword == "general" || p2.PrimaryKeyword == "general" {
				continue
			}

			// If one primary keyword contains the other (e.g. "email marketing" vs "email marketing automation")
			if strings.Contains(p1.PrimaryKeyword, p2.PrimaryKeyword) || strings.Contains(p2.PrimaryKeyword, p1.PrimaryKeyword) {
				pairKey := p1.PrimaryKeyword + " <-> " + p2.PrimaryKeyword
				if !cannibalizedPairs[pairKey] {
					cannibalizedPairs[pairKey] = true
					cannibalizationCount++

					findings = append(findings, Finding{
						AuditID:  p1.AuditID,
						TenantID: p1.TenantID,
						PageURL:  p1.PageURL,
						Category: "keywords",
						RuleID:   "keywords.cannibalization",
						Severity: SeverityWarning,
						Message: fmt.Sprintf("Keyword cannibalization overlap: Page targeting %q competes directly with %s targeting %q",
							p1.PrimaryKeyword, p2.PageURL, p2.PrimaryKeyword),
						Evidence: map[string]any{
							"keyword_1":      p1.PrimaryKeyword,
							"keyword_2":      p2.PrimaryKeyword,
							"competing_urls": []string{p1.PageURL, p2.PageURL},
						},
					})
				}
			}
		}
	}

	auditID := pages[0].AuditID
	tenantID := pages[0].TenantID

	summary := SiteKeywordSummary{
		AuditID:          auditID,
		TenantID:         tenantID,
		TotalKeywords:    len(siteKeywordMap),
		KeywordMap:       siteKeywordMap,
		Cannibalizations: cannibalizationCount,
		CompletedAt:      time.Now().UTC(),
	}

	return ExtractedSiteKeywords{
		PageKeywords:     pageKeywordProfiles,
		Findings:         findings,
		SiteSummary:      summary,
		UniqueKeywords:   len(siteKeywordMap),
		Cannibalizations: cannibalizationCount,
	}
}

//==========================================//
//             HELPER UTILITIES             //
//==========================================//

func tokenizeText(text string) []string {
	var tokens []string
	words := strings.Fields(text)

	for _, w := range words {
		clean := strings.ToLower(cleanWord(w))
		if len(clean) >= 2 && !englishStopwords[clean] {
			tokens = append(tokens, clean)
		}
	}
	return tokens
}

func generateNGrams(tokens []string) map[string]int {
	ngrams := make(map[string]int)

	// 1-grams (unigrams)
	for _, t := range tokens {
		if len(t) >= 3 && !englishStopwords[t] {
			ngrams[t]++
		}
	}

	// 2-grams (bigrams)
	for i := 0; i < len(tokens)-1; i++ {
		t1, t2 := tokens[i], tokens[i+1]
		if !englishStopwords[t1] && !englishStopwords[t2] && t1 != t2 {
			phrase := t1 + " " + t2
			ngrams[phrase]++
		}
	}

	// 3-grams (trigrams)
	for i := 0; i < len(tokens)-2; i++ {
		t1, t2, t3 := tokens[i], tokens[i+1], tokens[i+2]
		if !englishStopwords[t1] && !englishStopwords[t3] {
			phrase := t1 + " " + t2 + " " + t3
			ngrams[phrase]++
		}
	}

	return ngrams
}

func extractNamedEntities(rawText string) []string {
	// Look for sequences of 2 or 3 capitalized words (e.g. "Acme Corporation", "Google Search Console")
	re := regexp.MustCompile(`\b([A-Z][a-z0-9]+(?:\s+[A-Z][a-z0-9]+){1,2})\b`)
	matches := re.FindAllString(rawText, -1)

	entityMap := make(map[string]bool)
	var entities []string

	for _, m := range matches {
		clean := strings.TrimSpace(m)
		lower := strings.ToLower(clean)
		if !englishStopwords[lower] && !entityMap[clean] && len(clean) > 4 {
			entityMap[clean] = true
			entities = append(entities, clean)
		}
	}
	return entities
}

func cleanWord(w string) string {
	var b strings.Builder
	for _, r := range w {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func extractHeadingsText(headings []Heading, targetLevel int) string {
	var parts []string
	for _, h := range headings {
		if h.Level == targetLevel {
			parts = append(parts, h.Text)
		}
	}
	return strings.Join(parts, " ")
}

func extractSlug(pageURL string) string {
	u, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	last = strings.TrimSuffix(last, ".html")
	return strings.ReplaceAll(last, "-", " ")
}
