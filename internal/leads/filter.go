package leads

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/sushiAlii/lead-sushi/internal/audit"
	"github.com/sushiAlii/lead-sushi/internal/places"
)

type SiteStatus string

const (
	SiteNone   SiteStatus = "none"
	SiteSocial SiteStatus = "social_only"
	SiteOwn    SiteStatus = "has_site"
)

type Target string

const (
	TargetNoSite   Target = "nosite"
	TargetWeakSite Target = "weaksite"
)

const VerdictOK = "ok"

var australianAddress = regexp.MustCompile(`\b(NSW|VIC|QLD|WA|SA|TAS|ACT|NT) \d{4}\b`)

var nonSiteHosts = []string{
	"facebook.com",
	"fb.com",
	"instagram.com",
	"linktr.ee",
	"tiktok.com",
	"yellowpages.com.au",
	"truelocal.com.au",
	"hipages.com.au",
	"airtasker.com",
	"oneflare.com.au",
	"serviceseeking.com.au",
}

type Lead struct {
	places.Place
	Site  SiteStatus
	Audit *audit.Result
}

type Criteria struct {
	Target     Target
	MinReviews int
	MinRating  float64
}

func ClassifySite(raw string) SiteStatus {
	if strings.TrimSpace(raw) == "" {
		return SiteNone
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return SiteOwn
	}
	host := strings.ToLower(parsed.Hostname())
	for _, blocked := range nonSiteHosts {
		if host == blocked || strings.HasSuffix(host, "."+blocked) {
			return SiteSocial
		}
	}
	return SiteOwn
}

func (c Criteria) accepts(site SiteStatus) bool {
	if c.Target == TargetNoSite {
		return site == SiteNone || site == SiteSocial
	}
	return site == SiteOwn
}

func Verdict(p places.Place, c Criteria) string {
	site := ClassifySite(p.Website)
	checks := []struct {
		reason string
		passed bool
	}{
		{"closed", p.Status == "OPERATIONAL"},
		{"outside_au", australianAddress.MatchString(p.Address)},
		{"wrong_site_type", c.accepts(site)},
		{"few_reviews", p.Reviews >= c.MinReviews},
		{"low_rating", p.Rating >= c.MinRating},
	}
	for _, check := range checks {
		if !check.passed {
			return check.reason
		}
	}
	return VerdictOK
}

func New(p places.Place) Lead {
	return Lead{Place: p, Site: ClassifySite(p.Website)}
}
