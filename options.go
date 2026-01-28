package e2b

import (
	"net/http"
	"time"
)

// sandboxConfig holds configuration for creating a Sandbox.
type sandboxConfig struct {
	apiKey          string
	template        string
	timeout         time.Duration
	requestTimeout  time.Duration
	httpClient      *http.Client
	debug           bool
	secure          bool
	metadata        map[string]string
	envVars         map[string]string
	skipJupyterWait bool
}

// defaultSandboxConfig returns the default sandbox configuration.
func defaultSandboxConfig() *sandboxConfig {
	return &sandboxConfig{
		template:       DefaultTemplate,
		timeout:        DefaultTimeout,
		requestTimeout: DefaultRequestTimeout,
		secure:         true, // Enable secure mode by default for filesystem access
	}
}

// Option configures a Sandbox.
type Option func(*sandboxConfig)

// WithAPIKey sets the E2B API key.
func WithAPIKey(key string) Option {
	return func(c *sandboxConfig) {
		c.apiKey = key
	}
}

// WithTemplate sets the sandbox template.
func WithTemplate(template string) Option {
	return func(c *sandboxConfig) {
		c.template = template
	}
}

// WithTimeout sets the default timeout for code execution.
func WithTimeout(d time.Duration) Option {
	return func(c *sandboxConfig) {
		c.timeout = d
	}
}

// WithRequestTimeout sets the default timeout for HTTP requests.
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
func WithDebug(debug bool) Option {
	return func(c *sandboxConfig) {
		c.debug = debug
	}
}

// WithSecure enables secure mode for the sandbox.
// When enabled, the sandbox requires authentication tokens for filesystem access.
// This is enabled by default.
func WithSecure(secure bool) Option {
	return func(c *sandboxConfig) {
		c.secure = secure
	}
}

// WithMetadata sets sandbox metadata.
func WithMetadata(metadata map[string]string) Option {
	return func(c *sandboxConfig) {
		c.metadata = metadata
	}
}

// WithEnvVars sets default environment variables for the sandbox.
func WithEnvVars(envVars map[string]string) Option {
	return func(c *sandboxConfig) {
		c.envVars = envVars
	}
}

// WithSkipJupyterWait skips waiting for the Jupyter server to be ready.
// Use this option when using templates that don't include a Jupyter server,
// such as the "base" template, and you only need Commands or Filesystem operations.
func WithSkipJupyterWait(skip bool) Option {
	return func(c *sandboxConfig) {
		c.skipJupyterWait = skip
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
