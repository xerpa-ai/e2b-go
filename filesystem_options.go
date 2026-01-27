package e2b

import "time"

// filesystemConfig holds configuration for filesystem operations.
type filesystemConfig struct {
	user           string
	requestTimeout time.Duration
}

// defaultFilesystemConfig returns the default filesystem configuration.
func defaultFilesystemConfig() *filesystemConfig {
	return &filesystemConfig{}
}

// FilesystemOption configures filesystem operations.
type FilesystemOption func(*filesystemConfig)

// WithUser sets the user for filesystem operations.
// This affects the resolution of relative paths and ownership of created files.
func WithUser(user string) FilesystemOption {
	return func(c *filesystemConfig) {
		c.user = user
	}
}

// WithFilesystemRequestTimeout sets the request timeout for filesystem operations.
func WithFilesystemRequestTimeout(d time.Duration) FilesystemOption {
	return func(c *filesystemConfig) {
		c.requestTimeout = d
	}
}

// listConfig holds configuration for listing directories.
type listConfig struct {
	filesystemConfig
	depth uint32
}

// defaultListConfig returns the default list configuration.
func defaultListConfig() *listConfig {
	return &listConfig{
		depth: 1,
	}
}

// ListOption configures directory listing operations.
type ListOption func(*listConfig)

// WithListUser sets the user for the list operation.
func WithListUser(user string) ListOption {
	return func(c *listConfig) {
		c.user = user
	}
}

// WithListRequestTimeout sets the request timeout for the list operation.
func WithListRequestTimeout(d time.Duration) ListOption {
	return func(c *listConfig) {
		c.requestTimeout = d
	}
}

// WithDepth sets the depth of directory listing.
// Default is 1 (immediate children only).
func WithDepth(depth uint32) ListOption {
	return func(c *listConfig) {
		c.depth = depth
	}
}

// watchConfig holds configuration for watching directories.
type watchConfig struct {
	filesystemConfig
	recursive bool
	timeoutMs int64
	onExit    func(error)
}

// defaultWatchConfig returns the default watch configuration.
func defaultWatchConfig() *watchConfig {
	return &watchConfig{
		recursive: false,
		timeoutMs: 60000, // 60 seconds default
	}
}

// WatchOption configures directory watching operations.
type WatchOption func(*watchConfig)

// WithWatchUser sets the user for the watch operation.
func WithWatchUser(user string) WatchOption {
	return func(c *watchConfig) {
		c.user = user
	}
}

// WithWatchRequestTimeout sets the request timeout for the watch operation.
func WithWatchRequestTimeout(d time.Duration) WatchOption {
	return func(c *watchConfig) {
		c.requestTimeout = d
	}
}

// WithRecursive enables recursive directory watching.
func WithRecursive(recursive bool) WatchOption {
	return func(c *watchConfig) {
		c.recursive = recursive
	}
}

// WithWatchTimeout sets the timeout for the watch operation in milliseconds.
// Use 0 to disable timeout.
func WithWatchTimeout(timeoutMs int64) WatchOption {
	return func(c *watchConfig) {
		c.timeoutMs = timeoutMs
	}
}

// OnWatchExit sets a callback to be called when the watch operation stops.
func OnWatchExit(handler func(error)) WatchOption {
	return func(c *watchConfig) {
		c.onExit = handler
	}
}

// readConfig holds configuration for reading files.
type readConfig struct {
	filesystemConfig
	format ReadFormat
}

// defaultReadConfig returns the default read configuration.
func defaultReadConfig() *readConfig {
	return &readConfig{
		format: ReadFormatText,
	}
}

// ReadOption configures file reading operations.
type ReadOption func(*readConfig)

// WithReadUser sets the user for the read operation.
func WithReadUser(user string) ReadOption {
	return func(c *readConfig) {
		c.user = user
	}
}

// WithReadRequestTimeout sets the request timeout for the read operation.
func WithReadRequestTimeout(d time.Duration) ReadOption {
	return func(c *readConfig) {
		c.requestTimeout = d
	}
}

// WithFormat sets the format for reading file content.
func WithFormat(format ReadFormat) ReadOption {
	return func(c *readConfig) {
		c.format = format
	}
}

// writeConfig holds configuration for writing files.
type writeConfig struct {
	filesystemConfig
}

// defaultWriteConfig returns the default write configuration.
func defaultWriteConfig() *writeConfig {
	return &writeConfig{}
}

// WriteOption configures file writing operations.
type WriteOption func(*writeConfig)

// WithWriteUser sets the user for the write operation.
func WithWriteUser(user string) WriteOption {
	return func(c *writeConfig) {
		c.user = user
	}
}

// WithWriteRequestTimeout sets the request timeout for the write operation.
func WithWriteRequestTimeout(d time.Duration) WriteOption {
	return func(c *writeConfig) {
		c.requestTimeout = d
	}
}
