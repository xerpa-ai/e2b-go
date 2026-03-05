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

// VolumeInfo contains information about a persistent volume.
type VolumeInfo struct {
	// VolumeID is the unique identifier for the volume.
	VolumeID string `json:"volumeID"`
	// Name is the volume name.
	Name string `json:"name"`
}

// volumeCreateRequest represents the request body for creating a volume.
type volumeCreateRequest struct {
	Name string `json:"name"`
}

// volumeConfig holds configuration for volume API calls.
type volumeConfig struct {
	apiKey     string
	apiURL     string
	domain     string
	httpClient *http.Client
}

func defaultVolumeConfig() *volumeConfig {
	return &volumeConfig{
		domain: DefaultDomain,
	}
}

func applyVolumeEnv(cfg *volumeConfig) {
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

// VolumeOption configures volume API calls.
type VolumeOption func(*volumeConfig)

// WithVolumeAPIKey sets the API key for volume operations.
func WithVolumeAPIKey(apiKey string) VolumeOption {
	return func(c *volumeConfig) {
		c.apiKey = apiKey
	}
}

// WithVolumeAPIURL sets the API URL for volume operations.
func WithVolumeAPIURL(apiURL string) VolumeOption {
	return func(c *volumeConfig) {
		c.apiURL = apiURL
	}
}

// WithVolumeHTTPClient sets a custom HTTP client for volume operations.
func WithVolumeHTTPClient(client *http.Client) VolumeOption {
	return func(c *volumeConfig) {
		c.httpClient = client
	}
}

func volumeConfigFromOptions(opts []VolumeOption) *volumeConfig {
	cfg := defaultVolumeConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	applyVolumeEnv(cfg)
	return cfg
}

func setVolumeHeaders(req *http.Request, cfg *volumeConfig) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", cfg.apiKey)
	req.Header.Set("User-Agent", "e2b-go-sdk/"+Version)
}

// CreateVolume creates a new persistent volume.
//
// Example:
//
//	vol, err := e2b.CreateVolume(ctx, "my-volume")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("Volume ID:", vol.VolumeID)
func CreateVolume(ctx context.Context, name string, opts ...VolumeOption) (*VolumeInfo, error) {
	cfg := volumeConfigFromOptions(opts)

	if cfg.apiKey == "" {
		return nil, fmt.Errorf("%w: API key is required", ErrInvalidArgument)
	}

	data, err := json.Marshal(&volumeCreateRequest{Name: name})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.apiURL+"/volumes", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	setVolumeHeaders(httpReq, cfg)

	resp, err := cfg.httpClient.Do(httpReq)
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

	var vol VolumeInfo
	if err := json.Unmarshal(body, &vol); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &vol, nil
}

// ListVolumes returns all volumes for the authenticated team.
//
// Example:
//
//	volumes, err := e2b.ListVolumes(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, v := range volumes {
//	    fmt.Printf("Volume: %s (%s)\n", v.Name, v.VolumeID)
//	}
func ListVolumes(ctx context.Context, opts ...VolumeOption) ([]VolumeInfo, error) {
	cfg := volumeConfigFromOptions(opts)

	if cfg.apiKey == "" {
		return nil, fmt.Errorf("%w: API key is required", ErrInvalidArgument)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.apiURL+"/volumes", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	setVolumeHeaders(httpReq, cfg)

	resp, err := cfg.httpClient.Do(httpReq)
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

	var volumes []VolumeInfo
	if err := json.Unmarshal(body, &volumes); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return volumes, nil
}

// GetVolume returns information about a volume by ID.
//
// Example:
//
//	vol, err := e2b.GetVolume(ctx, "volume-id")
func GetVolume(ctx context.Context, volumeID string, opts ...VolumeOption) (*VolumeInfo, error) {
	cfg := volumeConfigFromOptions(opts)

	if cfg.apiKey == "" {
		return nil, fmt.Errorf("%w: API key is required", ErrInvalidArgument)
	}

	reqURL, _ := url.JoinPath(cfg.apiURL, "volumes", volumeID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	setVolumeHeaders(httpReq, cfg)

	resp, err := cfg.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: volume %s not found", ErrNotFound, volumeID)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	var vol VolumeInfo
	if err := json.Unmarshal(body, &vol); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &vol, nil
}

// DeleteVolume deletes a volume by ID.
//
// Example:
//
//	err := e2b.DeleteVolume(ctx, "volume-id")
func DeleteVolume(ctx context.Context, volumeID string, opts ...VolumeOption) error {
	cfg := volumeConfigFromOptions(opts)

	if cfg.apiKey == "" {
		return fmt.Errorf("%w: API key is required", ErrInvalidArgument)
	}

	reqURL, _ := url.JoinPath(cfg.apiURL, "volumes", volumeID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	setVolumeHeaders(httpReq, cfg)

	resp, err := cfg.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: volume %s not found", ErrNotFound, volumeID)
	}

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
