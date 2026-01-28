package e2b

import (
	"io"
	"time"

	filesystempb "github.com/xerpa-ai/e2b-go/internal/proto/filesystem"
)

// FileType represents the type of filesystem object.
type FileType string

const (
	// FileTypeFile indicates a regular file.
	FileTypeFile FileType = "file"
	// FileTypeDir indicates a directory.
	FileTypeDir FileType = "dir"
)

// mapProtoFileType converts a protobuf FileType to our FileType.
func mapProtoFileType(ft filesystempb.FileType) FileType {
	switch ft {
	case filesystempb.FileType_FILE_TYPE_FILE:
		return FileTypeFile
	case filesystempb.FileType_FILE_TYPE_DIRECTORY:
		return FileTypeDir
	default:
		return ""
	}
}

// EventType represents the type of filesystem event.
type EventType string

const (
	// EventTypeCreate indicates a file or directory was created.
	EventTypeCreate EventType = "create"
	// EventTypeWrite indicates a file was written to.
	EventTypeWrite EventType = "write"
	// EventTypeRemove indicates a file or directory was removed.
	EventTypeRemove EventType = "remove"
	// EventTypeRename indicates a file or directory was renamed.
	EventTypeRename EventType = "rename"
	// EventTypeChmod indicates file permissions were changed.
	EventTypeChmod EventType = "chmod"
)

// mapProtoEventType converts a protobuf EventType to our EventType.
func mapProtoEventType(et filesystempb.EventType) EventType {
	switch et {
	case filesystempb.EventType_EVENT_TYPE_CREATE:
		return EventTypeCreate
	case filesystempb.EventType_EVENT_TYPE_WRITE:
		return EventTypeWrite
	case filesystempb.EventType_EVENT_TYPE_REMOVE:
		return EventTypeRemove
	case filesystempb.EventType_EVENT_TYPE_RENAME:
		return EventTypeRename
	case filesystempb.EventType_EVENT_TYPE_CHMOD:
		return EventTypeChmod
	default:
		return ""
	}
}

// EntryInfo contains metadata about a file or directory.
type EntryInfo struct {
	// Name is the name of the file or directory.
	Name string

	// Type is the type of the filesystem object (file or directory).
	Type FileType

	// Path is the full path to the file or directory.
	Path string

	// Size is the size in bytes (0 for directories).
	Size int64

	// Mode is the file mode and permission bits.
	Mode uint32

	// Permissions is the string representation of permissions (e.g., "rwxr-xr-x").
	Permissions string

	// Owner is the owner of the file or directory.
	Owner string

	// Group is the group owner of the file or directory.
	Group string

	// ModifiedTime is the last modification time.
	ModifiedTime time.Time

	// SymlinkTarget is the target of the symlink, if this is a symlink.
	// nil if not a symlink.
	SymlinkTarget *string
}

// entryInfoFromProto converts a protobuf EntryInfo to our EntryInfo.
func entryInfoFromProto(entry *filesystempb.EntryInfo) *EntryInfo {
	if entry == nil {
		return nil
	}

	info := &EntryInfo{
		Name:        entry.Name,
		Type:        mapProtoFileType(entry.Type),
		Path:        entry.Path,
		Size:        entry.Size,
		Mode:        entry.Mode,
		Permissions: entry.Permissions,
		Owner:       entry.Owner,
		Group:       entry.Group,
	}

	// Convert timestamp
	if entry.ModifiedTime != nil {
		info.ModifiedTime = entry.ModifiedTime.AsTime()
	}

	// Handle optional symlink target
	if entry.SymlinkTarget != nil {
		target := *entry.SymlinkTarget
		info.SymlinkTarget = &target
	}

	return info
}

// WriteInfo contains basic information about a written file.
type WriteInfo struct {
	// Name is the name of the file.
	Name string

	// Type is the type of the filesystem object.
	Type FileType

	// Path is the full path to the file.
	Path string
}

// WriteEntry represents a file to be written.
type WriteEntry struct {
	// Path is the path where the file should be written.
	Path string

	// Data is the content to write. Can be string, []byte, or io.Reader.
	Data any
}

// FilesystemEvent represents a filesystem change event.
type FilesystemEvent struct {
	// Name is the name of the file or directory that changed.
	Name string

	// Type is the type of event that occurred.
	Type EventType
}

// filesystemEventFromProto converts a protobuf FilesystemEvent to our FilesystemEvent.
func filesystemEventFromProto(event *filesystempb.FilesystemEvent) *FilesystemEvent {
	if event == nil {
		return nil
	}
	return &FilesystemEvent{
		Name: event.Name,
		Type: mapProtoEventType(event.Type),
	}
}

// ReadFormat specifies the format for reading file content.
type ReadFormat string

const (
	// ReadFormatText returns content as a string (default).
	ReadFormatText ReadFormat = "text"
	// ReadFormatBytes returns content as []byte.
	ReadFormatBytes ReadFormat = "bytes"
	// ReadFormatStream returns content as an io.ReadCloser for streaming.
	ReadFormatStream ReadFormat = "stream"
)

// FileContent represents the content of a file in various formats.
type FileContent struct {
	text  string
	bytes []byte
}

// Text returns the file content as a string.
func (fc *FileContent) Text() string {
	if fc.text != "" {
		return fc.text
	}
	return string(fc.bytes)
}

// Bytes returns the file content as bytes.
func (fc *FileContent) Bytes() []byte {
	if fc.bytes != nil {
		return fc.bytes
	}
	return []byte(fc.text)
}

// getDataReader converts various data types to an io.Reader.
func getDataReader(data any) (io.Reader, error) {
	switch v := data.(type) {
	case string:
		return io.NopCloser(stringReader(v)), nil
	case []byte:
		return io.NopCloser(bytesReader(v)), nil
	case io.Reader:
		return v, nil
	default:
		return nil, ErrInvalidArgument
	}
}

// stringReader wraps a string as an io.Reader.
type stringReader string

func (s stringReader) Read(p []byte) (n int, err error) {
	n = copy(p, s)
	if n < len(s) {
		return n, nil
	}
	return n, io.EOF
}

// bytesReader wraps a byte slice as an io.Reader.
type bytesReader []byte

func (b bytesReader) Read(p []byte) (n int, err error) {
	n = copy(p, b)
	if n < len(b) {
		return n, nil
	}
	return n, io.EOF
}
