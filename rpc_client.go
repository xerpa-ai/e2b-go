package e2b

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
)

// rpcClient provides common RPC client functionality shared across
// Commands, Filesystem, and Pty services.
type rpcClient struct {
	httpClient   *http.Client
	envdBaseURL  string
	accessToken  string
	trafficToken string
	envdVersion  string
}

// newRPCClient creates a new rpcClient with common configuration.
func newRPCClient(sandbox *Sandbox) rpcClient {
	httpClient := sandbox.config.httpClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: sandbox.config.requestTimeout,
		}
	}

	return rpcClient{
		httpClient:   httpClient,
		envdBaseURL:  sandbox.getEnvdURL(),
		accessToken:  sandbox.accessToken,
		trafficToken: sandbox.TrafficAccessToken,
		envdVersion:  sandbox.envdVersion,
	}
}

// setRPCHeaders sets authentication headers on the Connect request.
func (r *rpcClient) setRPCHeaders(req connect.AnyRequest) {
	r.setRPCHeadersWithUser(req, "")
}

// setRPCHeadersWithUser sets authentication headers including user-based Basic auth.
func (r *rpcClient) setRPCHeadersWithUser(req connect.AnyRequest, user string) {
	req.Header().Set("User-Agent", "e2b-go-sdk/"+Version)
	if r.accessToken != "" {
		req.Header().Set(headerAccessToken, r.accessToken)
	}
	if r.trafficToken != "" {
		req.Header().Set(headerTrafficToken, r.trafficToken)
	}

	// Set Authorization header with Basic auth (username:)
	// If user is not specified and envd version < 0.4.0, default to "user"
	effectiveUser := user
	if effectiveUser == "" && r.compareVersion(EnvdVersionDefaultUser) < 0 {
		effectiveUser = "user"
	}

	if effectiveUser != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(effectiveUser + ":"))
		req.Header().Set("Authorization", "Basic "+encoded)
	}
}

// setStreamingHeaders sets headers for streaming requests including keepalive.
func (r *rpcClient) setStreamingHeaders(req connect.AnyRequest) {
	r.setStreamingHeadersWithUser(req, "")
}

// setStreamingHeadersWithUser sets headers for streaming requests with user-based auth.
func (r *rpcClient) setStreamingHeadersWithUser(req connect.AnyRequest, user string) {
	r.setRPCHeadersWithUser(req, user)
	req.Header().Set(KeepalivePingHeader, fmt.Sprintf("%d", KeepalivePingIntervalSec))
}

// compareVersion compares the envd version with the given version.
// Returns -1 if envdVersion < version, 0 if equal, 1 if envdVersion > version.
func (r *rpcClient) compareVersion(version string) int {
	return compareVersions(r.envdVersion, version)
}

// setHTTPHeaders sets authentication headers on an HTTP request.
func (r *rpcClient) setHTTPHeaders(req *http.Request) {
	if r.accessToken != "" {
		req.Header.Set(headerAccessToken, r.accessToken)
	}
	if r.trafficToken != "" {
		req.Header.Set(headerTrafficToken, r.trafficToken)
	}
}
