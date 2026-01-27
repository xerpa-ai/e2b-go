package e2b

import (
	"context"
	"encoding/json"
	"fmt"
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

	mu             sync.RWMutex
	config         *sandboxConfig
	httpClient     *httpClient
	closed         bool
	accessToken    string
	trafficToken   string
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

	// TODO: In a full implementation, this would call the E2B API to create
	// a new sandbox instance and get the sandbox ID, access tokens, etc.
	// For now, we'll set up the structure assuming the sandbox already exists.

	sandbox := &Sandbox{
		ID:     "", // Will be set after API call
		config: cfg,
	}

	return sandbox, nil
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
		config: cfg,
	}

	// Initialize the HTTP client for the Jupyter server
	sandbox.initHTTPClient()

	return sandbox, nil
}

// initHTTPClient initializes the HTTP client for Jupyter API calls.
func (s *Sandbox) initHTTPClient() {
	scheme := "https"
	if s.config.debug {
		scheme = "http"
	}

	// The host would typically come from the E2B API response
	// Format: {sandboxID}.e2b.dev or similar
	baseURL := fmt.Sprintf("%s://%s:%d", scheme, s.getHost(), JupyterPort)

	s.httpClient = newHTTPClient(
		s.config.httpClient,
		baseURL,
		s.accessToken,
		s.trafficToken,
	)
}

// getHost returns the sandbox host.
// In production, this would return the actual sandbox hostname.
func (s *Sandbox) getHost() string {
	// This would typically be something like:
	// return fmt.Sprintf("%s.e2b.dev", s.ID)
	return fmt.Sprintf("%s.e2b.dev", s.ID)
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

	// TODO: In a full implementation, this would call the E2B API to
	// terminate the sandbox instance.

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
