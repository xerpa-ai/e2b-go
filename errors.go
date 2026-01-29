package e2b

import (
	"errors"
	"fmt"
)

// Sentinel errors for common error conditions.
var (
	// ErrTimeout indicates that the code execution timed out.
	ErrTimeout = errors.New("e2b: execution timeout")

	// ErrRequestTimeout indicates that the HTTP request timed out.
	ErrRequestTimeout = errors.New("e2b: request timeout")

	// ErrNotFound indicates that a resource was not found.
	ErrNotFound = errors.New("e2b: resource not found")

	// ErrInvalidArgument indicates an invalid argument was provided.
	ErrInvalidArgument = errors.New("e2b: invalid argument")

	// ErrSandboxClosed indicates the sandbox has been closed.
	ErrSandboxClosed = errors.New("e2b: sandbox is closed")

	// ErrRateLimit indicates that the rate limit has been exceeded.
	ErrRateLimit = errors.New("e2b: rate limit exceeded")
)

// SandboxError represents an error returned by the sandbox API.
type SandboxError struct {
	// StatusCode is the HTTP status code.
	StatusCode int

	// Message is the error message.
	Message string

	// Err is the underlying error, if any.
	Err error
}

// Error implements the error interface.
func (e *SandboxError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("sandbox error status %d, %s: %v", e.StatusCode, e.Message, e.Err)
	}
	return fmt.Sprintf("sandbox error status %d, %s", e.StatusCode, e.Message)
}

// Unwrap returns the underlying error.
func (e *SandboxError) Unwrap() error {
	return e.Err
}

// Is checks if the error matches the target.
func (e *SandboxError) Is(target error) bool {
	if target == ErrNotFound && e.StatusCode == 404 {
		return true
	}
	if target == ErrTimeout && e.StatusCode == 502 {
		return true
	}
	return false
}

// NewSandboxError creates a new SandboxError.
func NewSandboxError(statusCode int, message string) *SandboxError {
	return &SandboxError{
		StatusCode: statusCode,
		Message:    message,
	}
}

// TimeoutError represents a timeout error with additional context.
type TimeoutError struct {
	// Type indicates whether it's an execution or request timeout.
	Type string

	// Duration is the timeout duration that was exceeded.
	Duration string
}

// Error implements the error interface.
func (e *TimeoutError) Error() string {
	if e.Duration != "" {
		return fmt.Sprintf("%s timeout exceeded after %s", e.Type, e.Duration)
	}
	return fmt.Sprintf("%s timeout exceeded", e.Type)
}

// Is checks if the error matches the target.
func (e *TimeoutError) Is(target error) bool {
	switch e.Type {
	case "execution":
		return target == ErrTimeout
	case "request":
		return target == ErrRequestTimeout
	default:
		return false
	}
}

// NewExecutionTimeoutError creates a new execution timeout error.
func NewExecutionTimeoutError() *TimeoutError {
	return &TimeoutError{
		Type: "execution",
	}
}

// NewRequestTimeoutError creates a new request timeout error.
func NewRequestTimeoutError() *TimeoutError {
	return &TimeoutError{
		Type: "request",
	}
}

// formatHTTPError converts an HTTP response to an appropriate error.
func formatHTTPError(statusCode int, body string) error {
	switch statusCode {
	case 404:
		return &SandboxError{
			StatusCode: statusCode,
			Message:    body,
			Err:        ErrNotFound,
		}
	case 502:
		return &SandboxError{
			StatusCode: statusCode,
			Message:    fmt.Sprintf("%s: This error is likely due to sandbox timeout. You can modify the sandbox timeout by passing a timeout option when starting the sandbox.", body),
			Err:        ErrTimeout,
		}
	default:
		return &SandboxError{
			StatusCode: statusCode,
			Message:    body,
		}
	}
}
