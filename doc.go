// Package e2b provides a Go SDK for the E2B Code Interpreter.
//
// E2B (https://e2b.dev) provides secure cloud sandboxes for running code.
// This SDK allows you to execute code in various programming languages
// within isolated sandbox environments.
//
// # Getting Started
//
// Create a new sandbox and execute code:
//
//	import "github.com/xerpa-ai/e2b-go"
//
//	// Create a new sandbox with context support (recommended)
//	ctx := context.Background()
//	sandbox, err := e2b.NewWithContext(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer sandbox.CloseWithContext(ctx)
//
//	// Execute Python code
//	execution, err := sandbox.RunCode(context.Background(), "print('Hello, World!')")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access results
//	fmt.Println(execution.Logs.Stdout) // ["Hello, World!"]
//
// # Supported Languages
//
// The SDK supports multiple programming languages:
//
//   - Python (default)
//   - JavaScript
//   - TypeScript
//   - R
//   - Java
//   - Bash
//
// Use the WithLanguage option to specify a language:
//
//	execution, err := sandbox.RunCode(ctx, "console.log('Hello')",
//	    e2b.WithLanguage(e2b.LanguageJavaScript))
//
// # Execution Contexts
//
// Contexts provide isolated state for code execution. Variables and imports
// in one context do not affect other contexts:
//
//	// Create a new context
//	execCtx, err := sandbox.CreateContext(ctx,
//	    e2b.WithContextLanguage(e2b.LanguagePython))
//
//	// Execute code in the context
//	sandbox.RunCode(ctx, "x = 42", e2b.WithContext(execCtx))
//
// # Streaming Output
//
// Use callbacks to receive output in real-time:
//
//	execution, err := sandbox.RunCode(ctx, code,
//	    e2b.OnStdout(func(msg e2b.OutputMessage) {
//	        fmt.Printf("stdout: %s\n", msg.Line)
//	    }),
//	    e2b.OnStderr(func(msg e2b.OutputMessage) {
//	        fmt.Printf("stderr: %s\n", msg.Line)
//	    }),
//	)
//
// # Charts
//
// The SDK can extract chart data from matplotlib and other plotting libraries:
//
//	execution, err := sandbox.RunCode(ctx, `
//	import matplotlib.pyplot as plt
//	plt.plot([1, 2, 3], [1, 4, 9])
//	plt.show()
//	`)
//
//	for _, result := range execution.Results {
//	    if result.Chart != nil {
//	        // Access chart data
//	        fmt.Printf("Chart type: %s\n", result.Chart.ChartType())
//	    }
//	}
//
// # Error Handling
//
// The SDK provides typed errors for common error conditions:
//
//	execution, err := sandbox.RunCode(ctx, code)
//	if errors.Is(err, e2b.ErrTimeout) {
//	    // Handle execution timeout
//	}
//	if errors.Is(err, e2b.ErrNotFound) {
//	    // Handle resource not found
//	}
//
// Execution errors (errors in the executed code) are returned in the
// Execution.Error field, not as Go errors:
//
//	execution, err := sandbox.RunCode(ctx, "1/0")
//	if execution.Error != nil {
//	    fmt.Println(execution.Error.Name)      // "ZeroDivisionError"
//	    fmt.Println(execution.Error.Value)     // "division by zero"
//	    fmt.Println(execution.Error.Traceback) // Full traceback
//	}
//
// # Filesystem Operations
//
// The sandbox provides filesystem access for reading and writing files:
//
//	// Write a file
//	_, err := sandbox.Files.Write(ctx, "/home/user/hello.txt", "Hello, World!")
//
//	// Read a file
//	content, err := sandbox.Files.Read(ctx, "/home/user/hello.txt")
//
//	// List directory contents
//	entries, err := sandbox.Files.List(ctx, "/home/user")
//
// # Command Execution
//
// Run shell commands in the sandbox:
//
//	// Run a command and wait for completion
//	result, err := sandbox.Commands.Run(ctx, "ls -la /home/user")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Stdout)
//
//	// Run a command in the background
//	handle, err := sandbox.Commands.RunBackground(ctx, "sleep 10")
//	// ... do other work ...
//	result, err := handle.Wait(ctx)
//
// # Configuration
//
// The SDK can be configured via options or environment variables:
//
//   - E2B_API_KEY: API key for authentication
//   - E2B_DOMAIN: Base domain (default: e2b.app)
//   - E2B_DEBUG: Enable debug mode (true/false)
//
// Or use functional options:
//
//	sandbox, err := e2b.New(
//	    e2b.WithAPIKey("your-api-key"),
//	    e2b.WithTimeout(10 * time.Minute),
//	    e2b.WithMetadata(map[string]string{"env": "production"}),
//	)
package e2b
