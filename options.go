package e2b

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// NetworkOptions configures network access for the sandbox.
type NetworkOptions struct {
	// AllowOut specifies allowed outbound traffic destinations (hostnames or IPs).
	// When set, only traffic to these destinations is allowed.
	AllowOut []string
	// DenyOut specifies denied outbound traffic destinations (hostnames or IPs).
	// When set, traffic to these destinations is blocked.
	DenyOut []string
	// AllowPublicTraffic allows or denies public traffic to the sandbox.
	AllowPublicTraffic bool
	// MaskRequestHost masks the request host header to this value.
	MaskRequestHost string
}

// SandboxLifecycle configures the sandbox lifecycle behavior.
type SandboxLifecycle struct {
	// OnTimeout specifies what happens when the sandbox times out.
	// "kill" (default) terminates the sandbox, "pause" pauses it.
	OnTimeout string
	// AutoResume enables automatic resumption of paused sandboxes when accessed.
	// Only valid when OnTimeout is "pause". Defaults to false.
	AutoResume bool
}

// VolumeMountConfig specifies a volume to mount in a sandbox.
type VolumeMountConfig struct {
	// Name is the volume name.
	Name string `json:"name"`
	// Path is the mount path inside the sandbox.
	Path string `json:"path"`
}

// sandboxConfig holds configuration for creating a Sandbox.
type sandboxConfig struct {
	apiKey              string              // E2B API key for authentication
	accessToken         string              // envd access token for sandbox operations
	domain              string              // base domain for E2B services (default: e2b.app)
	apiURL              string              // E2B API URL (default: https://api.{domain})
	sandboxURL          string              // sandbox connection URL override
	template            string              // sandbox template name or ID
	timeoutMs           time.Duration       // sandbox lifetime timeout
	requestTimeout      time.Duration       // default timeout for HTTP requests
	httpClient          *http.Client        // HTTP client for API requests
	debug               bool                // enable debug mode (uses HTTP instead of HTTPS)
	secure              bool                // enable secure mode for sandbox traffic
	allowInternetAccess bool                // allow sandbox to access the internet
	autoPause           bool                // automatically pause sandbox after timeout (deprecated)
	lifecycle           *SandboxLifecycle   // lifecycle configuration (replaces autoPause)
	volumeMounts        []VolumeMountConfig // volumes to mount in the sandbox
	metadata            map[string]string   // custom metadata for the sandbox
	envVars             map[string]string   // default environment variables
	network             *NetworkOptions     // network access configuration
	mcp                 map[string]any      // MCP server configuration
}

// defaultSandboxConfig returns the default sandbox configuration.
func defaultSandboxConfig() *sandboxConfig {
	return &sandboxConfig{
		domain:              DefaultDomain,
		template:            DefaultTemplate,
		timeoutMs:           DefaultSandboxTimeout,
		requestTimeout:      DefaultRequestTimeout,
		secure:              true, // Enable secure mode by default for filesystem access
		allowInternetAccess: true, // Allow internet access by default
	}
}

// applyEnvironment applies configuration from environment variables and CLI config.
// Resolution order: direct param > env var > CLI config file (~/.e2b/config.json).
func (c *sandboxConfig) applyEnvironment() {
	if c.apiKey == "" {
		c.apiKey = os.Getenv("E2B_API_KEY")
	}
	if c.accessToken == "" {
		c.accessToken = os.Getenv("E2B_ACCESS_TOKEN")
	}
	if c.domain == "" || c.domain == DefaultDomain {
		if envDomain := os.Getenv("E2B_DOMAIN"); envDomain != "" {
			c.domain = envDomain
		}
	}
	if c.apiURL == "" {
		c.apiURL = os.Getenv("E2B_API_URL")
	}
	if c.sandboxURL == "" {
		c.sandboxURL = os.Getenv("E2B_SANDBOX_URL")
	}
	if !c.debug {
		c.debug = os.Getenv("E2B_DEBUG") == "true"
	}

	// Fallback to CLI config file if credentials are still missing
	if c.apiKey == "" || c.accessToken == "" {
		if cliCfg := readCLIConfig(); cliCfg != nil {
			if c.apiKey == "" && cliCfg.TeamAPIKey != "" {
				c.apiKey = cliCfg.TeamAPIKey
			}
			if c.accessToken == "" && cliCfg.AccessToken != "" {
				c.accessToken = cliCfg.AccessToken
			}
		}
	}
}

// cliConfig represents the E2B CLI configuration file at ~/.e2b/config.json.
type cliConfig struct {
	Email       string `json:"email"`
	AccessToken string `json:"accessToken"`
	TeamName    string `json:"teamName"`
	TeamID      string `json:"teamId"`
	TeamAPIKey  string `json:"teamApiKey"`
}

// readCLIConfig reads the E2B CLI config from ~/.e2b/config.json.
// Returns nil if the file doesn't exist or can't be read.
func readCLIConfig() *cliConfig {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	data, err := os.ReadFile(filepath.Join(home, ".e2b", "config.json"))
	if err != nil {
		return nil
	}

	var cfg cliConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}

	return &cfg
}

// computeAPIURL sets the API URL if not already configured.
func (c *sandboxConfig) computeAPIURL() {
	if c.apiURL == "" {
		if c.debug {
			c.apiURL = "http://localhost:3000"
		} else {
			c.apiURL = fmt.Sprintf("https://api.%s", c.domain)
		}
	}
}

// ensureHTTPClient creates the HTTP client if not already set.
func (c *sandboxConfig) ensureHTTPClient() {
	if c.httpClient == nil {
		c.httpClient = &http.Client{
			Timeout: c.requestTimeout,
		}
	}
}

// Option configures a Sandbox.
type Option func(*sandboxConfig)

// WithAPIKey sets the E2B API key.
// Defaults to E2B_API_KEY environment variable.
func WithAPIKey(key string) Option {
	return func(c *sandboxConfig) {
		c.apiKey = key
	}
}

// WithAccessToken sets the E2B access token.
// Defaults to E2B_ACCESS_TOKEN environment variable.
func WithAccessToken(token string) Option {
	return func(c *sandboxConfig) {
		c.accessToken = token
	}
}

// WithDomain sets the E2B domain.
// Defaults to E2B_DOMAIN environment variable or "e2b.app".
func WithDomain(domain string) Option {
	return func(c *sandboxConfig) {
		c.domain = domain
	}
}

// WithAPIURL sets the E2B API URL.
// Defaults to E2B_API_URL environment variable or "https://api.{domain}".
// This is primarily used for internal development.
func WithAPIURL(url string) Option {
	return func(c *sandboxConfig) {
		c.apiURL = url
	}
}

// WithSandboxURL sets the sandbox connection URL.
// Defaults to E2B_SANDBOX_URL environment variable or "https://{port}-{sandboxID}.{domain}".
// This is primarily used for internal development.
func WithSandboxURL(url string) Option {
	return func(c *sandboxConfig) {
		c.sandboxURL = url
	}
}

// WithTemplate sets the sandbox template.
func WithTemplate(template string) Option {
	return func(c *sandboxConfig) {
		c.template = template
	}
}

// WithTimeout sets the sandbox lifetime timeout.
// Maximum time a sandbox can be kept alive is 24 hours for Pro users
// and 1 hour for Hobby users.
// Defaults to 5 minutes.
func WithTimeout(d time.Duration) Option {
	return func(c *sandboxConfig) {
		c.timeoutMs = d
	}
}

// WithRequestTimeout sets the default timeout for HTTP requests.
// Defaults to 60 seconds.
func WithRequestTimeout(d time.Duration) Option {
	return func(c *sandboxConfig) {
		c.requestTimeout = d
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *sandboxConfig) {
		c.httpClient = client
	}
}

// WithDebug enables debug mode (uses HTTP instead of HTTPS).
// Defaults to E2B_DEBUG environment variable or false.
func WithDebug(debug bool) Option {
	return func(c *sandboxConfig) {
		c.debug = debug
	}
}

// WithSecure enables secure mode for the sandbox.
// When enabled, the sandbox requires authentication tokens for traffic access.
// This is enabled by default.
func WithSecure(secure bool) Option {
	return func(c *sandboxConfig) {
		c.secure = secure
	}
}

// WithAllowInternetAccess enables or disables internet access for the sandbox.
// This is enabled by default.
func WithAllowInternetAccess(allow bool) Option {
	return func(c *sandboxConfig) {
		c.allowInternetAccess = allow
	}
}

// WithAutoPause enables automatic pausing of the sandbox after the timeout.
// When enabled, the sandbox will be paused instead of killed after timeout.
// Defaults to false.
//
// Deprecated: Use WithLifecycle instead.
func WithAutoPause(autoPause bool) Option {
	return func(c *sandboxConfig) {
		c.autoPause = autoPause
	}
}

// WithLifecycle sets the sandbox lifecycle configuration.
// This replaces WithAutoPause with a more structured approach.
//
// Example:
//
//	sandbox, err := e2b.NewWithContext(ctx, e2b.WithLifecycle(e2b.SandboxLifecycle{
//	    OnTimeout:  "pause",
//	    AutoResume: true,
//	}))
func WithLifecycle(lifecycle SandboxLifecycle) Option {
	return func(c *sandboxConfig) {
		c.lifecycle = &lifecycle
	}
}

// WithVolumeMounts sets volumes to mount in the sandbox.
//
// Example:
//
//	sandbox, err := e2b.NewWithContext(ctx, e2b.WithVolumeMounts([]e2b.VolumeMountConfig{
//	    {Name: "my-volume", Path: "/mnt/data"},
//	}))
func WithVolumeMounts(mounts []VolumeMountConfig) Option {
	return func(c *sandboxConfig) {
		c.volumeMounts = mounts
	}
}

// WithMetadata sets sandbox metadata.
func WithMetadata(metadata map[string]string) Option {
	return func(c *sandboxConfig) {
		c.metadata = metadata
	}
}

// WithEnvVars sets default environment variables for the sandbox.
// These can be overridden when executing commands or code.
func WithEnvVars(envVars map[string]string) Option {
	return func(c *sandboxConfig) {
		c.envVars = envVars
	}
}

// WithTraceparent sets the W3C Trace Context traceparent header as the
// TRACEPARENT environment variable in the sandbox, enabling distributed
// tracing propagation. The value must follow the W3C format:
// {version}-{trace-id}-{parent-id}-{trace-flags}
//
// See https://www.w3.org/TR/trace-context/#traceparent-header
func WithTraceparent(tp string) Option {
	return func(c *sandboxConfig) {
		if c.envVars == nil {
			c.envVars = make(map[string]string)
		}
		c.envVars["TRACEPARENT"] = tp
	}
}

// WithTracestate sets the W3C Trace Context tracestate header as the
// TRACESTATE environment variable in the sandbox. The value contains
// vendor-specific trace context data as comma-separated key=value pairs.
//
// See https://www.w3.org/TR/trace-context/#tracestate-header
func WithTracestate(ts string) Option {
	return func(c *sandboxConfig) {
		if c.envVars == nil {
			c.envVars = make(map[string]string)
		}
		c.envVars["TRACESTATE"] = ts
	}
}

// WithNetwork sets network options for the sandbox.
// This allows fine-grained control over network access.
//
// Example:
//
//	sandbox, err := e2b.New(e2b.WithNetwork(e2b.NetworkOptions{
//	    AllowOut: []string{"api.example.com"},
//	    DenyOut:  []string{"internal.example.com"},
//	    AllowPublicTraffic: true,
//	}))
func WithNetwork(opts NetworkOptions) Option {
	return func(c *sandboxConfig) {
		c.network = &opts
	}
}

// WithMcp sets MCP (Model Context Protocol) configuration for the sandbox.
// This enables MCP server capabilities in the sandbox.
//
// Example:
//
//	sandbox, err := e2b.New(e2b.WithMcp(map[string]any{
//	    "server": map[string]any{"enabled": true},
//	}))
func WithMcp(mcp map[string]any) Option {
	return func(c *sandboxConfig) {
		c.mcp = mcp
	}
}

// runConfig holds configuration for running code.
type runConfig struct {
	language       string
	context        *Context
	envVars        map[string]string
	timeout        *time.Duration // nil = use default, 0 = no timeout, >0 = use that value
	requestTimeout time.Duration
	onStdout       func(OutputMessage)
	onStderr       func(OutputMessage)
	onResult       func(*Result)
	onError        func(*ExecutionError)
}

// defaultRunConfig returns the default run configuration.
func defaultRunConfig() *runConfig {
	return &runConfig{}
}

// RunOption configures code execution.
type RunOption func(*runConfig)

// WithLanguage sets the programming language for code execution.
func WithLanguage(lang string) RunOption {
	return func(c *runConfig) {
		c.language = lang
	}
}

// WithContext sets the execution context.
// This is mutually exclusive with WithLanguage.
func WithContext(ctx *Context) RunOption {
	return func(c *runConfig) {
		c.context = ctx
	}
}

// WithRunEnvVars sets environment variables for code execution.
func WithRunEnvVars(envVars map[string]string) RunOption {
	return func(c *runConfig) {
		c.envVars = envVars
	}
}

// WithRunTimeout sets the timeout for code execution.
// Pass 0 for no timeout (infinite).
// If not called, defaults to DefaultCodeExecutionTimeout.
func WithRunTimeout(d time.Duration) RunOption {
	return func(c *runConfig) {
		c.timeout = &d
	}
}

// WithRunRequestTimeout sets the timeout for the HTTP request.
func WithRunRequestTimeout(d time.Duration) RunOption {
	return func(c *runConfig) {
		c.requestTimeout = d
	}
}

// OnStdout sets a callback for stdout output.
func OnStdout(handler func(OutputMessage)) RunOption {
	return func(c *runConfig) {
		c.onStdout = handler
	}
}

// OnStderr sets a callback for stderr output.
func OnStderr(handler func(OutputMessage)) RunOption {
	return func(c *runConfig) {
		c.onStderr = handler
	}
}

// OnResult sets a callback for execution results.
func OnResult(handler func(*Result)) RunOption {
	return func(c *runConfig) {
		c.onResult = handler
	}
}

// OnError sets a callback for execution errors.
func OnError(handler func(*ExecutionError)) RunOption {
	return func(c *runConfig) {
		c.onError = handler
	}
}

// contextConfig holds configuration for creating a context.
type contextConfig struct {
	language       string
	cwd            string
	requestTimeout time.Duration
}

// defaultContextConfig returns the default context configuration.
func defaultContextConfig() *contextConfig {
	return &contextConfig{}
}

// ContextOption configures context creation.
type ContextOption func(*contextConfig)

// WithContextLanguage sets the programming language for the context.
func WithContextLanguage(lang string) ContextOption {
	return func(c *contextConfig) {
		c.language = lang
	}
}

// WithCWD sets the current working directory for the context.
func WithCWD(cwd string) ContextOption {
	return func(c *contextConfig) {
		c.cwd = cwd
	}
}

// WithContextRequestTimeout sets the request timeout for context operations.
func WithContextRequestTimeout(d time.Duration) ContextOption {
	return func(c *contextConfig) {
		c.requestTimeout = d
	}
}
