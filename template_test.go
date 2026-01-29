package e2b

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewTemplate(t *testing.T) {
	template := NewTemplate()

	if template == nil {
		t.Fatal("NewTemplate() returned nil")
	}

	if template.baseImage != DefaultBaseImage {
		t.Errorf("baseImage = %v, want %v", template.baseImage, DefaultBaseImage)
	}

	if len(template.instructions) != 0 {
		t.Errorf("instructions length = %d, want 0", len(template.instructions))
	}
}

func TestNewTemplateWithOptions(t *testing.T) {
	template := NewTemplate(
		WithBuilderContextPath("/custom/path"),
		WithBuilderIgnorePatterns("*.log", "*.tmp"),
	)

	if template.contextPath != "/custom/path" {
		t.Errorf("contextPath = %v, want /custom/path", template.contextPath)
	}

	if len(template.ignorePatterns) != 2 {
		t.Errorf("ignorePatterns length = %d, want 2", len(template.ignorePatterns))
	}
}

func TestTemplateFromImage(t *testing.T) {
	template := NewTemplate()

	template.FromImage("python:3.11")
	if template.baseImage != "python:3.11" {
		t.Errorf("baseImage = %v, want python:3.11", template.baseImage)
	}

	// Test with registry credentials
	creds := &GeneralRegistry{Username: "user", Password: "pass"}
	template.FromImage("private.registry/image:latest", creds)
	if template.registryConfig != creds {
		t.Error("registryConfig not set correctly")
	}
}

func TestTemplateFromTemplate(t *testing.T) {
	template := NewTemplate()

	template.FromTemplate("my-base-template")
	if template.baseTemplate != "my-base-template" {
		t.Errorf("baseTemplate = %v, want my-base-template", template.baseTemplate)
	}

	if template.baseImage != "" {
		t.Errorf("baseImage should be empty when using FromTemplate")
	}
}

func TestTemplateFromPythonImage(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{"with version", "3.11", "python:3.11"},
		{"empty version", "", "python:3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := NewTemplate().FromPythonImage(tt.version)
			if template.baseImage != tt.expected {
				t.Errorf("baseImage = %v, want %v", template.baseImage, tt.expected)
			}
		})
	}
}

func TestTemplateFromNodeImage(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{"with version", "20", "node:20"},
		{"empty version", "", "node:lts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := NewTemplate().FromNodeImage(tt.version)
			if template.baseImage != tt.expected {
				t.Errorf("baseImage = %v, want %v", template.baseImage, tt.expected)
			}
		})
	}
}

func TestTemplateFromDebianImage(t *testing.T) {
	template := NewTemplate().FromDebianImage("bookworm")
	if template.baseImage != "debian:bookworm" {
		t.Errorf("baseImage = %v, want debian:bookworm", template.baseImage)
	}
}

func TestTemplateFromUbuntuImage(t *testing.T) {
	template := NewTemplate().FromUbuntuImage("24.04")
	if template.baseImage != "ubuntu:24.04" {
		t.Errorf("baseImage = %v, want ubuntu:24.04", template.baseImage)
	}
}

func TestTemplateFromAWSRegistry(t *testing.T) {
	creds := &AWSRegistry{
		AccessKeyID:     "AKIA...",
		SecretAccessKey: "secret",
		Region:          "us-west-2",
	}

	template := NewTemplate().FromAWSRegistry("123456789.dkr.ecr.us-west-2.amazonaws.com/image:latest", creds)

	if template.baseImage != "123456789.dkr.ecr.us-west-2.amazonaws.com/image:latest" {
		t.Errorf("baseImage not set correctly")
	}

	if template.registryConfig != creds {
		t.Error("registryConfig not set correctly")
	}
}

func TestTemplateFromGCPRegistry(t *testing.T) {
	creds := &GCPRegistry{
		ServiceAccountJSON: `{"type":"service_account"}`,
	}

	template := NewTemplate().FromGCPRegistry("gcr.io/project/image:latest", creds)

	if template.baseImage != "gcr.io/project/image:latest" {
		t.Errorf("baseImage not set correctly")
	}

	if template.registryConfig != creds {
		t.Error("registryConfig not set correctly")
	}
}

func TestTemplateRunCmd(t *testing.T) {
	template := NewTemplate().
		RunCmd("apt-get update").
		RunCmd("pip install numpy", WithStepUser("root"))

	if len(template.instructions) != 2 {
		t.Errorf("instructions length = %d, want 2", len(template.instructions))
	}

	// Check first instruction
	if template.instructions[0].Type != string(InstructionTypeRun) {
		t.Errorf("instruction type = %v, want RUN", template.instructions[0].Type)
	}

	if len(template.instructions[0].Args) != 1 || template.instructions[0].Args[0] != "apt-get update" {
		t.Errorf("instruction args not correct: %v", template.instructions[0].Args)
	}

	// Check second instruction with user
	if len(template.instructions[1].Args) != 2 {
		t.Errorf("instruction args length = %d, want 2", len(template.instructions[1].Args))
	}

	if template.instructions[1].Args[0] != "root" {
		t.Errorf("instruction user = %v, want root", template.instructions[1].Args[0])
	}
}

func TestTemplateCopy(t *testing.T) {
	template := NewTemplate().
		Copy("requirements.txt", "/app/").
		Copy("src/", "/app/src/", WithCopyUser("user"), WithCopyMode(0755))

	if len(template.instructions) != 2 {
		t.Errorf("instructions length = %d, want 2", len(template.instructions))
	}

	if template.instructions[0].Type != string(InstructionTypeCopy) {
		t.Errorf("instruction type = %v, want COPY", template.instructions[0].Type)
	}
}

func TestTemplateSetEnv(t *testing.T) {
	template := NewTemplate().
		SetEnv("NODE_ENV", "production").
		SetEnvs(map[string]string{
			"PORT": "8080",
			"HOST": "0.0.0.0",
		})

	// SetEnv + 2 from SetEnvs
	if len(template.instructions) != 3 {
		t.Errorf("instructions length = %d, want 3", len(template.instructions))
	}

	if template.instructions[0].Type != string(InstructionTypeEnv) {
		t.Errorf("instruction type = %v, want ENV", template.instructions[0].Type)
	}
}

func TestTemplateSetWorkdir(t *testing.T) {
	template := NewTemplate().SetWorkdir("/app")

	if len(template.instructions) != 1 {
		t.Errorf("instructions length = %d, want 1", len(template.instructions))
	}

	if template.instructions[0].Type != string(InstructionTypeWorkdir) {
		t.Errorf("instruction type = %v, want WORKDIR", template.instructions[0].Type)
	}

	if template.instructions[0].Args[0] != "/app" {
		t.Errorf("workdir = %v, want /app", template.instructions[0].Args[0])
	}
}

func TestTemplateSetUser(t *testing.T) {
	template := NewTemplate().SetUser("root")

	if len(template.instructions) != 1 {
		t.Errorf("instructions length = %d, want 1", len(template.instructions))
	}

	if template.instructions[0].Type != string(InstructionTypeUser) {
		t.Errorf("instruction type = %v, want USER", template.instructions[0].Type)
	}
}

func TestTemplateSetStartCmd(t *testing.T) {
	template := NewTemplate().SetStartCmd("python app.py")

	if template.startCmd != "python app.py" {
		t.Errorf("startCmd = %v, want python app.py", template.startCmd)
	}
}

func TestTemplateSetReadyCmd(t *testing.T) {
	template := NewTemplate().SetReadyCmd("curl localhost:8080/health")

	if template.readyCmd != "curl localhost:8080/health" {
		t.Errorf("readyCmd = %v, want curl localhost:8080/health", template.readyCmd)
	}
}

func TestTemplateSkipCache(t *testing.T) {
	template := NewTemplate().
		SkipCache().
		RunCmd("apt-get update")

	if !template.force {
		t.Error("force should be true after SkipCache()")
	}

	if !template.instructions[0].Force {
		t.Error("instruction force should be true after SkipCache()")
	}
}

func TestTemplatePipInstall(t *testing.T) {
	template := NewTemplate().
		PipInstall("numpy", "pandas")

	if len(template.instructions) != 1 {
		t.Errorf("instructions length = %d, want 1", len(template.instructions))
	}

	// Should contain pip install command
	if template.instructions[0].Type != string(InstructionTypeRun) {
		t.Error("PipInstall should create RUN instruction")
	}
}

func TestTemplateNpmInstall(t *testing.T) {
	template := NewTemplate().
		NpmInstall("express", "lodash")

	if len(template.instructions) != 1 {
		t.Errorf("instructions length = %d, want 1", len(template.instructions))
	}
}

func TestTemplateAptInstall(t *testing.T) {
	template := NewTemplate().
		AptInstall("vim", "git")

	if len(template.instructions) != 1 {
		t.Errorf("instructions length = %d, want 1", len(template.instructions))
	}
}

func TestTemplateGitClone(t *testing.T) {
	template := NewTemplate().
		GitClone("https://github.com/user/repo.git", "/app/repo")

	if len(template.instructions) != 1 {
		t.Errorf("instructions length = %d, want 1", len(template.instructions))
	}
}

func TestTemplateToBuildSpec(t *testing.T) {
	template := NewTemplate().
		FromPythonImage("3.11").
		RunCmd("pip install numpy").
		SetStartCmd("python app.py").
		SetReadyCmd("curl localhost:8080")

	spec := template.toBuildSpec()

	if spec.FromImage != "python:3.11" {
		t.Errorf("FromImage = %v, want python:3.11", spec.FromImage)
	}

	if len(spec.Steps) != 1 {
		t.Errorf("Steps length = %d, want 1", len(spec.Steps))
	}

	if spec.StartCmd != "python app.py" {
		t.Errorf("StartCmd = %v, want python app.py", spec.StartCmd)
	}

	if spec.ReadyCmd != "curl localhost:8080" {
		t.Errorf("ReadyCmd = %v, want curl localhost:8080", spec.ReadyCmd)
	}
}

func TestTemplateChaining(t *testing.T) {
	template := NewTemplate().
		FromPythonImage("3.11").
		SetWorkdir("/app").
		Copy("requirements.txt", "/app/").
		RunCmd("pip install -r requirements.txt").
		Copy(".", "/app/").
		SetEnv("PORT", "8080").
		SetStartCmd("python app.py").
		SetReadyCmd("curl localhost:8080")

	if len(template.instructions) != 5 {
		t.Errorf("instructions length = %d, want 5", len(template.instructions))
	}

	if template.baseImage != "python:3.11" {
		t.Error("baseImage not preserved through chaining")
	}

	if template.startCmd != "python app.py" {
		t.Error("startCmd not preserved through chaining")
	}
}

func TestTemplateOptions(t *testing.T) {
	cfg := defaultTemplateConfig()

	WithTemplateAPIKey("test-key")(cfg)
	if cfg.apiKey != "test-key" {
		t.Errorf("apiKey = %v, want test-key", cfg.apiKey)
	}

	WithTemplateAccessToken("test-token")(cfg)
	if cfg.accessToken != "test-token" {
		t.Errorf("accessToken = %v, want test-token", cfg.accessToken)
	}

	WithTemplateDomain("custom.domain")(cfg)
	if cfg.domain != "custom.domain" {
		t.Errorf("domain = %v, want custom.domain", cfg.domain)
	}

	WithTemplateAPIURL("https://custom.api")(cfg)
	if cfg.apiURL != "https://custom.api" {
		t.Errorf("apiURL = %v, want https://custom.api", cfg.apiURL)
	}

	WithTemplateRequestTimeout(30 * time.Second)(cfg)
	if cfg.requestTimeout != 30*time.Second {
		t.Errorf("requestTimeout = %v, want 30s", cfg.requestTimeout)
	}

	WithTemplateDebug(true)(cfg)
	if !cfg.debug {
		t.Error("debug should be true")
	}
}

func TestBuildOptions(t *testing.T) {
	cfg := defaultBuildConfig()

	WithBuildCPUCount(4)(cfg)
	if cfg.cpuCount != 4 {
		t.Errorf("cpuCount = %d, want 4", cfg.cpuCount)
	}

	WithBuildMemoryMB(2048)(cfg)
	if cfg.memoryMB != 2048 {
		t.Errorf("memoryMB = %d, want 2048", cfg.memoryMB)
	}

	WithBuildSkipCache(true)(cfg)
	if !cfg.skipCache {
		t.Error("skipCache should be true")
	}

	WithBuildLogsRefresh(500 * time.Millisecond)(cfg)
	if cfg.logsRefreshMs != 500*time.Millisecond {
		t.Errorf("logsRefreshMs = %v, want 500ms", cfg.logsRefreshMs)
	}

	WithBuildTeamID("team-123")(cfg)
	if cfg.teamID != "team-123" {
		t.Errorf("teamID = %v, want team-123", cfg.teamID)
	}

	WithBuildPollInterval(1 * time.Second)(cfg)
	if cfg.pollInterval != 1*time.Second {
		t.Errorf("pollInterval = %v, want 1s", cfg.pollInterval)
	}

	logsCalled := false
	WithBuildOnLogs(func(entry BuildLogEntry) {
		logsCalled = true
	})(cfg)
	cfg.onLogs(BuildLogEntry{})
	if !logsCalled {
		t.Error("onLogs callback not set correctly")
	}
}

func TestCopyOptions(t *testing.T) {
	cfg := defaultCopyConfig()

	WithCopyUser("root")(cfg)
	if cfg.user != "root" {
		t.Errorf("user = %v, want root", cfg.user)
	}

	WithCopyMode(0755)(cfg)
	if cfg.mode != 0755 {
		t.Errorf("mode = %o, want 755", cfg.mode)
	}

	WithCopyForceUpload(true)(cfg)
	if !cfg.forceUpload {
		t.Error("forceUpload should be true")
	}

	WithCopyResolveSymlinks(false)(cfg)
	if cfg.resolveSymlinks {
		t.Error("resolveSymlinks should be false")
	}
}

func TestStepOptions(t *testing.T) {
	cfg := defaultStepConfig()

	WithStepUser("admin")(cfg)
	if cfg.user != "admin" {
		t.Errorf("user = %v, want admin", cfg.user)
	}

	WithStepForce(true)(cfg)
	if !cfg.force {
		t.Error("force should be true")
	}
}

func TestGetBuildStatusOptions(t *testing.T) {
	cfg := defaultGetBuildStatusConfig()

	WithLogsOffset(100)(cfg)
	if cfg.logsOffset != 100 {
		t.Errorf("logsOffset = %d, want 100", cfg.logsOffset)
	}

	WithLogsLimit(50)(cfg)
	if cfg.limit != 50 {
		t.Errorf("limit = %d, want 50", cfg.limit)
	}

	WithLogsLevel(LogLevelError)(cfg)
	if cfg.level != LogLevelError {
		t.Errorf("level = %v, want error", cfg.level)
	}
}

func TestListTemplatesOptions(t *testing.T) {
	cfg := defaultListTemplatesConfig()

	WithListTeamID("team-456")(cfg)
	if cfg.teamID != "team-456" {
		t.Errorf("teamID = %v, want team-456", cfg.teamID)
	}
}

func TestGetTemplateOptions(t *testing.T) {
	cfg := defaultGetTemplateConfig()

	WithGetTemplateLimit(50)(cfg)
	if cfg.limit != 50 {
		t.Errorf("limit = %d, want 50", cfg.limit)
	}

	WithGetTemplateNextToken("next-token")(cfg)
	if cfg.nextToken != "next-token" {
		t.Errorf("nextToken = %v, want next-token", cfg.nextToken)
	}
}

func TestRegistryMarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		registry RegistryConfig
		wantType RegistryType
	}{
		{
			name:     "general registry",
			registry: &GeneralRegistry{Username: "user", Password: "pass"},
			wantType: RegistryTypeGeneral,
		},
		{
			name:     "aws registry",
			registry: &AWSRegistry{AccessKeyID: "AKIA", SecretAccessKey: "secret", Region: "us-east-1"},
			wantType: RegistryTypeAWS,
		},
		{
			name:     "gcp registry",
			registry: &GCPRegistry{ServiceAccountJSON: `{}`},
			wantType: RegistryTypeGCP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.registry.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v", err)
			}

			var result map[string]any
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if result["type"] != string(tt.wantType) {
				t.Errorf("type = %v, want %v", result["type"], tt.wantType)
			}
		})
	}
}

func TestTemplateBuildStatus(t *testing.T) {
	statuses := []TemplateBuildStatus{
		TemplateBuildStatusBuilding,
		TemplateBuildStatusWaiting,
		TemplateBuildStatusReady,
		TemplateBuildStatusError,
	}

	expected := []string{"building", "waiting", "ready", "error"}

	for i, status := range statuses {
		if string(status) != expected[i] {
			t.Errorf("status = %v, want %v", status, expected[i])
		}
	}
}

func TestLogLevel(t *testing.T) {
	levels := []LogLevel{
		LogLevelDebug,
		LogLevelInfo,
		LogLevelWarn,
		LogLevelError,
	}

	expected := []string{"debug", "info", "warn", "error"}

	for i, level := range levels {
		if string(level) != expected[i] {
			t.Errorf("level = %v, want %v", level, expected[i])
		}
	}
}

func TestInstructionType(t *testing.T) {
	types := []InstructionType{
		InstructionTypeCopy,
		InstructionTypeRun,
		InstructionTypeEnv,
		InstructionTypeWorkdir,
		InstructionTypeUser,
	}

	expected := []string{"COPY", "RUN", "ENV", "WORKDIR", "USER"}

	for i, typ := range types {
		if string(typ) != expected[i] {
			t.Errorf("type = %v, want %v", typ, expected[i])
		}
	}
}

func TestTemplateConstants(t *testing.T) {
	if DefaultTemplateCPU != 2 {
		t.Errorf("DefaultTemplateCPU = %d, want 2", DefaultTemplateCPU)
	}

	if DefaultTemplateMemory != 1024 {
		t.Errorf("DefaultTemplateMemory = %d, want 1024", DefaultTemplateMemory)
	}

	if MinTemplateCPU != 1 {
		t.Errorf("MinTemplateCPU = %d, want 1", MinTemplateCPU)
	}

	if MaxTemplateCPU != 32 {
		t.Errorf("MaxTemplateCPU = %d, want 32", MaxTemplateCPU)
	}

	if MinTemplateMemory != 128 {
		t.Errorf("MinTemplateMemory = %d, want 128", MinTemplateMemory)
	}

	if DefaultBaseImage != "e2bdev/base" {
		t.Errorf("DefaultBaseImage = %v, want e2bdev/base", DefaultBaseImage)
	}
}

func TestAliasExistsAPI(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantExists bool
		wantErr    bool
	}{
		{"exists (200)", http.StatusOK, true, false},
		{"not found (404)", http.StatusNotFound, false, false},
		{"forbidden (403)", http.StatusForbidden, true, false},
		{"server error (500)", http.StatusInternalServerError, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(map[string]string{"templateID": "test-id"})
				}
			}))
			defer server.Close()

			exists, err := AliasExists(context.Background(), "test-alias",
				WithTemplateAPIKey("test-key"),
				WithTemplateAPIURL(server.URL),
			)

			if (err != nil) != tt.wantErr {
				t.Errorf("AliasExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && exists != tt.wantExists {
				t.Errorf("AliasExists() = %v, want %v", exists, tt.wantExists)
			}
		})
	}
}

func TestListTemplatesAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %v, want GET", r.Method)
		}

		if r.Header.Get("X-API-Key") != "test-key" {
			t.Error("X-API-Key header not set")
		}

		templates := []TemplateInfo{
			{ID: "template-1", Aliases: []string{"alias-1"}},
			{ID: "template-2", Aliases: []string{"alias-2"}},
		}
		json.NewEncoder(w).Encode(templates)
	}))
	defer server.Close()

	templates, err := ListTemplates(context.Background(),
		WithTemplateAPIKey("test-key"),
		WithTemplateAPIURL(server.URL),
	)

	if err != nil {
		t.Fatalf("ListTemplates() error = %v", err)
	}

	if len(templates) != 2 {
		t.Errorf("templates length = %d, want 2", len(templates))
	}

	if templates[0].ID != "template-1" {
		t.Errorf("template ID = %v, want template-1", templates[0].ID)
	}
}

func TestDeleteTemplateAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Method = %v, want DELETE", r.Method)
		}

		if r.URL.Path != "/templates/template-123" {
			t.Errorf("Path = %v, want /templates/template-123", r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := DeleteTemplate(context.Background(), "template-123",
		WithTemplateAPIKey("test-key"),
		WithTemplateAPIURL(server.URL),
	)

	if err != nil {
		t.Errorf("DeleteTemplate() error = %v", err)
	}
}

func TestUpdateTemplateAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("Method = %v, want PATCH", r.Method)
		}

		var update TemplateUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if update.Public == nil || !*update.Public {
			t.Error("Public should be true")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	public := true
	err := UpdateTemplate(context.Background(), "template-123",
		&TemplateUpdate{Public: &public},
		WithTemplateAPIKey("test-key"),
		WithTemplateAPIURL(server.URL),
	)

	if err != nil {
		t.Errorf("UpdateTemplate() error = %v", err)
	}
}

func TestRequestBuildAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %v, want POST", r.Method)
		}

		if r.URL.Path != "/v3/templates" {
			t.Errorf("Path = %v, want /v3/templates", r.URL.Path)
		}

		var req templateBuildRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if req.Alias != "my-template" {
			t.Errorf("Alias = %v, want my-template", req.Alias)
		}

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(templateBuildResponse{
			TemplateID: "template-123",
			BuildID:    "build-456",
			Aliases:    []string{"my-template"},
			Public:     false,
		})
	}))
	defer server.Close()

	info, err := RequestBuild(context.Background(), "my-template",
		WithBuildTemplateOptions(
			WithTemplateAPIKey("test-key"),
			WithTemplateAPIURL(server.URL),
		),
	)

	if err != nil {
		t.Fatalf("RequestBuild() error = %v", err)
	}

	if info.TemplateID != "template-123" {
		t.Errorf("TemplateID = %v, want template-123", info.TemplateID)
	}

	if info.BuildID != "build-456" {
		t.Errorf("BuildID = %v, want build-456", info.BuildID)
	}
}

func TestGetBuildStatusAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %v, want GET", r.Method)
		}

		json.NewEncoder(w).Encode(TemplateBuildInfo{
			TemplateID: "template-123",
			BuildID:    "build-456",
			Status:     TemplateBuildStatusBuilding,
			Logs:       []string{"Building..."},
			LogEntries: []BuildLogEntry{
				{Level: LogLevelInfo, Message: "Building..."},
			},
		})
	}))
	defer server.Close()

	status, err := GetBuildStatus(context.Background(), "template-123", "build-456",
		WithTemplateAPIKey("test-key"),
		WithTemplateAPIURL(server.URL),
	)

	if err != nil {
		t.Fatalf("GetBuildStatus() error = %v", err)
	}

	if status.Status != TemplateBuildStatusBuilding {
		t.Errorf("Status = %v, want building", status.Status)
	}

	if len(status.LogEntries) != 1 {
		t.Errorf("LogEntries length = %d, want 1", len(status.LogEntries))
	}
}

func TestGetFileUploadLinkAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %v, want GET", r.Method)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(FileUploadInfo{
			Present: false,
			URL:     "https://storage.example.com/upload?token=abc123",
		})
	}))
	defer server.Close()

	info, err := GetFileUploadLink(context.Background(), "template-123", "abc123",
		WithTemplateAPIKey("test-key"),
		WithTemplateAPIURL(server.URL),
	)

	if err != nil {
		t.Fatalf("GetFileUploadLink() error = %v", err)
	}

	if info.Present {
		t.Error("Present should be false")
	}

	if info.URL == "" {
		t.Error("URL should not be empty")
	}
}

func TestAPIRequiresAuth(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "ListTemplates",
			fn: func() error {
				_, err := ListTemplates(context.Background())
				return err
			},
		},
		{
			name: "AliasExists",
			fn: func() error {
				_, err := AliasExists(context.Background(), "test")
				return err
			},
		},
		{
			name: "DeleteTemplate",
			fn: func() error {
				return DeleteTemplate(context.Background(), "test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Error("expected error for missing auth")
			}
		})
	}
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		name     string
		strs     []string
		sep      string
		expected string
	}{
		{"empty", []string{}, " ", ""},
		{"single", []string{"a"}, " ", "a"},
		{"multiple", []string{"a", "b", "c"}, " ", "a b c"},
		{"custom sep", []string{"a", "b"}, ",", "a,b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinStrings(tt.strs, tt.sep)
			if result != tt.expected {
				t.Errorf("joinStrings() = %v, want %v", result, tt.expected)
			}
		})
	}
}
