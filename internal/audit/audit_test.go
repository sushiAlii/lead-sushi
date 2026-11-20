package audit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func newTestAuditor() *Auditor {
	auditor := New("lead-sushi-test", 4)
	auditor.HTTP.Timeout = 2 * time.Second
	return auditor
}

func serve(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server.URL
}

func TestAuditGoodPageOnlyLosesPointsForPlainHTTP(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(goodPage))
	})

	result := newTestAuditor().Audit(context.Background(), url)

	if !slices.Equal(result.Problems, []string{"no-https"}) {
		t.Errorf("problems = %v, want only no-https (test server is plain http)", result.Problems)
	}
}

func TestAuditWeakPage(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(weakPage))
	})

	result := newTestAuditor().Audit(context.Background(), url)

	for _, want := range []string{"no-viewport", "no-call-to-action", "outdated", "builder:wix", "missing-seo"} {
		if !slices.Contains(result.Problems, want) {
			t.Errorf("problems %v missing %q", result.Problems, want)
		}
	}
	if result.Score < 5 {
		t.Errorf("score = %d, want at least 5", result.Score)
	}
}

func TestAuditSendsUserAgent(t *testing.T) {
	var got atomic.Value
	url := serve(t, func(w http.ResponseWriter, r *http.Request) {
		got.Store(r.Header.Get("User-Agent"))
		w.Write([]byte(goodPage))
	})

	newTestAuditor().Audit(context.Background(), url)

	if got.Load() != "lead-sushi-test" {
		t.Errorf("user agent = %v", got.Load())
	}
}

func TestAuditSlowSite(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(60 * time.Millisecond)
		w.Write([]byte(goodPage))
	})
	auditor := newTestAuditor()
	auditor.SlowAfter = 20 * time.Millisecond

	result := auditor.Audit(context.Background(), url)

	if !slices.Contains(result.Problems, "slow") {
		t.Errorf("problems = %v, want slow", result.Problems)
	}
}

func TestAuditServerErrorIsBroken(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	result := newTestAuditor().Audit(context.Background(), url)

	if !slices.Equal(result.Problems, []string{"broken"}) || result.Score != BrokenPoints {
		t.Errorf("got %d %v, want broken", result.Score, result.Problems)
	}
}

func TestAuditBotWallIsBlockedNotBroken(t *testing.T) {
	url := serve(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusForbidden)
	})

	result := newTestAuditor().Audit(context.Background(), url)

	if !result.Blocked || result.Score != 0 {
		t.Errorf("got %+v, want blocked with score 0", result)
	}
}

func TestAuditUnreachableHostIsBroken(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	result := newTestAuditor().Audit(context.Background(), url)

	if !slices.Equal(result.Problems, []string{"broken"}) {
		t.Errorf("problems = %v, want broken", result.Problems)
	}
}

func TestAuditAddsSchemeWhenMissing(t *testing.T) {
	if got := withScheme("example.com.au"); got != "https://example.com.au" {
		t.Errorf("withScheme = %q", got)
	}
	if got := withScheme("http://example.com.au"); got != "http://example.com.au" {
		t.Errorf("withScheme = %q", got)
	}
}

func TestRunKeepsResultsAlignedWithInput(t *testing.T) {
	good := serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(goodPage)) })
	weak := serve(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(weakPage)) })
	urls := []string{good, weak, good, weak, weak, good}
	var reported atomic.Int32

	results := newTestAuditor().Run(context.Background(), urls, func(done, total int) {
		reported.Add(1)
	})

	if len(results) != len(urls) || int(reported.Load()) != len(urls) {
		t.Fatalf("results = %d, progress calls = %d, want %d", len(results), reported.Load(), len(urls))
	}
	for i, url := range urls {
		isWeak := url == weak
		if (results[i].Score >= 5) != isWeak {
			t.Errorf("result %d score = %d, weak expected = %v", i, results[i].Score, isWeak)
		}
	}
}
