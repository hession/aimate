package websearch

import (
	"context"
)

// Result is a single search result entry.
type Result struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Source  string `json:"source"`
}

// Response is a normalized search response.
type Response struct {
	Query    string   `json:"query"`
	Provider string   `json:"provider"`
	Results  []Result `json:"results"`
}

// Provider performs web searches.
type Provider interface {
	Name() string
	Search(ctx context.Context, query string, limit int) (Response, error)
}
