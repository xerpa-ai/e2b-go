// Package e2b provides a Go SDK for the E2B Code Interpreter.
package e2b

import "time"

const (
	// Version is the SDK version.
	Version = "0.1.0"

	// DefaultTemplate is the default sandbox template.
	DefaultTemplate = "base"

	// DefaultDomain is the default E2B domain.
	DefaultDomain = "e2b.app"

	// JupyterPort is the port where the Jupyter server runs.
	JupyterPort = 49999

	// DefaultSandboxTimeout is the default timeout for sandbox lifetime.
	// After this timeout, the sandbox will be automatically killed.
	DefaultSandboxTimeout = 300 * time.Second // 5 minutes

	// DefaultCodeExecutionTimeout is the default timeout for code execution.
	DefaultCodeExecutionTimeout = 60 * time.Second // 60 seconds

	// DefaultRequestTimeout is the default timeout for HTTP requests.
	DefaultRequestTimeout = 60 * time.Second

	// KeepalivePingHeader is the header for keepalive ping interval.
	KeepalivePingHeader = "Keepalive-Ping-Interval"

	// KeepalivePingIntervalSec is the keepalive ping interval in seconds.
	KeepalivePingIntervalSec = 50

	// EnvdDebugFallback is the envd version used in debug mode.
	EnvdDebugFallback = "0.0.1"

	// DebugSandboxID is the sandbox ID used in debug mode.
	DebugSandboxID = "debug_sandbox_id"
)

// Language constants for code execution.
const (
	LanguagePython     = "python"
	LanguageJavaScript = "javascript"
	LanguageTypeScript = "typescript"
	LanguageR          = "r"
	LanguageJava       = "java"
	LanguageBash       = "bash"
)
