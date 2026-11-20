package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sushiAlii/lead-sushi/internal/audit"
	"github.com/sushiAlii/lead-sushi/internal/export"
	"github.com/sushiAlii/lead-sushi/internal/leads"
	"github.com/sushiAlii/lead-sushi/internal/places"
)

const (
	apiKeyEnv = "GOOGLE_PLACES_API_KEY"
	repoURL   = "https://github.com/sushiAlii/lead-sushi"
)

var version = "dev"

type config struct {
	city        string
	niche       string
	terms       string
	target      string
	minReviews  int
	minRating   float64
	minScore    int
	maxRequests int
	workers     int
	out         string
	showVersion bool
}

func main() {
	err := run(os.Args[1:], os.Stdout, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	cfg, err := parseFlags(args, stderr)
	if err != nil {
		return err
	}
	if cfg.showVersion {
		fmt.Fprintln(stdout, "lead-sushi", version)
		return nil
	}
	city, terms, err := resolveSearch(cfg)
	if err != nil {
		return err
	}
	apiKey := os.Getenv(apiKeyEnv)
	if apiKey == "" {
		return fmt.Errorf("%s is not set: create a Places API (New) key and export it first", apiKeyEnv)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := places.NewClient(apiKey, places.NewBudget(cfg.maxRequests))
	criteria := leads.Criteria{
		Target:     leads.Target(cfg.target),
		MinReviews: cfg.minReviews,
		MinRating:  cfg.minRating,
	}

	candidates, err := collect(ctx, client, places.Queries(city, terms), criteria, stdout)
	if err != nil {
		return err
	}
	if criteria.Target == leads.TargetWeakSite {
		candidates, err = auditAll(ctx, candidates, cfg, stdout, stderr)
		if err != nil {
			return err
		}
	}

	export.Sort(candidates)
	return writeCSV(cfg.out, candidates, stdout)
}

func parseFlags(args []string, stderr io.Writer) (config, error) {
	var cfg config
	fs := flag.NewFlagSet("lead-sushi", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.city, "city", "sydney", "city preset: "+strings.Join(places.CityNames(), ", "))
	fs.StringVar(&cfg.niche, "niche", "clinics", "niche preset: "+strings.Join(places.NicheNames(), ", "))
	fs.StringVar(&cfg.terms, "terms", "", "comma-separated search terms, overrides --niche")
	fs.StringVar(&cfg.target, "target", string(leads.TargetWeakSite), "weaksite: audit existing sites, nosite: businesses without one")
	fs.IntVar(&cfg.minReviews, "min-reviews", 10, "minimum Google review count")
	fs.Float64Var(&cfg.minRating, "min-rating", 4.0, "minimum Google rating")
	fs.IntVar(&cfg.minScore, "min-score", 5, "minimum weakness score to keep (weaksite target)")
	fs.IntVar(&cfg.maxRequests, "max-requests", 60, "cap on Places API requests for the whole run")
	fs.IntVar(&cfg.workers, "workers", 10, "concurrent website audits")
	fs.StringVar(&cfg.out, "out", "leads.csv", "output CSV path")
	fs.BoolVar(&cfg.showVersion, "version", false, "print version and exit")
	return cfg, fs.Parse(args)
}

func resolveSearch(cfg config) (places.City, []string, error) {
	city, ok := places.Cities[strings.ToLower(cfg.city)]
	if !ok {
		return places.City{}, nil, fmt.Errorf("unknown city %q (choose from %s)", cfg.city, strings.Join(places.CityNames(), ", "))
	}
	target := leads.Target(cfg.target)
	if target != leads.TargetNoSite && target != leads.TargetWeakSite {
		return places.City{}, nil, fmt.Errorf("unknown target %q (choose weaksite or nosite)", cfg.target)
	}
	if cfg.maxRequests < 1 || cfg.workers < 1 {
		return places.City{}, nil, errors.New("--max-requests and --workers must be at least 1")
	}
	terms, err := resolveTerms(cfg)
	return city, terms, err
}

func resolveTerms(cfg config) ([]string, error) {
	if cfg.terms != "" {
		return splitTerms(cfg.terms), nil
	}
	terms, ok := places.Niches[strings.ToLower(cfg.niche)]
	if !ok {
		return nil, fmt.Errorf("unknown niche %q (choose from %s)", cfg.niche, strings.Join(places.NicheNames(), ", "))
	}
	return terms, nil
}

func splitTerms(raw string) []string {
	var terms []string
	for _, term := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(term); trimmed != "" {
			terms = append(terms, trimmed)
		}
	}
	return terms
}

func collect(ctx context.Context, client *places.Client, queries []string, criteria leads.Criteria, stdout io.Writer) ([]leads.Lead, error) {
	seen := map[string]bool{}
	var candidates []leads.Lead
	for _, query := range queries {
		if client.Budget.Remaining() == 0 {
			fmt.Fprintln(stdout, "request budget exhausted")
			break
		}
		found, err := client.Search(ctx, query)
		if err != nil {
			return nil, err
		}
		outcomes := map[string]int{}
		for _, place := range found {
			verdict := leads.Verdict(place, criteria)
			if seen[place.ID] {
				verdict = "duplicate"
			}
			outcomes[verdict]++
			if verdict == leads.VerdictOK {
				seen[place.ID] = true
				candidates = append(candidates, leads.New(place))
			}
		}
		fmt.Fprintf(stdout, "%4d candidates | %3d left | %s | %v\n", len(candidates), client.Budget.Remaining(), query, outcomes)
	}
	return candidates, nil
}

func auditAll(ctx context.Context, candidates []leads.Lead, cfg config, stdout, stderr io.Writer) ([]leads.Lead, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}
	urls := make([]string, len(candidates))
	for i, candidate := range candidates {
		urls[i] = candidate.Website
	}
	userAgent := fmt.Sprintf("lead-sushi/%s (+%s)", version, repoURL)
	auditor := audit.New(userAgent, cfg.workers)

	fmt.Fprintf(stdout, "auditing %d websites with %d workers\n", len(urls), cfg.workers)
	results := auditor.Run(ctx, urls, func(done, total int) {
		fmt.Fprintf(stderr, "\raudited %d/%d", done, total)
	})
	fmt.Fprintln(stderr)
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("interrupted during audit: %w", err)
	}

	var weak []leads.Lead
	for i, candidate := range candidates {
		result := results[i]
		candidate.Audit = &result
		if result.Score >= cfg.minScore {
			weak = append(weak, candidate)
		}
	}
	fmt.Fprintf(stdout, "%d of %d sites scored %d or higher\n", len(weak), len(candidates), cfg.minScore)
	return weak, nil
}

func writeCSV(path string, rows []leads.Lead, stdout io.Writer) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := export.Write(file, rows); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %d leads to %s\n", len(rows), path)
	return nil
}
