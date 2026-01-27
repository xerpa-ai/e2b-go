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
//	// Create a new sandbox (requires E2B_API_KEY environment variable)
//	sandbox, err := e2b.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer sandbox.Close()
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
//	ctx, err := sandbox.CreateContext(context.Background(),
//	    e2b.WithContextLanguage(e2b.LanguagePython))
//
//	// Execute code in the context
//	sandbox.RunCode(context.Background(), "x = 42", e2b.WithContext(ctx))
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
package e2b
