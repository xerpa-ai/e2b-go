package e2b

import (
	"net/http"
	"time"
)

// sandboxConfig holds configuration for creating a Sandbox.
type sandboxConfig struct {
	apiKey              string
	accessToken         string
	domain              string
	apiURL              string
	sandboxURL          string
	template            string
	timeoutMs           time.Duration
	requestTimeout      time.Duration
	httpClient          *http.Client
	debug               bool
	secure              bool
	allowInternetAccess bool
	autoPause           bool
	metadata            map[string]string
	envVars             map[string]string
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
func WithAutoPause(autoPause bool) Option {
	return func(c *sandboxConfig) {
		c.autoPause = autoPause
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

// runConfig holds configuration for running code.
type runConfig struct {
	language       string
	context        *Context
	envVars        map[string]string
	timeout        time.Duration
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
func WithRunTimeout(d time.Duration) RunOption {
	return func(c *runConfig) {
		c.timeout = d
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
