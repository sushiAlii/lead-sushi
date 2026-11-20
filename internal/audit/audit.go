package audit

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultTimeout   = 10 * time.Second
	DefaultSlowAfter = 3 * time.Second
	DefaultMaxBytes  = 2 << 20
)

type Result struct {
	Score    int
	Problems []string
	FinalURL string
	Blocked  bool
}

type Auditor struct {
	HTTP      *http.Client
	UserAgent string
	Workers   int
	SlowAfter time.Duration
	MaxBytes  int64
}

func New(userAgent string, workers int) *Auditor {
	return &Auditor{
		HTTP:      &http.Client{Timeout: DefaultTimeout},
		UserAgent: userAgent,
		Workers:   workers,
		SlowAfter: DefaultSlowAfter,
		MaxBytes:  DefaultMaxBytes,
	}
}

func (a *Auditor) Run(ctx context.Context, urls []string, progress func(done, total int)) []Result {
	results := make([]Result, len(urls))
	jobs := make(chan int)
	var done atomic.Int64
	var wg sync.WaitGroup

	for w := 0; w < max(a.Workers, 1); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i] = a.Audit(ctx, urls[i])
				if progress != nil {
					progress(int(done.Add(1)), len(urls))
				}
			}
		}()
	}
	for i := range urls {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return results
}

func (a *Auditor) Audit(ctx context.Context, rawURL string) Result {
	target := withScheme(rawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return resultFrom(Facts{Broken: true}, target)
	}
	req.Header.Set("User-Agent", a.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	started := time.Now()
	resp, err := a.HTTP.Do(req)
	if err != nil {
		return resultFrom(Facts{Broken: true}, target)
	}
	defer resp.Body.Close()
	finalURL := resp.Request.URL.String()

	if isBotWall(resp.StatusCode) {
		return resultFrom(Facts{Blocked: true}, finalURL)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return resultFrom(Facts{Broken: true}, finalURL)
	}

	facts, err := Inspect(io.LimitReader(resp.Body, a.MaxBytes))
	if err != nil {
		return resultFrom(Facts{Broken: true}, finalURL)
	}
	facts.HTTPS = resp.Request.URL.Scheme == "https"
	facts.Slow = time.Since(started) > a.SlowAfter
	return resultFrom(facts, finalURL)
}

func resultFrom(f Facts, finalURL string) Result {
	score, problems := Score(f)
	return Result{Score: score, Problems: problems, FinalURL: finalURL, Blocked: f.Blocked}
}

func isBotWall(status int) bool {
	return status == http.StatusForbidden ||
		status == http.StatusTooManyRequests ||
		status == http.StatusServiceUnavailable
}

func withScheme(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.Contains(trimmed, "://") {
		return trimmed
	}
	return "https://" + trimmed
}
