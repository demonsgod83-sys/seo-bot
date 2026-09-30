package parser

import (
	"testing"
)

func TestExtractPageFacts(t *testing.T) {
	htmlSample := `<!DOCTYPE html>
<html lang="en-US">
<head>
	<meta charset="utf-8">
	<title>Acme Corporation - Super Widgets &amp; Tools</title>
	<meta name="description" content="The leading widget manufacturer since 1990.">
	<meta name="robots" content="index, follow, max-snippet:-1">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<link rel="canonical" href="https://example.com/products/widgets">
	<link rel="alternate" hreflang="es" href="/es/products/widgets">

	<!-- OpenGraph -->
	<meta property="og:title" content="Acme Widgets">
	<meta property="og:description" content="Discover our widgets">
	<meta property="og:image" content="https://example.com/og.png">

	<!-- Twitter Card -->
	<meta name="twitter:card" content="summary_large_image">
	<meta name="twitter:title" content="Twitter Acme">

	<!-- JSON-LD Structured Data -->
	<script type="application/ld+json">
	{
		"@context": "https://schema.org",
		"@type": "Product",
		"name": "Super Widget 3000"
	}
	</script>
	<style>
		.banner { color: red; }
	</style>
</head>
<body>
	<nav>
		<a href="/home">Home</a>
		<a href="/about">About</a>
		<span>Nav junk text to strip</span>
	</nav>

	<header>
		<h1>Company Logo Header</h1>
	</header>

	<main id="content">
		<h1>Main Product Overview</h1>
		<p>Welcome to Acme Corporation. We manufacture premium quality widgets for modern industrial applications.</p>
		
		<h2>Key Features</h2>
		<p>Our widgets are built from aerospace grade titanium alloy.</p>

		<!-- Subheading skipping H3 to H4 intentionally to test hierarchy fidelity -->
		<h4>Detailed Specifications</h4>
		<p>Engineered for high performance and durability under extreme conditions.</p>

		<!-- Second H1 to test multiple H1 fidelity -->
		<h1>Secondary Banner Heading</h1>

		<div class="links">
			<a href="/docs/manual.pdf">User Manual</a>
			<a href="https://external-partner.org" rel="nofollow sponsored">Partner Site</a>
			<a href="https://example.com/forum" rel="ugc">Community Forum</a>
		</div>

		<div class="gallery">
			<img src="/images/widget-blue.jpg" alt="Blue Super Widget" width="600" height="400" loading="lazy">
			<!-- Image missing alt attribute -->
			<img src="https://cdn.example.com/widget-red.png">
		</div>
	</main>

	<aside>
		<h3>Related Advertisements</h3>
		<p>Buy more things here from sponsors.</p>
	</aside>

	<footer>
		<p>&copy; 2026 Acme Corp. All rights reserved.</p>
	</footer>
</body>
</html>`

	pageURL := "https://example.com/products/widgets"
	facts, err := ExtractPageFacts(pageURL, []byte(htmlSample))
	if err != nil {
		t.Fatalf("ExtractPageFacts failed: %v", err)
	}

	// 1. Metadata tests
	if facts.Title != "Acme Corporation - Super Widgets & Tools" {
		t.Errorf("expected Title 'Acme Corporation - Super Widgets & Tools', got '%s'", facts.Title)
	}
	if facts.MetaDescription != "The leading widget manufacturer since 1990." {
		t.Errorf("expected MetaDescription, got '%s'", facts.MetaDescription)
	}
	if facts.MetaRobots != "index, follow, max-snippet:-1" {
		t.Errorf("expected MetaRobots, got '%s'", facts.MetaRobots)
	}
	if facts.CanonicalURL != "https://example.com/products/widgets" {
		t.Errorf("expected CanonicalURL 'https://example.com/products/widgets', got '%s'", facts.CanonicalURL)
	}
	if facts.DeclaredLang != "en-US" {
		t.Errorf("expected DeclaredLang 'en-US', got '%s'", facts.DeclaredLang)
	}
	if facts.Charset != "utf-8" {
		t.Errorf("expected Charset 'utf-8', got '%s'", facts.Charset)
	}

	// 2. OpenGraph & Twitter
	if facts.OpenGraph["og:title"] != "Acme Widgets" || facts.OpenGraph["og:image"] != "https://example.com/og.png" {
		t.Errorf("OpenGraph mismatch: %v", facts.OpenGraph)
	}
	if facts.TwitterCard["twitter:card"] != "summary_large_image" {
		t.Errorf("TwitterCard mismatch: %v", facts.TwitterCard)
	}

	// 3. Hreflang
	if len(facts.Hreflangs) != 1 || facts.Hreflangs[0].Hreflang != "es" || facts.Hreflangs[0].Href != "https://example.com/es/products/widgets" {
		t.Errorf("Hreflangs mismatch: %v", facts.Hreflangs)
	}

	// 4. Headings in document order (including H1 in header, H1 in main, H4 skip, second H1 in main, and H3 in aside)
	if len(facts.Headings) != 6 {
		t.Fatalf("expected 6 headings, got %d: %v", len(facts.Headings), facts.Headings)
	}
	expectedHeadings := []Heading{
		{Level: 1, Text: "Company Logo Header"},
		{Level: 1, Text: "Main Product Overview"},
		{Level: 2, Text: "Key Features"},
		{Level: 4, Text: "Detailed Specifications"},
		{Level: 1, Text: "Secondary Banner Heading"},
		{Level: 3, Text: "Related Advertisements"},
	}
	for i, h := range facts.Headings {
		if h.Level != expectedHeadings[i].Level || h.Text != expectedHeadings[i].Text {
			t.Errorf("heading[%d] mismatch: expected %+v, got %+v", i, expectedHeadings[i], h)
		}
	}

	// 5. Links
	if len(facts.Links) != 5 {
		t.Fatalf("expected 5 links (2 in nav + 3 in main), got %d: %+v", len(facts.Links), facts.Links)
	}

	// Verify external link with nofollow/sponsored
	var partnerLink *PageLink
	for _, l := range facts.Links {
		if l.Href == "https://external-partner.org" {
			partnerLink = &l
			break
		}
	}
	if partnerLink == nil {
		t.Fatalf("missing external partner link")
	}
	if partnerLink.IsInternal {
		t.Errorf("expected partner link to be external")
	}
	if !partnerLink.IsNoFollow || !partnerLink.IsSponsored {
		t.Errorf("expected nofollow and sponsored on partner link, got nofollow=%v, sponsored=%v", partnerLink.IsNoFollow, partnerLink.IsSponsored)
	}

	// 6. Images
	if len(facts.Images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(facts.Images))
	}
	img1 := facts.Images[0]
	if img1.Src != "https://example.com/images/widget-blue.jpg" || img1.Alt != "Blue Super Widget" || !img1.HasAlt || img1.Loading != "lazy" {
		t.Errorf("img1 mismatch: %+v", img1)
	}

	img2 := facts.Images[1]
	if img2.Src != "https://cdn.example.com/widget-red.png" || img2.HasAlt {
		t.Errorf("img2 expected has_alt=false, got %+v", img2)
	}

	// 7. Structured Data (JSON-LD)
	if len(facts.StructuredData) != 1 {
		t.Fatalf("expected 1 structured data block, got %d", len(facts.StructuredData))
	}

	// 8. Body Text & Word Count (Boilerplate <nav>, <footer>, <aside>, <script>, <style> stripped)
	if facts.WordCount < 20 {
		t.Errorf("expected word count >= 20, got %d", facts.WordCount)
	}
	if len(facts.BodyText) == 0 {
		t.Errorf("expected non-empty body text")
	}
}
