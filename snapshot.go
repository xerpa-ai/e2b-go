package e2b

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// SnapshotInfo contains information about a sandbox snapshot.
type SnapshotInfo struct {
	// SnapshotID is the template ID with tag, or namespaced name (e.g. my-snapshot:latest).
	SnapshotID string `json:"snapshotID"`
}

// snapshotCreateRequest represents the request body for creating a snapshot.
type snapshotCreateRequest struct {
	Name string `json:"name,omitempty"`
}

// CreateSnapshot creates a snapshot from this sandbox.
// The snapshot can later be used as a template to create new sandboxes.
//
// Example:
//
//	info, err := sandbox.CreateSnapshot(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("Snapshot ID:", info.SnapshotID)
func (s *Sandbox) CreateSnapshot(ctx context.Context, opts ...SnapshotCreateOption) (*SnapshotInfo, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrSandboxClosed
	}
	apiKey := s.config.apiKey
	apiURL := s.config.apiURL
	client := s.config.httpClient
	s.mu.RUnlock()

	return createSnapshot(ctx, client, apiURL, apiKey, s.ID, opts...)
}

// CreateSnapshotStatic creates a snapshot from a sandbox by ID.
// This is a package-level function that can be called without a sandbox instance.
func CreateSnapshotStatic(ctx context.Context, sandboxID string, apiKey string, opts ...SnapshotCreateOption) (*SnapshotInfo, error) {
	cfg := defaultSnapshotConfig()
	cfg.apiKey = apiKey
	applySnapshotEnv(cfg)

	if cfg.apiKey == "" {
		return nil, fmt.Errorf("%w: API key is required", ErrInvalidArgument)
	}

	return createSnapshot(ctx, cfg.httpClient, cfg.apiURL, cfg.apiKey, sandboxID, opts...)
}

func createSnapshot(ctx context.Context, client *http.Client, apiURL, apiKey, sandboxID string, opts ...SnapshotCreateOption) (*SnapshotInfo, error) {
	cfg := &snapshotCreateConfig{}
	for _, opt := range opts {
		opt.applyCreate(cfg)
	}

	reqBody := &snapshotCreateRequest{Name: cfg.name}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	reqURL, _ := url.JoinPath(apiURL, "sandboxes", sandboxID, "snapshots")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", apiKey)
	httpReq.Header.Set("User-Agent", "e2b-go-sdk/"+Version)

	if client == nil {
		client = &http.Client{Timeout: DefaultRequestTimeout}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	var info SnapshotInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &info, nil
}

// DeleteSnapshot deletes a snapshot by ID.
//
// Example:
//
//	err := e2b.DeleteSnapshot(ctx, "snapshot-id")
func DeleteSnapshot(ctx context.Context, snapshotID string, opts ...SnapshotOption) error {
	cfg := defaultSnapshotConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	applySnapshotEnv(cfg)

	if cfg.apiKey == "" {
		return fmt.Errorf("%w: API key is required", ErrInvalidArgument)
	}

	reqURL, _ := url.JoinPath(cfg.apiURL, "snapshots", snapshotID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("X-API-Key", cfg.apiKey)
	httpReq.Header.Set("User-Agent", "e2b-go-sdk/"+Version)

	resp, err := cfg.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: snapshot %s not found", ErrNotFound, snapshotID)
	}

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// SnapshotPaginator provides paginated access to snapshot listings.
type SnapshotPaginator struct {
	config    *snapshotConfig
	nextToken string
	hasNext   bool
	sandboxID string
	limit     int
}

// ListSnapshots creates a new SnapshotPaginator to iterate through snapshots.
//
// Example:
//
//	paginator := e2b.ListSnapshots(e2b.WithSnapshotLimit(10))
//	for paginator.HasNext() {
//	    snapshots, err := paginator.NextItems(ctx)
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    for _, s := range snapshots {
//	        fmt.Println(s.SnapshotID)
//	    }
//	}
func ListSnapshots(opts ...SnapshotListOption) *SnapshotPaginator {
	cfg := defaultSnapshotConfig()
	listCfg := &snapshotListConfig{limit: 25}

	for _, opt := range opts {
		opt.applyList(cfg, listCfg)
	}
	applySnapshotEnv(cfg)

	return &SnapshotPaginator{
		config:    cfg,
		hasNext:   true,
		sandboxID: listCfg.sandboxID,
		limit:     listCfg.limit,
	}
}

// HasNext returns true if there are more items to fetch.
func (p *SnapshotPaginator) HasNext() bool {
	return p.hasNext
}

// NextItems fetches the next page of snapshots.
func (p *SnapshotPaginator) NextItems(ctx context.Context) ([]SnapshotInfo, error) {
	if !p.hasNext {
		return []SnapshotInfo{}, nil
	}

	baseURL, _ := url.JoinPath(p.config.apiURL, "snapshots")
	params := url.Values{}

	if p.limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", p.limit))
	}
	if p.nextToken != "" {
		params.Set("nextToken", p.nextToken)
	}
	if p.sandboxID != "" {
		params.Set("sandboxID", p.sandboxID)
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

	var snapshots []SnapshotInfo
	if err := json.Unmarshal(body, &snapshots); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	p.nextToken = resp.Header.Get("X-Next-Token")
	p.hasNext = p.nextToken != ""

	return snapshots, nil
}

// ListAllSnapshots fetches all snapshots matching the query without pagination.
func ListAllSnapshots(ctx context.Context, opts ...SnapshotListOption) ([]SnapshotInfo, error) {
	paginator := ListSnapshots(opts...)
	var all []SnapshotInfo

	for paginator.HasNext() {
		items, err := paginator.NextItems(ctx)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
	}

	return all, nil
}

type snapshotConfig struct {
	apiKey     string
	apiURL     string
	domain     string
	httpClient *http.Client
}

type snapshotCreateConfig struct {
	name string
}

type snapshotListConfig struct {
	sandboxID string
	limit     int
}

func defaultSnapshotConfig() *snapshotConfig {
	return &snapshotConfig{
		domain: DefaultDomain,
	}
}

func applySnapshotEnv(cfg *snapshotConfig) {
	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv("E2B_API_KEY")
	}
	if cfg.apiURL == "" {
		cfg.apiURL = os.Getenv("E2B_API_URL")
	}
	if cfg.apiURL == "" {
		cfg.apiURL = fmt.Sprintf("https://api.%s", cfg.domain)
	}
	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{Timeout: DefaultRequestTimeout}
	}
}

// SnapshotOption configures snapshot API calls.
type SnapshotOption func(*snapshotConfig)

// WithSnapshotAPIKey sets the API key for snapshot operations.
func WithSnapshotAPIKey(apiKey string) SnapshotOption {
	return func(c *snapshotConfig) {
		c.apiKey = apiKey
	}
}

// WithSnapshotAPIURL sets the API URL for snapshot operations.
func WithSnapshotAPIURL(apiURL string) SnapshotOption {
	return func(c *snapshotConfig) {
		c.apiURL = apiURL
	}
}

// SnapshotCreateOption configures snapshot creation.
type SnapshotCreateOption interface {
	applyCreate(*snapshotCreateConfig)
}

type snapshotNameOption struct{ name string }

func (o snapshotNameOption) applyCreate(c *snapshotCreateConfig) { c.name = o.name }

// WithSnapshotName sets the name for the created snapshot.
func WithSnapshotName(name string) SnapshotCreateOption {
	return snapshotNameOption{name: name}
}

// SnapshotListOption configures snapshot listing.
type SnapshotListOption interface {
	applyList(*snapshotConfig, *snapshotListConfig)
}

type snapshotListAPIKeyOption struct{ apiKey string }

func (o snapshotListAPIKeyOption) applyList(c *snapshotConfig, _ *snapshotListConfig) {
	c.apiKey = o.apiKey
}

// WithSnapshotListAPIKey sets the API key for listing snapshots.
func WithSnapshotListAPIKey(apiKey string) SnapshotListOption {
	return snapshotListAPIKeyOption{apiKey: apiKey}
}

type snapshotListSandboxIDOption struct{ id string }

func (o snapshotListSandboxIDOption) applyList(_ *snapshotConfig, c *snapshotListConfig) {
	c.sandboxID = o.id
}

// WithSnapshotSandboxID filters snapshots by source sandbox ID.
func WithSnapshotSandboxID(sandboxID string) SnapshotListOption {
	return snapshotListSandboxIDOption{id: sandboxID}
}

type snapshotListLimitOption struct{ limit int }

func (o snapshotListLimitOption) applyList(_ *snapshotConfig, c *snapshotListConfig) {
	c.limit = o.limit
}

// WithSnapshotLimit sets the maximum number of snapshots per page.
func WithSnapshotLimit(limit int) SnapshotListOption {
	return snapshotListLimitOption{limit: limit}
}
