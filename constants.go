// Package e2b provides a Go SDK for the E2B Code Interpreter.
package e2b

import "time"

const (
	// DefaultTemplate is the default sandbox template.
	DefaultTemplate = "code-interpreter-v1"

	// JupyterPort is the port where the Jupyter server runs.
	JupyterPort = 49999

	// DefaultTimeout is the default timeout for code execution.
	DefaultTimeout = 300 * time.Second

	// DefaultRequestTimeout is the default timeout for HTTP requests.
	DefaultRequestTimeout = 60 * time.Second
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
