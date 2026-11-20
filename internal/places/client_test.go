package places

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func newTestClient(t *testing.T, budget int, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewClient("test-key", NewBudget(budget))
	client.Endpoint = server.URL
	client.PageDelay = 0
	return client
}

func pageJSON(t *testing.T, names []string, token string) []byte {
	t.Helper()
	places := make([]map[string]any, 0, len(names))
	for _, name := range names {
		places = append(places, map[string]any{
			"id":              "id-" + name,
			"displayName":     map[string]string{"text": name},
			"userRatingCount": 12,
			"rating":          4.5,
		})
	}
	body := map[string]any{"places": places}
	if token != "" {
		body["nextPageToken"] = token
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestSearchStopsAtMaxPages(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write(pageJSON(t, []string{"a", "b"}, "more"))
	})

	found, err := client.Search(context.Background(), "plumber")
	if err != nil {
		t.Fatal(err)
	}
	if got := int(calls.Load()); got != MaxPages {
		t.Errorf("calls = %d, want %d", got, MaxPages)
	}
	if len(found) != MaxPages*2 {
		t.Errorf("places = %d, want %d", len(found), MaxPages*2)
	}
}

func TestSearchStopsOnEmptyPage(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, 100, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Write(pageJSON(t, []string{"a"}, "more"))
			return
		}
		w.Write(pageJSON(t, nil, "more"))
	})

	found, err := client.Search(context.Background(), "plumber")
	if err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("calls = %d, want 2", got)
	}
	if len(found) != 1 {
		t.Errorf("places = %d, want 1", len(found))
	}
}

func TestSearchRespectsBudget(t *testing.T) {
	var calls atomic.Int32
	client := newTestClient(t, 2, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Write(pageJSON(t, []string{"a"}, "more"))
	})

	if _, err := client.Search(context.Background(), "plumber"); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("calls = %d, want 2", got)
	}
	if left := client.Budget.Remaining(); left != 0 {
		t.Errorf("remaining = %d, want 0", left)
	}
	if _, err := client.Search(context.Background(), "plumber"); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("calls after exhausted budget = %d, want 2", got)
	}
}

func TestSearchSendsExpectedRequest(t *testing.T) {
	var tokens []string
	client := newTestClient(t, 10, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Goog-Api-Key"); got != "test-key" {
			t.Errorf("api key header = %q", got)
		}
		if mask := r.Header.Get("X-Goog-FieldMask"); !strings.Contains(mask, "places.websiteUri") {
			t.Errorf("field mask missing websiteUri: %q", mask)
		}
		var body searchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.RegionCode != "AU" || body.TextQuery != "dentist in Manly" {
			t.Errorf("unexpected body: %+v", body)
		}
		tokens = append(tokens, body.PageToken)
		if len(tokens) == 1 {
			w.Write(pageJSON(t, []string{"a"}, "next-token"))
			return
		}
		w.Write(pageJSON(t, []string{"b"}, ""))
	})

	if _, err := client.Search(context.Background(), "dentist in Manly"); err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 2 || tokens[0] != "" || tokens[1] != "next-token" {
		t.Errorf("page tokens = %q", tokens)
	}
}

func TestSearchMapsFields(t *testing.T) {
	client := newTestClient(t, 10, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"places":[{"id":"x1","displayName":{"text":"Smile Dental"},
			"formattedAddress":"1 Main St, Manly NSW 2095","nationalPhoneNumber":"(02) 1111 2222",
			"websiteUri":"https://smile.example","rating":4.8,"userRatingCount":55,
			"businessStatus":"OPERATIONAL","googleMapsUri":"https://maps.example/x1"}]}`))
	})

	found, err := client.Search(context.Background(), "dentist")
	if err != nil {
		t.Fatal(err)
	}
	want := Place{
		ID: "x1", Name: "Smile Dental", Address: "1 Main St, Manly NSW 2095",
		Phone: "(02) 1111 2222", Website: "https://smile.example", Rating: 4.8,
		Reviews: 55, Status: "OPERATIONAL", MapsURL: "https://maps.example/x1",
	}
	if len(found) != 1 || found[0] != want {
		t.Errorf("got %+v, want %+v", found, want)
	}
}

func TestSearchReturnsErrorOnBadStatus(t *testing.T) {
	client := newTestClient(t, 10, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"API key not valid"}`, http.StatusForbidden)
	})

	_, err := client.Search(context.Background(), "plumber")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v, want 403 error", err)
	}
}

func TestSearchHonoursCancelledContext(t *testing.T) {
	client := newTestClient(t, 10, func(w http.ResponseWriter, r *http.Request) {
		w.Write(pageJSON(t, []string{"a"}, ""))
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.Search(ctx, "plumber"); err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
