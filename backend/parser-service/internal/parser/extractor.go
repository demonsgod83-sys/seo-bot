package parser

import (
	"bytes"
	"net/url"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

//==========================================//
//          HTML STRUCTURED EXTRACTOR       //
//==========================================//

// ExtractedPageData holds all facts extracted from a single HTML document.
type ExtractedPageData struct {
	Title           string
	MetaDescription string
	MetaRobots      string
	CanonicalURL    string
	Viewport        string
	Charset         string
	DeclaredLang    string

	Hreflangs       []HreflangTag
	OpenGraph       map[string]string
	TwitterCard     map[string]string
	Headings        []Heading
	Links           []PageLink
	Images          []PageImage
	StructuredData  []string

	BodyText        string
	WordCount       int
	CharCount       int
}

// ExtractPageFacts parses raw HTML bytes and extracts all SEO metadata, hierarchy, links, images, and body content.
func ExtractPageFacts(pageURL string, htmlBytes []byte) (*ExtractedPageData, error) {
	parsedBaseURL, err := url.Parse(pageURL)
	if err != nil {
		return nil, err
	}

	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return nil, err
	}

	data := &ExtractedPageData{
		OpenGraph:      make(map[string]string),
		TwitterCard:    make(map[string]string),
		Hreflangs:      make([]HreflangTag, 0),
		Headings:       make([]Heading, 0),
		Links:          make([]PageLink, 0),
		Images:         make([]PageImage, 0),
		StructuredData: make([]string, 0),
	}

	targetHost := strings.ToLower(parsedBaseURL.Host)
	if idx := strings.Index(targetHost, ":"); idx != -1 {
		targetHost = targetHost[:idx]
	}

	var bodyTextBuilder strings.Builder

	// Traverse the HTML AST
	var traverse func(*html.Node, bool)
	traverse = func(n *html.Node, inBoilerplate bool) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Html:
				for _, attr := range n.Attr {
					if strings.EqualFold(attr.Key, "lang") {
						data.DeclaredLang = strings.TrimSpace(attr.Val)
					}
				}

			case atom.Title:
				if data.Title == "" {
					data.Title = cleanText(extractNodeText(n))
				}

			case atom.Meta:
				processMetaTag(n, data)

			case atom.Link:
				processLinkTag(n, parsedBaseURL, data)

			case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
				level := int(n.Data[1] - '0')
				text := cleanText(extractNodeText(n))
				data.Headings = append(data.Headings, Heading{
					Level: level,
					Text:  text,
				})

			case atom.A:
				processAnchorTag(n, parsedBaseURL, targetHost, data)

			case atom.Img:
				processImgTag(n, parsedBaseURL, data)

			case atom.Script:
				// Extract JSON-LD structured data
				for _, attr := range n.Attr {
					if strings.EqualFold(attr.Key, "type") && strings.EqualFold(strings.TrimSpace(attr.Val), "application/ld+json") {
						scriptContent := strings.TrimSpace(extractNodeText(n))
						if scriptContent != "" {
							data.StructuredData = append(data.StructuredData, scriptContent)
						}
					}
				}
			}

			// Check boilerplate elements to exclude from body text
			if isBoilerplateElement(n.DataAtom) {
				inBoilerplate = true
			}
		}

		// Collect body text (only from visible non-boilerplate elements)
		if n.Type == html.TextNode && !inBoilerplate {
			txt := strings.TrimSpace(n.Data)
			if txt != "" {
				bodyTextBuilder.WriteString(txt)
				bodyTextBuilder.WriteString(" ")
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c, inBoilerplate)
		}
	}

	traverse(doc, false)

	// Clean body text and compute word/character counts
	cleanedBody := cleanText(bodyTextBuilder.String())
	data.BodyText = cleanedBody
	data.CharCount = len(cleanedBody)
	data.WordCount = countWords(cleanedBody)

	return data, nil
}

//==========================================//
//             HELPER PROCESSORS            //
//==========================================//

func processMetaTag(n *html.Node, data *ExtractedPageData) {
	var name, property, content, httpEquiv, charset string

	for _, attr := range n.Attr {
		key := strings.ToLower(strings.TrimSpace(attr.Key))
		val := strings.TrimSpace(attr.Val)

		switch key {
		case "name":
			name = strings.ToLower(val)
		case "property":
			property = strings.ToLower(val)
		case "content":
			content = val
		case "http-equiv":
			httpEquiv = strings.ToLower(val)
		case "charset":
			charset = val
		}
	}

	if charset != "" && data.Charset == "" {
		data.Charset = charset
	}

	if httpEquiv == "content-type" && strings.Contains(strings.ToLower(content), "charset=") && data.Charset == "" {
		idx := strings.Index(strings.ToLower(content), "charset=")
		data.Charset = strings.TrimSpace(content[idx+8:])
	}

	if name == "description" || property == "description" {
		if data.MetaDescription == "" {
			data.MetaDescription = content
		}
	}

	if name == "robots" || name == "googlebot" {
		if data.MetaRobots == "" {
			data.MetaRobots = content
		}
	}

	if name == "viewport" {
		if data.Viewport == "" {
			data.Viewport = content
		}
	}

	// OpenGraph tags
	if strings.HasPrefix(property, "og:") || strings.HasPrefix(name, "og:") {
		tagKey := property
		if tagKey == "" {
			tagKey = name
		}
		data.OpenGraph[tagKey] = content
	}

	// TwitterCard tags
	if strings.HasPrefix(name, "twitter:") || strings.HasPrefix(property, "twitter:") {
		tagKey := name
		if tagKey == "" {
			tagKey = property
		}
		data.TwitterCard[tagKey] = content
	}
}

func processLinkTag(n *html.Node, baseURL *url.URL, data *ExtractedPageData) {
	var rel, href, hreflang string

	for _, attr := range n.Attr {
		key := strings.ToLower(strings.TrimSpace(attr.Key))
		val := strings.TrimSpace(attr.Val)

		switch key {
		case "rel":
			rel = strings.ToLower(val)
		case "href":
			href = val
		case "hreflang":
			hreflang = strings.ToLower(val)
		}
	}

	if href == "" {
		return
	}

	parsedHref, err := url.Parse(href)
	if err != nil {
		return
	}
	resolved := baseURL.ResolveReference(parsedHref).String()

	if rel == "canonical" && data.CanonicalURL == "" {
		data.CanonicalURL = resolved
	}

	if (rel == "alternate" || strings.Contains(rel, "alternate")) && hreflang != "" {
		data.Hreflangs = append(data.Hreflangs, HreflangTag{
			Hreflang: hreflang,
			Href:     resolved,
		})
	}
}

func processAnchorTag(n *html.Node, baseURL *url.URL, targetHost string, data *ExtractedPageData) {
	var href, rel string
	for _, attr := range n.Attr {
		if strings.EqualFold(attr.Key, "href") {
			href = strings.TrimSpace(attr.Val)
		} else if strings.EqualFold(attr.Key, "rel") {
			rel = strings.ToLower(attr.Val)
		}
	}

	if href == "" || strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "tel:") {
		return
	}

	parsedHref, err := url.Parse(href)
	if err != nil {
		return
	}
	resolved := baseURL.ResolveReference(parsedHref)

	resolvedScheme := strings.ToLower(resolved.Scheme)
	if resolvedScheme != "http" && resolvedScheme != "https" {
		return
	}

	resolvedHost := strings.ToLower(resolved.Host)
	if idx := strings.Index(resolvedHost, ":"); idx != -1 {
		resolvedHost = resolvedHost[:idx]
	}

	isInternal := resolvedHost == targetHost
	isNoFollow := strings.Contains(rel, "nofollow")
	isSponsored := strings.Contains(rel, "sponsored")
	isUgc := strings.Contains(rel, "ugc")

	anchorText := cleanText(extractNodeText(n))

	data.Links = append(data.Links, PageLink{
		Href:        resolved.String(),
		Text:        anchorText,
		IsInternal:  isInternal,
		IsNoFollow:  isNoFollow,
		IsSponsored: isSponsored,
		IsUgc:       isUgc,
	})
}

func processImgTag(n *html.Node, baseURL *url.URL, data *ExtractedPageData) {
	var src, alt, width, height, loading string
	hasAlt := false

	for _, attr := range n.Attr {
		key := strings.ToLower(strings.TrimSpace(attr.Key))
		val := strings.TrimSpace(attr.Val)

		switch key {
		case "src":
			src = val
		case "alt":
			alt = val
			hasAlt = true
		case "width":
			width = val
		case "height":
			height = val
		case "loading":
			loading = val
		}
	}

	if src == "" {
		return
	}

	parsedSrc, err := url.Parse(src)
	if err == nil {
		src = baseURL.ResolveReference(parsedSrc).String()
	}

	data.Images = append(data.Images, PageImage{
		Src:     src,
		Alt:     alt,
		HasAlt:  hasAlt,
		Width:   width,
		Height:  height,
		Loading: loading,
	})
}

func isBoilerplateElement(a atom.Atom) bool {
	switch a {
	case atom.Nav, atom.Footer, atom.Header, atom.Aside, atom.Script, atom.Style,
		atom.Noscript, atom.Svg, atom.Form, atom.Iframe:
		return true
	}
	return false
}

func extractNodeText(n *html.Node) string {
	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(curr *html.Node) {
		if curr.Type == html.TextNode {
			buf.WriteString(curr.Data)
		}
		for c := curr.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return buf.String()
}

func cleanText(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

func countWords(s string) int {
	count := 0
	inWord := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if !inWord {
				count++
				inWord = true
			}
		} else {
			inWord = false
		}
	}
	return count
}
