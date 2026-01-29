package e2b

import (
	"context"
	"io"
)

// FilesystemReader provides read operations for the sandbox filesystem.
type FilesystemReader interface {
	// Read reads the content of a file as a string.
	Read(ctx context.Context, path string, opts ...ReadOption) (string, error)

	// ReadBytes reads the content of a file as bytes.
	ReadBytes(ctx context.Context, path string, opts ...ReadOption) ([]byte, error)

	// ReadStream reads the content of a file as a stream.
	ReadStream(ctx context.Context, path string, opts ...ReadOption) (io.ReadCloser, error)

	// List lists the contents of a directory.
	List(ctx context.Context, path string, opts ...ListOption) ([]*EntryInfo, error)

	// Exists checks if a file or directory exists.
	Exists(ctx context.Context, path string, opts ...FilesystemOption) (bool, error)

	// GetInfo returns information about a file or directory.
	GetInfo(ctx context.Context, path string, opts ...FilesystemOption) (*EntryInfo, error)
}

// FilesystemWriter provides write operations for the sandbox filesystem.
type FilesystemWriter interface {
	// Write writes content to a file.
	Write(ctx context.Context, path string, data any, opts ...WriteOption) (*WriteInfo, error)

	// WriteFiles writes multiple files to the sandbox.
	WriteFiles(ctx context.Context, files []WriteEntry, opts ...WriteOption) ([]*WriteInfo, error)

	// MakeDir creates a new directory.
	MakeDir(ctx context.Context, path string, opts ...FilesystemOption) (bool, error)

	// Remove removes a file or directory.
	Remove(ctx context.Context, path string, opts ...FilesystemOption) error

	// Rename renames or moves a file or directory.
	Rename(ctx context.Context, oldPath, newPath string, opts ...FilesystemOption) (*EntryInfo, error)
}

// FilesystemWatcher provides watch operations for the sandbox filesystem.
type FilesystemWatcher interface {
	// WatchDir watches a directory for filesystem events.
	WatchDir(ctx context.Context, path string, onEvent func(FilesystemEvent), opts ...WatchOption) (*WatchHandle, error)

	// CreateWatcher creates a non-streaming watcher for a directory.
	CreateWatcher(ctx context.Context, path string, opts ...WatchOption) (string, error)

	// GetWatcherEvents gets events from a non-streaming watcher.
	GetWatcherEvents(ctx context.Context, watcherID string, opts ...FilesystemOption) ([]*FilesystemEvent, error)

	// RemoveWatcher removes a non-streaming watcher.
	RemoveWatcher(ctx context.Context, watcherID string, opts ...FilesystemOption) error
}

// FilesystemService combines all filesystem operations.
type FilesystemService interface {
	FilesystemReader
	FilesystemWriter
	FilesystemWatcher
}

// CommandRunner provides command execution operations.
type CommandRunner interface {
	// Run executes a command and waits for it to complete.
	Run(ctx context.Context, cmd string, opts ...CommandOption) (*CommandResult, error)

	// RunBackground executes a command in the background.
	RunBackground(ctx context.Context, cmd string, opts ...CommandOption) (*CommandHandle, error)
}

// CommandManager provides command management operations.
type CommandManager interface {
	// List returns all running commands and PTY sessions.
	List(ctx context.Context, opts ...CommandRequestOption) ([]*ProcessInfo, error)

	// Kill terminates a running command by its process ID.
	Kill(ctx context.Context, pid uint32, opts ...CommandRequestOption) (bool, error)

	// SendStdin sends data to the stdin of a running command.
	SendStdin(ctx context.Context, pid uint32, data string, opts ...CommandRequestOption) error

	// Connect connects to a running command.
	Connect(ctx context.Context, pid uint32, opts ...CommandConnectOption) (*CommandHandle, error)
}

// CommandService combines all command operations.
type CommandService interface {
	CommandRunner
	CommandManager
}

// PtyService provides PTY (pseudo-terminal) operations.
type PtyService interface {
	// Create starts a new PTY.
	Create(ctx context.Context, size PtySize, opts ...PtyOption) (*CommandHandle, error)

	// Connect connects to an existing PTY.
	Connect(ctx context.Context, pid uint32, opts ...PtyConnectOption) (*CommandHandle, error)

	// Kill terminates a PTY by its process ID.
	Kill(ctx context.Context, pid uint32, opts ...PtyRequestOption) (bool, error)

	// SendStdin sends input data to a PTY.
	SendStdin(ctx context.Context, pid uint32, data []byte, opts ...PtyRequestOption) error

	// Resize changes the size of a PTY.
	Resize(ctx context.Context, pid uint32, size PtySize, opts ...PtyRequestOption) error
}

// Compile-time interface compliance checks.
var (
	_ FilesystemService = (*Filesystem)(nil)
	_ CommandService    = (*Commands)(nil)
	_ PtyService        = (*Pty)(nil)
)
