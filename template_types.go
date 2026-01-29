package e2b

import (
	"encoding/json"
	"time"
)

// TemplateBuildStatus represents the status of a template build.
type TemplateBuildStatus string

const (
	// TemplateBuildStatusBuilding indicates the build is in progress.
	TemplateBuildStatusBuilding TemplateBuildStatus = "building"
	// TemplateBuildStatusWaiting indicates the build is waiting to start.
	TemplateBuildStatusWaiting TemplateBuildStatus = "waiting"
	// TemplateBuildStatusReady indicates the build completed successfully.
	TemplateBuildStatusReady TemplateBuildStatus = "ready"
	// TemplateBuildStatusError indicates the build failed.
	TemplateBuildStatusError TemplateBuildStatus = "error"
)

// LogLevel represents the level of a log entry.
type LogLevel string

const (
	// LogLevelDebug is for debug messages.
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo is for informational messages.
	LogLevelInfo LogLevel = "info"
	// LogLevelWarn is for warning messages.
	LogLevelWarn LogLevel = "warn"
	// LogLevelError is for error messages.
	LogLevelError LogLevel = "error"
)

// InstructionType represents the type of build instruction.
type InstructionType string

const (
	// InstructionTypeCopy copies files into the template.
	InstructionTypeCopy InstructionType = "COPY"
	// InstructionTypeRun executes a command.
	InstructionTypeRun InstructionType = "RUN"
	// InstructionTypeEnv sets environment variables.
	InstructionTypeEnv InstructionType = "ENV"
	// InstructionTypeWorkdir sets the working directory.
	InstructionTypeWorkdir InstructionType = "WORKDIR"
	// InstructionTypeUser sets the user for subsequent commands.
	InstructionTypeUser InstructionType = "USER"
)

// TemplateInfo represents an E2B template.
type TemplateInfo struct {
	// ID is the unique identifier of the template.
	ID string `json:"templateID"`
	// Aliases are the template aliases.
	Aliases []string `json:"aliases"`
	// BuildID is the identifier of the last successful build.
	BuildID string `json:"buildID"`
	// BuildStatus is the current build status.
	BuildStatus TemplateBuildStatus `json:"buildStatus"`
	// BuildCount is the number of times the template was built.
	BuildCount int `json:"buildCount"`
	// SpawnCount is the number of times the template was used.
	SpawnCount int `json:"spawnCount"`
	// CPUCount is the number of CPU cores for the sandbox.
	CPUCount int `json:"cpuCount"`
	// MemoryMB is the memory for the sandbox in MiB.
	MemoryMB int `json:"memoryMB"`
	// DiskSizeMB is the disk size for the sandbox in MiB.
	DiskSizeMB int `json:"diskSizeMB"`
	// EnvdVersion is the version of the envd running in the sandbox.
	EnvdVersion string `json:"envdVersion"`
	// Public indicates whether the template is public.
	Public bool `json:"public"`
	// CreatedAt is when the template was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the template was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
	// LastSpawnedAt is when the template was last used (can be nil).
	LastSpawnedAt *time.Time `json:"lastSpawnedAt"`
	// CreatedBy contains information about who created the template.
	CreatedBy *TeamUser `json:"createdBy"`
}

// TeamUser represents a team user who created a template.
type TeamUser struct {
	// ID is the user ID.
	ID string `json:"id"`
	// Email is the user's email.
	Email string `json:"email"`
}

// TemplateBuild represents a single template build.
type TemplateBuild struct {
	// BuildID is the unique identifier of the build.
	BuildID string `json:"buildID"`
	// Status is the current build status.
	Status TemplateBuildStatus `json:"status"`
	// CPUCount is the number of CPU cores.
	CPUCount int `json:"cpuCount"`
	// MemoryMB is the memory in MiB.
	MemoryMB int `json:"memoryMB"`
	// DiskSizeMB is the disk size in MiB (optional).
	DiskSizeMB int `json:"diskSizeMB,omitempty"`
	// EnvdVersion is the envd version (optional).
	EnvdVersion string `json:"envdVersion,omitempty"`
	// CreatedAt is when the build was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the build was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
	// FinishedAt is when the build finished (optional).
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

// TemplateWithBuilds represents a template with its build history.
type TemplateWithBuilds struct {
	// ID is the unique identifier of the template.
	ID string `json:"templateID"`
	// Aliases are the template aliases.
	Aliases []string `json:"aliases"`
	// Builds is the list of builds for this template.
	Builds []TemplateBuild `json:"builds"`
	// Public indicates whether the template is public.
	Public bool `json:"public"`
	// SpawnCount is the number of times the template was used.
	SpawnCount int `json:"spawnCount"`
	// CreatedAt is when the template was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is when the template was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
	// LastSpawnedAt is when the template was last used (can be nil).
	LastSpawnedAt *time.Time `json:"lastSpawnedAt"`
}

// BuildLogEntry represents a log entry from the build process.
type BuildLogEntry struct {
	// Level is the log level.
	Level LogLevel `json:"level"`
	// Message is the log message content.
	Message string `json:"message"`
	// Timestamp is when the log entry was created.
	Timestamp time.Time `json:"timestamp"`
	// Step is the build step related to this log entry (optional).
	Step string `json:"step,omitempty"`
}

// BuildStatusReason contains details about the build status.
type BuildStatusReason struct {
	// Message is the status reason message.
	Message string `json:"message"`
	// Step is the step that failed (optional).
	Step string `json:"step,omitempty"`
	// LogEntries are log entries related to the status reason.
	LogEntries []BuildLogEntry `json:"logEntries,omitempty"`
}

// TemplateBuildInfo contains information about a build status query.
type TemplateBuildInfo struct {
	// TemplateID is the template identifier.
	TemplateID string `json:"templateID"`
	// BuildID is the build identifier.
	BuildID string `json:"buildID"`
	// Status is the current build status.
	Status TemplateBuildStatus `json:"status"`
	// Logs is a list of log messages (deprecated, use LogEntries).
	Logs []string `json:"logs"`
	// LogEntries is a list of structured log entries.
	LogEntries []BuildLogEntry `json:"logEntries"`
	// Reason contains details about the build status (for errors).
	Reason *BuildStatusReason `json:"reason,omitempty"`
}

// TemplateStep represents a step in the template build process.
type TemplateStep struct {
	// Type is the instruction type (COPY, RUN, ENV, WORKDIR, USER).
	Type string `json:"type"`
	// Args are the arguments for the step.
	Args []string `json:"args,omitempty"`
	// FilesHash is the hash of files used in COPY steps.
	FilesHash string `json:"filesHash,omitempty"`
	// Force indicates whether to force rebuild regardless of cache.
	Force bool `json:"force,omitempty"`
}

// RegistryType represents the type of Docker registry.
type RegistryType string

const (
	// RegistryTypeGeneral is for generic Docker registries.
	RegistryTypeGeneral RegistryType = "registry"
	// RegistryTypeAWS is for AWS ECR.
	RegistryTypeAWS RegistryType = "aws"
	// RegistryTypeGCP is for Google Container Registry.
	RegistryTypeGCP RegistryType = "gcp"
)

// RegistryConfig is an interface for registry configurations.
type RegistryConfig interface {
	registryConfig()
	// MarshalJSON returns the JSON encoding of the registry config.
	MarshalJSON() ([]byte, error)
}

// GeneralRegistry represents credentials for a generic Docker registry.
type GeneralRegistry struct {
	// Username is the registry username.
	Username string `json:"username"`
	// Password is the registry password.
	Password string `json:"password"`
}

func (r *GeneralRegistry) registryConfig() { /* marker method for RegistryConfig interface */ }

// MarshalJSON implements json.Marshaler for GeneralRegistry.
func (r *GeneralRegistry) MarshalJSON() ([]byte, error) {
	type alias GeneralRegistry
	return json.Marshal(&struct {
		Type RegistryType `json:"type"`
		*alias
	}{
		Type:  RegistryTypeGeneral,
		alias: (*alias)(r),
	})
}

// AWSRegistry represents credentials for AWS ECR.
type AWSRegistry struct {
	// AccessKeyID is the AWS access key ID.
	AccessKeyID string `json:"awsAccessKeyId"`
	// SecretAccessKey is the AWS secret access key.
	SecretAccessKey string `json:"awsSecretAccessKey"`
	// Region is the AWS region.
	Region string `json:"awsRegion"`
}

func (r *AWSRegistry) registryConfig() { /* marker method for RegistryConfig interface */ }

// MarshalJSON implements json.Marshaler for AWSRegistry.
func (r *AWSRegistry) MarshalJSON() ([]byte, error) {
	type alias AWSRegistry
	return json.Marshal(&struct {
		Type RegistryType `json:"type"`
		*alias
	}{
		Type:  RegistryTypeAWS,
		alias: (*alias)(r),
	})
}

// GCPRegistry represents credentials for Google Container Registry.
type GCPRegistry struct {
	// ServiceAccountJSON is the service account JSON.
	ServiceAccountJSON string `json:"serviceAccountJson"`
}

func (r *GCPRegistry) registryConfig() { /* marker method for RegistryConfig interface */ }

// MarshalJSON implements json.Marshaler for GCPRegistry.
func (r *GCPRegistry) MarshalJSON() ([]byte, error) {
	type alias GCPRegistry
	return json.Marshal(&struct {
		Type RegistryType `json:"type"`
		*alias
	}{
		Type:  RegistryTypeGCP,
		alias: (*alias)(r),
	})
}

// BuildInfo contains information returned after requesting a build.
type BuildInfo struct {
	// TemplateID is the template identifier.
	TemplateID string `json:"templateID"`
	// BuildID is the build identifier.
	BuildID string `json:"buildID"`
	// Aliases are the template aliases.
	Aliases []string `json:"aliases"`
	// Public indicates whether the template is public.
	Public bool `json:"public"`
}

// FileUploadInfo contains information about a file upload URL.
type FileUploadInfo struct {
	// Present indicates whether the file is already cached.
	Present bool `json:"present"`
	// URL is the presigned upload URL (empty if Present is true).
	URL string `json:"url,omitempty"`
}

// TemplateUpdate contains fields for updating a template.
type TemplateUpdate struct {
	// Public sets whether the template should be public.
	Public *bool `json:"public,omitempty"`
}

// TemplateBuildSpec represents the specification for starting a build.
type TemplateBuildSpec struct {
	// FromImage is the base Docker image.
	FromImage string `json:"fromImage,omitempty"`
	// FromTemplate is the base E2B template.
	FromTemplate string `json:"fromTemplate,omitempty"`
	// FromImageRegistry is the registry configuration for private images.
	FromImageRegistry RegistryConfig `json:"fromImageRegistry,omitempty"`
	// StartCmd is the command to run on startup.
	StartCmd string `json:"startCmd,omitempty"`
	// ReadyCmd is the command to check readiness.
	ReadyCmd string `json:"readyCmd,omitempty"`
	// Steps is the list of build steps.
	Steps []TemplateStep `json:"steps,omitempty"`
	// Force indicates whether to force rebuild regardless of cache.
	Force bool `json:"force,omitempty"`
}

// MarshalJSON implements json.Marshaler for TemplateBuildSpec.
func (s *TemplateBuildSpec) MarshalJSON() ([]byte, error) {
	type Alias TemplateBuildSpec
	aux := &struct {
		FromImageRegistry json.RawMessage `json:"fromImageRegistry,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}

	if s.FromImageRegistry != nil {
		data, err := s.FromImageRegistry.MarshalJSON()
		if err != nil {
			return nil, err
		}
		aux.FromImageRegistry = data
	}

	return json.Marshal(aux)
}

// templateBuildRequest represents the request body for POST /v3/templates.
type templateBuildRequest struct {
	Alias    string   `json:"alias,omitempty"`
	Names    []string `json:"names,omitempty"`
	CPUCount int      `json:"cpuCount,omitempty"`
	MemoryMB int      `json:"memoryMB,omitempty"`
	TeamID   string   `json:"teamID,omitempty"`
}

// templateBuildResponse represents the response from POST /v3/templates.
type templateBuildResponse struct {
	TemplateID string   `json:"templateID"`
	BuildID    string   `json:"buildID"`
	Aliases    []string `json:"aliases"`
	Public     bool     `json:"public"`
}

// templateAliasResponse represents the response from GET /templates/aliases/{alias}.
type templateAliasResponse struct {
	TemplateID string `json:"templateID"`
}
