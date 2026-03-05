package e2b

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// SandboxLogEntry represents a single sandbox log entry.
type SandboxLogEntry struct {
	// Level is the log level (debug, info, warn, error).
	Level string `json:"level"`
	// Message is the log message content.
	Message string `json:"message"`
	// Timestamp is when the log entry was created.
	Timestamp string `json:"timestamp"`
	// Fields contains additional structured fields.
	Fields map[string]string `json:"fields,omitempty"`
}

// sandboxLogsV2Response represents the response from the v2 logs endpoint.
type sandboxLogsV2Response struct {
	Logs []SandboxLogEntry `json:"logs"`
}

// LogDirection specifies the direction for log pagination.
type LogDirection string

const (
	// LogDirectionForward fetches logs from oldest to newest.
	LogDirectionForward LogDirection = "forward"
	// LogDirectionBackward fetches logs from newest to oldest.
	LogDirectionBackward LogDirection = "backward"
)

// logsConfig holds configuration for fetching sandbox logs.
type logsConfig struct {
	cursor         *int64
	limit          int
	direction      LogDirection
	level          string
	search         string
	requestTimeout time.Duration
}

// LogsOption configures sandbox log retrieval.
type LogsOption func(*logsConfig)

// WithLogsCursor sets the cursor (timestamp ms) for log pagination.
func WithLogsCursor(cursor int64) LogsOption {
	return func(c *logsConfig) {
		c.cursor = &cursor
	}
}

// WithLogsLogLimit sets the maximum number of log entries to return.
func WithLogsLogLimit(limit int) LogsOption {
	return func(c *logsConfig) {
		c.limit = limit
	}
}

// WithLogsDirection sets the pagination direction.
func WithLogsDirection(direction LogDirection) LogsOption {
	return func(c *logsConfig) {
		c.direction = direction
	}
}

// WithLogsMinLevel sets the minimum log level to return.
func WithLogsMinLevel(level string) LogsOption {
	return func(c *logsConfig) {
		c.level = level
	}
}

// WithLogsSearch sets a case-sensitive substring match on log messages.
func WithLogsSearch(search string) LogsOption {
	return func(c *logsConfig) {
		c.search = search
	}
}

// WithLogsRequestTimeout sets the request timeout for log retrieval.
func WithLogsRequestTimeout(d time.Duration) LogsOption {
	return func(c *logsConfig) {
		c.requestTimeout = d
	}
}

// GetLogs returns logs for this sandbox.
//
// Example:
//
//	logs, err := sandbox.GetLogs(ctx, e2b.WithLogsLogLimit(100))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, entry := range logs {
//	    fmt.Printf("[%s] %s: %s\n", entry.Timestamp, entry.Level, entry.Message)
//	}
func (s *Sandbox) GetLogs(ctx context.Context, opts ...LogsOption) ([]SandboxLogEntry, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrSandboxClosed
	}
	apiKey := s.config.apiKey
	apiURL := s.config.apiURL
	client := s.config.httpClient
	s.mu.RUnlock()

	return getSandboxLogsInternal(ctx, client, apiURL, apiKey, s.ID, opts...)
}

// GetSandboxLogs returns logs for a sandbox by ID.
// This is a static method that can be called without a sandbox instance.
//
// Example:
//
//	logs, err := e2b.GetSandboxLogs(ctx, "sandbox-id",
//	    e2b.WithLogsMinLevel("warn"),
//	    e2b.WithLogsLogLimit(50),
//	)
func GetSandboxLogs(ctx context.Context, sandboxID string, opts ...LogsOption) ([]SandboxLogEntry, error) {
	apiKey := os.Getenv("E2B_API_KEY")
	apiURL := os.Getenv("E2B_API_URL")
	if apiURL == "" {
		domain := os.Getenv("E2B_DOMAIN")
		if domain == "" {
			domain = DefaultDomain
		}
		apiURL = fmt.Sprintf("https://api.%s", domain)
	}

	client := &http.Client{Timeout: DefaultRequestTimeout}
	return getSandboxLogsInternal(ctx, client, apiURL, apiKey, sandboxID, opts...)
}

func getSandboxLogsInternal(ctx context.Context, client *http.Client, apiURL, apiKey, sandboxID string, opts ...LogsOption) ([]SandboxLogEntry, error) {
	cfg := &logsConfig{
		limit: 1000,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.requestTimeout)
		defer cancel()
	}

	baseURL, _ := url.JoinPath(apiURL, "v2", "sandboxes", sandboxID, "logs")
	params := url.Values{}

	if cfg.cursor != nil {
		params.Set("cursor", fmt.Sprintf("%d", *cfg.cursor))
	}
	if cfg.limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", cfg.limit))
	}
	if cfg.direction != "" {
		params.Set("direction", string(cfg.direction))
	}
	if cfg.level != "" {
		params.Set("level", cfg.level)
	}
	if cfg.search != "" {
		params.Set("search", cfg.search)
	}

	reqURL := baseURL
	if len(params) > 0 {
		reqURL = baseURL + "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("User-Agent", "e2b-go-sdk/"+Version)

	if client == nil {
		client = &http.Client{Timeout: DefaultRequestTimeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: sandbox %s not found", ErrNotFound, sandboxID)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	var logsResp sandboxLogsV2Response
	if err := json.Unmarshal(body, &logsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return logsResp.Logs, nil
}
