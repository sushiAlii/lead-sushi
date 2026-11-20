package leads

import (
	"testing"

	"github.com/sushiAlii/lead-sushi/internal/places"
)

func TestClassifySite(t *testing.T) {
	cases := []struct {
		raw  string
		want SiteStatus
	}{
		{"", SiteNone},
		{"   ", SiteNone},
		{"https://www.facebook.com/somebiz", SiteSocial},
		{"https://m.facebook.com/somebiz", SiteSocial},
		{"https://instagram.com/somebiz", SiteSocial},
		{"https://www.hipages.com.au/connect/somebiz", SiteSocial},
		{"https://somebiz.com.au", SiteOwn},
		{"https://notfacebook.com", SiteOwn},
		{"https://facebook.com.evil.example", SiteOwn},
	}
	for _, c := range cases {
		if got := ClassifySite(c.raw); got != c.want {
			t.Errorf("ClassifySite(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestVerdict(t *testing.T) {
	good := places.Place{
		Status:  "OPERATIONAL",
		Address: "1 Main St, Manly NSW 2095",
		Website: "https://example.com.au",
		Rating:  4.6,
		Reviews: 40,
	}
	weakSite := Criteria{Target: TargetWeakSite, MinReviews: 10, MinRating: 4.0}
	noSite := Criteria{Target: TargetNoSite, MinReviews: 10, MinRating: 4.0}

	cases := []struct {
		name     string
		mutate   func(p *places.Place)
		criteria Criteria
		want     string
	}{
		{"accepted for weaksite", func(p *places.Place) {}, weakSite, VerdictOK},
		{"closed", func(p *places.Place) { p.Status = "CLOSED_PERMANENTLY" }, weakSite, "closed"},
		{"uk address", func(p *places.Place) { p.Address = "94 Princes Rd, Liverpool L8 8AD, UK" }, weakSite, "outside_au"},
		{"social page rejected for weaksite", func(p *places.Place) { p.Website = "https://facebook.com/x" }, weakSite, "wrong_site_type"},
		{"no site rejected for weaksite", func(p *places.Place) { p.Website = "" }, weakSite, "wrong_site_type"},
		{"few reviews", func(p *places.Place) { p.Reviews = 3 }, weakSite, "few_reviews"},
		{"low rating", func(p *places.Place) { p.Rating = 3.2 }, weakSite, "low_rating"},
		{"own site rejected for nosite", func(p *places.Place) {}, noSite, "wrong_site_type"},
		{"empty site accepted for nosite", func(p *places.Place) { p.Website = "" }, noSite, VerdictOK},
		{"social accepted for nosite", func(p *places.Place) { p.Website = "https://instagram.com/x" }, noSite, VerdictOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			place := good
			c.mutate(&place)
			if got := Verdict(place, c.criteria); got != c.want {
				t.Errorf("Verdict = %q, want %q", got, c.want)
			}
		})
	}
}
