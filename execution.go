package e2b

import (
	"encoding/json"
)

// Execution represents the result of a code cell execution.
type Execution struct {
	// Results contains the output results (main result and display calls).
	Results []*Result `json:"results"`

	// Logs contains stdout and stderr output.
	Logs *Logs `json:"logs"`

	// Error contains error information if an error occurred.
	Error *ExecutionError `json:"error,omitempty"`

	// ExecutionCount is the cell execution count.
	ExecutionCount int `json:"execution_count,omitempty"`
}

// Text returns the text representation of the main result.
func (e *Execution) Text() string {
	for _, r := range e.Results {
		if r.IsMainResult {
			return r.Text
		}
	}
	return ""
}

// MarshalJSON implements json.Marshaler.
func (e *Execution) MarshalJSON() ([]byte, error) {
	type Alias Execution
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	})
}

// ExecutionError represents an error that occurred during code execution.
type ExecutionError struct {
	// Name is the error type name.
	Name string `json:"name"`

	// Value is the error message.
	Value string `json:"value"`

	// Traceback is the full error traceback.
	Traceback string `json:"traceback"`
}

// Error implements the error interface.
func (e *ExecutionError) Error() string {
	return e.Name + ": " + e.Value
}

// Logs represents stdout and stderr output from code execution.
type Logs struct {
	// Stdout contains lines printed to stdout.
	Stdout []string `json:"stdout"`

	// Stderr contains lines printed to stderr.
	Stderr []string `json:"stderr"`
}

// NewLogs creates a new Logs instance with initialized slices.
func NewLogs() *Logs {
	return &Logs{
		Stdout: make([]string, 0),
		Stderr: make([]string, 0),
	}
}

// Result represents the data displayed as a result of executing a cell.
// It can contain multiple formats similar to IPython kernel output.
type Result struct {
	// Text is the plain text representation.
	Text string `json:"text,omitempty"`

	// HTML is the HTML representation.
	HTML string `json:"html,omitempty"`

	// Markdown is the Markdown representation.
	Markdown string `json:"markdown,omitempty"`

	// SVG is the SVG representation.
	SVG string `json:"svg,omitempty"`

	// PNG is the base64-encoded PNG image.
	PNG string `json:"png,omitempty"`

	// JPEG is the base64-encoded JPEG image.
	JPEG string `json:"jpeg,omitempty"`

	// PDF is the PDF representation.
	PDF string `json:"pdf,omitempty"`

	// LaTeX is the LaTeX representation.
	LaTeX string `json:"latex,omitempty"`

	// JSON is the JSON representation.
	JSON map[string]any `json:"json,omitempty"`

	// JavaScript is the JavaScript representation.
	JavaScript string `json:"javascript,omitempty"`

	// Data is structured data (e.g., DataFrame).
	Data map[string]any `json:"data,omitempty"`

	// Chart contains extracted chart data.
	Chart Chart `json:"chart,omitempty"`

	// IsMainResult indicates whether this is the cell result vs display output.
	IsMainResult bool `json:"is_main_result"`

	// Extra contains additional custom data.
	Extra map[string]any `json:"extra,omitempty"`
}

// Formats returns all available formats of the result.
func (r *Result) Formats() []string {
	var formats []string

	if r.Text != "" {
		formats = append(formats, "text")
	}
	if r.HTML != "" {
		formats = append(formats, "html")
	}
	if r.Markdown != "" {
		formats = append(formats, "markdown")
	}
	if r.SVG != "" {
		formats = append(formats, "svg")
	}
	if r.PNG != "" {
		formats = append(formats, "png")
	}
	if r.JPEG != "" {
		formats = append(formats, "jpeg")
	}
	if r.PDF != "" {
		formats = append(formats, "pdf")
	}
	if r.LaTeX != "" {
		formats = append(formats, "latex")
	}
	if r.JSON != nil {
		formats = append(formats, "json")
	}
	if r.JavaScript != "" {
		formats = append(formats, "javascript")
	}
	if r.Data != nil {
		formats = append(formats, "data")
	}
	if r.Chart != nil {
		formats = append(formats, "chart")
	}

	for key := range r.Extra {
		formats = append(formats, key)
	}

	return formats
}

// OutputMessage represents a streaming output message.
type OutputMessage struct {
	// Line is the output line content.
	Line string `json:"line"`

	// Timestamp is the Unix epoch in nanoseconds.
	Timestamp int64 `json:"timestamp"`

	// Error indicates whether this is an error output.
	Error bool `json:"error"`
}

// String returns the line content.
func (o OutputMessage) String() string {
	return o.Line
}
