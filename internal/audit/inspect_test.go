package audit

import (
	"strings"
	"testing"
)

const goodPage = `<!doctype html><html><head>
<title>Smile Dental</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="description" content="Family dentist in Manly">
</head><body>
<h1>Welcome</h1>
<a href="tel:0291112222">Call us</a>
<footer>© 2026 Smile Dental</footer>
</body></html>`

const weakPage = `<html><head>
<script src="https://static.wixstatic.com/bundle.js"></script>
</head><body>
<font color="red">Welcome</font>
<a href="https://facebook.com/smile">Follow us on Facebook</a>
<p>Copyright 2014 - 2016 Smile Dental</p>
</body></html>`

func inspect(t *testing.T, page string) Facts {
	t.Helper()
	facts, err := Inspect(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	return facts
}

func TestInspectGoodPage(t *testing.T) {
	facts := inspect(t, goodPage)

	if !facts.HasViewport || !facts.HasTitle || !facts.HasDescription || !facts.HasH1 || !facts.HasCTA {
		t.Errorf("expected all basics present: %+v", facts)
	}
	if facts.LegacyMarkup || facts.Builder != "" {
		t.Errorf("unexpected legacy or builder flags: %+v", facts)
	}
	if facts.CopyrightYear != 2026 {
		t.Errorf("copyright year = %d, want 2026", facts.CopyrightYear)
	}
}

func TestInspectWeakPage(t *testing.T) {
	facts := inspect(t, weakPage)

	if facts.HasViewport || facts.HasTitle || facts.HasDescription || facts.HasH1 {
		t.Errorf("expected missing basics: %+v", facts)
	}
	if facts.HasCTA {
		t.Error("a Facebook link must not count as a call to action")
	}
	if !facts.LegacyMarkup {
		t.Error("font tag should count as legacy markup")
	}
	if facts.Builder != "wix" {
		t.Errorf("builder = %q, want wix", facts.Builder)
	}
	if facts.CopyrightYear != 2016 {
		t.Errorf("copyright year = %d, want latest of range (2016)", facts.CopyrightYear)
	}
}

func TestInspectCallToActionVariants(t *testing.T) {
	cases := map[string]string{
		"contact link": `<a href="/contact-us">Get in touch</a>`,
		"booking text": `<a href="/x">Book online</a>`,
		"appointments": `<a href="/appointments">Visit</a>`,
		"quote":        `<a href="/x">Get a quote</a>`,
		"contact form": `<form action="/send"></form>`,
		"telephone":    `<a href="TEL:+61291112222">Ring</a>`,
	}
	for name, body := range cases {
		if !inspect(t, "<html><body>"+body+"</body></html>").HasCTA {
			t.Errorf("%s: expected call to action", name)
		}
	}
}

func TestInspectIgnoresScriptText(t *testing.T) {
	facts := inspect(t, `<html><body><script>var c = "copyright 2010";</script></body></html>`)

	if facts.CopyrightYear != 0 {
		t.Errorf("copyright year = %d, want 0 (script text ignored)", facts.CopyrightYear)
	}
}

func TestInspectBuilderMarkers(t *testing.T) {
	cases := map[string]string{
		"weebly":      `<link href="https://cdn2.editmysite.com/x.css"><script src="https://www.weebly.com/js/a.js"></script>`,
		"squarespace": `<meta name="generator" content="Squarespace 7.1">`,
		"godaddy":     `<img src="https://img1.wsimg.com/logo.png">`,
		"yola":        `<script src="https://site.yolasite.com/x.js"></script>`,
	}
	for want, head := range cases {
		facts := inspect(t, "<html><head>"+head+"</head></html>")
		if facts.Builder != want {
			t.Errorf("builder = %q, want %q", facts.Builder, want)
		}
	}
}
