package places

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	DefaultEndpoint = "https://places.googleapis.com/v1/places:searchText"
	MaxPages        = 3
	pageSize        = 20
	errorBodyLimit  = 512
	fieldMask       = "nextPageToken,places.id,places.displayName,places.formattedAddress," +
		"places.nationalPhoneNumber,places.websiteUri,places.rating,places.userRatingCount," +
		"places.businessStatus,places.googleMapsUri"
)

type Place struct {
	ID      string
	Name    string
	Address string
	Phone   string
	Website string
	Rating  float64
	Reviews int
	Status  string
	MapsURL string
}

type Budget struct {
	mu        sync.Mutex
	remaining int
}

func NewBudget(total int) *Budget {
	return &Budget{remaining: total}
}

func (b *Budget) Take() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.remaining <= 0 {
		return false
	}
	b.remaining--
	return true
}

func (b *Budget) Remaining() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.remaining
}

type Client struct {
	APIKey    string
	Endpoint  string
	HTTP      *http.Client
	Budget    *Budget
	PageDelay time.Duration
}

func NewClient(apiKey string, budget *Budget) *Client {
	return &Client{
		APIKey:    apiKey,
		Endpoint:  DefaultEndpoint,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		Budget:    budget,
		PageDelay: 1500 * time.Millisecond,
	}
}

func (c *Client) Search(ctx context.Context, query string) ([]Place, error) {
	var found []Place
	token := ""
	for page := 0; page < MaxPages; page++ {
		if !c.Budget.Take() {
			break
		}
		batch, next, err := c.fetch(ctx, query, token)
		if err != nil {
			return found, err
		}
		found = append(found, batch...)
		if next == "" || len(batch) == 0 {
			break
		}
		token = next
		if err := pause(ctx, c.PageDelay); err != nil {
			return found, err
		}
	}
	return found, nil
}

type searchRequest struct {
	TextQuery  string `json:"textQuery"`
	RegionCode string `json:"regionCode"`
	PageSize   int    `json:"pageSize"`
	PageToken  string `json:"pageToken,omitempty"`
}

type searchResponse struct {
	Places        []wirePlace `json:"places"`
	NextPageToken string      `json:"nextPageToken"`
}

type wirePlace struct {
	ID          string `json:"id"`
	DisplayName struct {
		Text string `json:"text"`
	} `json:"displayName"`
	FormattedAddress    string  `json:"formattedAddress"`
	NationalPhoneNumber string  `json:"nationalPhoneNumber"`
	WebsiteURI          string  `json:"websiteUri"`
	Rating              float64 `json:"rating"`
	UserRatingCount     int     `json:"userRatingCount"`
	BusinessStatus      string  `json:"businessStatus"`
	GoogleMapsURI       string  `json:"googleMapsUri"`
}

func (w wirePlace) toPlace() Place {
	return Place{
		ID:      w.ID,
		Name:    w.DisplayName.Text,
		Address: w.FormattedAddress,
		Phone:   w.NationalPhoneNumber,
		Website: w.WebsiteURI,
		Rating:  w.Rating,
		Reviews: w.UserRatingCount,
		Status:  w.BusinessStatus,
		MapsURL: w.GoogleMapsURI,
	}
}

func (c *Client) fetch(ctx context.Context, query, token string) ([]Place, string, error) {
	payload, err := json.Marshal(searchRequest{
		TextQuery:  query,
		RegionCode: "AU",
		PageSize:   pageSize,
		PageToken:  token,
	})
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", c.APIKey)
	req.Header.Set("X-Goog-FieldMask", fieldMask)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyLimit))
		return nil, "", fmt.Errorf("places api: %s: %s", resp.Status, bytes.TrimSpace(snippet))
	}

	var decoded searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, "", fmt.Errorf("places api: decode response: %w", err)
	}
	places := make([]Place, 0, len(decoded.Places))
	for _, w := range decoded.Places {
		places = append(places, w.toPlace())
	}
	return places, decoded.NextPageToken, nil
}

func pause(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
