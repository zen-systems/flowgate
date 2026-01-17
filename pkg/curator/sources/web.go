package sources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// TavilySource provides web search via the Tavily API.
// Tavily is used as a raw data source - we disable their AI summary
// and let the curator do the thinking.
type TavilySource struct {
	apiKey     string
	httpClient *http.Client
	maxResults int
}

// TavilyOption configures a TavilySource.
type TavilyOption func(*TavilySource)

// WithTavilyAPIKey sets the API key (alternative to env var).
func WithTavilyAPIKey(key string) TavilyOption {
	return func(t *TavilySource) {
		t.apiKey = key
	}
}

// WithMaxResults sets the maximum search results to return.
func WithMaxResults(max int) TavilyOption {
	return func(t *TavilySource) {
		t.maxResults = max
	}
}

// NewWebSource creates a new Tavily-backed web source.
func NewWebSource(opts ...TavilyOption) *TavilySource {
	t := &TavilySource{
		apiKey: os.Getenv("TAVILY_API_KEY"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxResults: 5,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Name returns the source identifier.
func (t *TavilySource) Name() string {
	return "web"
}

// Available returns true if the API key is configured.
func (t *TavilySource) Available() bool {
	return t.apiKey != ""
}

// tavilyRequest is the request payload for Tavily API.
type tavilyRequest struct {
	Query         string `json:"query"`
	SearchDepth   string `json:"search_depth"`
	IncludeAnswer bool   `json:"include_answer"`
	MaxResults    int    `json:"max_results"`
}

// tavilyResponse is the response from Tavily API.
type tavilyResponse struct {
	Results []tavilyResult `json:"results"`
}

type tavilyResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// Query searches the web for information matching the query.
func (t *TavilySource) Query(ctx context.Context, query string) ([]QueryResult, error) {
	if !t.Available() {
		return nil, fmt.Errorf("Tavily API key not configured")
	}

	// Build request - CRITICAL: include_answer=false
	// We want raw data, not Tavily's AI summary. The curator does the thinking.
	payload := tavilyRequest{
		Query:         query,
		SearchDepth:   "advanced", // Get full page content
		IncludeAnswer: false,      // Shut up and give me the data
		MaxResults:    t.maxResults,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tavily API error: status %d", resp.StatusCode)
	}

	var tavilyResp tavilyResponse
	if err := json.NewDecoder(resp.Body).Decode(&tavilyResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Transform to QueryResult
	results := make([]QueryResult, 0, len(tavilyResp.Results))
	for _, r := range tavilyResp.Results {
		results = append(results, QueryResult{
			Content:    r.Content,
			Path:       r.URL,
			Confidence: r.Score,
			Timestamp:  time.Now(), // Web results are current
			Metadata: map[string]string{
				"title":  r.Title,
				"source": "tavily",
			},
		})
	}

	return results, nil
}
