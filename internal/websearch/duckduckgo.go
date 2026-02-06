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

type DuckDuckGoProvider struct {
	baseURL   string
	userAgent string
	client    *http.Client
}

func NewDuckDuckGoProvider(baseURL, userAgent string, timeout time.Duration) *DuckDuckGoProvider {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.duckduckgo.com"
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "AIMate/0.1"
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &DuckDuckGoProvider{
		baseURL:   strings.TrimRight(baseURL, "/"),
		userAgent: userAgent,
		client:    &http.Client{Timeout: timeout},
	}
}

func (p *DuckDuckGoProvider) Name() string {
	return "duckduckgo"
}

type ddgResult struct {
	Text     string `json:"Text"`
	FirstURL string `json:"FirstURL"`
}

type ddgTopic struct {
	Text     string     `json:"Text"`
	FirstURL string     `json:"FirstURL"`
	Topics   []ddgTopic `json:"Topics"`
}

type ddgResponse struct {
	Heading       string      `json:"Heading"`
	AbstractText  string      `json:"AbstractText"`
	AbstractURL   string      `json:"AbstractURL"`
	Results       []ddgResult `json:"Results"`
	RelatedTopics []ddgTopic  `json:"RelatedTopics"`
}

func (p *DuckDuckGoProvider) Search(ctx context.Context, query string, limit int) (Response, error) {
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
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("no_html", "1")
	params.Set("skip_disambig", "1")
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

	var payload ddgResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Response{}, fmt.Errorf("failed to decode response: %w", err)
	}

	results := make([]Result, 0, limit)
	seen := make(map[string]bool)
	addResult := func(title, link, snippet string) {
		if len(results) >= limit {
			return
		}
		link = strings.TrimSpace(link)
		if link == "" || seen[link] {
			return
		}
		seen[link] = true
		results = append(results, Result{
			Title:   strings.TrimSpace(title),
			URL:     link,
			Snippet: strings.TrimSpace(snippet),
			Source:  p.Name(),
		})
	}

	if payload.AbstractText != "" {
		title := payload.Heading
		if title == "" {
			title = payload.AbstractText
		}
		addResult(title, payload.AbstractURL, payload.AbstractText)
	}

	for _, res := range payload.Results {
		addResult(res.Text, res.FirstURL, res.Text)
	}

	var walkTopics func(topics []ddgTopic)
	walkTopics = func(topics []ddgTopic) {
		for _, topic := range topics {
			if len(results) >= limit {
				return
			}
			if len(topic.Topics) > 0 {
				walkTopics(topic.Topics)
				continue
			}
			addResult(topic.Text, topic.FirstURL, topic.Text)
		}
	}
	walkTopics(payload.RelatedTopics)

	return Response{
		Query:    query,
		Provider: p.Name(),
		Results:  results,
	}, nil
}
