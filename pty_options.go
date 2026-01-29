package e2b

import "time"

// ptyConfig holds configuration for PTY creation.
type ptyConfig struct {
	user           string
	cwd            string
	envs           map[string]string
	timeout        time.Duration
	requestTimeout time.Duration
	onStdout       func(output string)
	onStderr       func(output string)
}

// defaultPtyConfig returns a default ptyConfig.
func defaultPtyConfig() *ptyConfig {
	return &ptyConfig{
		timeout:        60 * time.Second,
		requestTimeout: DefaultRequestTimeout,
	}
}

// PtyOption configures PTY creation behavior.
type PtyOption func(*ptyConfig)

// WithPtyUser sets the user to run the PTY as.
func WithPtyUser(user string) PtyOption {
	return func(c *ptyConfig) {
		c.user = user
	}
}

// WithPtyCwd sets the working directory for the PTY.
func WithPtyCwd(cwd string) PtyOption {
	return func(c *ptyConfig) {
		c.cwd = cwd
	}
}

// WithPtyEnvs sets environment variables for the PTY.
func WithPtyEnvs(envs map[string]string) PtyOption {
	return func(c *ptyConfig) {
		c.envs = envs
	}
}

// WithPtyTimeout sets the timeout for the PTY connection.
func WithPtyTimeout(d time.Duration) PtyOption {
	return func(c *ptyConfig) {
		c.timeout = d
	}
}

// WithPtyRequestTimeout sets the request timeout for the PTY.
func WithPtyRequestTimeout(d time.Duration) PtyOption {
	return func(c *ptyConfig) {
		c.requestTimeout = d
	}
}

// OnPtyStdout sets a callback for PTY stdout output.
func OnPtyStdout(handler func(output string)) PtyOption {
	return func(c *ptyConfig) {
		c.onStdout = handler
	}
}

// OnPtyStderr sets a callback for PTY stderr output.
func OnPtyStderr(handler func(output string)) PtyOption {
	return func(c *ptyConfig) {
		c.onStderr = handler
	}
}

// ptyConnectConfig holds configuration for connecting to a PTY.
type ptyConnectConfig struct {
	timeout        time.Duration
	requestTimeout time.Duration
	onStdout       func(output string)
	onStderr       func(output string)
}

// PtyConnectOption configures PTY connection behavior.
type PtyConnectOption func(*ptyConnectConfig)

// WithPtyConnectTimeout sets the timeout for the PTY connection.
func WithPtyConnectTimeout(d time.Duration) PtyConnectOption {
	return func(c *ptyConnectConfig) {
		c.timeout = d
	}
}

// WithPtyConnectRequestTimeout sets the request timeout for connecting to PTY.
func WithPtyConnectRequestTimeout(d time.Duration) PtyConnectOption {
	return func(c *ptyConnectConfig) {
		c.requestTimeout = d
	}
}

// OnPtyConnectStdout sets a callback for PTY stdout output when connecting.
func OnPtyConnectStdout(handler func(output string)) PtyConnectOption {
	return func(c *ptyConnectConfig) {
		c.onStdout = handler
	}
}

// OnPtyConnectStderr sets a callback for PTY stderr output when connecting.
func OnPtyConnectStderr(handler func(output string)) PtyConnectOption {
	return func(c *ptyConnectConfig) {
		c.onStderr = handler
	}
}

// ptyRequestConfig holds configuration for PTY requests.
type ptyRequestConfig struct {
	requestTimeout time.Duration
}

// PtyRequestOption configures PTY request behavior.
type PtyRequestOption func(*ptyRequestConfig)

// WithPtyReqTimeout sets the request timeout for PTY operations.
func WithPtyReqTimeout(d time.Duration) PtyRequestOption {
	return func(c *ptyRequestConfig) {
		c.requestTimeout = d
	}
}
