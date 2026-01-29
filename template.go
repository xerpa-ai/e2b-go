package e2b

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// ============== Template Builder ==============

// TemplateBuilder provides a fluent API for building E2B templates.
type TemplateBuilder struct {
	baseImage      string
	baseTemplate   string
	registryConfig RegistryConfig
	startCmd       string
	readyCmd       string
	force          bool
	forceNextLayer bool
	instructions   []TemplateStep
	contextPath    string
	ignorePatterns []string
}

// NewTemplate creates a new template builder.
//
// Example:
//
//	template := e2b.NewTemplate()
//	template.FromPythonImage("3.11").
//	    RunCmd("pip install numpy").
//	    SetStartCmd("python -m http.server 8080")
func NewTemplate(opts ...TemplateBuilderOption) *TemplateBuilder {
	cfg := defaultTemplateBuilderConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return &TemplateBuilder{
		baseImage:      DefaultBaseImage,
		contextPath:    cfg.contextPath,
		ignorePatterns: cfg.ignorePatterns,
		instructions:   make([]TemplateStep, 0),
	}
}

// FromImage sets the base Docker image.
//
// Example:
//
//	template.FromImage("python:3.11")
//	template.FromImage("myregistry.com/myimage:latest", &e2b.GeneralRegistry{
//	    Username: "user",
//	    Password: "pass",
//	})
func (b *TemplateBuilder) FromImage(image string, registry ...RegistryConfig) *TemplateBuilder {
	b.baseImage = image
	b.baseTemplate = ""
	if len(registry) > 0 {
		b.registryConfig = registry[0]
	}
	return b
}

// FromTemplate sets the base E2B template.
//
// Example:
//
//	template.FromTemplate("my-base-template")
func (b *TemplateBuilder) FromTemplate(templateID string) *TemplateBuilder {
	b.baseTemplate = templateID
	b.baseImage = ""
	b.registryConfig = nil
	return b
}

// FromBaseImage uses E2B's default base image (e2bdev/base).
func (b *TemplateBuilder) FromBaseImage() *TemplateBuilder {
	return b.FromImage(DefaultBaseImage)
}

// FromPythonImage uses a Python Docker image.
//
// Example:
//
//	template.FromPythonImage("3.11")
func (b *TemplateBuilder) FromPythonImage(version string) *TemplateBuilder {
	if version == "" {
		version = "3"
	}
	return b.FromImage(fmt.Sprintf("python:%s", version))
}

// FromNodeImage uses a Node.js Docker image.
//
// Example:
//
//	template.FromNodeImage("20")
func (b *TemplateBuilder) FromNodeImage(version string) *TemplateBuilder {
	if version == "" {
		version = "lts"
	}
	return b.FromImage(fmt.Sprintf("node:%s", version))
}

// FromDebianImage uses a Debian Docker image.
//
// Example:
//
//	template.FromDebianImage("bookworm")
func (b *TemplateBuilder) FromDebianImage(variant string) *TemplateBuilder {
	if variant == "" {
		variant = "stable"
	}
	return b.FromImage(fmt.Sprintf("debian:%s", variant))
}

// FromUbuntuImage uses an Ubuntu Docker image.
//
// Example:
//
//	template.FromUbuntuImage("24.04")
func (b *TemplateBuilder) FromUbuntuImage(variant string) *TemplateBuilder {
	if variant == "" {
		variant = "latest"
	}
	return b.FromImage(fmt.Sprintf("ubuntu:%s", variant))
}

// FromBunImage uses a Bun Docker image.
//
// Example:
//
//	template.FromBunImage("1.0")
func (b *TemplateBuilder) FromBunImage(version string) *TemplateBuilder {
	if version == "" {
		version = "latest"
	}
	return b.FromImage(fmt.Sprintf("oven/bun:%s", version))
}

// FromAWSRegistry uses an image from AWS ECR.
//
// Example:
//
//	template.FromAWSRegistry("123456789.dkr.ecr.us-west-2.amazonaws.com/myimage:latest",
//	    &e2b.AWSRegistry{
//	        AccessKeyID:     "AKIA...",
//	        SecretAccessKey: "...",
//	        Region:          "us-west-2",
//	    })
func (b *TemplateBuilder) FromAWSRegistry(image string, credentials *AWSRegistry) *TemplateBuilder {
	b.baseImage = image
	b.baseTemplate = ""
	b.registryConfig = credentials
	return b
}

// FromGCPRegistry uses an image from Google Container Registry.
//
// Example:
//
//	template.FromGCPRegistry("gcr.io/myproject/myimage:latest",
//	    &e2b.GCPRegistry{
//	        ServiceAccountJSON: "...",
//	    })
func (b *TemplateBuilder) FromGCPRegistry(image string, credentials *GCPRegistry) *TemplateBuilder {
	b.baseImage = image
	b.baseTemplate = ""
	b.registryConfig = credentials
	return b
}

// RunCmd executes a shell command.
//
// Example:
//
//	template.RunCmd("apt-get update && apt-get install -y vim")
//	template.RunCmd("pip install numpy", e2b.WithStepUser("root"))
func (b *TemplateBuilder) RunCmd(cmd string, opts ...StepOption) *TemplateBuilder {
	cfg := defaultStepConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	args := []string{cmd}
	if cfg.user != "" {
		args = append([]string{cfg.user}, args...)
	}

	b.instructions = append(b.instructions, TemplateStep{
		Type:  string(InstructionTypeRun),
		Args:  args,
		Force: cfg.force || b.forceNextLayer,
	})
	b.forceNextLayer = false
	return b
}

// Copy copies files into the template.
//
// Example:
//
//	template.Copy("requirements.txt", "/app/")
//	template.Copy("src/", "/app/src/", e2b.WithCopyUser("root"))
func (b *TemplateBuilder) Copy(src, dest string, opts ...CopyOption) *TemplateBuilder {
	cfg := defaultCopyConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	args := []string{src, dest}
	if cfg.user != "" {
		args = append(args, cfg.user)
	}
	if cfg.mode != 0 {
		args = append(args, fmt.Sprintf("%o", cfg.mode))
	}

	b.instructions = append(b.instructions, TemplateStep{
		Type:  string(InstructionTypeCopy),
		Args:  args,
		Force: cfg.forceUpload || b.forceNextLayer,
		// FilesHash will be computed during build
	})
	b.forceNextLayer = false
	return b
}

// SetEnv sets environment variables.
//
// Example:
//
//	template.SetEnv("NODE_ENV", "production")
func (b *TemplateBuilder) SetEnv(key, value string) *TemplateBuilder {
	b.instructions = append(b.instructions, TemplateStep{
		Type:  string(InstructionTypeEnv),
		Args:  []string{key, value},
		Force: b.forceNextLayer,
	})
	b.forceNextLayer = false
	return b
}

// SetEnvs sets multiple environment variables.
//
// Example:
//
//	template.SetEnvs(map[string]string{
//	    "NODE_ENV": "production",
//	    "PORT": "8080",
//	})
func (b *TemplateBuilder) SetEnvs(envs map[string]string) *TemplateBuilder {
	for key, value := range envs {
		b.SetEnv(key, value)
	}
	return b
}

// SetWorkdir sets the working directory.
//
// Example:
//
//	template.SetWorkdir("/app")
func (b *TemplateBuilder) SetWorkdir(dir string) *TemplateBuilder {
	b.instructions = append(b.instructions, TemplateStep{
		Type:  string(InstructionTypeWorkdir),
		Args:  []string{dir},
		Force: b.forceNextLayer,
	})
	b.forceNextLayer = false
	return b
}

// SetUser sets the user for subsequent commands.
//
// Example:
//
//	template.SetUser("root")
func (b *TemplateBuilder) SetUser(user string) *TemplateBuilder {
	b.instructions = append(b.instructions, TemplateStep{
		Type:  string(InstructionTypeUser),
		Args:  []string{user},
		Force: b.forceNextLayer,
	})
	b.forceNextLayer = false
	return b
}

// SetStartCmd sets the command to run on startup.
//
// Example:
//
//	template.SetStartCmd("python -m http.server 8080")
func (b *TemplateBuilder) SetStartCmd(cmd string) *TemplateBuilder {
	b.startCmd = cmd
	return b
}

// SetReadyCmd sets the command to check if the sandbox is ready.
//
// Example:
//
//	template.SetReadyCmd("curl -s http://localhost:8080/health")
func (b *TemplateBuilder) SetReadyCmd(cmd string) *TemplateBuilder {
	b.readyCmd = cmd
	return b
}

// SkipCache forces all subsequent steps to rebuild regardless of cache.
func (b *TemplateBuilder) SkipCache() *TemplateBuilder {
	b.force = true
	b.forceNextLayer = true
	return b
}

// PipInstall installs Python packages.
//
// Example:
//
//	template.PipInstall("numpy", "pandas")
//	template.PipInstall() // installs from requirements.txt in current directory
func (b *TemplateBuilder) PipInstall(packages ...string) *TemplateBuilder {
	var cmd string
	if len(packages) == 0 {
		cmd = "pip install -r requirements.txt"
	} else {
		cmd = fmt.Sprintf("pip install %s", joinStrings(packages, " "))
	}
	return b.RunCmd(cmd)
}

// NpmInstall installs Node.js packages.
//
// Example:
//
//	template.NpmInstall("express", "lodash")
//	template.NpmInstall() // installs from package.json
func (b *TemplateBuilder) NpmInstall(packages ...string) *TemplateBuilder {
	var cmd string
	if len(packages) == 0 {
		cmd = "npm install"
	} else {
		cmd = fmt.Sprintf("npm install %s", joinStrings(packages, " "))
	}
	return b.RunCmd(cmd)
}

// AptInstall installs Debian/Ubuntu packages.
//
// Example:
//
//	template.AptInstall("vim", "curl", "git")
func (b *TemplateBuilder) AptInstall(packages ...string) *TemplateBuilder {
	cmd := fmt.Sprintf("apt-get update && apt-get install -y %s", joinStrings(packages, " "))
	return b.RunCmd(cmd, WithStepUser("root"))
}

// GitClone clones a Git repository.
//
// Example:
//
//	template.GitClone("https://github.com/user/repo.git", "/app/repo")
func (b *TemplateBuilder) GitClone(repoURL string, dest string) *TemplateBuilder {
	cmd := fmt.Sprintf("git clone %s %s", repoURL, dest)
	return b.RunCmd(cmd)
}

// toBuildSpec converts the builder to a TemplateBuildSpec for the API.
func (b *TemplateBuilder) toBuildSpec() *TemplateBuildSpec {
	spec := &TemplateBuildSpec{
		Steps:    b.instructions,
		StartCmd: b.startCmd,
		ReadyCmd: b.readyCmd,
		Force:    b.force,
	}

	if b.baseTemplate != "" {
		spec.FromTemplate = b.baseTemplate
	} else {
		spec.FromImage = b.baseImage
		spec.FromImageRegistry = b.registryConfig
	}

	return spec
}

// Build deploys the template and waits for completion.
//
// Example:
//
//	info, err := template.Build(ctx, "my-template",
//	    e2b.WithBuildOnLogs(func(log e2b.BuildLogEntry) {
//	        fmt.Println(log.Message)
//	    }),
//	)
func (b *TemplateBuilder) Build(ctx context.Context, alias string, opts ...BuildOption) (*BuildInfo, error) {
	cfg := defaultBuildConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Get template config from build config or create default
	templateCfg := cfg.templateConfig
	if templateCfg == nil {
		templateCfg = defaultTemplateConfig()
	}
	applyTemplateEnvConfig(templateCfg)

	// Request build
	buildInfo, err := requestBuildInternal(ctx, alias, cfg, templateCfg)
	if err != nil {
		return nil, err
	}

	// Trigger build with spec
	spec := b.toBuildSpec()
	if err := triggerBuildInternal(ctx, buildInfo.TemplateID, buildInfo.BuildID, spec, templateCfg); err != nil {
		return nil, err
	}

	// Wait for build to complete
	if err := waitForBuildInternal(ctx, buildInfo.TemplateID, buildInfo.BuildID, cfg, templateCfg); err != nil {
		return nil, err
	}

	return buildInfo, nil
}

// BuildInBackground deploys the template without waiting for completion.
//
// Example:
//
//	info, err := template.BuildInBackground(ctx, "my-template")
//	// Later: status, err := e2b.GetBuildStatus(ctx, info.TemplateID, info.BuildID, opts...)
func (b *TemplateBuilder) BuildInBackground(ctx context.Context, alias string, opts ...BuildOption) (*BuildInfo, error) {
	cfg := defaultBuildConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Get template config from build config or create default
	templateCfg := cfg.templateConfig
	if templateCfg == nil {
		templateCfg = defaultTemplateConfig()
	}
	applyTemplateEnvConfig(templateCfg)

	// Request build
	buildInfo, err := requestBuildInternal(ctx, alias, cfg, templateCfg)
	if err != nil {
		return nil, err
	}

	// Trigger build with spec
	spec := b.toBuildSpec()
	if err := triggerBuildInternal(ctx, buildInfo.TemplateID, buildInfo.BuildID, spec, templateCfg); err != nil {
		return nil, err
	}

	return buildInfo, nil
}

// ============== API Functions ==============

// applyTemplateEnvConfig applies environment variables to template config.
func applyTemplateEnvConfig(cfg *templateConfig) {
	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv("E2B_API_KEY")
	}
	if cfg.accessToken == "" {
		cfg.accessToken = os.Getenv("E2B_ACCESS_TOKEN")
	}
	if cfg.domain == "" || cfg.domain == DefaultDomain {
		if envDomain := os.Getenv("E2B_DOMAIN"); envDomain != "" {
			cfg.domain = envDomain
		}
	}
	if cfg.apiURL == "" {
		cfg.apiURL = os.Getenv("E2B_API_URL")
	}
	if !cfg.debug {
		cfg.debug = os.Getenv("E2B_DEBUG") == "true"
	}

	// Compute API URL if not provided
	if cfg.apiURL == "" {
		if cfg.debug {
			cfg.apiURL = "http://localhost:3000"
		} else {
			cfg.apiURL = fmt.Sprintf("https://api.%s", cfg.domain)
		}
	}

	// Create HTTP client if not provided
	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{
			Timeout: cfg.requestTimeout,
		}
	}
}

// templateConfigFromOptions creates a template config from options.
func templateConfigFromOptions(opts []TemplateOption) *templateConfig {
	cfg := defaultTemplateConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	applyTemplateEnvConfig(cfg)
	return cfg
}

// RequestBuild creates a new template build request.
// This is the first step in the build process.
//
// Example:
//
//	info, err := e2b.RequestBuild(ctx, "my-template",
//	    e2b.WithBuildCPUCount(4),
//	    e2b.WithBuildMemoryMB(2048),
//	)
func RequestBuild(ctx context.Context, alias string, opts ...BuildOption) (*BuildInfo, error) {
	cfg := defaultBuildConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	templateCfg := cfg.templateConfig
	if templateCfg == nil {
		templateCfg = defaultTemplateConfig()
	}
	applyTemplateEnvConfig(templateCfg)

	return requestBuildInternal(ctx, alias, cfg, templateCfg)
}

// requestBuildInternal is the internal implementation of RequestBuild.
func requestBuildInternal(ctx context.Context, alias string, cfg *buildConfig, templateCfg *templateConfig) (*BuildInfo, error) {
	if templateCfg.apiKey == "" && templateCfg.accessToken == "" {
		return nil, fmt.Errorf("%w: API key or access token is required", ErrInvalidArgument)
	}

	reqBody := &templateBuildRequest{
		Alias:    alias,
		CPUCount: cfg.cpuCount,
		MemoryMB: cfg.memoryMB,
		TeamID:   cfg.teamID,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, templateCfg.apiURL+"/v3/templates", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	setTemplateHeaders(httpReq, templateCfg)

	resp, err := templateCfg.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var buildResp templateBuildResponse
	if err := json.Unmarshal(respBody, &buildResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &BuildInfo{
		TemplateID: buildResp.TemplateID,
		BuildID:    buildResp.BuildID,
		Aliases:    buildResp.Aliases,
		Public:     buildResp.Public,
	}, nil
}

// TriggerBuild starts a template build with the given specification.
//
// Example:
//
//	err := e2b.TriggerBuild(ctx, templateID, buildID, &e2b.TemplateBuildSpec{
//	    FromImage: "python:3.11",
//	    Steps: []e2b.TemplateStep{
//	        {Type: "RUN", Args: []string{"pip install numpy"}},
//	    },
//	})
func TriggerBuild(ctx context.Context, templateID, buildID string, spec *TemplateBuildSpec, opts ...TemplateOption) error {
	cfg := templateConfigFromOptions(opts)
	return triggerBuildInternal(ctx, templateID, buildID, spec, cfg)
}

// triggerBuildInternal is the internal implementation of TriggerBuild.
func triggerBuildInternal(ctx context.Context, templateID, buildID string, spec *TemplateBuildSpec, cfg *templateConfig) error {
	if cfg.apiKey == "" && cfg.accessToken == "" {
		return fmt.Errorf("%w: API key or access token is required", ErrInvalidArgument)
	}

	data, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v2/templates/%s/builds/%s", cfg.apiURL, templateID, buildID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	setTemplateHeaders(httpReq, cfg)

	resp, err := cfg.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// GetBuildStatus retrieves the status of a template build.
//
// Example:
//
//	status, err := e2b.GetBuildStatus(ctx, templateID, buildID,
//	    e2b.WithLogsOffset(100),
//	)
func GetBuildStatus(ctx context.Context, templateID, buildID string, opts ...TemplateOption) (*TemplateBuildInfo, error) {
	cfg := templateConfigFromOptions(opts)
	return getBuildStatusInternal(ctx, templateID, buildID, 0, cfg)
}

// GetBuildStatusWithOptions retrieves the status with additional options.
func GetBuildStatusWithOptions(ctx context.Context, templateID, buildID string, statusOpts []GetBuildStatusOption, opts ...TemplateOption) (*TemplateBuildInfo, error) {
	cfg := templateConfigFromOptions(opts)
	statusCfg := defaultGetBuildStatusConfig()
	for _, opt := range statusOpts {
		opt(statusCfg)
	}
	return getBuildStatusInternal(ctx, templateID, buildID, statusCfg.logsOffset, cfg)
}

// getBuildStatusInternal is the internal implementation of GetBuildStatus.
func getBuildStatusInternal(ctx context.Context, templateID, buildID string, logsOffset int, cfg *templateConfig) (*TemplateBuildInfo, error) {
	if cfg.apiKey == "" && cfg.accessToken == "" {
		return nil, fmt.Errorf("%w: API key or access token is required", ErrInvalidArgument)
	}

	endpoint := fmt.Sprintf("%s/templates/%s/builds/%s/status", cfg.apiURL, templateID, buildID)
	if logsOffset > 0 {
		endpoint = fmt.Sprintf("%s?logsOffset=%d", endpoint, logsOffset)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	setTemplateHeaders(httpReq, cfg)

	resp, err := cfg.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var buildInfo TemplateBuildInfo
	if err := json.Unmarshal(respBody, &buildInfo); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &buildInfo, nil
}

// WaitForBuild polls until a build completes or fails.
//
// Example:
//
//	err := e2b.WaitForBuild(ctx, templateID, buildID,
//	    e2b.WithBuildOnLogs(func(log e2b.BuildLogEntry) {
//	        fmt.Println(log.Message)
//	    }),
//	)
func WaitForBuild(ctx context.Context, templateID, buildID string, opts ...BuildOption) error {
	cfg := defaultBuildConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	templateCfg := cfg.templateConfig
	if templateCfg == nil {
		templateCfg = defaultTemplateConfig()
	}
	applyTemplateEnvConfig(templateCfg)

	return waitForBuildInternal(ctx, templateID, buildID, cfg, templateCfg)
}

// waitForBuildInternal is the internal implementation of WaitForBuild.
func waitForBuildInternal(ctx context.Context, templateID, buildID string, cfg *buildConfig, templateCfg *templateConfig) error {
	logsOffset := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		status, err := getBuildStatusInternal(ctx, templateID, buildID, logsOffset, templateCfg)
		if err != nil {
			return err
		}

		// Send log entries to callback
		if cfg.onLogs != nil {
			for _, entry := range status.LogEntries {
				cfg.onLogs(entry)
			}
		}
		logsOffset += len(status.LogEntries)

		switch status.Status {
		case TemplateBuildStatusReady:
			return nil
		case TemplateBuildStatusError:
			msg := "build failed"
			if status.Reason != nil {
				msg = status.Reason.Message
			}
			return fmt.Errorf("template build failed: %s", msg)
		case TemplateBuildStatusBuilding, TemplateBuildStatusWaiting:
			// Continue polling
		default:
			return fmt.Errorf("unknown build status: %s", status.Status)
		}

		time.Sleep(cfg.pollInterval)
	}
}

// GetFileUploadLink gets a presigned URL for uploading layer files.
//
// Example:
//
//	upload, err := e2b.GetFileUploadLink(ctx, templateID, filesHash)
//	if !upload.Present {
//	    // Upload file to upload.URL
//	}
func GetFileUploadLink(ctx context.Context, templateID, hash string, opts ...TemplateOption) (*FileUploadInfo, error) {
	cfg := templateConfigFromOptions(opts)

	if cfg.apiKey == "" && cfg.accessToken == "" {
		return nil, fmt.Errorf("%w: API key or access token is required", ErrInvalidArgument)
	}

	endpoint := fmt.Sprintf("%s/templates/%s/files/%s", cfg.apiURL, templateID, hash)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	setTemplateHeaders(httpReq, cfg)

	resp, err := cfg.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var uploadInfo FileUploadInfo
	if err := json.Unmarshal(respBody, &uploadInfo); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &uploadInfo, nil
}

// AliasExists checks if a template alias already exists.
//
// Example:
//
//	exists, err := e2b.AliasExists(ctx, "my-template")
func AliasExists(ctx context.Context, alias string, opts ...TemplateOption) (bool, error) {
	cfg := templateConfigFromOptions(opts)

	if cfg.apiKey == "" && cfg.accessToken == "" {
		return false, fmt.Errorf("%w: API key or access token is required", ErrInvalidArgument)
	}

	endpoint := fmt.Sprintf("%s/templates/aliases/%s", cfg.apiURL, url.PathEscape(alias))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	setTemplateHeaders(httpReq, cfg)

	resp, err := cfg.httpClient.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	case http.StatusForbidden:
		// Alias exists but user doesn't have access
		return true, nil
	default:
		respBody, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}
}

// ListTemplates returns all templates for the authenticated user.
//
// Example:
//
//	templates, err := e2b.ListTemplates(ctx)
func ListTemplates(ctx context.Context, opts ...TemplateOption) ([]TemplateInfo, error) {
	cfg := templateConfigFromOptions(opts)

	if cfg.apiKey == "" && cfg.accessToken == "" {
		return nil, fmt.Errorf("%w: API key or access token is required", ErrInvalidArgument)
	}

	endpoint := cfg.apiURL + "/templates"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	setTemplateHeaders(httpReq, cfg)

	resp, err := cfg.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var templates []TemplateInfo
	if err := json.Unmarshal(respBody, &templates); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return templates, nil
}

// GetTemplateByID retrieves a template with its build history.
//
// Example:
//
//	template, err := e2b.GetTemplateByID(ctx, "template-id")
func GetTemplateByID(ctx context.Context, templateID string, opts ...TemplateOption) (*TemplateWithBuilds, error) {
	cfg := templateConfigFromOptions(opts)

	if cfg.apiKey == "" && cfg.accessToken == "" {
		return nil, fmt.Errorf("%w: API key or access token is required", ErrInvalidArgument)
	}

	endpoint := fmt.Sprintf("%s/templates/%s", cfg.apiURL, templateID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	setTemplateHeaders(httpReq, cfg)

	resp, err := cfg.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var template TemplateWithBuilds
	if err := json.Unmarshal(respBody, &template); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &template, nil
}

// DeleteTemplate deletes a template by ID.
//
// Example:
//
//	err := e2b.DeleteTemplate(ctx, "template-id")
func DeleteTemplate(ctx context.Context, templateID string, opts ...TemplateOption) error {
	cfg := templateConfigFromOptions(opts)

	if cfg.apiKey == "" && cfg.accessToken == "" {
		return fmt.Errorf("%w: API key or access token is required", ErrInvalidArgument)
	}

	endpoint := fmt.Sprintf("%s/templates/%s", cfg.apiURL, templateID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	setTemplateHeaders(httpReq, cfg)

	resp, err := cfg.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// UpdateTemplate updates a template's properties.
//
// Example:
//
//	public := true
//	err := e2b.UpdateTemplate(ctx, "template-id", &e2b.TemplateUpdate{
//	    Public: &public,
//	})
func UpdateTemplate(ctx context.Context, templateID string, update *TemplateUpdate, opts ...TemplateOption) error {
	cfg := templateConfigFromOptions(opts)

	if cfg.apiKey == "" && cfg.accessToken == "" {
		return fmt.Errorf("%w: API key or access token is required", ErrInvalidArgument)
	}

	data, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/templates/%s", cfg.apiURL, templateID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	setTemplateHeaders(httpReq, cfg)

	resp, err := cfg.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// ============== Helper Functions ==============

// setTemplateHeaders sets common headers for template API requests.
func setTemplateHeaders(req *http.Request, cfg *templateConfig) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "e2b-go-sdk/"+Version)

	if cfg.apiKey != "" {
		req.Header.Set("X-API-Key", cfg.apiKey)
	}
	if cfg.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.accessToken)
	}
}

// joinStrings joins strings with a separator.
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
