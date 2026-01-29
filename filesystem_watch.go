package e2b

import (
	"context"
	"fmt"
	"sync"

	"connectrpc.com/connect"
	filesystempb "github.com/xerpa-ai/e2b-go/internal/proto/filesystem"
)

// WatchHandle represents a handle to a directory watch operation.
// Use Stop() to stop watching.
type WatchHandle struct {
	cancel   context.CancelFunc
	done     chan struct{}
	err      error
	errMu    sync.RWMutex
	stopped  bool
	stoppedM sync.RWMutex
}

// Stop stops the watch operation.
func (h *WatchHandle) Stop() {
	h.stoppedM.Lock()
	if h.stopped {
		h.stoppedM.Unlock()
		return
	}
	h.stopped = true
	h.stoppedM.Unlock()

	h.cancel()
	<-h.done
}

// Wait waits for the watch operation to complete.
// Returns the error that caused the watch to stop, if any.
func (h *WatchHandle) Wait() error {
	<-h.done
	h.errMu.RLock()
	defer h.errMu.RUnlock()
	return h.err
}

// IsStopped returns true if the watch operation has been stopped.
func (h *WatchHandle) IsStopped() bool {
	h.stoppedM.RLock()
	defer h.stoppedM.RUnlock()
	return h.stopped
}

// setError sets the error that caused the watch to stop.
func (h *WatchHandle) setError(err error) {
	h.errMu.Lock()
	defer h.errMu.Unlock()
	h.err = err
}

// WatchDir watches a directory for filesystem events.
//
// The onEvent callback is called for each filesystem event.
// Returns a WatchHandle that can be used to stop watching.
//
// Example:
//
//	handle, err := sandbox.Files.WatchDir(ctx, "/home/user", func(event e2b.FilesystemEvent) {
//	    fmt.Printf("Event: %s on %s\n", event.Type, event.Name)
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer handle.Stop()
func (fs *Filesystem) WatchDir(
	ctx context.Context,
	path string,
	onEvent func(FilesystemEvent),
	opts ...WatchOption,
) (*WatchHandle, error) {
	cfg := defaultWatchConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Check if recursive watch is supported
	if cfg.recursive && fs.compareVersion(EnvdVersionRecursiveWatch) < 0 {
		return nil, fmt.Errorf("%w: recursive watch requires envd version >= %s (current: %s)",
			ErrInvalidArgument, EnvdVersionRecursiveWatch, fs.envdVersion)
	}

	// Create cancellable context for the entire watch operation
	watchCtx, cancel := context.WithCancel(ctx)

	// Create request
	req := connect.NewRequest(&filesystempb.WatchDirRequest{
		Path:      path,
		Recursive: cfg.recursive,
	})
	fs.setStreamingHeadersWithUser(req, cfg.user)

	// Start streaming - use the watch context directly
	// The stream will remain open until explicitly cancelled
	stream, err := fs.filesystemClient.WatchDir(watchCtx, req)
	if err != nil {
		cancel()
		return nil, fs.wrapRPCError(ctx, err)
	}

	// Wait for start event
	if !stream.Receive() {
		cancel()
		if err := stream.Err(); err != nil {
			return nil, fs.wrapRPCError(ctx, err)
		}
		return nil, fmt.Errorf("stream closed before start event")
	}

	msg := stream.Msg()
	if msg.GetStart() == nil {
		cancel()
		return nil, fmt.Errorf("expected start event, got %T", msg.GetEvent())
	}

	// Create handle
	handle := &WatchHandle{
		cancel: cancel,
		done:   make(chan struct{}),
	}

	// Start event processing goroutine
	go func() {
		defer close(handle.done)
		defer cancel()

		for stream.Receive() {
			msg := stream.Msg()

			switch event := msg.GetEvent().(type) {
			case *filesystempb.WatchDirResponse_Filesystem:
				if event.Filesystem != nil && onEvent != nil {
					fsEvent := filesystemEventFromProto(event.Filesystem)
					if fsEvent != nil {
						onEvent(*fsEvent)
					}
				}
			case *filesystempb.WatchDirResponse_Keepalive:
				// Keepalive event, ignore
			case *filesystempb.WatchDirResponse_Start:
				// Unexpected start event after initial start
			}
		}

		// Check for errors
		if err := stream.Err(); err != nil {
			handle.setError(err)
			if cfg.onExit != nil {
				cfg.onExit(err)
			}
		} else {
			if cfg.onExit != nil {
				cfg.onExit(nil)
			}
		}
	}()

	return handle, nil
}

// CreateWatcher creates a non-streaming watcher for a directory.
// Use GetWatcherEvents to poll for events and RemoveWatcher to stop watching.
//
// Example:
//
//	watcherID, err := sandbox.Files.CreateWatcher(ctx, "/home/user")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer sandbox.Files.RemoveWatcher(ctx, watcherID)
//
//	// Poll for events
//	events, err := sandbox.Files.GetWatcherEvents(ctx, watcherID)
func (fs *Filesystem) CreateWatcher(ctx context.Context, path string, opts ...WatchOption) (string, error) {
	cfg := defaultWatchConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Check if recursive watch is supported
	if cfg.recursive && fs.compareVersion(EnvdVersionRecursiveWatch) < 0 {
		return "", fmt.Errorf("%w: recursive watch requires envd version >= %s (current: %s)",
			ErrInvalidArgument, EnvdVersionRecursiveWatch, fs.envdVersion)
	}

	ctx, cancel := fs.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	req := connect.NewRequest(&filesystempb.CreateWatcherRequest{
		Path:      path,
		Recursive: cfg.recursive,
	})
	fs.setRPCHeadersWithUser(req, cfg.user)

	resp, err := fs.filesystemClient.CreateWatcher(ctx, req)
	if err != nil {
		return "", fs.wrapRPCError(ctx, err)
	}

	return resp.Msg.WatcherId, nil
}

// GetWatcherEvents gets events from a non-streaming watcher.
//
// Example:
//
//	events, err := sandbox.Files.GetWatcherEvents(ctx, watcherID)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, event := range events {
//	    fmt.Printf("Event: %s on %s\n", event.Type, event.Name)
//	}
func (fs *Filesystem) GetWatcherEvents(ctx context.Context, watcherID string, opts ...FilesystemOption) ([]*FilesystemEvent, error) {
	cfg := defaultFilesystemConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := fs.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	req := connect.NewRequest(&filesystempb.GetWatcherEventsRequest{
		WatcherId: watcherID,
	})
	fs.setRPCHeadersWithUser(req, cfg.user)

	resp, err := fs.filesystemClient.GetWatcherEvents(ctx, req)
	if err != nil {
		return nil, fs.wrapRPCError(ctx, err)
	}

	events := make([]*FilesystemEvent, 0, len(resp.Msg.Events))
	for _, e := range resp.Msg.Events {
		if event := filesystemEventFromProto(e); event != nil {
			events = append(events, event)
		}
	}

	return events, nil
}

// RemoveWatcher removes a non-streaming watcher.
//
// Example:
//
//	err := sandbox.Files.RemoveWatcher(ctx, watcherID)
func (fs *Filesystem) RemoveWatcher(ctx context.Context, watcherID string, opts ...FilesystemOption) error {
	cfg := defaultFilesystemConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := fs.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	req := connect.NewRequest(&filesystempb.RemoveWatcherRequest{
		WatcherId: watcherID,
	})
	fs.setRPCHeadersWithUser(req, cfg.user)

	_, err := fs.filesystemClient.RemoveWatcher(ctx, req)
	if err != nil {
		return fs.wrapRPCError(ctx, err)
	}

	return nil
}
