package e2b

import "time"

// commandConfig holds configuration for running a command.
type commandConfig struct {
	cwd            string
	envs           map[string]string
	user           string
	timeout        time.Duration
	requestTimeout time.Duration
	onStdout       func(output string)
	onStderr       func(output string)
	stdin          *bool
	tag            *string
}

// defaultCommandConfig returns the default command configuration.
// Default timeout is 60 seconds as per official SDKs.
func defaultCommandConfig() *commandConfig {
	return &commandConfig{
		timeout: 60 * time.Second,
	}
}

// CommandOption configures command execution.
type CommandOption func(*commandConfig)

// WithCommandCwd sets the working directory for the command.
func WithCommandCwd(cwd string) CommandOption {
	return func(c *commandConfig) {
		c.cwd = cwd
	}
}

// WithCommandEnvs sets the environment variables for the command.
// This overrides the default environment variables from Sandbox constructor.
func WithCommandEnvs(envs map[string]string) CommandOption {
	return func(c *commandConfig) {
		c.envs = envs
	}
}

// WithCommandUser sets the user to run the command as.
// Defaults to the sandbox template's default user if not specified.
func WithCommandUser(user string) CommandOption {
	return func(c *commandConfig) {
		c.user = user
	}
}

// WithStdin enables or disables stdin for the command.
// If true, the command will have a stdin stream that you can send data to
// using Commands.SendStdin or CommandHandle.SendStdin.
// Default is false.
//
// Note: Explicitly setting stdin to false requires envd version >= 0.3.0.
// On older versions, stdin is always enabled and cannot be disabled.
func WithStdin(enabled bool) CommandOption {
	return func(c *commandConfig) {
		c.stdin = &enabled
	}
}

// WithTag sets a custom tag for identifying the command.
// This can be used to identify special commands like start commands in custom templates.
func WithTag(tag string) CommandOption {
	return func(c *commandConfig) {
		c.tag = &tag
	}
}

// WithCommandTimeout sets the timeout for the command connection in seconds.
// Using 0 will not limit the command connection time.
// Default is 60 seconds.
func WithCommandTimeout(d time.Duration) CommandOption {
	return func(c *commandConfig) {
		c.timeout = d
	}
}

// WithCommandRequestTimeout sets the timeout for the API request.
func WithCommandRequestTimeout(d time.Duration) CommandOption {
	return func(c *commandConfig) {
		c.requestTimeout = d
	}
}

// OnCommandStdout sets a callback for command stdout output.
// The callback is called with each chunk of stdout data as it arrives.
func OnCommandStdout(handler func(output string)) CommandOption {
	return func(c *commandConfig) {
		c.onStdout = handler
	}
}

// OnCommandStderr sets a callback for command stderr output.
// The callback is called with each chunk of stderr data as it arrives.
func OnCommandStderr(handler func(output string)) CommandOption {
	return func(c *commandConfig) {
		c.onStderr = handler
	}
}

// commandConnectConfig holds configuration for connecting to a command.
type commandConnectConfig struct {
	timeout        time.Duration
	requestTimeout time.Duration
	onStdout       func(output string)
	onStderr       func(output string)
}

// defaultCommandConnectConfig returns the default connect configuration.
func defaultCommandConnectConfig() *commandConnectConfig {
	return &commandConnectConfig{
		timeout: 60 * time.Second,
	}
}

// CommandConnectOption configures command connection.
type CommandConnectOption func(*commandConnectConfig)

// WithConnectTimeout sets the timeout for the command connection.
// Using 0 will not limit the command connection time.
// Default is 60 seconds.
func WithConnectTimeout(d time.Duration) CommandConnectOption {
	return func(c *commandConnectConfig) {
		c.timeout = d
	}
}

// WithConnectRequestTimeout sets the timeout for the API request.
func WithConnectRequestTimeout(d time.Duration) CommandConnectOption {
	return func(c *commandConnectConfig) {
		c.requestTimeout = d
	}
}

// OnConnectStdout sets a callback for stdout output when connecting.
func OnConnectStdout(handler func(output string)) CommandConnectOption {
	return func(c *commandConnectConfig) {
		c.onStdout = handler
	}
}

// OnConnectStderr sets a callback for stderr output when connecting.
func OnConnectStderr(handler func(output string)) CommandConnectOption {
	return func(c *commandConnectConfig) {
		c.onStderr = handler
	}
}

// commandRequestConfig holds configuration for command requests (list, kill, sendStdin).
type commandRequestConfig struct {
	requestTimeout time.Duration
}

// defaultCommandRequestConfig returns the default request configuration.
func defaultCommandRequestConfig() *commandRequestConfig {
	return &commandRequestConfig{}
}

// CommandRequestOption configures command requests.
type CommandRequestOption func(*commandRequestConfig)

// WithCmdRequestTimeout sets the timeout for the API request.
func WithCmdRequestTimeout(d time.Duration) CommandRequestOption {
	return func(c *commandRequestConfig) {
		c.requestTimeout = d
	}
}
