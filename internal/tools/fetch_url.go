package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/hession/aimate/internal/config"
)

const defaultFetchMaxBytes = int64(200000)

// FetchURLTool retrieves a URL and returns content.
type FetchURLTool struct {
	userAgent      string
	timeout        time.Duration
	defaultMaxSize int64
}

// NewFetchURLTool creates a URL fetch tool from config.
func NewFetchURLTool(cfg *config.Config) *FetchURLTool {
	userAgent := "AIMate/0.1"
	timeout := 15 * time.Second
	if cfg != nil {
		if strings.TrimSpace(cfg.WebSearch.UserAgent) != "" {
			userAgent = cfg.WebSearch.UserAgent
		}
		if cfg.WebSearch.TimeoutSeconds > 0 {
			timeout = time.Duration(cfg.WebSearch.TimeoutSeconds) * time.Second
		}
	}
	return &FetchURLTool{
		userAgent:      userAgent,
		timeout:        timeout,
		defaultMaxSize: defaultFetchMaxBytes,
	}
}

func (t *FetchURLTool) Name() string {
	return "fetch_url"
}

func (t *FetchURLTool) Description() string {
	return "Fetch a URL and return readable content for downstream use."
}

func (t *FetchURLTool) Parameters() []ParameterDef {
	return []ParameterDef{
		{
			Name:        "url",
			Type:        "string",
			Description: "URL to fetch",
			Required:    true,
		},
		{
			Name:        "max_bytes",
			Type:        "number",
			Description: "Maximum bytes to read from the response body",
			Required:    false,
		},
		{
			Name:        "strip_html",
			Type:        "boolean",
			Description: "Whether to strip HTML tags when content is HTML",
			Required:    false,
		},
	}
}

func (t *FetchURLTool) Execute(args map[string]any) (string, error) {
	rawURL, ok := args["url"].(string)
	if !ok || strings.TrimSpace(rawURL) == "" {
		return "", fmt.Errorf("missing required parameter: url")
	}

	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" {
		return "", fmt.Errorf("invalid url: %s", rawURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported url scheme: %s", parsed.Scheme)
	}

	maxBytes := t.defaultMaxSize
	if val, ok := args["max_bytes"].(float64); ok && val > 0 {
		maxBytes = int64(val)
	}

	stripHTML := true
	if val, ok := args["strip_html"].(bool); ok {
		stripHTML = val
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", t.userAgent)

	client := &http.Client{Timeout: t.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	content := string(body)
	if stripHTML && strings.Contains(strings.ToLower(contentType), "text/html") {
		content = stripHTMLTags(content)
	}

	payload := map[string]any{
		"url":          parsed.String(),
		"status":       resp.StatusCode,
		"content_type": contentType,
		"content":      content,
	}

	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to encode response: %w", err)
	}

	return string(encoded), nil
}

var (
	scriptTag = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleTag  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	allTags   = regexp.MustCompile(`(?s)<[^>]+>`)
)

func stripHTMLTags(input string) string {
	trimmed := scriptTag.ReplaceAllString(input, " ")
	trimmed = styleTag.ReplaceAllString(trimmed, " ")
	trimmed = allTags.ReplaceAllString(trimmed, " ")
	trimmed = html.UnescapeString(trimmed)
	return strings.Join(strings.Fields(trimmed), " ")
}
