package e2b

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Sandbox represents an E2B cloud sandbox for code execution.
//
// The sandbox allows you to:
//   - Access Linux OS
//   - Create, list, and delete files and directories
//   - Run commands
//   - Run isolated code
//   - Access the internet
//
// Use New to create a new sandbox instance.
type Sandbox struct {
	// ID is the unique identifier for this sandbox.
	ID string

	// Domain is the base domain for sandbox traffic.
	Domain string

	// Files provides filesystem operations for the sandbox.
	Files *Filesystem

	// Commands provides command execution operations for the sandbox.
	Commands *Commands

	mu           sync.RWMutex
	config       *sandboxConfig
	httpClient   *httpClient
	closed       bool
	accessToken  string
	trafficToken string
}

// E2B API base URL
const e2bAPIBaseURL = "https://api.e2b.app"

// sandboxCreateRequest represents the request body for creating a sandbox.
type sandboxCreateRequest struct {
	TemplateID          string            `json:"templateID"`
	Timeout             int               `json:"timeout,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	EnvVars             map[string]string `json:"envVars,omitempty"`
	Secure              bool              `json:"secure"`
	AllowInternetAccess bool              `json:"allow_internet_access"`
}

// sandboxCreateResponse represents the response from creating a sandbox.
type sandboxCreateResponse struct {
	SandboxID          string `json:"sandboxID"`
	TemplateID         string `json:"templateID"`
	ClientID           string `json:"clientID"`
	EnvdVersion        string `json:"envdVersion"`
	EnvdAccessToken    string `json:"envdAccessToken"`
	TrafficAccessToken string `json:"trafficAccessToken"`
	Domain             string `json:"domain"`
}

// New creates a new Sandbox instance.
//
// The API key can be provided via the WithAPIKey option or the E2B_API_KEY
// environment variable.
//
// Example:
//
//	sandbox, err := e2b.New(e2b.WithAPIKey("your-api-key"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer sandbox.Close()
func New(opts ...Option) (*Sandbox, error) {
	cfg := defaultSandboxConfig()

	for _, opt := range opts {
		opt(cfg)
	}

	// Get API key from environment if not provided
	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv("E2B_API_KEY")
	}

	if cfg.apiKey == "" {
		return nil, fmt.Errorf("%w: API key is required (use WithAPIKey or set E2B_API_KEY)", ErrInvalidArgument)
	}

	// Create HTTP client if not provided
	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{
			Timeout: cfg.requestTimeout,
		}
	}

	// Create sandbox via E2B API
	createReq := &sandboxCreateRequest{
		TemplateID:          cfg.template,
		Timeout:             int(cfg.timeout.Seconds()),
		Metadata:            cfg.metadata,
		EnvVars:             cfg.envVars,
		Secure:              cfg.secure,
		AllowInternetAccess: true, // Allow internet access by default
	}

	createResp, err := createSandbox(cfg.httpClient, cfg.apiKey, createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox: %w", err)
	}

	// Use the domain from API response, or fallback to e2b.dev
	domain := createResp.Domain
	if domain == "" {
		domain = "e2b.dev"
	}

	sandbox := &Sandbox{
		ID:           createResp.SandboxID,
		Domain:       domain,
		config:       cfg,
		accessToken:  createResp.EnvdAccessToken,
		trafficToken: createResp.TrafficAccessToken,
	}

	// Initialize the HTTP client for Jupyter API calls
	sandbox.initHTTPClient()

	// Initialize the Filesystem
	sandbox.Files = newFilesystem(sandbox)

	// Initialize the Commands
	sandbox.Commands = newCommands(sandbox)

	if err := sandbox.waitForReady(); err != nil {
		// Clean up if the sandbox fails to become ready
		sandbox.Close()
		return nil, fmt.Errorf("sandbox not ready: %w", err)
	}

	return sandbox, nil
}

// waitForReady waits for the sandbox to be ready.
// If skipJupyterWait is true, it waits for the envd service instead of the Jupyter server.
func (s *Sandbox) waitForReady() error {
	// If skipping Jupyter wait, wait for the envd service instead
	if s.config.skipJupyterWait {
		return s.waitForEnvd()
	}

	maxRetries := 30
	retryInterval := 1 * time.Second

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		// Try to list contexts - this is a lightweight operation that
		// will succeed when the Jupyter server is ready
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := s.ListContexts(ctx)
		cancel()

		if err == nil {
			return nil
		}
		lastErr = err

		// If the error indicates the port is not ready, wait and retry
		if i < maxRetries-1 {
			time.Sleep(retryInterval)
		}
	}

	return fmt.Errorf("timeout waiting for Jupyter server (sandbox=%s, domain=%s, url=%s): %v",
		s.ID, s.Domain, s.GetHost(JupyterPort), lastErr)
}

// waitForEnvd waits for the envd service to be ready.
// This is used when skipJupyterWait is true.
func (s *Sandbox) waitForEnvd() error {
	maxRetries := 30
	retryInterval := 1 * time.Second

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		// Try to list processes - this is a lightweight operation that
		// will succeed when the envd service is ready
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := s.Commands.List(ctx)
		cancel()

		if err == nil {
			return nil
		}
		lastErr = err

		// If the error indicates the port is not ready, wait and retry
		if i < maxRetries-1 {
			time.Sleep(retryInterval)
		}
	}

	return fmt.Errorf("timeout waiting for envd service (sandbox=%s, domain=%s, url=%s): %v",
		s.ID, s.Domain, s.GetHost(EnvdPort), lastErr)
}

// createSandbox calls the E2B API to create a new sandbox.
func createSandbox(client *http.Client, apiKey string, req *sandboxCreateRequest) (*sandboxCreateResponse, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, e2bAPIBaseURL+"/sandboxes", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var createResp sandboxCreateResponse
	if err := json.Unmarshal(respBody, &createResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &createResp, nil
}

// Connect connects to an existing sandbox by ID.
//
// Example:
//
//	sandbox, err := e2b.Connect("sandbox-id", e2b.WithAPIKey("your-api-key"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer sandbox.Close()
func Connect(sandboxID string, opts ...Option) (*Sandbox, error) {
	cfg := defaultSandboxConfig()

	for _, opt := range opts {
		opt(cfg)
	}

	// Get API key from environment if not provided
	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv("E2B_API_KEY")
	}

	if cfg.apiKey == "" {
		return nil, fmt.Errorf("%w: API key is required", ErrInvalidArgument)
	}

	if sandboxID == "" {
		return nil, fmt.Errorf("%w: sandbox ID is required", ErrInvalidArgument)
	}

	// Create HTTP client if not provided
	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{
			Timeout: cfg.requestTimeout,
		}
	}

	sandbox := &Sandbox{
		ID:     sandboxID,
		Domain: "e2b.dev", // Default domain for connecting
		config: cfg,
	}

	// Initialize the HTTP client for the Jupyter server
	sandbox.initHTTPClient()

	// Initialize the Filesystem
	sandbox.Files = newFilesystem(sandbox)

	// Initialize the Commands
	sandbox.Commands = newCommands(sandbox)

	return sandbox, nil
}

// initHTTPClient initializes the HTTP client for Jupyter API calls.
func (s *Sandbox) initHTTPClient() {
	scheme := "https"
	if s.config.debug {
		scheme = "http"
	}

	// E2B URL format: https://{port}-{sandboxID}.{domain}
	baseURL := fmt.Sprintf("%s://%s", scheme, s.GetHost(JupyterPort))

	s.httpClient = newHTTPClient(
		s.config.httpClient,
		baseURL,
		s.accessToken,
		s.trafficToken,
	)
}

// GetHost returns the sandbox host for a given port.
// The E2B URL format is: {port}-{sandboxID}.{domain}
func (s *Sandbox) GetHost(port int) string {
	if s.config.debug {
		return fmt.Sprintf("localhost:%d", port)
	}
	return fmt.Sprintf("%d-%s.%s", port, s.ID, s.Domain)
}

// RunCode executes code in the sandbox.
//
// The code is executed in a stateful environment where variables, imports,
// and function definitions persist across calls.
//
// By default, code is executed as Python. Use WithLanguage or WithContext
// to execute code in a different language or context.
//
// Example:
//
//	execution, err := sandbox.RunCode(ctx, "x = 1; x")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(execution.Text()) // Output: 1
func (s *Sandbox) RunCode(ctx context.Context, code string, opts ...RunOption) (*Execution, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrSandboxClosed
	}
	s.mu.RUnlock()

	cfg := defaultRunConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Validate that language and context are not both provided
	if cfg.language != "" && cfg.context != nil {
		return nil, fmt.Errorf("%w: cannot provide both language and context", ErrInvalidArgument)
	}

	// Set timeout
	timeout := cfg.timeout
	if timeout == 0 {
		timeout = s.config.timeout
	}

	// Create context with timeout if needed
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Prepare request
	reqBody := &executeRequest{
		Code:    code,
		EnvVars: cfg.envVars,
	}

	if cfg.context != nil {
		reqBody.ContextID = cfg.context.ID
	} else if cfg.language != "" {
		reqBody.Language = cfg.language
	}

	// Initialize execution result
	execution := &Execution{
		Results: make([]*Result, 0),
		Logs:    NewLogs(),
	}

	// Execute streaming request
	_, err := s.httpClient.doStreamRequest(ctx, "/execute", reqBody, func(sr *streamResponse) error {
		return parseStreamResponse(sr, execution, cfg)
	})

	if err != nil {
		// Check for context deadline exceeded (timeout)
		if ctx.Err() == context.DeadlineExceeded {
			return nil, NewExecutionTimeoutError()
		}
		return nil, err
	}

	return execution, nil
}

// CreateContext creates a new execution context.
//
// Contexts provide isolated state for code execution. Variables and imports
// in one context do not affect other contexts.
//
// Example:
//
//	ctx, err := sandbox.CreateContext(context.Background(),
//	    e2b.WithContextLanguage("python"),
//	    e2b.WithCWD("/home/user/project"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (s *Sandbox) CreateContext(ctx context.Context, opts ...ContextOption) (*Context, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrSandboxClosed
	}
	s.mu.RUnlock()

	cfg := defaultContextConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Set request timeout
	timeout := cfg.requestTimeout
	if timeout == 0 {
		timeout = s.config.requestTimeout
	}

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	reqBody := &contextCreateRequest{
		Language: cfg.language,
		CWD:      cfg.cwd,
	}

	respBody, statusCode, err := s.httpClient.doRequest(ctx, http.MethodPost, "/contexts", reqBody)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, NewRequestTimeoutError()
		}
		return nil, err
	}

	if statusCode != http.StatusOK && statusCode != http.StatusCreated {
		return nil, formatHTTPError(statusCode, string(respBody))
	}

	var ctxResp contextResponse
	if err := json.Unmarshal(respBody, &ctxResp); err != nil {
		return nil, fmt.Errorf("failed to parse context response: %w", err)
	}

	return ctxResp.toContext(), nil
}

// ListContexts returns all execution contexts in the sandbox.
func (s *Sandbox) ListContexts(ctx context.Context) ([]*Context, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrSandboxClosed
	}
	s.mu.RUnlock()

	// Set request timeout
	timeout := s.config.requestTimeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	respBody, statusCode, err := s.httpClient.doRequest(ctx, http.MethodGet, "/contexts", nil)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, NewRequestTimeoutError()
		}
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, formatHTTPError(statusCode, string(respBody))
	}

	var ctxResps []contextResponse
	if err := json.Unmarshal(respBody, &ctxResps); err != nil {
		return nil, fmt.Errorf("failed to parse contexts response: %w", err)
	}

	contexts := make([]*Context, len(ctxResps))
	for i, ctxResp := range ctxResps {
		contexts[i] = ctxResp.toContext()
	}

	return contexts, nil
}

// RemoveContext removes an execution context.
//
// The contextID can be either a Context.ID string or a *Context.
func (s *Sandbox) RemoveContext(ctx context.Context, contextID string) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrSandboxClosed
	}
	s.mu.RUnlock()

	if contextID == "" {
		return fmt.Errorf("%w: context ID is required", ErrInvalidArgument)
	}

	// Set request timeout
	timeout := s.config.requestTimeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	path := fmt.Sprintf("/contexts/%s", contextID)
	respBody, statusCode, err := s.httpClient.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return NewRequestTimeoutError()
		}
		return err
	}

	if statusCode != http.StatusOK && statusCode != http.StatusNoContent {
		return formatHTTPError(statusCode, string(respBody))
	}

	return nil
}

// RestartContext restarts an execution context, clearing its state.
func (s *Sandbox) RestartContext(ctx context.Context, contextID string) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrSandboxClosed
	}
	s.mu.RUnlock()

	if contextID == "" {
		return fmt.Errorf("%w: context ID is required", ErrInvalidArgument)
	}

	// Set request timeout
	timeout := s.config.requestTimeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	path := fmt.Sprintf("/contexts/%s/restart", contextID)
	respBody, statusCode, err := s.httpClient.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return NewRequestTimeoutError()
		}
		return err
	}

	if statusCode != http.StatusOK && statusCode != http.StatusNoContent {
		return formatHTTPError(statusCode, string(respBody))
	}

	return nil
}

// Close closes the sandbox and releases resources.
//
// After calling Close, the sandbox cannot be used for further operations.
func (s *Sandbox) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	// Kill the sandbox via E2B API
	if s.ID != "" && s.config != nil && s.config.apiKey != "" {
		_ = killSandbox(s.config.httpClient, s.config.apiKey, s.ID)
	}

	return nil
}

// killSandbox calls the E2B API to terminate a sandbox.
func killSandbox(client *http.Client, apiKey, sandboxID string) error {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	req, err := http.NewRequest(http.MethodDelete, e2bAPIBaseURL+"/sandboxes/"+sandboxID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Key", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 204 No Content is success, 404 means already killed
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// SetTimeout sets the default timeout for code execution.
func (s *Sandbox) SetTimeout(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.timeout = d
}

// Timeout returns the current default timeout for code execution.
func (s *Sandbox) Timeout() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.timeout
}

// IsClosed returns whether the sandbox has been closed.
func (s *Sandbox) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}
