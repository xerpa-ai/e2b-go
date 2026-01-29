# E2B Code Interpreter Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/xerpa-ai/e2b-go.svg)](https://pkg.go.dev/github.com/xerpa-ai/e2b-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/xerpa-ai/e2b-go)](https://goreportcard.com/report/github.com/xerpa-ai/e2b-go)

A Go SDK for the [E2B](https://e2b.dev) Code Interpreter - secure cloud sandboxes for running code.

## Features

- Execute code in secure cloud sandboxes
- Support for multiple programming languages (Python, JavaScript, TypeScript, R, Java, Bash)
- Streaming output with real-time callbacks
- Isolated execution contexts with persistent state
- **Full filesystem operations** (read, write, list, mkdir, remove, rename, watch)
- Chart data extraction from matplotlib and other plotting libraries
- Functional options pattern for idiomatic Go configuration

## Installation

```bash
go get github.com/xerpa-ai/e2b-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/xerpa-ai/e2b-go"
)

func main() {
    // Create a new sandbox (uses E2B_API_KEY environment variable)
    sandbox, err := e2b.New()
    if err != nil {
        log.Fatal(err)
    }
    defer sandbox.Close()

    // Execute Python code
    execution, err := sandbox.RunCode(context.Background(), "x = 1 + 1; x")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(execution.Text()) // Output: 2
}
```

## Configuration

### API Key

Set your E2B API key via environment variable or option:

```go
// Via environment variable
os.Setenv("E2B_API_KEY", "your-api-key")
sandbox, err := e2b.New()

// Via option
sandbox, err := e2b.New(e2b.WithAPIKey("your-api-key"))
```

### Other Options

```go
sandbox, err := e2b.New(
    e2b.WithAPIKey("your-api-key"),
    e2b.WithTemplate("custom-template"),
    e2b.WithTimeout(60 * time.Second),
    e2b.WithRequestTimeout(30 * time.Second),
)
```

## Multi-Language Support

Execute code in different programming languages:

```go
// Python (default)
execution, _ := sandbox.RunCode(ctx, "print('Hello from Python')")

// JavaScript
execution, _ := sandbox.RunCode(ctx, "console.log('Hello from JS')",
    e2b.WithLanguage(e2b.LanguageJavaScript))

// TypeScript
execution, _ := sandbox.RunCode(ctx, "const msg: string = 'Hello'; console.log(msg)",
    e2b.WithLanguage(e2b.LanguageTypeScript))

// Bash
execution, _ := sandbox.RunCode(ctx, "echo 'Hello from Bash'",
    e2b.WithLanguage(e2b.LanguageBash))
```

## Streaming Output

Receive output in real-time using callbacks:

```go
execution, err := sandbox.RunCode(ctx, code,
    e2b.OnStdout(func(msg e2b.OutputMessage) {
        fmt.Printf("[stdout] %s\n", msg.Line)
    }),
    e2b.OnStderr(func(msg e2b.OutputMessage) {
        fmt.Printf("[stderr] %s\n", msg.Line)
    }),
    e2b.OnResult(func(result *e2b.Result) {
        fmt.Printf("[result] %s\n", result.Text)
    }),
    e2b.OnError(func(err *e2b.ExecutionError) {
        fmt.Printf("[error] %s: %s\n", err.Name, err.Value)
    }),
)
```

## Execution Contexts

Create isolated execution contexts with persistent state:

```go
// Create a new context
execCtx, err := sandbox.CreateContext(ctx,
    e2b.WithContextLanguage(e2b.LanguagePython),
    e2b.WithCWD("/home/user/project"),
)
if err != nil {
    log.Fatal(err)
}

// Execute code in the context - state persists
sandbox.RunCode(ctx, "x = 42", e2b.WithContext(execCtx))
sandbox.RunCode(ctx, "y = x * 2", e2b.WithContext(execCtx))

execution, _ := sandbox.RunCode(ctx, "y", e2b.WithContext(execCtx))
fmt.Println(execution.Text()) // Output: 84

// List all contexts
contexts, _ := sandbox.ListContexts(ctx)

// Restart context (clears state)
sandbox.RestartContext(ctx, execCtx.ID)

// Remove context
sandbox.RemoveContext(ctx, execCtx.ID)
```

## Environment Variables

Pass environment variables to code execution:

```go
execution, err := sandbox.RunCode(ctx, 
    "import os; print(os.environ.get('MY_VAR'))",
    e2b.WithRunEnvVars(map[string]string{
        "MY_VAR": "hello",
    }),
)
```

## Chart Data Extraction

Extract data from matplotlib and other plotting libraries:

```go
code := `
import matplotlib.pyplot as plt
plt.plot([1, 2, 3], [1, 4, 9])
plt.title('Square Numbers')
plt.show()
`

execution, _ := sandbox.RunCode(ctx, code)

for _, result := range execution.Results {
    if result.Chart != nil {
        fmt.Printf("Chart type: %s\n", result.Chart.ChartType())
        fmt.Printf("Chart title: %s\n", result.Chart.ChartTitle())
        
        // Access chart-specific data
        if lineChart, ok := result.Chart.(*e2b.LineChart); ok {
            for _, series := range lineChart.Data {
                fmt.Printf("Series: %s, Points: %d\n", 
                    series.Label, len(series.Points))
            }
        }
    }
}
```

Supported chart types:
- `LineChart`
- `ScatterChart`
- `BarChart`
- `PieChart`
- `BoxAndWhiskerChart`
- `SuperChart` (contains multiple sub-charts)

## Filesystem Operations

The SDK provides full filesystem access to the sandbox via the `Files` field:

### Read and Write Files

```go
// Write a file
info, err := sandbox.Files.Write(ctx, "/home/user/hello.txt", "Hello, World!")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Wrote file: %s\n", info.Path)

// Read a file
content, err := sandbox.Files.Read(ctx, "/home/user/hello.txt")
if err != nil {
    log.Fatal(err)
}
fmt.Println(content) // Output: Hello, World!

// Read as bytes
data, err := sandbox.Files.ReadBytes(ctx, "/home/user/binary.bin")

// Write multiple files at once
infos, err := sandbox.Files.WriteFiles(ctx, []e2b.WriteEntry{
    {Path: "/home/user/file1.txt", Data: "Content 1"},
    {Path: "/home/user/file2.txt", Data: "Content 2"},
})
```

### Directory Operations

```go
// Create a directory (creates parent directories if needed)
created, err := sandbox.Files.MakeDir(ctx, "/home/user/mydir/subdir")
if created {
    fmt.Println("Directory created")
}

// List directory contents
entries, err := sandbox.Files.List(ctx, "/home/user")
for _, entry := range entries {
    fmt.Printf("%s (%s, %d bytes)\n", entry.Name, entry.Type, entry.Size)
}

// List with depth
entries, err := sandbox.Files.List(ctx, "/home/user", e2b.WithDepth(3))
```

### File Information

```go
// Check if file exists
exists, err := sandbox.Files.Exists(ctx, "/home/user/file.txt")
if exists {
    fmt.Println("File exists")
}

// Get file information
info, err := sandbox.Files.GetInfo(ctx, "/home/user/file.txt")
fmt.Printf("Name: %s\n", info.Name)
fmt.Printf("Type: %s\n", info.Type)           // "file" or "dir"
fmt.Printf("Size: %d bytes\n", info.Size)
fmt.Printf("Permissions: %s\n", info.Permissions) // e.g., "rw-r--r--"
fmt.Printf("Owner: %s\n", info.Owner)
fmt.Printf("Modified: %v\n", info.ModifiedTime)
```

### Rename and Remove

```go
// Rename/move a file
info, err := sandbox.Files.Rename(ctx, "/home/user/old.txt", "/home/user/new.txt")

// Remove a file or directory
err := sandbox.Files.Remove(ctx, "/home/user/file.txt")
```

### Watch Directory for Changes

```go
// Watch a directory for changes
handle, err := sandbox.Files.WatchDir(ctx, "/home/user", 
    func(event e2b.FilesystemEvent) {
        fmt.Printf("Event: %s on %s\n", event.Type, event.Name)
    },
    e2b.WithRecursive(true), // Watch subdirectories too
)
if err != nil {
    log.Fatal(err)
}
defer handle.Stop()

// Event types: create, write, remove, rename, chmod

// Alternative: Non-streaming watcher (poll-based)
watcherID, err := sandbox.Files.CreateWatcher(ctx, "/home/user")
defer sandbox.Files.RemoveWatcher(ctx, watcherID)

// Poll for events
events, err := sandbox.Files.GetWatcherEvents(ctx, watcherID)
for _, event := range events {
    fmt.Printf("%s: %s\n", event.Type, event.Name)
}
```

### Filesystem Options

```go
// Set user for operations (affects path resolution and ownership)
content, err := sandbox.Files.Read(ctx, "file.txt", e2b.WithReadUser("root"))

// Set request timeout
info, err := sandbox.Files.Write(ctx, "/path", data,
    e2b.WithWriteRequestTimeout(30*time.Second))
```

## Error Handling

### Go Errors

Handle SDK errors using standard Go error handling:

```go
execution, err := sandbox.RunCode(ctx, code)
if err != nil {
    if errors.Is(err, e2b.ErrTimeout) {
        // Handle execution timeout
    }
    if errors.Is(err, e2b.ErrNotFound) {
        // Handle resource not found
    }
    if errors.Is(err, e2b.ErrSandboxClosed) {
        // Handle closed sandbox
    }
    log.Fatal(err)
}
```

### Execution Errors

Errors in the executed code are returned in the `Execution.Error` field:

```go
execution, err := sandbox.RunCode(ctx, "1 / 0")
if err != nil {
    log.Fatal(err) // SDK error
}

if execution.Error != nil {
    // Code execution error
    fmt.Println(execution.Error.Name)      // "ZeroDivisionError"
    fmt.Println(execution.Error.Value)     // "division by zero"
    fmt.Println(execution.Error.Traceback) // Full traceback
}
```

## Execution Results

Access execution results in multiple formats:

```go
execution, _ := sandbox.RunCode(ctx, code)

// Main result text
fmt.Println(execution.Text())

// Logs
fmt.Println(execution.Logs.Stdout)
fmt.Println(execution.Logs.Stderr)

// All results (including display outputs)
for _, result := range execution.Results {
    // Available formats
    fmt.Println(result.Formats()) // ["text", "html", "png", ...]
    
    // Access specific formats
    if result.Text != "" {
        fmt.Println(result.Text)
    }
    if result.HTML != "" {
        fmt.Println(result.HTML)
    }
    if result.PNG != "" {
        // Base64-encoded PNG image
    }
    if result.Chart != nil {
        // Extracted chart data
    }
}
```

## API Reference

### Sandbox Methods

| Method | Description |
|--------|-------------|
| `New(opts ...Option)` | Create a new sandbox |
| `Connect(id string, opts ...Option)` | Connect to an existing sandbox |
| `RunCode(ctx, code, opts ...RunOption)` | Execute code |
| `CreateContext(ctx, opts ...ContextOption)` | Create execution context |
| `ListContexts(ctx)` | List all contexts |
| `RemoveContext(ctx, contextID)` | Remove a context |
| `RestartContext(ctx, contextID)` | Restart a context |
| `Close()` | Close the sandbox |

### Filesystem Methods (sandbox.Files)

| Method | Description |
|--------|-------------|
| `Read(ctx, path, opts...)` | Read file content as string |
| `ReadBytes(ctx, path, opts...)` | Read file content as bytes |
| `Write(ctx, path, data, opts...)` | Write content to a file |
| `WriteFiles(ctx, files, opts...)` | Write multiple files |
| `List(ctx, path, opts...)` | List directory contents |
| `MakeDir(ctx, path, opts...)` | Create a directory |
| `Remove(ctx, path, opts...)` | Remove a file or directory |
| `Rename(ctx, oldPath, newPath, opts...)` | Rename/move a file |
| `Exists(ctx, path, opts...)` | Check if path exists |
| `GetInfo(ctx, path, opts...)` | Get file/directory metadata |
| `WatchDir(ctx, path, callback, opts...)` | Watch directory for changes |
| `CreateWatcher(ctx, path, opts...)` | Create a poll-based watcher |
| `GetWatcherEvents(ctx, watcherID, opts...)` | Get events from watcher |
| `RemoveWatcher(ctx, watcherID, opts...)` | Remove a watcher |

### Options

#### Sandbox Options
- `WithAPIKey(key)` - Set API key
- `WithTemplate(template)` - Set sandbox template
- `WithTimeout(duration)` - Set default execution timeout
- `WithRequestTimeout(duration)` - Set HTTP request timeout
- `WithHTTPClient(client)` - Set custom HTTP client
- `WithDebug(bool)` - Enable debug mode

#### Run Options
- `WithLanguage(lang)` - Set programming language
- `WithContext(ctx)` - Use specific execution context
- `WithRunEnvVars(envs)` - Set environment variables
- `WithRunTimeout(duration)` - Set execution timeout
- `OnStdout(handler)` - Callback for stdout
- `OnStderr(handler)` - Callback for stderr
- `OnResult(handler)` - Callback for results
- `OnError(handler)` - Callback for errors

#### Context Options
- `WithContextLanguage(lang)` - Set context language
- `WithCWD(path)` - Set working directory
- `WithContextRequestTimeout(duration)` - Set request timeout

#### Filesystem Options
- `WithUser(user)` - Set user for operations
- `WithFilesystemRequestTimeout(duration)` - Set request timeout
- `WithDepth(depth)` - Set directory listing depth
- `WithRecursive(bool)` - Enable recursive directory watching
- `WithWatchTimeout(ms)` - Set watch timeout in milliseconds
- `OnWatchExit(handler)` - Callback when watch stops

## MCP Integration

The SDK supports MCP (Model Context Protocol) for connecting AI models to tools:

```go
// Create sandbox with MCP configuration
sandbox, err := e2b.New(
    e2b.WithAPIKey("your-api-key"),
    e2b.WithMcp(map[string]any{
        "browserbase": map[string]any{
            "apiKey": os.Getenv("BROWSERBASE_API_KEY"),
        },
    }),
)

// Get MCP gateway URL and token for client connections
mcpUrl := sandbox.GetMcpUrl()
token, err := sandbox.GetMcpToken(ctx)

// Connect your MCP client to mcpUrl with Authorization: Bearer {token}
```

## Feature Parity with Official SDKs

This Go SDK provides feature parity with the official Python and JavaScript SDKs for:
- Core sandbox management (create, connect, kill, list, pause/resume)
- Filesystem operations
- Command execution and PTY
- Code execution with streaming
- Chart data extraction
- MCP configuration and integration
- Template building (with extended programmatic builder API)

### Not Implemented

The following features from the official E2B ecosystem are not yet implemented:

- **Desktop SDK** - The official `@e2b/desktop` package provides browser/desktop automation (mouse, keyboard, screenshots, video streaming). This is a separate product that may be considered for a future `e2b-desktop-go` module if there is demand.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Links

- [E2B Documentation](https://e2b.dev/docs)
- [E2B Website](https://e2b.dev)
- [Go Package Documentation](https://pkg.go.dev/github.com/xerpa-ai/e2b-go)
