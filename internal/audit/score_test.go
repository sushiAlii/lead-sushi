package audit

import (
	"reflect"
	"testing"
)

func healthy() Facts {
	return Facts{
		HTTPS: true, HasViewport: true, HasTitle: true, HasDescription: true,
		HasH1: true, HasCTA: true, CopyrightYear: 2026,
	}
}

func TestScore(t *testing.T) {
	cases := []struct {
		name         string
		mutate       func(f *Facts)
		wantScore    int
		wantProblems []string
	}{
		{"healthy site", func(f *Facts) {}, 0, nil},
		{"no viewport", func(f *Facts) { f.HasViewport = false }, 3, []string{"no-viewport"}},
		{"no call to action", func(f *Facts) { f.HasCTA = false }, 3, []string{"no-call-to-action"}},
		{"no https", func(f *Facts) { f.HTTPS = false }, 2, []string{"no-https"}},
		{"slow", func(f *Facts) { f.Slow = true }, 2, []string{"slow"}},
		{"old copyright", func(f *Facts) { f.CopyrightYear = 2018 }, 2, []string{"outdated"}},
		{"cutoff year counts as outdated", func(f *Facts) { f.CopyrightYear = OutdatedYear }, 2, []string{"outdated"}},
		{"year after cutoff is fine", func(f *Facts) { f.CopyrightYear = OutdatedYear + 1 }, 0, nil},
		{"unknown year is fine", func(f *Facts) { f.CopyrightYear = 0 }, 0, nil},
		{"legacy markup", func(f *Facts) { f.LegacyMarkup = true }, 2, []string{"outdated"}},
		{"builder", func(f *Facts) { f.Builder = "wix" }, 2, []string{"builder:wix"}},
		{"missing title", func(f *Facts) { f.HasTitle = false }, 1, []string{"missing-seo"}},
		{"missing description and h1 count once", func(f *Facts) { f.HasDescription, f.HasH1 = false, false }, 1, []string{"missing-seo"}},
		{
			"everything wrong",
			func(f *Facts) { *f = Facts{Slow: true, CopyrightYear: 2012, Builder: "wix"} },
			3 + 3 + 2 + 2 + 2 + 2 + 1,
			[]string{"no-viewport", "no-call-to-action", "no-https", "slow", "outdated", "builder:wix", "missing-seo"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			facts := healthy()
			c.mutate(&facts)

			score, problems := Score(facts)

			if score != c.wantScore {
				t.Errorf("score = %d, want %d", score, c.wantScore)
			}
			if !reflect.DeepEqual(problems, c.wantProblems) {
				t.Errorf("problems = %v, want %v", problems, c.wantProblems)
			}
		})
	}
}

func TestScoreBrokenAlwaysClearsDefaultThreshold(t *testing.T) {
	score, problems := Score(Facts{Broken: true})

	if score < 5 {
		t.Errorf("broken score = %d, want at least the default --min-score of 5", score)
	}
	if !reflect.DeepEqual(problems, []string{"broken"}) {
		t.Errorf("problems = %v", problems)
	}
}

func TestScoreBlockedIsNeverALead(t *testing.T) {
	score, problems := Score(Facts{Blocked: true})

	if score != 0 || !reflect.DeepEqual(problems, []string{"blocked"}) {
		t.Errorf("got %d %v, want 0 [blocked]", score, problems)
	}
}
