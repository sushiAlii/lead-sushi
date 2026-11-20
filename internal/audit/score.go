package audit

const (
	OutdatedYear = 2019
	BrokenPoints = 5
)

type rule struct {
	points int
	check  func(Facts) string
}

var rules = []rule{
	{3, func(f Facts) string { return flag(!f.HasViewport, "no-viewport") }},
	{3, func(f Facts) string { return flag(!f.HasCTA, "no-call-to-action") }},
	{2, func(f Facts) string { return flag(!f.HTTPS, "no-https") }},
	{2, func(f Facts) string { return flag(f.Slow, "slow") }},
	{2, func(f Facts) string { return flag(isOutdated(f), "outdated") }},
	{2, func(f Facts) string { return flag(f.Builder != "", "builder:"+f.Builder) }},
	{1, func(f Facts) string { return flag(missingBasics(f), "missing-seo") }},
}

func Score(f Facts) (int, []string) {
	if f.Blocked {
		return 0, []string{"blocked"}
	}
	if f.Broken {
		return BrokenPoints, []string{"broken"}
	}
	total := 0
	var problems []string
	for _, r := range rules {
		if problem := r.check(f); problem != "" {
			total += r.points
			problems = append(problems, problem)
		}
	}
	return total, problems
}

func flag(failed bool, label string) string {
	if failed {
		return label
	}
	return ""
}

func isOutdated(f Facts) bool {
	return f.LegacyMarkup || (f.CopyrightYear > 0 && f.CopyrightYear <= OutdatedYear)
}

func missingBasics(f Facts) bool {
	return !f.HasTitle || !f.HasDescription || !f.HasH1
}
