package e2b

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// executeRequest represents the request body for code execution.
type executeRequest struct {
	Code      string            `json:"code"`
	ContextID string            `json:"context_id,omitempty"`
	Language  string            `json:"language,omitempty"`
	EnvVars   map[string]string `json:"env_vars,omitempty"`
}

// contextCreateRequest represents the request body for creating a context.
type contextCreateRequest struct {
	Language string `json:"language,omitempty"`
	CWD      string `json:"cwd,omitempty"`
}

// streamResponse represents a single line in the streaming response.
type streamResponse struct {
	Type           string         `json:"type"`
	Text           string         `json:"text,omitempty"`
	Timestamp      int64          `json:"timestamp,omitempty"`
	Name           string         `json:"name,omitempty"`
	Value          string         `json:"value,omitempty"`
	Traceback      string         `json:"traceback,omitempty"`
	ExecutionCount int            `json:"execution_count,omitempty"`
	IsMainResult   bool           `json:"is_main_result,omitempty"`
	HTML           string         `json:"html,omitempty"`
	Markdown       string         `json:"markdown,omitempty"`
	SVG            string         `json:"svg,omitempty"`
	PNG            string         `json:"png,omitempty"`
	JPEG           string         `json:"jpeg,omitempty"`
	PDF            string         `json:"pdf,omitempty"`
	LaTeX          string         `json:"latex,omitempty"`
	JSON           map[string]any `json:"json,omitempty"`
	JavaScript     string         `json:"javascript,omitempty"`
	Data           map[string]any `json:"data,omitempty"`
	Chart          map[string]any `json:"chart,omitempty"`
	Extra          map[string]any `json:"extra,omitempty"`
}

// httpClient wraps the standard http.Client with sandbox-specific functionality.
type httpClient struct {
	client       *http.Client
	baseURL      string
	accessToken  string
	trafficToken string
}

// newHTTPClient creates a new httpClient.
func newHTTPClient(client *http.Client, baseURL, accessToken, trafficToken string) *httpClient {
	if client == nil {
		client = &http.Client{}
	}
	return &httpClient{
		client:       client,
		baseURL:      baseURL,
		accessToken:  accessToken,
		trafficToken: trafficToken,
	}
}

// setHeaders sets common headers for all requests.
func (c *httpClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "e2b-go-sdk/"+Version)
	if c.accessToken != "" {
		req.Header.Set("X-Access-Token", c.accessToken)
	}
	if c.trafficToken != "" {
		req.Header.Set("E2B-Traffic-Access-Token", c.trafficToken)
	}
}

// doRequest performs an HTTP request and returns the response body.
func (c *httpClient) doRequest(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// doStreamRequest performs a streaming HTTP request.
func (c *httpClient) doStreamRequest(
	ctx context.Context,
	path string,
	body any,
	handler func(*streamResponse) error,
) (int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, reqBody)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, formatHTTPError(resp.StatusCode, string(respBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	// Increase buffer size for large responses
	const maxCapacity = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var sr streamResponse
		if err := json.Unmarshal([]byte(line), &sr); err != nil {
			continue
		}

		if err := handler(&sr); err != nil {
			return resp.StatusCode, err
		}
	}

	if err := scanner.Err(); err != nil {
		return resp.StatusCode, fmt.Errorf("error reading stream: %w", err)
	}

	return resp.StatusCode, nil
}

// parseStreamResponse processes a streaming response and updates the execution.
func parseStreamResponse(
	sr *streamResponse,
	execution *Execution,
	cfg *runConfig,
) error {
	switch sr.Type {
	case "result":
		result := &Result{
			Text:         sr.Text,
			HTML:         sr.HTML,
			Markdown:     sr.Markdown,
			SVG:          sr.SVG,
			PNG:          sr.PNG,
			JPEG:         sr.JPEG,
			PDF:          sr.PDF,
			LaTeX:        sr.LaTeX,
			JSON:         sr.JSON,
			JavaScript:   sr.JavaScript,
			Data:         sr.Data,
			IsMainResult: sr.IsMainResult,
			Extra:        sr.Extra,
		}

		// Parse chart if present
		if sr.Chart != nil {
			chart, err := DeserializeChart(sr.Chart)
			if err == nil {
				result.Chart = chart
			}
		}

		execution.Results = append(execution.Results, result)

		if cfg.onResult != nil {
			cfg.onResult(result)
		}

	case "stdout":
		execution.Logs.Stdout = append(execution.Logs.Stdout, sr.Text)

		if cfg.onStdout != nil {
			cfg.onStdout(OutputMessage{
				Line:      sr.Text,
				Timestamp: sr.Timestamp,
				Error:     false,
			})
		}

	case "stderr":
		execution.Logs.Stderr = append(execution.Logs.Stderr, sr.Text)

		if cfg.onStderr != nil {
			cfg.onStderr(OutputMessage{
				Line:      sr.Text,
				Timestamp: sr.Timestamp,
				Error:     true,
			})
		}

	case "error":
		execution.Error = &ExecutionError{
			Name:      sr.Name,
			Value:     sr.Value,
			Traceback: sr.Traceback,
		}

		if cfg.onError != nil {
			cfg.onError(execution.Error)
		}

	case "number_of_executions":
		execution.ExecutionCount = sr.ExecutionCount
	}

	return nil
}
