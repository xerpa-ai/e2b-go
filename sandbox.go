package e2b

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
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
	// TrafficAccessToken is used for accessing sandbox services with restricted public traffic.
	TrafficAccessToken string

	// Files provides filesystem operations for the sandbox.
	Files *Filesystem
	// Commands provides command execution operations for the sandbox.
	Commands *Commands
	// Pty provides pseudo-terminal operations for the sandbox.
	Pty *Pty

	// mu protects concurrent access to sandbox state.
	mu sync.RWMutex
	// config holds the sandbox configuration.
	config *sandboxConfig
	// httpClient is used for API requests.
	httpClient *httpClient
	// closed indicates whether the sandbox has been closed.
	closed bool
	// accessToken is the envd access token.
	accessToken string
	// envdVersion is the version of the envd service.
	envdVersion string
}

// networkRequestOptions represents network options in the API request.
type networkRequestOptions struct {
	AllowOut           []string `json:"allowOut,omitempty"`
	DenyOut            []string `json:"denyOut,omitempty"`
	AllowPublicTraffic bool     `json:"allowPublicTraffic,omitempty"`
	MaskRequestHost    string   `json:"maskRequestHost,omitempty"`
}

// sandboxCreateRequest represents the request body for creating a sandbox.
type sandboxCreateRequest struct {
	TemplateID          string                 `json:"templateID"`
	Timeout             int                    `json:"timeout,omitempty"`
	Metadata            map[string]string      `json:"metadata,omitempty"`
	EnvVars             map[string]string      `json:"envVars,omitempty"`
	Secure              bool                   `json:"secure"`
	AllowInternetAccess bool                   `json:"allow_internet_access"`
	AutoPause           bool                   `json:"autoPause"`
	Network             *networkRequestOptions `json:"network,omitempty"`
	Mcp                 map[string]any         `json:"mcp,omitempty"`
}

// sandboxConnectRequest represents the request body for connecting to a sandbox.
type sandboxConnectRequest struct {
	Timeout int `json:"timeout,omitempty"`
}

// sandboxConnectResponse represents the response from connecting to a sandbox.
type sandboxConnectResponse struct {
	SandboxID          string `json:"sandboxID"`
	Domain             string `json:"domain"`
	EnvdVersion        string `json:"envdVersion"`
	EnvdAccessToken    string `json:"envdAccessToken"`
	TrafficAccessToken string `json:"trafficAccessToken"`
}

// sandboxTimeoutRequest represents the request body for setting sandbox timeout.
type sandboxTimeoutRequest struct {
	Timeout int `json:"timeout"`
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

	// Get configuration from environment variables if not provided
	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv("E2B_API_KEY")
	}
	if cfg.accessToken == "" {
		cfg.accessToken = os.Getenv("E2B_ACCESS_TOKEN")
	}
	if cfg.domain == "" || cfg.domain == DefaultDomain {
		if envDomain := os.Getenv("E2B_DOMAIN"); envDomain != "" {
			cfg.domain = envDomain
		}
	}
	if cfg.apiURL == "" {
		cfg.apiURL = os.Getenv("E2B_API_URL")
	}
	if cfg.sandboxURL == "" {
		cfg.sandboxURL = os.Getenv("E2B_SANDBOX_URL")
	}
	if !cfg.debug {
		cfg.debug = os.Getenv("E2B_DEBUG") == "true"
	}

	// Compute API URL if not provided
	if cfg.apiURL == "" {
		if cfg.debug {
			cfg.apiURL = "http://localhost:3000"
		} else {
			cfg.apiURL = fmt.Sprintf("https://api.%s", cfg.domain)
		}
	}

	// Create HTTP client if not provided
	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{
			Timeout: cfg.requestTimeout,
		}
	}

	// In debug mode, return a mock sandbox without calling the API
	if cfg.debug {
		sandbox := &Sandbox{
			ID:          DebugSandboxID,
			Domain:      cfg.domain,
			config:      cfg,
			envdVersion: EnvdDebugFallback,
		}
		sandbox.initHTTPClient()
		sandbox.Files = newFilesystem(sandbox)
		sandbox.Commands = newCommands(sandbox)
		sandbox.Pty = newPty(sandbox)
		return sandbox, nil
	}

	// Validate API key (required for non-debug mode)
	if cfg.apiKey == "" {
		return nil, fmt.Errorf("%w: API key is required (use WithAPIKey or set E2B_API_KEY)", ErrInvalidArgument)
	}

	// Create sandbox via E2B API
	createReq := &sandboxCreateRequest{
		TemplateID:          cfg.template,
		Timeout:             int(cfg.timeoutMs.Seconds()),
		Metadata:            cfg.metadata,
		EnvVars:             cfg.envVars,
		Secure:              cfg.secure,
		AllowInternetAccess: cfg.allowInternetAccess,
		AutoPause:           cfg.autoPause,
		Mcp:                 cfg.mcp,
	}

	// Add network options if specified
	if cfg.network != nil {
		createReq.Network = &networkRequestOptions{
			AllowOut:           cfg.network.AllowOut,
			DenyOut:            cfg.network.DenyOut,
			AllowPublicTraffic: cfg.network.AllowPublicTraffic,
			MaskRequestHost:    cfg.network.MaskRequestHost,
		}
	}

	createResp, err := createSandbox(cfg.httpClient, cfg.apiURL, cfg.apiKey, createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox: %w", err)
	}

	// Use the domain from API response, or fallback to configured domain
	domain := createResp.Domain
	if domain == "" {
		domain = cfg.domain
	}

	sandbox := &Sandbox{
		ID:                 createResp.SandboxID,
		Domain:             domain,
		TrafficAccessToken: createResp.TrafficAccessToken,
		config:             cfg,
		accessToken:        createResp.EnvdAccessToken,
		envdVersion:        createResp.EnvdVersion,
	}

	// Initialize the HTTP client for Jupyter API calls
	sandbox.initHTTPClient()

	// Initialize the Filesystem
	sandbox.Files = newFilesystem(sandbox)

	// Initialize the Commands
	sandbox.Commands = newCommands(sandbox)

	// Initialize the PTY
	sandbox.Pty = newPty(sandbox)

	return sandbox, nil
}

// createSandbox calls the E2B API to create a new sandbox.
func createSandbox(client *http.Client, apiURL, apiKey string, req *sandboxCreateRequest) (*sandboxCreateResponse, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, apiURL+"/sandboxes", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", apiKey)
	httpReq.Header.Set("User-Agent", "e2b-go-sdk/"+Version)

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
// If the sandbox is paused, it will be automatically resumed.
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

	// Get configuration from environment variables if not provided
	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv("E2B_API_KEY")
	}
	if cfg.domain == "" || cfg.domain == DefaultDomain {
		if envDomain := os.Getenv("E2B_DOMAIN"); envDomain != "" {
			cfg.domain = envDomain
		}
	}
	if cfg.apiURL == "" {
		cfg.apiURL = os.Getenv("E2B_API_URL")
	}
	if !cfg.debug {
		cfg.debug = os.Getenv("E2B_DEBUG") == "true"
	}

	// Compute API URL if not provided
	if cfg.apiURL == "" {
		if cfg.debug {
			cfg.apiURL = "http://localhost:3000"
		} else {
			cfg.apiURL = fmt.Sprintf("https://api.%s", cfg.domain)
		}
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

	// In debug mode, return a mock sandbox without calling the API
	if cfg.debug {
		sandbox := &Sandbox{
			ID:          sandboxID,
			Domain:      cfg.domain,
			config:      cfg,
			envdVersion: EnvdDebugFallback,
		}
		sandbox.initHTTPClient()
		sandbox.Files = newFilesystem(sandbox)
		sandbox.Commands = newCommands(sandbox)
		sandbox.Pty = newPty(sandbox)
		return sandbox, nil
	}

	// Validate API key (required for non-debug mode)
	if cfg.apiKey == "" {
		return nil, fmt.Errorf("%w: API key is required", ErrInvalidArgument)
	}

	// Connect to sandbox via E2B API
	connectResp, err := connectSandbox(cfg.httpClient, cfg.apiURL, cfg.apiKey, sandboxID, int(cfg.timeoutMs.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to sandbox: %w", err)
	}

	// Use the domain from API response, or fallback to configured domain
	domain := connectResp.Domain
	if domain == "" {
		domain = cfg.domain
	}

	sandbox := &Sandbox{
		ID:                 sandboxID,
		Domain:             domain,
		TrafficAccessToken: connectResp.TrafficAccessToken,
		config:             cfg,
		accessToken:        connectResp.EnvdAccessToken,
		envdVersion:        connectResp.EnvdVersion,
	}

	// Initialize the HTTP client for the Jupyter server
	sandbox.initHTTPClient()

	// Initialize the Filesystem
	sandbox.Files = newFilesystem(sandbox)

	// Initialize the Commands
	sandbox.Commands = newCommands(sandbox)

	// Initialize the PTY
	sandbox.Pty = newPty(sandbox)

	return sandbox, nil
}

// connectSandbox calls the E2B API to connect to an existing sandbox.
func connectSandbox(client *http.Client, apiURL, apiKey, sandboxID string, timeout int) (*sandboxConnectResponse, error) {
	reqBody, err := json.Marshal(&sandboxConnectRequest{Timeout: timeout})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/sandboxes/%s/connect", apiURL, sandboxID)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", apiKey)
	httpReq.Header.Set("User-Agent", "e2b-go-sdk/"+Version)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: sandbox %s not found", ErrNotFound, sandboxID)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var connectResp sandboxConnectResponse
	if err := json.Unmarshal(respBody, &connectResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &connectResp, nil
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
		s.TrafficAccessToken,
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

// getEnvdURL returns the envd service URL for the sandbox.
// Respects sandboxURL override, debug mode, and default URL format.
func (s *Sandbox) getEnvdURL() string {
	if s.config.sandboxURL != "" {
		return s.config.sandboxURL
	}

	scheme := "https"
	if s.config.debug {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s", scheme, s.GetHost(EnvdPort))
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

	// Set code execution timeout (separate from sandbox lifetime timeout)
	// nil = use default, 0 = no timeout, >0 = use that value
	var timeout time.Duration
	if cfg.timeout == nil {
		// Not set, use default
		timeout = DefaultCodeExecutionTimeout
	} else if *cfg.timeout == 0 {
		// Explicitly set to 0 means no timeout
		timeout = 0
	} else {
		timeout = *cfg.timeout
	}

	// Create context with timeout if needed (skip if timeout is 0 for no timeout)
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

	// Kill the sandbox via E2B API (skip in debug mode)
	if !s.config.debug && s.ID != "" && s.config != nil && s.config.apiKey != "" {
		_ = killSandbox(s.config.httpClient, s.config.apiURL, s.config.apiKey, s.ID)
	}

	return nil
}

// killSandbox calls the E2B API to terminate a sandbox.
func killSandbox(client *http.Client, apiURL, apiKey, sandboxID string) error {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	req, err := http.NewRequest(http.MethodDelete, apiURL+"/sandboxes/"+sandboxID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("User-Agent", "e2b-go-sdk/"+Version)

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

// SetTimeout sets the sandbox lifetime timeout.
// This method can extend or reduce the sandbox timeout.
// Maximum time a sandbox can be kept alive is 24 hours for Pro users
// and 1 hour for Hobby users.
func (s *Sandbox) SetTimeout(ctx context.Context, d time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Skip API call in debug mode
	if s.config.debug {
		s.config.timeoutMs = d
		return nil
	}

	// Call API to set timeout
	if err := setSandboxTimeout(ctx, s.config.httpClient, s.config.apiURL, s.config.apiKey, s.ID, int(d.Seconds())); err != nil {
		return err
	}

	s.config.timeoutMs = d
	return nil
}

// setSandboxTimeout calls the E2B API to set sandbox timeout.
func setSandboxTimeout(ctx context.Context, client *http.Client, apiURL, apiKey, sandboxID string, timeout int) error {
	reqBody, err := json.Marshal(&sandboxTimeoutRequest{Timeout: timeout})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/sandboxes/%s/timeout", apiURL, sandboxID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", apiKey)
	httpReq.Header.Set("User-Agent", "e2b-go-sdk/"+Version)

	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: sandbox %s not found", ErrNotFound, sandboxID)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// Timeout returns the current sandbox lifetime timeout.
func (s *Sandbox) Timeout() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.timeoutMs
}

// IsClosed returns whether the sandbox has been closed.
func (s *Sandbox) IsClosed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.closed
}

// IsRunning checks if the sandbox is running by calling the health endpoint.
func (s *Sandbox) IsRunning(ctx context.Context) (bool, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return false, nil
	}
	s.mu.RUnlock()

	// Build health check URL
	scheme := "https"
	if s.config.debug {
		scheme = "http"
	}
	healthURL := fmt.Sprintf("%s://%s/health", scheme, s.GetHost(EnvdPort))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "e2b-go-sdk/"+Version)
	if s.accessToken != "" {
		req.Header.Set(headerAccessToken, s.accessToken)
	}

	resp, err := s.config.httpClient.Do(req)
	if err != nil {
		return false, nil // Connection error likely means sandbox is not running
	}
	defer resp.Body.Close()

	// 502 means sandbox is not running
	if resp.StatusCode == http.StatusBadGateway {
		return false, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("health check failed (status %d): %s", resp.StatusCode, string(body))
	}

	return true, nil
}

// urlConfig holds configuration for URL generation.
type urlConfig struct {
	signatureExpiration int    // seconds, 0 means no expiration
	user                string // user for path resolution
}

// URLOption configures URL generation behavior.
type URLOption func(*urlConfig)

// WithSignatureExpiration sets the signature expiration time in seconds.
// If not set or set to 0, the signature will not expire.
func WithSignatureExpiration(seconds int) URLOption {
	return func(c *urlConfig) {
		c.signatureExpiration = seconds
	}
}

// WithURLUser sets the user for URL path resolution.
func WithURLUser(user string) URLOption {
	return func(c *urlConfig) {
		c.user = user
	}
}

// fileURL builds the base file URL with optional query parameters.
func (s *Sandbox) fileURL(path, username string) string {
	scheme := "https"
	if s.config.debug {
		scheme = "http"
	}

	u := &url.URL{
		Scheme: scheme,
		Host:   s.GetHost(EnvdPort),
		Path:   "/files",
	}

	params := url.Values{}
	if path != "" {
		params.Set("path", path)
	}
	if username != "" {
		params.Set("username", username)
	}

	if len(params) > 0 {
		u.RawQuery = params.Encode()
	}

	return u.String()
}

// UploadURL returns the URL to upload a file to the sandbox.
// You have to send a POST request to this URL with the file as multipart/form-data.
//
// When the sandbox is created with secure mode (default), the URL will include
// a signature for authentication. You can optionally set an expiration time
// for the signature using WithSignatureExpiration.
//
// Example:
//
//	url, err := sandbox.UploadURL("/path/to/file")
//	url, err := sandbox.UploadURL("/path/to/file", e2b.WithSignatureExpiration(3600)) // 1 hour
func (s *Sandbox) UploadURL(path string, opts ...URLOption) (string, error) {
	cfg := &urlConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Default user handling for older envd versions
	user := cfg.user
	if user == "" && s.compareVersion(EnvdVersionDefaultUser) < 0 {
		user = "user"
	}

	baseURL := s.fileURL(path, user)

	if s.accessToken == "" {
		return baseURL, nil
	}

	// Add signature parameters
	u, _ := url.Parse(baseURL)
	params := u.Query()

	sig, exp := getSignature(path, "write", user, s.accessToken, cfg.signatureExpiration)
	params.Set("signature", sig)
	if exp > 0 {
		params.Set("signature_expiration", fmt.Sprintf("%d", exp))
	}

	u.RawQuery = params.Encode()
	return u.String(), nil
}

// DownloadURL returns the URL to download a file from the sandbox.
//
// When the sandbox is created with secure mode (default), the URL will include
// a signature for authentication. You can optionally set an expiration time
// for the signature using WithSignatureExpiration.
//
// Example:
//
//	url, err := sandbox.DownloadURL("/path/to/file")
//	url, err := sandbox.DownloadURL("/path/to/file", e2b.WithSignatureExpiration(3600)) // 1 hour
func (s *Sandbox) DownloadURL(path string, opts ...URLOption) (string, error) {
	cfg := &urlConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Default user handling for older envd versions
	user := cfg.user
	if user == "" && s.compareVersion(EnvdVersionDefaultUser) < 0 {
		user = "user"
	}

	baseURL := s.fileURL(path, user)

	if s.accessToken == "" {
		return baseURL, nil
	}

	// Add signature parameters
	u, _ := url.Parse(baseURL)
	params := u.Query()

	sig, exp := getSignature(path, "read", user, s.accessToken, cfg.signatureExpiration)
	params.Set("signature", sig)
	if exp > 0 {
		params.Set("signature_expiration", fmt.Sprintf("%d", exp))
	}

	u.RawQuery = params.Encode()
	return u.String(), nil
}

// compareVersion compares the envd version with the given version.
// Returns -1 if envdVersion < version, 0 if equal, 1 if envdVersion > version.
func (s *Sandbox) compareVersion(version string) int {
	return compareVersions(s.envdVersion, version)
}

// getSignature generates a v1 signature for sandbox file URLs.
// Returns the signature string and expiration timestamp (0 if no expiration).
func getSignature(path, operation, user, accessToken string, expirationSeconds int) (string, int64) {
	var expiration int64
	if expirationSeconds > 0 {
		expiration = time.Now().Unix() + int64(expirationSeconds)
	}

	// Build the raw string to hash
	var raw string
	if expiration == 0 {
		raw = fmt.Sprintf("%s:%s:%s:%s", path, operation, user, accessToken)
	} else {
		raw = fmt.Sprintf("%s:%s:%s:%s:%d", path, operation, user, accessToken, expiration)
	}

	// SHA256 hash
	hash := sha256.Sum256([]byte(raw))

	// Base64 encode without padding
	encoded := base64.StdEncoding.EncodeToString(hash[:])
	encoded = strings.TrimRight(encoded, "=")

	return "v1_" + encoded, expiration
}

// SandboxInfo contains information about a sandbox.
type SandboxInfo struct {
	SandboxID   string            `json:"sandboxID"`
	TemplateID  string            `json:"templateID"`
	Alias       string            `json:"alias,omitempty"`
	ClientID    string            `json:"clientID"`
	StartedAt   string            `json:"startedAt"`
	EndAt       string            `json:"endAt"`
	CpuCount    int               `json:"cpuCount"`
	MemoryMB    int               `json:"memoryMB"`
	DiskSizeMB  int               `json:"diskSizeMB"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	State       SandboxState      `json:"state"`
	EnvdVersion string            `json:"envdVersion"`
}

// GetInfo returns information about this sandbox.
func (s *Sandbox) GetInfo(ctx context.Context) (*SandboxInfo, error) {
	return GetSandboxInfo(ctx, s.ID, s.config.httpClient, s.config.apiURL, s.config.apiKey)
}

// GetSandboxInfo returns information about a sandbox by ID.
// This is a static method that can be called without a sandbox instance.
func GetSandboxInfo(ctx context.Context, sandboxID string, client *http.Client, apiURL, apiKey string) (*SandboxInfo, error) {
	if client == nil {
		client = &http.Client{Timeout: DefaultRequestTimeout}
	}

	url := fmt.Sprintf("%s/sandboxes/%s", apiURL, sandboxID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("User-Agent", "e2b-go-sdk/"+Version)

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
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var info SandboxInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &info, nil
}

// SandboxMetrics contains resource usage metrics for a sandbox.
type SandboxMetrics struct {
	CPUCount      int       `json:"cpuCount"`
	CPUUsedPct    float64   `json:"cpuUsedPct"`
	MemUsed       int64     `json:"memUsed"`
	MemTotal      int64     `json:"memTotal"`
	DiskUsed      int64     `json:"diskUsed"`
	DiskTotal     int64     `json:"diskTotal"`
	TimestampUnix int64     `json:"timestampUnix"`
	Timestamp     time.Time `json:"timestamp"` // deprecated but kept for compatibility
}

// metricsConfig holds configuration for GetMetrics.
type metricsConfig struct {
	start          *time.Time
	end            *time.Time
	requestTimeout time.Duration
}

// MetricsOption configures GetMetrics behavior.
type MetricsOption func(*metricsConfig)

// WithMetricsStart sets the start time for metrics query.
func WithMetricsStart(t time.Time) MetricsOption {
	return func(c *metricsConfig) {
		c.start = &t
	}
}

// WithMetricsEnd sets the end time for metrics query.
func WithMetricsEnd(t time.Time) MetricsOption {
	return func(c *metricsConfig) {
		c.end = &t
	}
}

// WithMetricsRequestTimeout sets the request timeout for metrics query.
func WithMetricsRequestTimeout(d time.Duration) MetricsOption {
	return func(c *metricsConfig) {
		c.requestTimeout = d
	}
}

// GetMetrics returns resource usage metrics for this sandbox.
// Metrics include CPU, memory, and disk usage information.
//
// Example:
//
//	metrics, err := sandbox.GetMetrics(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, m := range metrics {
//	    fmt.Printf("CPU: %.2f%%, Memory: %d/%d bytes\n", m.CPUUsedPct, m.MemUsed, m.MemTotal)
//	}
func (s *Sandbox) GetMetrics(ctx context.Context, opts ...MetricsOption) ([]SandboxMetrics, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrSandboxClosed
	}
	s.mu.RUnlock()

	cfg := &metricsConfig{
		requestTimeout: s.config.requestTimeout,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return GetSandboxMetrics(ctx, s.ID, s.config.httpClient, s.config.apiURL, s.config.apiKey, cfg)
}

// GetSandboxMetrics returns resource usage metrics for a sandbox by ID.
// This is a static method that can be called without a sandbox instance.
func GetSandboxMetrics(ctx context.Context, sandboxID string, client *http.Client, apiURL, apiKey string, cfg *metricsConfig) ([]SandboxMetrics, error) {
	if client == nil {
		client = &http.Client{Timeout: DefaultRequestTimeout}
	}

	url := fmt.Sprintf("%s/sandboxes/%s/metrics", apiURL, sandboxID)

	// Add query parameters for time range
	params := ""
	if cfg != nil {
		if cfg.start != nil {
			params += fmt.Sprintf("start=%d", cfg.start.Unix())
		}
		if cfg.end != nil {
			if params != "" {
				params += "&"
			}
			params += fmt.Sprintf("end=%d", cfg.end.Unix())
		}
		if params != "" {
			url += "?" + params
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("User-Agent", "e2b-go-sdk/"+Version)

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
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var metrics []SandboxMetrics
	if err := json.Unmarshal(body, &metrics); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return metrics, nil
}

// Kill terminates a sandbox by ID.
// This is a static method that can be called without a sandbox instance.
func Kill(ctx context.Context, sandboxID string, opts ...Option) error {
	cfg := defaultSandboxConfig()
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

	// Skip in debug mode
	if cfg.debug {
		return nil
	}

	if cfg.apiKey == "" {
		return fmt.Errorf("%w: API key is required", ErrInvalidArgument)
	}

	client := cfg.httpClient
	if client == nil {
		client = &http.Client{Timeout: DefaultRequestTimeout}
	}

	return killSandbox(client, cfg.apiURL, cfg.apiKey, sandboxID)
}

// BetaPause pauses this sandbox.
// This is a beta feature and may change in the future.
//
// A paused sandbox can be resumed by calling Connect with the sandbox ID.
//
// Example:
//
//	err := sandbox.BetaPause(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Later, resume with:
//	// sandbox, err := e2b.Connect(sandboxID)
func (s *Sandbox) BetaPause(ctx context.Context) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrSandboxClosed
	}
	apiKey := s.config.apiKey
	apiURL := s.config.apiURL
	client := s.config.httpClient
	debug := s.config.debug
	s.mu.RUnlock()

	if debug {
		return nil
	}

	return pauseSandbox(ctx, client, apiURL, apiKey, s.ID)
}

// BetaPause pauses a sandbox by ID.
// This is a beta feature and may change in the future.
// This is a static method that can be called without a sandbox instance.
func BetaPause(ctx context.Context, sandboxID string, opts ...Option) error {
	cfg := defaultSandboxConfig()
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

	// Skip in debug mode
	if cfg.debug {
		return nil
	}

	if cfg.apiKey == "" {
		return fmt.Errorf("%w: API key is required", ErrInvalidArgument)
	}

	client := cfg.httpClient
	if client == nil {
		client = &http.Client{Timeout: DefaultRequestTimeout}
	}

	return pauseSandbox(ctx, client, cfg.apiURL, cfg.apiKey, sandboxID)
}

// pauseSandbox calls the E2B API to pause a sandbox.
func pauseSandbox(ctx context.Context, client *http.Client, apiURL, apiKey, sandboxID string) error {
	if client == nil {
		client = &http.Client{Timeout: DefaultRequestTimeout}
	}

	url := fmt.Sprintf("%s/sandboxes/%s/pause", apiURL, sandboxID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("User-Agent", "e2b-go-sdk/"+Version)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 409 Conflict means sandbox is already paused - treat as success
	if resp.StatusCode == http.StatusConflict {
		return nil
	}

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: sandbox %s not found", ErrNotFound, sandboxID)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// mcpTokenPath is the path to the MCP token file in the sandbox.
const mcpTokenPath = "/etc/mcp-gateway/.token"

// GetMcpToken retrieves the MCP (Model Context Protocol) token from the sandbox.
// The token is read from /etc/mcp-gateway/.token in the sandbox filesystem.
//
// Example:
//
//	token, err := sandbox.GetMcpToken(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("MCP Token:", token)
func (s *Sandbox) GetMcpToken(ctx context.Context) (string, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return "", ErrSandboxClosed
	}
	s.mu.RUnlock()

	// Read the token file from the sandbox filesystem
	content, err := s.Files.Read(ctx, mcpTokenPath)
	if err != nil {
		return "", fmt.Errorf("failed to read MCP token: %w", err)
	}

	// Trim any whitespace from the token
	token := strings.TrimSpace(string(content))
	if token == "" {
		return "", fmt.Errorf("MCP token is empty")
	}

	return token, nil
}
