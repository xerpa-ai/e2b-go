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

	// EnvdPort is the port for the envd service.
	EnvdPort = 49983

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
	EnvdDebugFallback = "99.99.99"

	// DebugSandboxID is the sandbox ID used in debug mode.
	DebugSandboxID = "debug_sandbox_id"

	// EnvdVersionDefaultUser is the envd version that supports default user.
	// Below this version, user defaults to "user" if not specified.
	EnvdVersionDefaultUser = "0.4.0"

	// EnvdVersionRecursiveWatch is the envd version that supports recursive watch.
	// Below this version, recursive watch is not supported.
	EnvdVersionRecursiveWatch = "0.1.4"

	// EnvdVersionCommandsStdin is the envd version that supports stdin control.
	// Below this version, stdin is always enabled and cannot be disabled.
	EnvdVersionCommandsStdin = "0.3.0"
)

// Language constants for code execution.
const (
	LanguagePython     = "python"
	LanguageJavaScript = "javascript"
	LanguageTypeScript = "typescript"
	LanguageR          = "r"
	LanguageJava       = "java"
	LanguageBash       = "bash"
	LanguageDeno       = "deno"
)

// Template constants.
const (
	// DefaultTemplateCPU is the default number of CPU cores for templates.
	DefaultTemplateCPU = 2

	// DefaultTemplateMemory is the default memory in MiB for templates.
	DefaultTemplateMemory = 1024

	// MinTemplateCPU is the minimum number of CPU cores allowed.
	MinTemplateCPU = 1

	// MaxTemplateCPU is the maximum number of CPU cores allowed.
	MaxTemplateCPU = 32

	// MinTemplateMemory is the minimum memory in MiB allowed.
	MinTemplateMemory = 128

	// DefaultBaseImage is the default Docker base image for templates.
	DefaultBaseImage = "e2bdev/base"
)
