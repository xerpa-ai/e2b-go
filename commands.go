package e2b

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	processpb "github.com/xerpa-ai/e2b-go/internal/proto/process"
	"github.com/xerpa-ai/e2b-go/internal/proto/process/processpbconnect"
	"golang.org/x/mod/semver"
)

// Commands provides methods for executing commands in the sandbox.
type Commands struct {
	rpcClient    processpbconnect.ProcessClient
	httpClient   *http.Client
	envdBaseURL  string
	accessToken  string
	trafficToken string
	sandbox      *Sandbox
	envdVersion  string
}

// newCommands creates a new Commands instance.
func newCommands(sandbox *Sandbox) *Commands {
	envdBaseURL := sandbox.getEnvdURL()

	httpClient := sandbox.config.httpClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: sandbox.config.requestTimeout,
		}
	}

	rpcClient := processpbconnect.NewProcessClient(
		httpClient,
		envdBaseURL,
		connect.WithGRPCWeb(),
	)

	return &Commands{
		rpcClient:    rpcClient,
		httpClient:   httpClient,
		envdBaseURL:  envdBaseURL,
		accessToken:  sandbox.accessToken,
		trafficToken: sandbox.TrafficAccessToken,
		sandbox:      sandbox,
		envdVersion:  sandbox.envdVersion,
	}
}

// setRPCHeaders sets authentication headers on the Connect request.
func (c *Commands) setRPCHeaders(req connect.AnyRequest) {
	c.setRPCHeadersWithUser(req, "")
}

// setRPCHeadersWithUser sets authentication headers including user-based Basic auth.
func (c *Commands) setRPCHeadersWithUser(req connect.AnyRequest, user string) {
	req.Header().Set("User-Agent", "e2b-go-sdk/"+Version)
	if c.accessToken != "" {
		req.Header().Set(headerAccessToken, c.accessToken)
	}
	if c.trafficToken != "" {
		req.Header().Set(headerTrafficToken, c.trafficToken)
	}

	// Set Authorization header with Basic auth (username:)
	// If user is not specified and envd version < 0.4.0, default to "user"
	effectiveUser := user
	if effectiveUser == "" && c.compareVersion(EnvdVersionDefaultUser) < 0 {
		effectiveUser = "user"
	}

	if effectiveUser != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(effectiveUser + ":"))
		req.Header().Set("Authorization", "Basic "+encoded)
	}
}

// compareVersion compares the envd version with the given version.
// Returns -1 if envdVersion < version, 0 if equal, 1 if envdVersion > version.
func (c *Commands) compareVersion(version string) int {
	// Add "v" prefix for semver comparison if not present
	v1 := c.envdVersion
	if v1 != "" && v1[0] != 'v' {
		v1 = "v" + v1
	}
	v2 := version
	if v2 != "" && v2[0] != 'v' {
		v2 = "v" + v2
	}
	return semver.Compare(v1, v2)
}

// setStreamingHeaders sets headers for streaming requests including keepalive.
func (c *Commands) setStreamingHeaders(req connect.AnyRequest) {
	c.setStreamingHeadersWithUser(req, "")
}

// setStreamingHeadersWithUser sets headers for streaming requests with user-based auth.
func (c *Commands) setStreamingHeadersWithUser(req connect.AnyRequest, user string) {
	c.setRPCHeadersWithUser(req, user)
	req.Header().Set(KeepalivePingHeader, fmt.Sprintf("%d", KeepalivePingIntervalSec))
}

// applyTimeout applies the appropriate timeout to the context.
func (c *Commands) applyTimeout(ctx context.Context, configTimeout time.Duration) (context.Context, context.CancelFunc) {
	timeout := configTimeout
	if timeout == 0 {
		timeout = c.sandbox.config.requestTimeout
	}
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return ctx, func() {}
}

// List returns all running commands and PTY sessions.
//
// Example:
//
//	processes, err := sandbox.Commands.List(ctx)
//	for _, p := range processes {
//	    fmt.Printf("PID: %d, Command: %s\n", p.PID, p.Cmd)
//	}
func (c *Commands) List(ctx context.Context, opts ...CommandRequestOption) ([]*ProcessInfo, error) {
	cfg := defaultCommandRequestConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := c.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	req := connect.NewRequest(&processpb.ListRequest{})
	c.setRPCHeaders(req)

	resp, err := c.rpcClient.List(ctx, req)
	if err != nil {
		return nil, c.wrapRPCError(ctx, err)
	}

	processes := make([]*ProcessInfo, 0, len(resp.Msg.GetProcesses()))
	for _, p := range resp.Msg.GetProcesses() {
		processes = append(processes, processInfoFromProto(p))
	}

	return processes, nil
}

// Kill terminates a running command by its process ID.
// It uses SIGKILL signal to kill the command.
//
// Returns true if the command was killed, false if the command was not found.
//
// Example:
//
//	killed, err := sandbox.Commands.Kill(ctx, pid)
//	if killed {
//	    fmt.Println("Command killed")
//	}
func (c *Commands) Kill(ctx context.Context, pid uint32, opts ...CommandRequestOption) (bool, error) {
	cfg := defaultCommandRequestConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := c.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	req := connect.NewRequest(&processpb.SendSignalRequest{
		Process: &processpb.ProcessSelector{
			Selector: &processpb.ProcessSelector_Pid{
				Pid: pid,
			},
		},
		Signal: processpb.Signal_SIGNAL_SIGKILL,
	})
	c.setRPCHeaders(req)

	_, err := c.rpcClient.SendSignal(ctx, req)
	if err != nil {
		// Check for not found error
		if connectErr, ok := err.(*connect.Error); ok {
			if connectErr.Code() == connect.CodeNotFound {
				return false, nil
			}
		}
		return false, c.wrapRPCError(ctx, err)
	}

	return true, nil
}

// SendStdin sends data to the stdin of a running command.
//
// Example:
//
//	err := sandbox.Commands.SendStdin(ctx, pid, "input data\n")
func (c *Commands) SendStdin(ctx context.Context, pid uint32, data string, opts ...CommandRequestOption) error {
	cfg := defaultCommandRequestConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := c.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	req := connect.NewRequest(&processpb.SendInputRequest{
		Process: &processpb.ProcessSelector{
			Selector: &processpb.ProcessSelector_Pid{
				Pid: pid,
			},
		},
		Input: &processpb.ProcessInput{
			Input: &processpb.ProcessInput_Stdin{
				Stdin: []byte(data),
			},
		},
	})
	c.setRPCHeaders(req)

	_, err := c.rpcClient.SendInput(ctx, req)
	if err != nil {
		return c.wrapRPCError(ctx, err)
	}

	return nil
}

// Run executes a command and waits for it to complete.
// Returns the command result with stdout, stderr, and exit code.
//
// If the command exits with a non-zero exit code, it returns a CommandExitError.
//
// Example:
//
//	result, err := sandbox.Commands.Run(ctx, "ls -la")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Stdout)
func (c *Commands) Run(ctx context.Context, cmd string, opts ...CommandOption) (*CommandResult, error) {
	handle, err := c.start(ctx, cmd, opts...)
	if err != nil {
		return nil, err
	}

	return handle.Wait(ctx)
}

// RunBackground executes a command in the background and returns a handle to interact with it.
// The command continues running and you can use the handle to wait for completion,
// stream output, or kill the process.
//
// Example:
//
//	handle, err := sandbox.Commands.RunBackground(ctx, "sleep 10",
//	    OnCommandStdout(func(output string) {
//	        fmt.Print(output)
//	    }),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Do other work...
//	result, err := handle.Wait(ctx)
func (c *Commands) RunBackground(ctx context.Context, cmd string, opts ...CommandOption) (*CommandHandle, error) {
	return c.start(ctx, cmd, opts...)
}

// start is the internal method that starts a command and returns a handle.
func (c *Commands) start(ctx context.Context, cmd string, opts ...CommandOption) (*CommandHandle, error) {
	cfg := defaultCommandConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Build the process config
	// Python SDK uses: /bin/bash -l -c cmd
	processConfig := &processpb.ProcessConfig{
		Cmd:  "/bin/bash",
		Args: []string{"-l", "-c", cmd},
		Envs: cfg.envs,
	}

	// Set cwd if provided
	if cfg.cwd != "" {
		processConfig.Cwd = &cfg.cwd
	}

	req := connect.NewRequest(&processpb.StartRequest{
		Process: processConfig,
		Stdin:   &cfg.stdin,
		Tag:     cfg.tag,
	})
	c.setStreamingHeadersWithUser(req, cfg.user)

	// Create context with timeout for the stream
	var streamCtx context.Context
	var streamCancel context.CancelFunc
	if cfg.timeout > 0 {
		streamCtx, streamCancel = context.WithTimeout(ctx, cfg.timeout)
	} else {
		streamCtx, streamCancel = context.WithCancel(ctx)
	}

	stream, err := c.rpcClient.Start(streamCtx, req)
	if err != nil {
		streamCancel()
		return nil, c.wrapRPCError(ctx, err)
	}

	// Read events until we get a StartEvent
	// The server may send data events before the start event
	var pid uint32
	var earlyStdout []byte
	var earlyStderr []byte
	eventCount := 0
	maxEvents := 100 // Safety limit

	for eventCount < maxEvents {
		if !stream.Receive() {
			streamCancel()
			streamErr := stream.Err()

			// If we collected some data but stream ended without Start/End events,
			// the command likely completed - return the collected output
			if eventCount > 0 && (len(earlyStdout) > 0 || len(earlyStderr) > 0) {
				result := &CommandResult{
					Stdout:   string(earlyStdout),
					Stderr:   string(earlyStderr),
					ExitCode: 0,
				}

				handle := &CommandHandle{
					pid:    0,
					done:   make(chan struct{}),
					result: result,
				}
				close(handle.done)
				return handle, nil
			}

			if streamErr != nil {
				return nil, fmt.Errorf("failed to start process: stream error after %d events: %w", eventCount, c.wrapRPCError(ctx, streamErr))
			}
			return nil, fmt.Errorf("failed to start process: stream ended after %d events, no output received", eventCount)
		}
		eventCount++

		resp := stream.Msg()
		event := resp.GetEvent()
		if event == nil {
			continue
		}

		// Check for start event
		if startEvent := event.GetStart(); startEvent != nil {
			pid = startEvent.GetPid()
			break
		}

		// Collect any early data events
		if dataEvent := event.GetData(); dataEvent != nil {
			if stdout := dataEvent.GetStdout(); stdout != nil {
				earlyStdout = append(earlyStdout, stdout...)
			}
			if stderr := dataEvent.GetStderr(); stderr != nil {
				earlyStderr = append(earlyStderr, stderr...)
			}
			continue
		}

		// If we get an end event before start, extract result
		if endEvent := event.GetEnd(); endEvent != nil {
			streamCancel()

			exitCode := int(endEvent.GetExitCode())
			errorMsg := ""
			if endEvent.Error != nil {
				errorMsg = *endEvent.Error
			}

			result := &CommandResult{
				Stdout:   string(earlyStdout),
				Stderr:   string(earlyStderr),
				ExitCode: exitCode,
				Error:    errorMsg,
			}

			if exitCode != 0 {
				return nil, &CommandExitError{
					Stdout:       result.Stdout,
					Stderr:       result.Stderr,
					ExitCode:     exitCode,
					ErrorMessage: errorMsg,
				}
			}

			// Create a dummy handle that is already done
			handle := &CommandHandle{
				pid:    0,
				done:   make(chan struct{}),
				result: result,
			}
			close(handle.done)
			return handle, nil
		}

		// Keepalive events are ignored
	}

	if eventCount >= maxEvents {
		streamCancel()
		return nil, fmt.Errorf("failed to start process: received %d events but no start event", eventCount)
	}

	// Create the handle with a kill function that cancels the stream
	handle := newCommandHandle(
		pid,
		stream,
		func() (bool, error) {
			streamCancel()
			return c.Kill(context.Background(), pid)
		},
		cfg.onStdout,
		cfg.onStderr,
	)

	// Process any early data that was received before the start event
	if len(earlyStdout) > 0 {
		handle.appendStdout(string(earlyStdout))
		if cfg.onStdout != nil {
			cfg.onStdout(string(earlyStdout))
		}
	}
	if len(earlyStderr) > 0 {
		handle.appendStderr(string(earlyStderr))
		if cfg.onStderr != nil {
			cfg.onStderr(string(earlyStderr))
		}
	}

	return handle, nil
}

// Connect connects to a running command and returns a handle to interact with it.
// You can use the handle to wait for the command to finish and get execution results.
//
// Example:
//
//	handle, err := sandbox.Commands.Connect(ctx, pid,
//	    OnConnectStdout(func(output string) {
//	        fmt.Print(output)
//	    }),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	result, err := handle.Wait(ctx)
func (c *Commands) Connect(ctx context.Context, pid uint32, opts ...CommandConnectOption) (*CommandHandle, error) {
	cfg := defaultCommandConnectConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	req := connect.NewRequest(&processpb.ConnectRequest{
		Process: &processpb.ProcessSelector{
			Selector: &processpb.ProcessSelector_Pid{
				Pid: pid,
			},
		},
	})
	c.setStreamingHeaders(req)

	// Create context with timeout for the stream
	var streamCtx context.Context
	var streamCancel context.CancelFunc
	if cfg.timeout > 0 {
		streamCtx, streamCancel = context.WithTimeout(ctx, cfg.timeout)
	} else {
		streamCtx, streamCancel = context.WithCancel(ctx)
	}

	stream, err := c.rpcClient.Connect(streamCtx, req)
	if err != nil {
		streamCancel()
		return nil, c.wrapRPCError(ctx, err)
	}

	// Read the first event which should be a StartEvent
	if !stream.Receive() {
		streamCancel()
		if err := stream.Err(); err != nil {
			return nil, c.wrapRPCError(ctx, err)
		}
		return nil, fmt.Errorf("failed to connect to process: no start event received")
	}

	resp := stream.Msg()
	event := resp.GetEvent()
	if event == nil {
		streamCancel()
		return nil, fmt.Errorf("failed to connect to process: expected event, got nil")
	}

	startEvent := event.GetStart()
	if startEvent == nil {
		streamCancel()
		return nil, fmt.Errorf("failed to connect to process: expected start event, got %T", event.GetEvent())
	}

	// Create the handle
	handle := newCommandHandleFromConnect(
		startEvent.GetPid(),
		stream,
		func() (bool, error) {
			streamCancel()
			return c.Kill(context.Background(), pid)
		},
		cfg.onStdout,
		cfg.onStderr,
	)

	return handle, nil
}

// wrapRPCError wraps RPC errors with appropriate context.
func (c *Commands) wrapRPCError(ctx context.Context, err error) error {
	if ctx.Err() == context.DeadlineExceeded {
		return NewRequestTimeoutError()
	}

	if connectErr, ok := err.(*connect.Error); ok {
		switch connectErr.Code() {
		case connect.CodeNotFound:
			return fmt.Errorf("%w: %s", ErrNotFound, connectErr.Message())
		case connect.CodeInvalidArgument:
			return fmt.Errorf("%w: %s", ErrInvalidArgument, connectErr.Message())
		case connect.CodeDeadlineExceeded:
			return NewRequestTimeoutError()
		case connect.CodeUnavailable:
			return fmt.Errorf("sandbox unavailable: %s", connectErr.Message())
		default:
			return fmt.Errorf("RPC error (%s): %s", connectErr.Code(), connectErr.Message())
		}
	}

	return err
}
