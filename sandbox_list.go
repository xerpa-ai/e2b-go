package e2b

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// SandboxState represents the state of a sandbox.
type SandboxState string

const (
	// SandboxStateRunning indicates the sandbox is running.
	SandboxStateRunning SandboxState = "running"
	// SandboxStatePaused indicates the sandbox is paused.
	SandboxStatePaused SandboxState = "paused"
)

// SandboxQuery defines the query parameters for listing sandboxes.
type SandboxQuery struct {
	// Metadata filters sandboxes by metadata key-value pairs.
	Metadata map[string]string
	// State filters sandboxes by their state (running, paused).
	State []SandboxState
}

// sandboxListConfig holds configuration for sandbox listing.
type sandboxListConfig struct {
	apiKey     string
	apiURL     string
	domain     string
	debug      bool
	httpClient *http.Client
	query      *SandboxQuery
	limit      int
}

// SandboxListOption configures List behavior.
type SandboxListOption func(*sandboxListConfig)

// WithListAPIKey sets the API key for listing sandboxes.
func WithListAPIKey(apiKey string) SandboxListOption {
	return func(c *sandboxListConfig) {
		c.apiKey = apiKey
	}
}

// WithListAPIURL sets the API URL for listing sandboxes.
func WithListAPIURL(apiURL string) SandboxListOption {
	return func(c *sandboxListConfig) {
		c.apiURL = apiURL
	}
}

// WithListQuery sets the query filter for listing sandboxes.
func WithListQuery(q *SandboxQuery) SandboxListOption {
	return func(c *sandboxListConfig) {
		c.query = q
	}
}

// WithListLimit sets the maximum number of sandboxes to return per page.
func WithListLimit(limit int) SandboxListOption {
	return func(c *sandboxListConfig) {
		c.limit = limit
	}
}

// WithListHTTPClient sets the HTTP client for listing sandboxes.
func WithListHTTPClient(client *http.Client) SandboxListOption {
	return func(c *sandboxListConfig) {
		c.httpClient = client
	}
}

// SandboxPaginator provides paginated access to sandbox listings.
type SandboxPaginator struct {
	config    *sandboxListConfig
	nextToken string
	hasNext   bool
}

// List creates a new SandboxPaginator to iterate through sandboxes.
//
// Example:
//
//	paginator := e2b.List(e2b.WithListLimit(10))
//	for paginator.HasNext() {
//	    sandboxes, err := paginator.NextItems(ctx)
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    for _, s := range sandboxes {
//	        fmt.Println(s.SandboxID)
//	    }
//	}
func List(opts ...SandboxListOption) *SandboxPaginator {
	cfg := &sandboxListConfig{
		domain: DefaultDomain,
		limit:  25, // default page size
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// Get configuration from environment variables if not provided
	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv("E2B_API_KEY")
	}
	if cfg.apiURL == "" {
		cfg.apiURL = os.Getenv("E2B_API_URL")
	}
	if cfg.apiURL == "" {
		if cfg.debug {
			cfg.apiURL = "http://localhost:3000"
		} else {
			cfg.apiURL = fmt.Sprintf("https://api.%s", cfg.domain)
		}
	}

	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{Timeout: DefaultRequestTimeout}
	}

	return &SandboxPaginator{
		config:  cfg,
		hasNext: true,
	}
}

// HasNext returns true if there are more items to fetch.
func (p *SandboxPaginator) HasNext() bool {
	return p.hasNext
}

// NextItems fetches the next page of sandboxes.
// Returns an empty slice when there are no more items.
func (p *SandboxPaginator) NextItems(ctx context.Context) ([]SandboxInfo, error) {
	if !p.hasNext {
		return []SandboxInfo{}, nil
	}

	// Build URL with query parameters
	baseURL, _ := url.JoinPath(p.config.apiURL, "v2", "sandboxes")
	params := url.Values{}

	if p.config.limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", p.config.limit))
	}

	if p.nextToken != "" {
		params.Set("next_token", p.nextToken)
	}

	// Add metadata filters
	if p.config.query != nil {
		if len(p.config.query.Metadata) > 0 {
			for k, v := range p.config.query.Metadata {
				params.Add("metadata", fmt.Sprintf("%s=%s", k, v))
			}
		}
		if len(p.config.query.State) > 0 {
			states := make([]string, len(p.config.query.State))
			for i, s := range p.config.query.State {
				states[i] = string(s)
			}
			params.Set("state", strings.Join(states, ","))
		}
	}

	reqURL := baseURL
	if len(params) > 0 {
		reqURL = baseURL + "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Key", p.config.apiKey)
	req.Header.Set("User-Agent", "e2b-go-sdk/"+Version)

	resp, err := p.config.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response body as array directly (API returns array, not wrapped object)
	var sandboxes []SandboxInfo
	if err := json.Unmarshal(body, &sandboxes); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Update pagination state - nextToken comes from response header
	p.nextToken = resp.Header.Get("X-Next-Token")
	p.hasNext = p.nextToken != ""

	return sandboxes, nil
}

// ListAll fetches all sandboxes matching the query without pagination.
// This is a convenience method that returns all results in a single call.
//
// Example:
//
//	sandboxes, err := e2b.ListAll(ctx, e2b.WithListQuery(&e2b.SandboxQuery{
//	    State: []e2b.SandboxState{e2b.SandboxStateRunning},
//	}))
func ListAll(ctx context.Context, opts ...SandboxListOption) ([]SandboxInfo, error) {
	paginator := List(opts...)
	var all []SandboxInfo

	for paginator.HasNext() {
		items, err := paginator.NextItems(ctx)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
	}

	return all, nil
}
