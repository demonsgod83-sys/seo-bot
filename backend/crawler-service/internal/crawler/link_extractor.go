package crawler

import (
	"bytes"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

//==========================================//
//          URL NORMALIZATION               //
//==========================================//

// NormalizeURL applies canonical transformations:
// - Lowercases scheme and host
// - Strips default ports (:80 for http, :443 for https)
// - Strips tracking fragments (#...)
// - Strips trailing slash on root and paths (except pure root without path)
func NormalizeURL(u *url.URL) string {
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Host)

	// Strip default ports
	if scheme == "http" && strings.HasSuffix(host, ":80") {
		host = strings.TrimSuffix(host, ":80")
	} else if scheme == "https" && strings.HasSuffix(host, ":443") {
		host = strings.TrimSuffix(host, ":443")
	}

	path := u.Path
	if path == "/" {
		path = ""
	} else if strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}

	clean := &url.URL{
		Scheme:   scheme,
		Host:     host,
		Path:     path,
		RawQuery: u.RawQuery,
	}

	return clean.String()
}

//==========================================//
//          HTML LINK EXTRACTION            //
//==========================================//

// ExtractInternalLinks parses HTML body, discovers all <a href="..."> links,
// resolves relative paths, filters for same-domain only, and returns de-duplicated clean URLs.
func ExtractInternalLinks(baseParsedURL *url.URL, htmlBody []byte) []string {
	doc, err := html.Parse(bytes.NewReader(htmlBody))
	if err != nil {
		return nil
	}

	targetHost := strings.ToLower(baseParsedURL.Host)
	if idx := strings.Index(targetHost, ":"); idx != -1 {
		targetHost = targetHost[:idx]
	}

	discovered := make(map[string]bool)
	var results []string

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && strings.ToLower(n.Data) == "a" {
			for _, attr := range n.Attr {
				if strings.ToLower(attr.Key) == "href" {
					href := strings.TrimSpace(attr.Val)
					if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "tel:") {
						continue
					}

					parsedHref, err := url.Parse(href)
					if err != nil {
						continue
					}

					// Resolve relative paths against base URL
					resolved := baseParsedURL.ResolveReference(parsedHref)

					// Must be http or https
					resolvedScheme := strings.ToLower(resolved.Scheme)
					if resolvedScheme != "http" && resolvedScheme != "https" {
						continue
					}

					// Must be same domain
					resolvedHost := strings.ToLower(resolved.Host)
					resolvedHostOnly := resolvedHost
					if idx := strings.Index(resolvedHostOnly, ":"); idx != -1 {
						resolvedHostOnly = resolvedHostOnly[:idx]
					}

					if resolvedHostOnly != targetHost {
						continue
					}

					normalized := NormalizeURL(resolved)
					if !discovered[normalized] {
						discovered[normalized] = true
						results = append(results, normalized)
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)
	return results
}
