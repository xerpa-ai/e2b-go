package e2b

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"connectrpc.com/connect"
	filesystempb "github.com/xerpa-ai/e2b-go/internal/proto/filesystem"
	"github.com/xerpa-ai/e2b-go/internal/proto/filesystem/filesystempbconnect"
	"golang.org/x/mod/semver"
)

// EnvdPort is the port for the envd service.
const EnvdPort = 49983

// HTTP header constants
const (
	headerAccessToken  = "X-Access-Token"
	headerTrafficToken = "E2B-Traffic-Access-Token"
)

// API path constants
const filesAPIPath = "/files"

// Filesystem provides operations for interacting with the sandbox filesystem.
type Filesystem struct {
	httpClient   *http.Client
	envdBaseURL  string
	rpcClient    filesystempbconnect.FilesystemClient
	accessToken  string
	trafficToken string
	sandbox      *Sandbox
	envdVersion  string
}

// newFilesystem creates a new Filesystem instance.
func newFilesystem(sandbox *Sandbox) *Filesystem {
	envdBaseURL := sandbox.getEnvdURL()

	// Create HTTP client for file operations
	httpClient := sandbox.config.httpClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: sandbox.config.requestTimeout,
		}
	}

	// Create RPC client for filesystem operations
	rpcClient := filesystempbconnect.NewFilesystemClient(
		httpClient,
		envdBaseURL,
		connect.WithGRPCWeb(),
	)

	return &Filesystem{
		httpClient:   httpClient,
		envdBaseURL:  envdBaseURL,
		rpcClient:    rpcClient,
		accessToken:  sandbox.accessToken,
		trafficToken: sandbox.TrafficAccessToken,
		sandbox:      sandbox,
		envdVersion:  sandbox.envdVersion,
	}
}

// buildFileURL builds a URL for file operations.
func (fs *Filesystem) buildFileURL(path, user string) (string, error) {
	u, err := url.Parse(fs.envdBaseURL + filesAPIPath)
	if err != nil {
		return "", err
	}

	q := u.Query()
	if path != "" {
		q.Set("path", path)
	}
	if user != "" {
		q.Set("username", user)
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// setHTTPHeaders sets authentication headers on the HTTP request.
func (fs *Filesystem) setHTTPHeaders(req *http.Request) {
	if fs.accessToken != "" {
		req.Header.Set(headerAccessToken, fs.accessToken)
	}
	if fs.trafficToken != "" {
		req.Header.Set(headerTrafficToken, fs.trafficToken)
	}
}

// setRPCHeaders sets authentication headers on the Connect request.
func (fs *Filesystem) setRPCHeaders(req connect.AnyRequest) {
	fs.setRPCHeadersWithUser(req, "")
}

// setRPCHeadersWithUser sets authentication headers including user-based Basic auth.
func (fs *Filesystem) setRPCHeadersWithUser(req connect.AnyRequest, user string) {
	req.Header().Set("User-Agent", "e2b-go-sdk/"+Version)
	if fs.accessToken != "" {
		req.Header().Set(headerAccessToken, fs.accessToken)
	}
	if fs.trafficToken != "" {
		req.Header().Set(headerTrafficToken, fs.trafficToken)
	}

	// Set Authorization header with Basic auth (username:)
	// If user is not specified and envd version < 0.4.0, default to "user"
	effectiveUser := user
	if effectiveUser == "" && fs.compareVersion(EnvdVersionDefaultUser) < 0 {
		effectiveUser = "user"
	}

	if effectiveUser != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(effectiveUser + ":"))
		req.Header().Set("Authorization", "Basic "+encoded)
	}
}

// compareVersion compares the envd version with the given version.
// Returns -1 if envdVersion < version, 0 if equal, 1 if envdVersion > version.
func (fs *Filesystem) compareVersion(version string) int {
	// Add "v" prefix for semver comparison if not present
	v1 := fs.envdVersion
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
func (fs *Filesystem) setStreamingHeaders(req connect.AnyRequest) {
	fs.setStreamingHeadersWithUser(req, "")
}

// setStreamingHeadersWithUser sets headers for streaming requests with user-based auth.
func (fs *Filesystem) setStreamingHeadersWithUser(req connect.AnyRequest, user string) {
	fs.setRPCHeadersWithUser(req, user)
	req.Header().Set(KeepalivePingHeader, fmt.Sprintf("%d", KeepalivePingIntervalSec))
}

// applyTimeout applies the appropriate timeout to the context.
func (fs *Filesystem) applyTimeout(ctx context.Context, configTimeout time.Duration) (context.Context, context.CancelFunc) {
	timeout := configTimeout
	if timeout == 0 {
		timeout = fs.sandbox.config.requestTimeout
	}
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return ctx, func() {}
}

// Read reads the content of a file.
//
// Example:
//
//	content, err := sandbox.Files.Read(ctx, "/home/user/file.txt")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(content)
func (fs *Filesystem) Read(ctx context.Context, path string, opts ...ReadOption) (string, error) {
	data, err := fs.ReadBytes(ctx, path, opts...)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReadStream reads the content of a file as a stream.
// The caller is responsible for closing the returned ReadCloser.
//
// Example:
//
//	stream, err := sandbox.Files.ReadStream(ctx, "/home/user/largefile.bin")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer stream.Close()
//	// Process stream in chunks
//	buf := make([]byte, 4096)
//	for {
//	    n, err := stream.Read(buf)
//	    // ... process data
//	}
func (fs *Filesystem) ReadStream(ctx context.Context, path string, opts ...ReadOption) (io.ReadCloser, error) {
	cfg := defaultReadConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := fs.applyTimeout(ctx, cfg.requestTimeout)

	// Build URL
	reqURL, err := fs.buildFileURL(path, cfg.user)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	fs.setHTTPHeaders(req)

	// Execute request
	resp, err := fs.httpClient.Do(req)
	if err != nil {
		cancel()
		if ctx.Err() == context.DeadlineExceeded {
			return nil, NewRequestTimeoutError()
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Check status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()
		return nil, fs.handleHTTPError(resp.StatusCode, body)
	}

	// Return a wrapper that cancels context when closed
	return &streamReadCloser{
		body:   resp.Body,
		cancel: cancel,
	}, nil
}

// streamReadCloser wraps an io.ReadCloser and cancels the context when closed.
type streamReadCloser struct {
	body   io.ReadCloser
	cancel context.CancelFunc
}

func (s *streamReadCloser) Read(p []byte) (n int, err error) {
	return s.body.Read(p)
}

func (s *streamReadCloser) Close() error {
	err := s.body.Close()
	s.cancel()
	return err
}

// ReadBytes reads the content of a file as bytes.
//
// Example:
//
//	data, err := sandbox.Files.ReadBytes(ctx, "/home/user/file.bin")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (fs *Filesystem) ReadBytes(ctx context.Context, path string, opts ...ReadOption) ([]byte, error) {
	cfg := defaultReadConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := fs.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	// Build URL
	reqURL, err := fs.buildFileURL(path, cfg.user)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	fs.setHTTPHeaders(req)

	// Execute request
	resp, err := fs.httpClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, NewRequestTimeoutError()
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fs.handleHTTPError(resp.StatusCode, body)
	}

	// Read response
	return io.ReadAll(resp.Body)
}

// Write writes content to a file.
//
// Writing to a file that doesn't exist creates the file.
// Writing to a file that already exists overwrites the file.
// Writing to a file at a path that doesn't exist creates the necessary directories.
//
// Example:
//
//	info, err := sandbox.Files.Write(ctx, "/home/user/file.txt", "Hello, World!")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (fs *Filesystem) Write(ctx context.Context, path string, data any, opts ...WriteOption) (*WriteInfo, error) {
	cfg := defaultWriteConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := fs.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	// Convert data to reader
	dataReader, err := toReader(data)
	if err != nil {
		return nil, err
	}

	// Create multipart form
	body, contentType, err := fs.createMultipartBody([]fileData{{path: path, reader: dataReader}})
	if err != nil {
		return nil, err
	}

	// Build URL
	reqURL, err := fs.buildFileURL(path, cfg.user)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Execute request
	infos, err := fs.doWriteRequest(ctx, reqURL, body, contentType)
	if err != nil {
		return nil, err
	}

	if len(infos) == 0 {
		return nil, fmt.Errorf("no file information returned")
	}

	return &infos[0], nil
}

// WriteFiles writes multiple files to the sandbox.
//
// Example:
//
//	infos, err := sandbox.Files.WriteFiles(ctx, []e2b.WriteEntry{
//	    {Path: "/home/user/file1.txt", Data: "Content 1"},
//	    {Path: "/home/user/file2.txt", Data: "Content 2"},
//	})
func (fs *Filesystem) WriteFiles(ctx context.Context, files []WriteEntry, opts ...WriteOption) ([]*WriteInfo, error) {
	if len(files) == 0 {
		return nil, nil
	}

	cfg := defaultWriteConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := fs.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	// Convert files to readers
	fileDataList := make([]fileData, len(files))
	for i, f := range files {
		reader, err := toReader(f.Data)
		if err != nil {
			return nil, err
		}
		fileDataList[i] = fileData{path: f.Path, reader: reader}
	}

	// Create multipart form
	body, contentType, err := fs.createMultipartBody(fileDataList)
	if err != nil {
		return nil, err
	}

	// Build URL
	reqURL, err := fs.buildFileURL("", cfg.user)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Execute request
	infos, err := fs.doWriteRequest(ctx, reqURL, body, contentType)
	if err != nil {
		return nil, err
	}

	result := make([]*WriteInfo, len(infos))
	for i := range infos {
		result[i] = &infos[i]
	}

	return result, nil
}

// fileData holds file path and reader for multipart upload.
type fileData struct {
	path   string
	reader io.Reader
}

// toReader converts various data types to io.Reader.
func toReader(data any) (io.Reader, error) {
	switch v := data.(type) {
	case string:
		return bytes.NewReader([]byte(v)), nil
	case []byte:
		return bytes.NewReader(v), nil
	case io.Reader:
		return v, nil
	default:
		return nil, fmt.Errorf("%w: data must be string, []byte, or io.Reader", ErrInvalidArgument)
	}
}

// createMultipartBody creates a multipart form body for file upload.
func (fs *Filesystem) createMultipartBody(files []fileData) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for _, f := range files {
		part, err := writer.CreateFormFile("file", f.path)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create form file: %w", err)
		}
		if _, err := io.Copy(part, f.reader); err != nil {
			return nil, "", fmt.Errorf("failed to write data: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	return &buf, writer.FormDataContentType(), nil
}

// doWriteRequest executes a file write request.
func (fs *Filesystem) doWriteRequest(ctx context.Context, reqURL string, body *bytes.Buffer, contentType string) ([]WriteInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	fs.setHTTPHeaders(req)

	resp, err := fs.httpClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, NewRequestTimeoutError()
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fs.handleHTTPError(resp.StatusCode, respBody)
	}

	var infos []WriteInfo
	if err := json.NewDecoder(resp.Body).Decode(&infos); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return infos, nil
}

// handleHTTPError converts HTTP errors to appropriate error types.
func (fs *Filesystem) handleHTTPError(statusCode int, body []byte) error {
	var errResp struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &errResp)

	message := errResp.Message
	if message == "" {
		message = string(body)
	}

	switch statusCode {
	case http.StatusBadRequest:
		return fmt.Errorf("%w: %s", ErrInvalidArgument, message)
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication error: %s", message)
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, message)
	case http.StatusTooManyRequests:
		return fmt.Errorf("rate limited: %s", message)
	case http.StatusBadGateway:
		return NewRequestTimeoutError()
	case http.StatusInsufficientStorage:
		return fmt.Errorf("not enough disk space: %s", message)
	default:
		return fmt.Errorf("HTTP error %d: %s", statusCode, message)
	}
}

// List lists the contents of a directory.
//
// Example:
//
//	entries, err := sandbox.Files.List(ctx, "/home/user")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, entry := range entries {
//	    fmt.Printf("%s (%s)\n", entry.Name, entry.Type)
//	}
func (fs *Filesystem) List(ctx context.Context, path string, opts ...ListOption) ([]*EntryInfo, error) {
	cfg := defaultListConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.depth < 1 {
		return nil, fmt.Errorf("%w: depth must be at least 1", ErrInvalidArgument)
	}

	ctx, cancel := fs.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	req := connect.NewRequest(&filesystempb.ListDirRequest{
		Path:  path,
		Depth: cfg.depth,
	})
	fs.setRPCHeadersWithUser(req, cfg.user)

	resp, err := fs.rpcClient.ListDir(ctx, req)
	if err != nil {
		return nil, fs.wrapRPCError(ctx, err)
	}

	entries := make([]*EntryInfo, 0, len(resp.Msg.Entries))
	for _, entry := range resp.Msg.Entries {
		if info := entryInfoFromProto(entry); info != nil && info.Type != "" {
			entries = append(entries, info)
		}
	}

	return entries, nil
}

// MakeDir creates a new directory.
//
// Creates all directories along the path if they don't exist.
// Returns true if the directory was created, false if it already exists.
//
// Example:
//
//	created, err := sandbox.Files.MakeDir(ctx, "/home/user/newdir")
func (fs *Filesystem) MakeDir(ctx context.Context, path string, opts ...FilesystemOption) (bool, error) {
	cfg := defaultFilesystemConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := fs.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	req := connect.NewRequest(&filesystempb.MakeDirRequest{Path: path})
	fs.setRPCHeadersWithUser(req, cfg.user)

	_, err := fs.rpcClient.MakeDir(ctx, req)
	if err != nil {
		if connectErr, ok := err.(*connect.Error); ok && connectErr.Code() == connect.CodeAlreadyExists {
			return false, nil
		}
		return false, fs.wrapRPCError(ctx, err)
	}

	return true, nil
}

// Remove removes a file or directory.
//
// Example:
//
//	err := sandbox.Files.Remove(ctx, "/home/user/file.txt")
func (fs *Filesystem) Remove(ctx context.Context, path string, opts ...FilesystemOption) error {
	cfg := defaultFilesystemConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := fs.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	req := connect.NewRequest(&filesystempb.RemoveRequest{Path: path})
	fs.setRPCHeadersWithUser(req, cfg.user)

	_, err := fs.rpcClient.Remove(ctx, req)
	if err != nil {
		return fs.wrapRPCError(ctx, err)
	}

	return nil
}

// Rename renames or moves a file or directory.
//
// Example:
//
//	info, err := sandbox.Files.Rename(ctx, "/home/user/old.txt", "/home/user/new.txt")
func (fs *Filesystem) Rename(ctx context.Context, oldPath, newPath string, opts ...FilesystemOption) (*EntryInfo, error) {
	cfg := defaultFilesystemConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := fs.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	req := connect.NewRequest(&filesystempb.MoveRequest{
		Source:      oldPath,
		Destination: newPath,
	})
	fs.setRPCHeadersWithUser(req, cfg.user)

	resp, err := fs.rpcClient.Move(ctx, req)
	if err != nil {
		return nil, fs.wrapRPCError(ctx, err)
	}

	return entryInfoFromProto(resp.Msg.Entry), nil
}

// Exists checks if a file or directory exists.
//
// Example:
//
//	exists, err := sandbox.Files.Exists(ctx, "/home/user/file.txt")
func (fs *Filesystem) Exists(ctx context.Context, path string, opts ...FilesystemOption) (bool, error) {
	cfg := defaultFilesystemConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := fs.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	req := connect.NewRequest(&filesystempb.StatRequest{Path: path})
	fs.setRPCHeadersWithUser(req, cfg.user)

	_, err := fs.rpcClient.Stat(ctx, req)
	if err != nil {
		if connectErr, ok := err.(*connect.Error); ok && connectErr.Code() == connect.CodeNotFound {
			return false, nil
		}
		return false, fs.wrapRPCError(ctx, err)
	}

	return true, nil
}

// GetInfo returns information about a file or directory.
//
// Example:
//
//	info, err := sandbox.Files.GetInfo(ctx, "/home/user/file.txt")
//	fmt.Printf("Size: %d bytes\n", info.Size)
func (fs *Filesystem) GetInfo(ctx context.Context, path string, opts ...FilesystemOption) (*EntryInfo, error) {
	cfg := defaultFilesystemConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := fs.applyTimeout(ctx, cfg.requestTimeout)
	defer cancel()

	req := connect.NewRequest(&filesystempb.StatRequest{Path: path})
	fs.setRPCHeadersWithUser(req, cfg.user)

	resp, err := fs.rpcClient.Stat(ctx, req)
	if err != nil {
		return nil, fs.wrapRPCError(ctx, err)
	}

	if resp.Msg.Entry == nil {
		return nil, fmt.Errorf("no entry information returned")
	}

	return entryInfoFromProto(resp.Msg.Entry), nil
}

// wrapRPCError wraps RPC errors with appropriate context.
func (fs *Filesystem) wrapRPCError(ctx context.Context, err error) error {
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
