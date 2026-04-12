package claude

// Option configures a Prompt or Session.
type Option func(*Config)

// Config holds all configuration for a Claude Code session.
type Config struct {
	CLIPath                         string
	Model                           string
	SystemPrompt                    *SystemPromptConfig
	Cwd                             string
	Env                             map[string]string
	AllowedTools                    []string
	DisallowedTools                 []string
	Tools                           *ToolsConfig
	PermissionMode                  PermissionMode
	AllowDangerouslySkipPermissions bool
	CanUseTool                      CanUseToolFunc
	MaxTurns                        *int
	MaxBudgetUSD                    *float64
	Thinking                        *ThinkingConfig
	Effort                          string
	MaxThinkingTokens               *int
	IncludePartialMessages          bool
	MCPServers                      map[string]MCPServerConfig
	SdkMcpServers                   map[string]*SdkMcpServer
	Hooks                           map[HookEvent][]HookCallbackMatcher
	Agents                          map[string]AgentDefinition
	Resume                          string
	ResumeAt                        string
	SessionID                       string
	ForkSession                     bool
	Continue                        bool
	NoPersistSession                bool
	OutputFormat                    *OutputFormat
	Verbose                         bool
	Debug                           bool
	DebugFile                       string
	SettingSources                  []SettingSource
	Settings                        any // string (path) or map
	Plugins                         []PluginConfig
	Betas                           []string
	AdditionalDirectories           []string
	ExtraArgs                       map[string]string
	FallbackModel                   string
	StderrCallback                  func(string)
	PromptSuggestions               bool
	AgentProgressSummaries          bool
	StrictMCPConfig                 bool
	Sandbox                         *SandboxSettings
	TaskBudget                      *int
	SpawnProcess                    SpawnProcessFunc
	AgentName                       string
}

// PermissionMode controls how tool executions are handled.
type PermissionMode string

const (
	PermissionDefault           PermissionMode = "default"
	PermissionAcceptEdits       PermissionMode = "acceptEdits"
	PermissionBypassPermissions PermissionMode = "bypassPermissions"
	PermissionPlan              PermissionMode = "plan"
	PermissionDontAsk           PermissionMode = "dontAsk"
	PermissionAuto              PermissionMode = "auto"
)

// SystemPromptConfig holds system prompt configuration.
type SystemPromptConfig struct {
	// Text is a custom system prompt string.
	Text string
	// Preset uses the Claude Code default prompt.
	Preset bool
	// Append is appended to the preset prompt.
	Append string
}

// ToolsConfig configures which built-in tools are available.
type ToolsConfig struct {
	// Names is a list of specific tool names. Empty disables all built-in tools.
	Names []string
	// Preset uses all default Claude Code tools.
	Preset bool
}

// ThinkingConfig controls Claude's reasoning behavior.
type ThinkingConfig struct {
	Type         string `json:"type"` // adaptive, enabled, disabled
	BudgetTokens int    `json:"budgetTokens,omitempty"`
}

// OutputFormat configures structured output.
type OutputFormat struct {
	Type   string         `json:"type"` // json_schema
	Schema map[string]any `json:"schema"`
}

// SettingSource identifies which settings files to load.
type SettingSource string

const (
	SettingSourceUser    SettingSource = "user"
	SettingSourceProject SettingSource = "project"
	SettingSourceLocal   SettingSource = "local"
)

// PluginConfig configures a plugin.
type PluginConfig struct {
	Type string `json:"type"` // local
	Path string `json:"path"`
}

// SandboxSettings configures command execution isolation.
type SandboxSettings struct {
	Enabled                    bool            `json:"enabled"`
	AutoAllowBashIfSandboxed   bool            `json:"autoAllowBashIfSandboxed,omitempty"`
	Network                    *SandboxNetwork `json:"network,omitempty"`
}

// SandboxNetwork configures sandbox network options.
type SandboxNetwork struct {
	AllowLocalBinding bool     `json:"allowLocalBinding,omitempty"`
	AllowUnixSockets  []string `json:"allowUnixSockets,omitempty"`
}

// CanUseToolFunc is the permission callback signature.
type CanUseToolFunc func(toolName string, input map[string]any, opts CanUseToolOptions) (PermissionResult, error)

// CanUseToolOptions provides context for permission decisions.
type CanUseToolOptions struct {
	Suggestions    []any  // PermissionUpdate suggestions
	BlockedPath    string
	DecisionReason string
	Title          string
	DisplayName    string
	Description    string
	ToolUseID      string
	AgentID        string
}

// PermissionResult is returned by the permission callback.
type PermissionResult struct {
	Behavior           string         `json:"behavior"` // allow, deny
	UpdatedInput       map[string]any `json:"updatedInput,omitempty"`
	UpdatedPermissions []any          `json:"updatedPermissions,omitempty"`
	Message            string         `json:"message,omitempty"`
	Interrupt          bool           `json:"interrupt,omitempty"`
	ToolUseID          string         `json:"toolUseID,omitempty"`
}

// SpawnProcessFunc is a custom function to spawn the Claude Code process.
type SpawnProcessFunc func(opts SpawnOptions) SpawnedProcess

// SpawnOptions are passed to the custom spawn function.
type SpawnOptions struct {
	Command string
	Args    []string
	Cwd     string
	Env     map[string]string
}

// SpawnedProcess is the interface for a custom process.
type SpawnedProcess interface {
	Stdin() interface{ Write([]byte) (int, error) }
	Stdout() interface{ Read([]byte) (int, error) }
	Stderr() interface{ Read([]byte) (int, error) }
	Wait() error
	Kill() error
}

// --- Functional option constructors ---

func WithModel(model string) Option {
	return func(c *Config) { c.Model = model }
}

func WithSystemPrompt(prompt string) Option {
	return func(c *Config) { c.SystemPrompt = &SystemPromptConfig{Text: prompt} }
}

func WithSystemPromptPreset(appendText string) Option {
	return func(c *Config) {
		c.SystemPrompt = &SystemPromptConfig{Preset: true, Append: appendText}
	}
}

func WithCwd(cwd string) Option {
	return func(c *Config) { c.Cwd = cwd }
}

func WithCLIPath(path string) Option {
	return func(c *Config) { c.CLIPath = path }
}

func WithEnv(env map[string]string) Option {
	return func(c *Config) { c.Env = env }
}

func WithAllowedTools(tools ...string) Option {
	return func(c *Config) { c.AllowedTools = tools }
}

func WithDisallowedTools(tools ...string) Option {
	return func(c *Config) { c.DisallowedTools = tools }
}

func WithToolsPreset() Option {
	return func(c *Config) { c.Tools = &ToolsConfig{Preset: true} }
}

func WithToolNames(names ...string) Option {
	return func(c *Config) { c.Tools = &ToolsConfig{Names: names} }
}

func WithPermissionMode(mode PermissionMode) Option {
	return func(c *Config) { c.PermissionMode = mode }
}

func WithAllowDangerouslySkipPermissions() Option {
	return func(c *Config) { c.AllowDangerouslySkipPermissions = true }
}

func WithCanUseTool(fn CanUseToolFunc) Option {
	return func(c *Config) { c.CanUseTool = fn }
}

func WithMaxTurns(n int) Option {
	return func(c *Config) { c.MaxTurns = &n }
}

func WithMaxBudgetUSD(n float64) Option {
	return func(c *Config) { c.MaxBudgetUSD = &n }
}

// WithTaskBudget sets the maximum token budget for background tasks.
func WithTaskBudget(total int) Option {
	return func(c *Config) { c.TaskBudget = &total }
}

func WithThinking(cfg ThinkingConfig) Option {
	return func(c *Config) { c.Thinking = &cfg }
}

func WithEffort(effort string) Option {
	return func(c *Config) { c.Effort = effort }
}

func WithMaxThinkingTokens(n int) Option {
	return func(c *Config) { c.MaxThinkingTokens = &n }
}

func WithIncludePartialMessages() Option {
	return func(c *Config) { c.IncludePartialMessages = true }
}

func WithSdkMcpServer(name string, srv *SdkMcpServer) Option {
	return func(c *Config) {
		if c.SdkMcpServers == nil {
			c.SdkMcpServers = make(map[string]*SdkMcpServer)
		}
		c.SdkMcpServers[name] = srv
	}
}

func WithMCPServer(name string, cfg MCPServerConfig) Option {
	return func(c *Config) {
		if c.MCPServers == nil {
			c.MCPServers = make(map[string]MCPServerConfig)
		}
		c.MCPServers[name] = cfg
	}
}

func WithHook(event HookEvent, matcher HookCallbackMatcher) Option {
	return func(c *Config) {
		if c.Hooks == nil {
			c.Hooks = make(map[HookEvent][]HookCallbackMatcher)
		}
		c.Hooks[event] = append(c.Hooks[event], matcher)
	}
}

func WithAgent(name string, def AgentDefinition) Option {
	return func(c *Config) {
		if c.Agents == nil {
			c.Agents = make(map[string]AgentDefinition)
		}
		c.Agents[name] = def
	}
}

func WithResume(sessionID string) Option {
	return func(c *Config) { c.Resume = sessionID }
}

func WithResumeAt(messageUUID string) Option {
	return func(c *Config) { c.ResumeAt = messageUUID }
}

func WithSessionID(id string) Option {
	return func(c *Config) { c.SessionID = id }
}

func WithForkSession() Option {
	return func(c *Config) { c.ForkSession = true }
}

func WithContinue() Option {
	return func(c *Config) { c.Continue = true }
}

func WithNoPersistSession() Option {
	return func(c *Config) { c.NoPersistSession = true }
}

func WithOutputFormat(fmt OutputFormat) Option {
	return func(c *Config) { c.OutputFormat = &fmt }
}

func WithVerbose() Option {
	return func(c *Config) { c.Verbose = true }
}

func WithDebug() Option {
	return func(c *Config) { c.Debug = true }
}

func WithDebugFile(path string) Option {
	return func(c *Config) { c.DebugFile = path; c.Debug = true }
}

func WithSettingSources(sources ...SettingSource) Option {
	return func(c *Config) { c.SettingSources = sources }
}

func WithSettings(settings any) Option {
	return func(c *Config) { c.Settings = settings }
}

func WithPlugins(plugins ...PluginConfig) Option {
	return func(c *Config) { c.Plugins = plugins }
}

func WithBetas(betas ...string) Option {
	return func(c *Config) { c.Betas = betas }
}

func WithAdditionalDirectories(dirs ...string) Option {
	return func(c *Config) { c.AdditionalDirectories = dirs }
}

func WithExtraArgs(args map[string]string) Option {
	return func(c *Config) { c.ExtraArgs = args }
}

func WithFallbackModel(model string) Option {
	return func(c *Config) { c.FallbackModel = model }
}

func WithStderrCallback(fn func(string)) Option {
	return func(c *Config) { c.StderrCallback = fn }
}

func WithPromptSuggestions() Option {
	return func(c *Config) { c.PromptSuggestions = true }
}

func WithAgentProgressSummaries() Option {
	return func(c *Config) { c.AgentProgressSummaries = true }
}

func WithStrictMCPConfig() Option {
	return func(c *Config) { c.StrictMCPConfig = true }
}

func WithSandbox(s SandboxSettings) Option {
	return func(c *Config) { c.Sandbox = &s }
}

func WithSpawnProcess(fn SpawnProcessFunc) Option {
	return func(c *Config) { c.SpawnProcess = fn }
}

func WithAgentName(name string) Option {
	return func(c *Config) { c.AgentName = name }
}

// applyOptions creates a Config from a list of options.
func applyOptions(opts []Option) *Config {
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}
