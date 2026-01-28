package e2b

import (
	"context"
	"strings"
	"sync"

	"connectrpc.com/connect"
	processpb "github.com/xerpa-ai/e2b-go/internal/proto/process"
)

// CommandHandle represents a handle to a running command.
// It provides methods for waiting for the command to finish,
// retrieving stdout/stderr, and killing the command.
type CommandHandle struct {
	pid        uint32
	handleKill func() (bool, error)

	mu       sync.RWMutex
	stdout   strings.Builder
	stderr   strings.Builder
	result   *CommandResult
	err      error
	done     chan struct{}
	canceled bool

	onStdout func(string)
	onStderr func(string)
}

// newCommandHandle creates a new CommandHandle for Start responses and starts processing events.
func newCommandHandle(
	pid uint32,
	stream *connect.ServerStreamForClient[processpb.StartResponse],
	handleKill func() (bool, error),
	onStdout func(string),
	onStderr func(string),
) *CommandHandle {
	h := &CommandHandle{
		pid:        pid,
		handleKill: handleKill,
		done:       make(chan struct{}),
		onStdout:   onStdout,
		onStderr:   onStderr,
	}

	// Start background goroutine to process events
	go h.processStartEvents(stream)

	return h
}

// newCommandHandleFromConnect creates a new CommandHandle for Connect responses and starts processing events.
func newCommandHandleFromConnect(
	pid uint32,
	stream *connect.ServerStreamForClient[processpb.ConnectResponse],
	handleKill func() (bool, error),
	onStdout func(string),
	onStderr func(string),
) *CommandHandle {
	h := &CommandHandle{
		pid:        pid,
		handleKill: handleKill,
		done:       make(chan struct{}),
		onStdout:   onStdout,
		onStderr:   onStderr,
	}

	// Start background goroutine to process events
	go h.processConnectEvents(stream)

	return h
}

// processStartEvents reads events from a Start stream and updates internal state.
func (h *CommandHandle) processStartEvents(stream *connect.ServerStreamForClient[processpb.StartResponse]) {
	defer close(h.done)

	for stream.Receive() {
		h.mu.Lock()
		if h.canceled {
			h.mu.Unlock()
			return
		}
		h.mu.Unlock()

		resp := stream.Msg()
		event := resp.GetEvent()
		if event != nil {
			h.handleEvent(event)
		}
	}

	// Check for stream error
	if err := stream.Err(); err != nil {
		h.mu.Lock()
		if h.err == nil {
			h.err = err
		}
		h.mu.Unlock()
	}
}

// processConnectEvents reads events from a Connect stream and updates internal state.
func (h *CommandHandle) processConnectEvents(stream *connect.ServerStreamForClient[processpb.ConnectResponse]) {
	defer close(h.done)

	for stream.Receive() {
		h.mu.Lock()
		if h.canceled {
			h.mu.Unlock()
			return
		}
		h.mu.Unlock()

		resp := stream.Msg()
		event := resp.GetEvent()
		if event != nil {
			h.handleEvent(event)
		}
	}

	// Check for stream error
	if err := stream.Err(); err != nil {
		h.mu.Lock()
		if h.err == nil {
			h.err = err
		}
		h.mu.Unlock()
	}
}

// handleEvent processes a single event from the stream.
func (h *CommandHandle) handleEvent(event *processpb.ProcessEvent) {
	switch e := event.GetEvent().(type) {
	case *processpb.ProcessEvent_Data:
		h.handleDataEvent(e.Data)
	case *processpb.ProcessEvent_End:
		h.handleEndEvent(e.End)
	case *processpb.ProcessEvent_Start:
		// Start event already handled during initialization
	case *processpb.ProcessEvent_Keepalive:
		// Keepalive events are ignored
	}
}

// handleDataEvent processes stdout/stderr data.
func (h *CommandHandle) handleDataEvent(data *processpb.ProcessEvent_DataEvent) {
	// Handle stdout
	if stdout := data.GetStdout(); stdout != nil {
		out := string(stdout)
		h.mu.Lock()
		h.stdout.WriteString(out)
		callback := h.onStdout
		h.mu.Unlock()

		if callback != nil {
			callback(out)
		}
	}

	// Handle stderr
	if stderr := data.GetStderr(); stderr != nil {
		out := string(stderr)
		h.mu.Lock()
		h.stderr.WriteString(out)
		callback := h.onStderr
		h.mu.Unlock()

		if callback != nil {
			callback(out)
		}
	}

	// PTY output is currently ignored
}

// handleEndEvent processes the end event when the command finishes.
func (h *CommandHandle) handleEndEvent(end *processpb.ProcessEvent_EndEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	exitCode := int(end.GetExitCode())
	errorMsg := ""
	if end.Error != nil {
		errorMsg = *end.Error
	}

	h.result = &CommandResult{
		Stdout:   h.stdout.String(),
		Stderr:   h.stderr.String(),
		ExitCode: exitCode,
		Error:    errorMsg,
	}
}

// PID returns the process ID of the command.
func (h *CommandHandle) PID() uint32 {
	return h.pid
}

// Stdout returns the accumulated stdout output.
func (h *CommandHandle) Stdout() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.stdout.String()
}

// Stderr returns the accumulated stderr output.
func (h *CommandHandle) Stderr() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.stderr.String()
}

// appendStdout appends data to the stdout buffer.
// This is used for handling early data received before the start event.
func (h *CommandHandle) appendStdout(data string) {
	h.mu.Lock()
	h.stdout.WriteString(data)
	h.mu.Unlock()
}

// appendStderr appends data to the stderr buffer.
// This is used for handling early data received before the start event.
func (h *CommandHandle) appendStderr(data string) {
	h.mu.Lock()
	h.stderr.WriteString(data)
	h.mu.Unlock()
}

// Error returns the error message from command execution, if any.
// Returns empty string if the command is still running or finished successfully.
func (h *CommandHandle) Error() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.result == nil {
		return ""
	}
	return h.result.Error
}

// ExitCode returns the exit code of the command.
// Returns nil if the command is still running.
func (h *CommandHandle) ExitCode() *int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.result == nil {
		return nil
	}
	exitCode := h.result.ExitCode
	return &exitCode
}

// Wait waits for the command to finish and returns the result.
// If the command exits with a non-zero exit code, it returns a CommandExitError.
func (h *CommandHandle) Wait(ctx context.Context) (*CommandResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-h.done:
		// Command finished
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.err != nil {
		return nil, h.err
	}

	if h.result == nil {
		return nil, ErrNotFound
	}

	if h.result.ExitCode != 0 {
		return nil, &CommandExitError{
			Stdout:       h.result.Stdout,
			Stderr:       h.result.Stderr,
			ExitCode:     h.result.ExitCode,
			ErrorMessage: h.result.Error,
		}
	}

	return h.result, nil
}

// Kill terminates the command.
// It uses SIGKILL signal to kill the command.
// Returns true if the command was killed, false if the command was not found.
func (h *CommandHandle) Kill() (bool, error) {
	return h.handleKill()
}

// Disconnect stops receiving events from the command.
// The command is not killed, but the SDK stops receiving events.
// You can reconnect to the command using Commands.Connect.
func (h *CommandHandle) Disconnect() {
	h.mu.Lock()
	h.canceled = true
	h.mu.Unlock()
}
