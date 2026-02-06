package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SearXNGProvider struct {
	baseURL   string
	userAgent string
	apiKey    string
	client    *http.Client
}

func NewSearXNGProvider(baseURL, userAgent, apiKey string, timeout time.Duration) *SearXNGProvider {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "AIMate/0.1"
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &SearXNGProvider{
		baseURL:   strings.TrimRight(baseURL, "/"),
		userAgent: userAgent,
		apiKey:    strings.TrimSpace(apiKey),
		client:    &http.Client{Timeout: timeout},
	}
}

func (p *SearXNGProvider) Name() string {
	return "searxng"
}

type searxngResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

type searxngResponse struct {
	Query   string          `json:"query"`
	Results []searxngResult `json:"results"`
}

func (p *SearXNGProvider) Search(ctx context.Context, query string, limit int) (Response, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Response{}, fmt.Errorf("query cannot be empty")
	}
	if limit <= 0 {
		limit = 5
	}

	endpoint, err := url.Parse(p.baseURL)
	if err != nil {
		return Response{}, fmt.Errorf("invalid base url: %w", err)
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/search"

	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("categories", "general")
	params.Set("language", "auto")
	params.Set("safesearch", "1")
	params.Set("count", fmt.Sprintf("%d", limit))
	if p.apiKey != "" {
		params.Set("apikey", p.apiKey)
	}
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Response{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", p.userAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, fmt.Errorf("search request failed with status %d", resp.StatusCode)
	}

	var payload searxngResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Response{}, fmt.Errorf("failed to decode response: %w", err)
	}

	results := make([]Result, 0, limit)
	for _, res := range payload.Results {
		if len(results) >= limit {
			break
		}
		results = append(results, Result{
			Title:   strings.TrimSpace(res.Title),
			URL:     strings.TrimSpace(res.URL),
			Snippet: strings.TrimSpace(res.Content),
			Source:  p.Name(),
		})
	}

	return Response{
		Query:    query,
		Provider: p.Name(),
		Results:  results,
	}, nil
}
