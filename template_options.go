package e2b

import (
	"net/http"
	"time"
)

// templateConfig holds configuration for template API calls.
type templateConfig struct {
	apiKey         string
	accessToken    string
	domain         string
	apiURL         string
	httpClient     *http.Client
	requestTimeout time.Duration
	debug          bool
}

// defaultTemplateConfig returns the default template configuration.
func defaultTemplateConfig() *templateConfig {
	return &templateConfig{
		domain:         DefaultDomain,
		requestTimeout: DefaultRequestTimeout,
	}
}

// TemplateOption configures template API calls.
type TemplateOption func(*templateConfig)

// WithTemplateAPIKey sets the E2B API key for template operations.
// Defaults to E2B_API_KEY environment variable.
func WithTemplateAPIKey(key string) TemplateOption {
	return func(c *templateConfig) {
		c.apiKey = key
	}
}

// WithTemplateAccessToken sets the E2B access token for template operations.
// Defaults to E2B_ACCESS_TOKEN environment variable.
func WithTemplateAccessToken(token string) TemplateOption {
	return func(c *templateConfig) {
		c.accessToken = token
	}
}

// WithTemplateDomain sets the E2B domain for template operations.
// Defaults to E2B_DOMAIN environment variable or "e2b.app".
func WithTemplateDomain(domain string) TemplateOption {
	return func(c *templateConfig) {
		c.domain = domain
	}
}

// WithTemplateAPIURL sets the E2B API URL for template operations.
// Defaults to E2B_API_URL environment variable or "https://api.{domain}".
func WithTemplateAPIURL(url string) TemplateOption {
	return func(c *templateConfig) {
		c.apiURL = url
	}
}

// WithTemplateHTTPClient sets a custom HTTP client for template operations.
func WithTemplateHTTPClient(client *http.Client) TemplateOption {
	return func(c *templateConfig) {
		c.httpClient = client
	}
}

// WithTemplateRequestTimeout sets the timeout for template API requests.
// Defaults to 60 seconds.
func WithTemplateRequestTimeout(d time.Duration) TemplateOption {
	return func(c *templateConfig) {
		c.requestTimeout = d
	}
}

// WithTemplateDebug enables debug mode for template operations.
// In debug mode, the API connects to http://localhost:3000.
func WithTemplateDebug(debug bool) TemplateOption {
	return func(c *templateConfig) {
		c.debug = debug
	}
}

// buildConfig holds configuration for building a template.
type buildConfig struct {
	cpuCount       int
	memoryMB       int
	skipCache      bool
	logsRefreshMs  time.Duration
	onLogs         func(BuildLogEntry)
	teamID         string
	requestTimeout time.Duration
	pollInterval   time.Duration
	templateConfig *templateConfig
}

// defaultBuildConfig returns the default build configuration.
func defaultBuildConfig() *buildConfig {
	return &buildConfig{
		cpuCount:       DefaultTemplateCPU,
		memoryMB:       DefaultTemplateMemory,
		logsRefreshMs:  200 * time.Millisecond,
		pollInterval:   200 * time.Millisecond,
		requestTimeout: DefaultRequestTimeout,
	}
}

// BuildOption configures template building.
type BuildOption func(*buildConfig)

// WithBuildCPUCount sets the number of CPU cores for the template.
// Defaults to 2.
func WithBuildCPUCount(count int) BuildOption {
	return func(c *buildConfig) {
		c.cpuCount = count
	}
}

// WithBuildMemoryMB sets the memory in MiB for the template.
// Defaults to 1024.
func WithBuildMemoryMB(memoryMB int) BuildOption {
	return func(c *buildConfig) {
		c.memoryMB = memoryMB
	}
}

// WithBuildSkipCache forces a rebuild without using cache.
func WithBuildSkipCache(skip bool) BuildOption {
	return func(c *buildConfig) {
		c.skipCache = skip
	}
}

// WithBuildLogsRefresh sets how often to poll for build logs.
// Defaults to 200ms.
func WithBuildLogsRefresh(d time.Duration) BuildOption {
	return func(c *buildConfig) {
		c.logsRefreshMs = d
	}
}

// WithBuildOnLogs sets a callback to receive build log entries.
func WithBuildOnLogs(handler func(BuildLogEntry)) BuildOption {
	return func(c *buildConfig) {
		c.onLogs = handler
	}
}

// WithBuildTeamID sets the team ID for the build.
func WithBuildTeamID(teamID string) BuildOption {
	return func(c *buildConfig) {
		c.teamID = teamID
	}
}

// WithBuildRequestTimeout sets the timeout for build API requests.
func WithBuildRequestTimeout(d time.Duration) BuildOption {
	return func(c *buildConfig) {
		c.requestTimeout = d
	}
}

// WithBuildPollInterval sets the interval for polling build status.
// Defaults to 200ms.
func WithBuildPollInterval(d time.Duration) BuildOption {
	return func(c *buildConfig) {
		c.pollInterval = d
	}
}

// WithBuildTemplateOptions applies TemplateOptions to the build config.
func WithBuildTemplateOptions(opts ...TemplateOption) BuildOption {
	return func(c *buildConfig) {
		if c.templateConfig == nil {
			c.templateConfig = defaultTemplateConfig()
		}
		for _, opt := range opts {
			opt(c.templateConfig)
		}
	}
}

// templateBuilderConfig holds configuration for creating a TemplateBuilder.
type templateBuilderConfig struct {
	contextPath    string
	ignorePatterns []string
}

// defaultTemplateBuilderConfig returns the default template builder configuration.
func defaultTemplateBuilderConfig() *templateBuilderConfig {
	return &templateBuilderConfig{
		contextPath:    ".",
		ignorePatterns: []string{},
	}
}

// TemplateBuilderOption configures a TemplateBuilder.
type TemplateBuilderOption func(*templateBuilderConfig)

// WithBuilderContextPath sets the path to the directory containing files to be copied.
// Defaults to the current directory.
func WithBuilderContextPath(path string) TemplateBuilderOption {
	return func(c *templateBuilderConfig) {
		c.contextPath = path
	}
}

// WithBuilderIgnorePatterns sets glob patterns to ignore when copying files.
func WithBuilderIgnorePatterns(patterns ...string) TemplateBuilderOption {
	return func(c *templateBuilderConfig) {
		c.ignorePatterns = patterns
	}
}

// stepConfig holds configuration for a build step.
type stepConfig struct {
	user  string
	force bool
}

// defaultStepConfig returns the default step configuration.
func defaultStepConfig() *stepConfig {
	return &stepConfig{}
}

// StepOption configures a build step.
type StepOption func(*stepConfig)

// WithStepUser sets the user for running the step.
func WithStepUser(user string) StepOption {
	return func(c *stepConfig) {
		c.user = user
	}
}

// WithStepForce forces the step to run regardless of cache.
func WithStepForce(force bool) StepOption {
	return func(c *stepConfig) {
		c.force = force
	}
}

// copyConfig holds configuration for a copy operation.
type copyConfig struct {
	user            string
	mode            uint32
	forceUpload     bool
	resolveSymlinks bool
}

// defaultCopyConfig returns the default copy configuration.
func defaultCopyConfig() *copyConfig {
	return &copyConfig{
		resolveSymlinks: true,
	}
}

// CopyOption configures a copy operation.
type CopyOption func(*copyConfig)

// WithCopyUser sets the user/owner for copied files.
func WithCopyUser(user string) CopyOption {
	return func(c *copyConfig) {
		c.user = user
	}
}

// WithCopyMode sets the file mode (permissions) for copied files.
func WithCopyMode(mode uint32) CopyOption {
	return func(c *copyConfig) {
		c.mode = mode
	}
}

// WithCopyForceUpload forces re-upload even if cached.
func WithCopyForceUpload(force bool) CopyOption {
	return func(c *copyConfig) {
		c.forceUpload = force
	}
}

// WithCopyResolveSymlinks sets whether to resolve symlinks when copying.
// Defaults to true.
func WithCopyResolveSymlinks(resolve bool) CopyOption {
	return func(c *copyConfig) {
		c.resolveSymlinks = resolve
	}
}

// getBuildStatusConfig holds configuration for getting build status.
type getBuildStatusConfig struct {
	logsOffset int
	limit      int
	level      LogLevel
}

// defaultGetBuildStatusConfig returns the default get build status configuration.
func defaultGetBuildStatusConfig() *getBuildStatusConfig {
	return &getBuildStatusConfig{
		logsOffset: 0,
		limit:      100,
	}
}

// GetBuildStatusOption configures getting build status.
type GetBuildStatusOption func(*getBuildStatusConfig)

// WithLogsOffset sets the offset for retrieving logs.
func WithLogsOffset(offset int) GetBuildStatusOption {
	return func(c *getBuildStatusConfig) {
		c.logsOffset = offset
	}
}

// WithLogsLimit sets the maximum number of log entries to retrieve.
func WithLogsLimit(limit int) GetBuildStatusOption {
	return func(c *getBuildStatusConfig) {
		c.limit = limit
	}
}

// WithLogsLevel filters logs by level.
func WithLogsLevel(level LogLevel) GetBuildStatusOption {
	return func(c *getBuildStatusConfig) {
		c.level = level
	}
}

// listTemplatesConfig holds configuration for listing templates.
type listTemplatesConfig struct {
	teamID string
}

// defaultListTemplatesConfig returns the default list templates configuration.
func defaultListTemplatesConfig() *listTemplatesConfig {
	return &listTemplatesConfig{}
}

// ListTemplatesOption configures listing templates.
type ListTemplatesOption func(*listTemplatesConfig)

// WithListTeamID filters templates by team ID.
func WithListTeamID(teamID string) ListTemplatesOption {
	return func(c *listTemplatesConfig) {
		c.teamID = teamID
	}
}

// getTemplateConfig holds configuration for getting a template.
type getTemplateConfig struct {
	limit     int
	nextToken string
}

// defaultGetTemplateConfig returns the default get template configuration.
func defaultGetTemplateConfig() *getTemplateConfig {
	return &getTemplateConfig{
		limit: 100,
	}
}

// GetTemplateOption configures getting a template.
type GetTemplateOption func(*getTemplateConfig)

// WithGetTemplateLimit sets the maximum number of builds to retrieve.
func WithGetTemplateLimit(limit int) GetTemplateOption {
	return func(c *getTemplateConfig) {
		c.limit = limit
	}
}

// WithGetTemplateNextToken sets the pagination token for retrieving builds.
func WithGetTemplateNextToken(token string) GetTemplateOption {
	return func(c *getTemplateConfig) {
		c.nextToken = token
	}
}
