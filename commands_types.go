package e2b

import (
	"fmt"

	processpb "github.com/xerpa-ai/e2b-go/internal/proto/process"
)

// CommandResult represents the result of a command execution.
type CommandResult struct {
	// Stdout is the command's standard output.
	Stdout string

	// Stderr is the command's standard error output.
	Stderr string

	// ExitCode is the command's exit code.
	// 0 indicates successful completion.
	ExitCode int

	// Error is the error message from command execution, if any.
	Error string
}

// ProcessInfo contains information about a running process in the sandbox.
type ProcessInfo struct {
	// PID is the process ID.
	PID uint32

	// Tag is a custom tag used for identifying special commands
	// like start command in custom templates.
	Tag string

	// Cmd is the command that was executed.
	Cmd string

	// Args are the command arguments.
	Args []string

	// Envs are the environment variables used for the command.
	Envs map[string]string

	// Cwd is the working directory of the command.
	Cwd string
}

// processInfoFromProto converts a protobuf ProcessInfo to our ProcessInfo type.
func processInfoFromProto(p *processpb.ProcessInfo) *ProcessInfo {
	if p == nil {
		return nil
	}

	config := p.GetConfig()
	if config == nil {
		return &ProcessInfo{
			PID:  p.GetPid(),
			Tag:  getStringValue(p.Tag),
			Args: []string{},
			Envs: make(map[string]string),
		}
	}

	args := config.GetArgs()
	if args == nil {
		args = []string{}
	}

	envs := config.GetEnvs()
	if envs == nil {
		envs = make(map[string]string)
	}

	return &ProcessInfo{
		PID:  p.GetPid(),
		Tag:  getStringValue(p.Tag),
		Cmd:  config.GetCmd(),
		Args: args,
		Envs: envs,
		Cwd:  config.GetCwd(),
	}
}

// getStringValue safely gets a string value from a pointer.
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// CommandExitError is returned when a command exits with a non-zero exit code.
type CommandExitError struct {
	// Stdout is the command's standard output.
	Stdout string

	// Stderr is the command's standard error output.
	Stderr string

	// ExitCode is the non-zero exit code.
	ExitCode int

	// ErrorMessage is the error message from command execution.
	ErrorMessage string
}

// Error implements the error interface.
func (e *CommandExitError) Error() string {
	return fmt.Sprintf("command exited with code %d and error:\n%s", e.ExitCode, e.Stderr)
}
