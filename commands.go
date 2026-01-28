package e2b

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	processpb "github.com/xerpa-ai/e2b-go/internal/proto/process"
	"github.com/xerpa-ai/e2b-go/internal/proto/process/processpbconnect"
)

// Commands provides methods for executing commands in the sandbox.
type Commands struct {
	rpcClient    processpbconnect.ProcessClient
	httpClient   *http.Client
	envdBaseURL  string
	accessToken  string
	trafficToken string
	sandbox      *Sandbox
}

// newCommands creates a new Commands instance.
func newCommands(sandbox *Sandbox) *Commands {
	scheme := "https"
	if sandbox.config.debug {
		scheme = "http"
	}

	envdBaseURL := fmt.Sprintf("%s://%d-%s.%s", scheme, EnvdPort, sandbox.ID, sandbox.Domain)

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
		trafficToken: sandbox.trafficToken,
		sandbox:      sandbox,
	}
}

// setRPCHeaders sets authentication headers on the Connect request.
func (c *Commands) setRPCHeaders(req connect.AnyRequest) {
	if c.accessToken != "" {
		req.Header().Set(headerAccessToken, c.accessToken)
	}
	if c.trafficToken != "" {
		req.Header().Set(headerTrafficToken, c.trafficToken)
	}
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

	req := connect.NewRequest(&processpb.KillRequest{
		Pid: pid,
	})
	c.setRPCHeaders(req)

	resp, err := c.rpcClient.Kill(ctx, req)
	if err != nil {
		// Check for not found error
		if connectErr, ok := err.(*connect.Error); ok {
			if connectErr.Code() == connect.CodeNotFound {
				return false, nil
			}
		}
		return false, c.wrapRPCError(ctx, err)
	}

	return resp.Msg.GetKilled(), nil
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

	req := connect.NewRequest(&processpb.StreamInputRequest{
		Pid:   pid,
		Stdin: []byte(data),
	})
	c.setRPCHeaders(req)

	_, err := c.rpcClient.StreamInput(ctx, req)
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
		Cwd:  cfg.cwd,
	}

	req := connect.NewRequest(&processpb.StartRequest{
		Config: processConfig,
	})
	c.setRPCHeaders(req)

	// Set user header for authentication
	if cfg.user != "" {
		req.Header().Set("User", cfg.user)
	}

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

	// Read the first event which should be a StartEvent
	if !stream.Receive() {
		streamCancel()
		if err := stream.Err(); err != nil {
			return nil, c.wrapRPCError(ctx, err)
		}
		return nil, fmt.Errorf("failed to start process: no start event received")
	}

	firstEvent := stream.Msg()
	startEvent := firstEvent.GetStart()
	if startEvent == nil {
		streamCancel()
		return nil, fmt.Errorf("failed to start process: expected start event, got %T", firstEvent.GetEvent())
	}

	pid := startEvent.GetPid()

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
		Pid: pid,
	})
	c.setRPCHeaders(req)

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

	firstEvent := stream.Msg()
	startEvent := firstEvent.GetStart()
	if startEvent == nil {
		streamCancel()
		return nil, fmt.Errorf("failed to connect to process: expected start event, got %T", firstEvent.GetEvent())
	}

	// Create the handle
	handle := newCommandHandle(
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
