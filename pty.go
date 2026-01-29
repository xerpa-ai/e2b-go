package e2b

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	processpb "github.com/xerpa-ai/e2b-go/internal/proto/process"
	"github.com/xerpa-ai/e2b-go/internal/proto/process/processpbconnect"
)

// PtySize represents the size of a PTY terminal.
type PtySize struct {
	Rows uint32
	Cols uint32
}

// Pty provides methods for interacting with PTYs (pseudo-terminals) in the sandbox.
type Pty struct {
	rpcClient

	processClient processpbconnect.ProcessClient
	sandbox       *Sandbox
}

// newPty creates a new Pty instance.
func newPty(sandbox *Sandbox) *Pty {
	base := newRPCClient(sandbox)

	processClient := processpbconnect.NewProcessClient(
		base.httpClient,
		base.envdBaseURL,
		connect.WithGRPCWeb(),
	)

	return &Pty{
		rpcClient:     base,
		processClient: processClient,
		sandbox:       sandbox,
	}
}

// Create starts a new PTY (pseudo-terminal).
//
// Example:
//
//	handle, err := sandbox.Pty.Create(ctx, e2b.PtySize{Rows: 24, Cols: 80})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer handle.Kill()
func (p *Pty) Create(ctx context.Context, size PtySize, opts ...PtyOption) (*CommandHandle, error) {
	cfg := defaultPtyConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Set default environment variables for terminal
	if cfg.envs == nil {
		cfg.envs = make(map[string]string)
	}
	if _, ok := cfg.envs["TERM"]; !ok {
		cfg.envs["TERM"] = "xterm-256color"
	}
	if _, ok := cfg.envs["LANG"]; !ok {
		cfg.envs["LANG"] = "C.UTF-8"
	}
	if _, ok := cfg.envs["LC_ALL"]; !ok {
		cfg.envs["LC_ALL"] = "C.UTF-8"
	}

	var cwdPtr *string
	if cfg.cwd != "" {
		cwdPtr = &cfg.cwd
	}

	req := connect.NewRequest(&processpb.StartRequest{
		Process: &processpb.ProcessConfig{
			Cmd:  "/bin/bash",
			Args: []string{"-i", "-l"},
			Envs: cfg.envs,
			Cwd:  cwdPtr,
		},
		Pty: &processpb.PTY{
			Size: &processpb.PTY_Size{
				Rows: size.Rows,
				Cols: size.Cols,
			},
		},
	})

	p.setStreamingHeadersWithUser(req, cfg.user)

	stream, err := p.processClient.Start(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create PTY: %w", err)
	}

	// Read the first event to get the PID
	if !stream.Receive() {
		if err := stream.Err(); err != nil {
			return nil, fmt.Errorf("failed to receive start event: %w", err)
		}
		return nil, fmt.Errorf("stream closed before receiving start event")
	}

	msg := stream.Msg()
	if msg.GetEvent() == nil || msg.GetEvent().GetStart() == nil {
		return nil, fmt.Errorf("expected start event, got %v", msg)
	}

	pid := msg.GetEvent().GetStart().GetPid()

	handle := &CommandHandle{
		pid:      pid,
		pty:      p,
		stream:   stream,
		done:     make(chan struct{}),
		exitCode: -1,
		onStdout: cfg.onStdout,
		onStderr: cfg.onStderr,
		isPty:    true,
	}

	// Start processing events in the background
	go handle.processPtyEvents()

	return handle, nil
}

// Connect connects to an existing running PTY.
//
// Example:
//
//	handle, err := sandbox.Pty.Connect(ctx, pid)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (p *Pty) Connect(ctx context.Context, pid uint32, opts ...PtyConnectOption) (*CommandHandle, error) {
	cfg := &ptyConnectConfig{
		timeout: 60 * time.Second,
	}
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

	p.setStreamingHeaders(req)

	stream, err := p.processClient.Connect(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PTY: %w", err)
	}

	// Read the first event to confirm connection
	if !stream.Receive() {
		if err := stream.Err(); err != nil {
			return nil, fmt.Errorf("failed to receive start event: %w", err)
		}
		return nil, fmt.Errorf("stream closed before receiving start event")
	}

	msg := stream.Msg()
	if msg.GetEvent() == nil || msg.GetEvent().GetStart() == nil {
		return nil, fmt.Errorf("expected start event, got %v", msg)
	}

	handle := &CommandHandle{
		pid:           pid,
		pty:           p,
		connectStream: stream,
		done:          make(chan struct{}),
		exitCode:      -1,
		onStdout:      cfg.onStdout,
		onStderr:      cfg.onStderr,
		isPty:         true,
	}

	// Start processing events in the background
	go handle.processPtyConnectEvents()

	return handle, nil
}

// Kill terminates a PTY by its process ID.
//
// Returns true if the PTY was killed, false if it was not found.
func (p *Pty) Kill(ctx context.Context, pid uint32, opts ...PtyRequestOption) (bool, error) {
	cfg := &ptyRequestConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	req := connect.NewRequest(&processpb.SendSignalRequest{
		Process: &processpb.ProcessSelector{
			Selector: &processpb.ProcessSelector_Pid{
				Pid: pid,
			},
		},
		Signal: processpb.Signal_SIGNAL_SIGKILL,
	})

	p.setRPCHeaders(req)

	_, err := p.processClient.SendSignal(ctx, req)
	if err != nil {
		// Check if it's a not found error
		if connect.CodeOf(err) == connect.CodeNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to kill PTY: %w", err)
	}

	return true, nil
}

// SendStdin sends input data to a PTY.
//
// Note: For PTY, use []byte data as terminal input can include special characters.
func (p *Pty) SendStdin(ctx context.Context, pid uint32, data []byte, opts ...PtyRequestOption) error {
	cfg := &ptyRequestConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	req := connect.NewRequest(&processpb.SendInputRequest{
		Process: &processpb.ProcessSelector{
			Selector: &processpb.ProcessSelector_Pid{
				Pid: pid,
			},
		},
		Input: &processpb.ProcessInput{
			Input: &processpb.ProcessInput_Pty{
				Pty: data,
			},
		},
	})

	p.setRPCHeaders(req)

	_, err := p.processClient.SendInput(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send input to PTY: %w", err)
	}

	return nil
}

// Resize changes the size of a PTY.
// Call this when the terminal window is resized.
func (p *Pty) Resize(ctx context.Context, pid uint32, size PtySize, opts ...PtyRequestOption) error {
	cfg := &ptyRequestConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	req := connect.NewRequest(&processpb.UpdateRequest{
		Process: &processpb.ProcessSelector{
			Selector: &processpb.ProcessSelector_Pid{
				Pid: pid,
			},
		},
		Pty: &processpb.PTY{
			Size: &processpb.PTY_Size{
				Rows: size.Rows,
				Cols: size.Cols,
			},
		},
	})

	p.setRPCHeaders(req)

	_, err := p.processClient.Update(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to resize PTY: %w", err)
	}

	return nil
}
