package e2b

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewSandbox(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{
			name:    "without API key",
			opts:    []Option{},
			wantErr: true,
		},
		{
			name:    "with API key",
			opts:    []Option{WithAPIKey("test-api-key")},
			wantErr: false,
		},
		{
			name: "with custom timeout",
			opts: []Option{
				WithAPIKey("test-api-key"),
				WithTimeout(10 * time.Second),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sandbox, err := New(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if sandbox != nil {
				sandbox.Close()
			}
		})
	}
}

func TestSandboxClose(t *testing.T) {
	sandbox, err := New(WithAPIKey("test-api-key"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := sandbox.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if !sandbox.IsClosed() {
		t.Error("IsClosed() should return true after Close()")
	}

	// Closing again should not error
	if err := sandbox.Close(); err != nil {
		t.Errorf("Close() second call error = %v", err)
	}
}

func TestSandboxRunCodeClosed(t *testing.T) {
	sandbox, err := New(WithAPIKey("test-api-key"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	sandbox.Close()

	_, err = sandbox.RunCode(context.Background(), "x = 1")
	if err != ErrSandboxClosed {
		t.Errorf("RunCode() error = %v, want %v", err, ErrSandboxClosed)
	}
}

func TestExecutionText(t *testing.T) {
	tests := []struct {
		name      string
		execution *Execution
		want      string
	}{
		{
			name: "with main result",
			execution: &Execution{
				Results: []*Result{
					{Text: "display output", IsMainResult: false},
					{Text: "main result", IsMainResult: true},
				},
			},
			want: "main result",
		},
		{
			name: "without main result",
			execution: &Execution{
				Results: []*Result{
					{Text: "display output", IsMainResult: false},
				},
			},
			want: "",
		},
		{
			name: "empty results",
			execution: &Execution{
				Results: []*Result{},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.execution.Text(); got != tt.want {
				t.Errorf("Execution.Text() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResultFormats(t *testing.T) {
	result := &Result{
		Text:  "text",
		HTML:  "<p>html</p>",
		PNG:   "base64png",
		JSON:  map[string]any{"key": "value"},
		Extra: map[string]any{"custom": "data"},
	}

	formats := result.Formats()

	expected := []string{"text", "html", "png", "json", "custom"}
	if len(formats) != len(expected) {
		t.Errorf("Formats() returned %d formats, want %d", len(formats), len(expected))
	}

	formatMap := make(map[string]bool)
	for _, f := range formats {
		formatMap[f] = true
	}

	for _, e := range expected {
		if !formatMap[e] {
			t.Errorf("Formats() missing expected format: %s", e)
		}
	}
}

func TestOutputMessage(t *testing.T) {
	msg := OutputMessage{
		Line:      "test output",
		Timestamp: 1234567890,
		Error:     false,
	}

	if msg.String() != "test output" {
		t.Errorf("String() = %v, want %v", msg.String(), "test output")
	}
}

func TestExecutionError(t *testing.T) {
	execErr := &ExecutionError{
		Name:      "ZeroDivisionError",
		Value:     "division by zero",
		Traceback: "Traceback...",
	}

	expected := "ZeroDivisionError: division by zero"
	if execErr.Error() != expected {
		t.Errorf("Error() = %v, want %v", execErr.Error(), expected)
	}
}

func TestSandboxError(t *testing.T) {
	tests := []struct {
		name       string
		err        *SandboxError
		target     error
		wantIs     bool
		wantString string
	}{
		{
			name: "not found error",
			err: &SandboxError{
				StatusCode: 404,
				Message:    "context not found",
			},
			target:     ErrNotFound,
			wantIs:     true,
			wantString: "sandbox error status 404, context not found",
		},
		{
			name: "timeout error",
			err: &SandboxError{
				StatusCode: 502,
				Message:    "execution timeout",
			},
			target:     ErrTimeout,
			wantIs:     true,
			wantString: "sandbox error status 502, execution timeout",
		},
		{
			name: "generic error",
			err: &SandboxError{
				StatusCode: 500,
				Message:    "internal error",
			},
			target:     ErrNotFound,
			wantIs:     false,
			wantString: "sandbox error status 500, internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Is(tt.target); got != tt.wantIs {
				t.Errorf("Is() = %v, want %v", got, tt.wantIs)
			}
			if got := tt.err.Error(); got != tt.wantString {
				t.Errorf("Error() = %v, want %v", got, tt.wantString)
			}
		})
	}
}

func TestRunOptions(t *testing.T) {
	cfg := defaultRunConfig()

	// Test all options
	WithLanguage("python")(cfg)
	if cfg.language != "python" {
		t.Errorf("WithLanguage() language = %v, want python", cfg.language)
	}

	ctx := &Context{ID: "test-ctx"}
	WithContext(ctx)(cfg)
	if cfg.context != ctx {
		t.Error("WithContext() context not set correctly")
	}

	envVars := map[string]string{"KEY": "VALUE"}
	WithRunEnvVars(envVars)(cfg)
	if cfg.envVars["KEY"] != "VALUE" {
		t.Error("WithRunEnvVars() envVars not set correctly")
	}

	WithRunTimeout(5 * time.Second)(cfg)
	if cfg.timeout == nil || *cfg.timeout != 5*time.Second {
		t.Errorf("WithRunTimeout() timeout = %v, want 5s", cfg.timeout)
	}

	stdoutCalled := false
	OnStdout(func(msg OutputMessage) {
		stdoutCalled = true
	})(cfg)
	cfg.onStdout(OutputMessage{})
	if !stdoutCalled {
		t.Error("OnStdout() handler not set correctly")
	}
}

func TestContextOptions(t *testing.T) {
	cfg := defaultContextConfig()

	WithContextLanguage("javascript")(cfg)
	if cfg.language != "javascript" {
		t.Errorf("WithContextLanguage() language = %v, want javascript", cfg.language)
	}

	WithCWD("/home/user")(cfg)
	if cfg.cwd != "/home/user" {
		t.Errorf("WithCWD() cwd = %v, want /home/user", cfg.cwd)
	}

	WithContextRequestTimeout(10 * time.Second)(cfg)
	if cfg.requestTimeout != 10*time.Second {
		t.Errorf("WithContextRequestTimeout() timeout = %v, want 10s", cfg.requestTimeout)
	}
}

func TestParseStreamResponse(t *testing.T) {
	execution := &Execution{
		Results: []*Result{},
		Logs:    NewLogs(),
	}
	cfg := defaultRunConfig()

	// Test stdout
	stdoutResp := &streamResponse{
		Type:      "stdout",
		Text:      "hello",
		Timestamp: 123,
	}
	parseStreamResponse(stdoutResp, execution, cfg)
	if len(execution.Logs.Stdout) != 1 || execution.Logs.Stdout[0] != "hello" {
		t.Errorf("stdout not parsed correctly: %v", execution.Logs.Stdout)
	}

	// Test stderr
	stderrResp := &streamResponse{
		Type:      "stderr",
		Text:      "error msg",
		Timestamp: 124,
	}
	parseStreamResponse(stderrResp, execution, cfg)
	if len(execution.Logs.Stderr) != 1 || execution.Logs.Stderr[0] != "error msg" {
		t.Errorf("stderr not parsed correctly: %v", execution.Logs.Stderr)
	}

	// Test result
	resultResp := &streamResponse{
		Type:         "result",
		Text:         "42",
		IsMainResult: true,
	}
	parseStreamResponse(resultResp, execution, cfg)
	if len(execution.Results) != 1 || execution.Results[0].Text != "42" {
		t.Errorf("result not parsed correctly: %v", execution.Results)
	}

	// Test error
	errorResp := &streamResponse{
		Type:      "error",
		Name:      "ValueError",
		Value:     "invalid value",
		Traceback: "traceback...",
	}
	parseStreamResponse(errorResp, execution, cfg)
	if execution.Error == nil || execution.Error.Name != "ValueError" {
		t.Errorf("error not parsed correctly: %v", execution.Error)
	}

	// Test execution count
	countResp := &streamResponse{
		Type:           "number_of_executions",
		ExecutionCount: 5,
	}
	parseStreamResponse(countResp, execution, cfg)
	if execution.ExecutionCount != 5 {
		t.Errorf("execution count not parsed correctly: %d", execution.ExecutionCount)
	}
}

func TestHTTPClient(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Content-Type header not set")
		}
		if r.Header.Get("X-Access-Token") != "test-token" {
			t.Error("X-Access-Token header not set")
		}

		// Return test response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := newHTTPClient(nil, server.URL, "test-token", "")

	body, statusCode, err := client.doRequest(context.Background(), http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("doRequest() error = %v", err)
	}

	if statusCode != http.StatusOK {
		t.Errorf("statusCode = %d, want 200", statusCode)
	}

	var resp map[string]string
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("response status = %v, want ok", resp["status"])
	}
}

func TestConstants(t *testing.T) {
	if DefaultTemplate != "base" {
		t.Errorf("DefaultTemplate = %v, want base", DefaultTemplate)
	}

	if JupyterPort != 49999 {
		t.Errorf("JupyterPort = %d, want 49999", JupyterPort)
	}

	if DefaultSandboxTimeout != 300*time.Second {
		t.Errorf("DefaultSandboxTimeout = %v, want 300s", DefaultSandboxTimeout)
	}

	if DefaultCodeExecutionTimeout != 60*time.Second {
		t.Errorf("DefaultCodeExecutionTimeout = %v, want 60s", DefaultCodeExecutionTimeout)
	}
}

func TestLanguageConstants(t *testing.T) {
	languages := []string{
		LanguagePython,
		LanguageJavaScript,
		LanguageTypeScript,
		LanguageR,
		LanguageJava,
		LanguageBash,
	}

	expected := []string{"python", "javascript", "typescript", "r", "java", "bash"}

	for i, lang := range languages {
		if lang != expected[i] {
			t.Errorf("Language constant = %v, want %v", lang, expected[i])
		}
	}
}
