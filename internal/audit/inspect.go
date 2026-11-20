package audit

import (
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type Facts struct {
	Broken         bool
	Blocked        bool
	HTTPS          bool
	Slow           bool
	HasViewport    bool
	HasTitle       bool
	HasDescription bool
	HasH1          bool
	HasCTA         bool
	LegacyMarkup   bool
	Builder        string
	CopyrightYear  int
}

type builderMarker struct {
	needle string
	name   string
}

var builderMarkers = []builderMarker{
	{"wixstatic.com", "wix"},
	{"parastorage.com", "wix"},
	{"wix.com", "wix"},
	{"weebly", "weebly"},
	{"yolasite", "yola"},
	{"squarespace", "squarespace"},
	{"wsimg.com", "godaddy"},
	{"godaddy", "godaddy"},
}

var (
	ctaPattern       = regexp.MustCompile(`(?i)\b(book|booking|appointments?|contact|enquir\w*|quote)\b`)
	copyrightPattern = regexp.MustCompile(`(?i)(?:©|copyright)[^\n]{0,40}`)
	yearPattern      = regexp.MustCompile(`\b(?:19|20)\d{2}\b`)
	legacyTags       = map[string]bool{"font": true, "marquee": true, "blink": true}
	skippedContent   = map[string]bool{"script": true, "style": true, "noscript": true}
)

func Inspect(r io.Reader) (Facts, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return Facts{}, err
	}
	var facts Facts
	var text strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			text.WriteString(n.Data)
			text.WriteByte(' ')
		case html.ElementNode:
			inspectElement(n, &facts)
			if skippedContent[n.Data] {
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	facts.CopyrightYear = latestCopyrightYear(text.String())
	return facts, nil
}

func inspectElement(n *html.Node, facts *Facts) {
	detectBuilder(n, facts)
	if legacyTags[n.Data] {
		facts.LegacyMarkup = true
	}
	switch n.Data {
	case "meta":
		inspectMeta(n, facts)
	case "title":
		facts.HasTitle = strings.TrimSpace(textOf(n)) != ""
	case "h1":
		facts.HasH1 = true
	case "form":
		facts.HasCTA = true
	case "a":
		if isCallToAction(n) {
			facts.HasCTA = true
		}
	}
}

func inspectMeta(n *html.Node, facts *Facts) {
	content := strings.TrimSpace(attr(n, "content"))
	switch strings.ToLower(attr(n, "name")) {
	case "viewport":
		facts.HasViewport = true
	case "description":
		facts.HasDescription = content != ""
	}
}

func detectBuilder(n *html.Node, facts *Facts) {
	if facts.Builder != "" {
		return
	}
	candidates := []string{attr(n, "src"), attr(n, "href")}
	if n.Data == "meta" && strings.EqualFold(attr(n, "name"), "generator") {
		candidates = append(candidates, attr(n, "content"))
	}
	for _, candidate := range candidates {
		lowered := strings.ToLower(candidate)
		for _, marker := range builderMarkers {
			if strings.Contains(lowered, marker.needle) {
				facts.Builder = marker.name
				return
			}
		}
	}
}

func isCallToAction(n *html.Node) bool {
	href := strings.TrimSpace(attr(n, "href"))
	if strings.HasPrefix(strings.ToLower(href), "tel:") {
		return true
	}
	path := href
	if parsed, err := url.Parse(href); err == nil {
		path = parsed.Path
	}
	return ctaPattern.MatchString(textOf(n)) || ctaPattern.MatchString(path)
}

func latestCopyrightYear(text string) int {
	latest := 0
	for _, phrase := range copyrightPattern.FindAllString(text, -1) {
		for _, raw := range yearPattern.FindAllString(phrase, -1) {
			if year, err := strconv.Atoi(raw); err == nil && year > latest {
				latest = year
			}
		}
	}
	return latest
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func textOf(n *html.Node) string {
	var b strings.Builder
	var collect func(*html.Node)
	collect = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(n)
	return b.String()
}
