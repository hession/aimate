package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hession/aimate/internal/config"
	"github.com/hession/aimate/internal/websearch"
)

// WebSearchTool searches the web using a configured provider.
type WebSearchTool struct {
	provider     websearch.Provider
	defaultLimit int
}

// NewWebSearchTool creates a web search tool from config.
func NewWebSearchTool(cfg *config.Config) *WebSearchTool {
	providerName := "duckduckgo"
	defaultLimit := 5
	baseURL := ""
	userAgent := ""
	apiKey := ""
	timeout := 15 * time.Second
	if cfg != nil {
		if strings.TrimSpace(cfg.WebSearch.Provider) != "" {
			providerName = cfg.WebSearch.Provider
		}
		if cfg.WebSearch.DefaultLimit > 0 {
			defaultLimit = cfg.WebSearch.DefaultLimit
		}
		if strings.TrimSpace(cfg.WebSearch.BaseURL) != "" {
			baseURL = cfg.WebSearch.BaseURL
		}
		if strings.TrimSpace(cfg.WebSearch.UserAgent) != "" {
			userAgent = cfg.WebSearch.UserAgent
		}
		if cfg.WebSearch.TimeoutSeconds > 0 {
			timeout = time.Duration(cfg.WebSearch.TimeoutSeconds) * time.Second
		}
	}

	providerName = strings.ToLower(strings.TrimSpace(providerName))
	var provider websearch.Provider
	switch providerName {
	case "searxng":
		provider = websearch.NewSearXNGProvider(baseURL, userAgent, apiKey, timeout)
	case "duckduckgo", "ddg":
		provider = websearch.NewDuckDuckGoProvider(baseURL, userAgent, timeout)
	default:
		provider = websearch.NewDuckDuckGoProvider(baseURL, userAgent, timeout)
	}

	return &WebSearchTool{
		provider:     provider,
		defaultLimit: defaultLimit,
	}
}

func (t *WebSearchTool) Name() string {
	return "search_web"
}

func (t *WebSearchTool) Description() string {
	return "Search the web for fresh information and return a list of sources."
}

func (t *WebSearchTool) Parameters() []ParameterDef {
	return []ParameterDef{
		{
			Name:        "query",
			Type:        "string",
			Description: "Search query",
			Required:    true,
		},
		{
			Name:        "limit",
			Type:        "number",
			Description: "Number of results to return (default from config)",
			Required:    false,
		},
	}
}

func (t *WebSearchTool) Execute(args map[string]any) (string, error) {
	query, ok := args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("missing required parameter: query")
	}

	limit := t.defaultLimit
	if val, ok := args["limit"].(float64); ok && val > 0 {
		limit = int(val)
	}

	resp, err := t.provider.Search(context.Background(), query, limit)
	if err != nil {
		return "", err
	}

	payload, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to encode response: %w", err)
	}

	return string(payload), nil
}
