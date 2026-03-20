package process

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Config holds the process configuration for building CLI args.
type Config struct {
	Streaming                       bool
	Model                           string
	SystemPrompt                    any    // string, or struct{Type,Preset,Append}
	Cwd                             string
	AllowedTools                    []string
	DisallowedTools                 []string
	Tools                           any // []string or struct{Type,Preset}
	PermissionMode                  string
	AllowDangerouslySkipPermissions bool
	PermissionPromptTool            string
	MaxTurns                        *int
	MaxBudgetUSD                    *float64
	Thinking                        any // ThinkingConfig
	Effort                          string
	MaxThinkingTokens               *int
	IncludePartialMessages          bool
	MCPServers                      map[string]any
	Agents                          map[string]any
	Hooks                           map[string]any
	Resume                          string
	ResumeAt                        string
	SessionID                       string
	ForkSession                     bool
	Continue                        bool
	NoPersistSession                bool
	OutputFormat                    any
	Verbose                         bool
	Debug                           bool
	DebugFile                       string
	SettingSources                  []string
	Settings                        any
	Plugins                         []any
	Betas                           []string
	AdditionalDirs                  []string
	ExtraArgs                       map[string]string
	FallbackModel                   string
	PromptSuggestions               bool
	AgentProgressSummaries          bool
	StrictMCPConfig                 bool
	Sandbox                         any
	AgentName                       string
}

// BuildArgs constructs the CLI arguments from config.
// Matches the official Python SDK's _build_command() behavior:
// always uses --output-format stream-json --verbose --input-format stream-json.
// Never uses --print (deprecated for stream-json mode).
func BuildArgs(cfg Config) []string {
	args := []string{"--output-format", "stream-json", "--verbose", "--input-format", "stream-json"}

	if cfg.Streaming {
		args = append(args, "--replay-user-messages")
	}

	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	args = appendSystemPrompt(args, cfg.SystemPrompt)

	// Note: cwd is set via cmd.Dir in process.Start(), not as a CLI flag.

	if len(cfg.AllowedTools) > 0 {
		args = append(args, "--allowed-tools", strings.Join(cfg.AllowedTools, ","))
	}

	if len(cfg.DisallowedTools) > 0 {
		args = append(args, "--disallowed-tools", strings.Join(cfg.DisallowedTools, ","))
	}

	args = appendTools(args, cfg.Tools)

	if cfg.PermissionMode != "" {
		if cfg.PermissionMode == "bypassPermissions" && cfg.AllowDangerouslySkipPermissions {
			args = append(args, "--dangerously-skip-permissions")
		} else {
			args = append(args, "--permission-mode", cfg.PermissionMode)
		}
	}

	if cfg.PermissionPromptTool != "" {
		args = append(args, "--permission-prompt-tool", cfg.PermissionPromptTool)
	}

	if cfg.MaxTurns != nil {
		args = append(args, "--max-turns", fmt.Sprintf("%d", *cfg.MaxTurns))
	}

	if cfg.MaxBudgetUSD != nil {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%g", *cfg.MaxBudgetUSD))
	}

	if cfg.Thinking != nil {
		if data, err := json.Marshal(cfg.Thinking); err == nil {
			args = append(args, "--thinking", string(data))
		}
	}

	if cfg.Effort != "" {
		args = append(args, "--effort", cfg.Effort)
	}

	if cfg.MaxThinkingTokens != nil {
		args = append(args, "--max-thinking-tokens", fmt.Sprintf("%d", *cfg.MaxThinkingTokens))
	}

	if cfg.IncludePartialMessages {
		args = append(args, "--include-partial-messages")
	}

	if len(cfg.MCPServers) > 0 {
		if data, err := json.Marshal(cfg.MCPServers); err == nil {
			args = append(args, "--mcp-servers", string(data))
		}
	}

	if len(cfg.Agents) > 0 {
		if data, err := json.Marshal(cfg.Agents); err == nil {
			args = append(args, "--agents", string(data))
		}
	}

	if len(cfg.Hooks) > 0 {
		if data, err := json.Marshal(cfg.Hooks); err == nil {
			args = append(args, "--hooks", string(data))
		}
	}

	if cfg.Resume != "" {
		args = append(args, "--resume", cfg.Resume)
	}

	if cfg.ResumeAt != "" {
		args = append(args, "--resume-session-at", cfg.ResumeAt)
	}

	if cfg.SessionID != "" {
		args = append(args, "--session-id", cfg.SessionID)
	}

	if cfg.ForkSession {
		args = append(args, "--fork-session")
	}

	if cfg.Continue {
		args = append(args, "--continue")
	}

	if cfg.NoPersistSession {
		args = append(args, "--no-session-persistence")
	}

	if cfg.OutputFormat != nil {
		// --json-schema expects the raw schema, not the {type, schema} wrapper.
		// Extract the "schema" field if present, otherwise pass as-is.
		if data, err := json.Marshal(cfg.OutputFormat); err == nil {
			var wrapper struct {
				Schema json.RawMessage `json:"schema"`
			}
			if json.Unmarshal(data, &wrapper) == nil && len(wrapper.Schema) > 0 {
				args = append(args, "--json-schema", string(wrapper.Schema))
			} else {
				args = append(args, "--json-schema", string(data))
			}
		}
	}

	if cfg.Verbose {
		args = append(args, "--verbose")
	}

	if cfg.DebugFile != "" {
		args = append(args, "--debug-file", cfg.DebugFile)
	} else if cfg.Debug {
		args = append(args, "--debug")
	}

	// Always pass --setting-sources (matching Python SDK).
	// Empty string means "load no settings" (SDK isolation mode).
	args = append(args, "--setting-sources", strings.Join(cfg.SettingSources, ","))

	if cfg.Settings != nil {
		switch v := cfg.Settings.(type) {
		case string:
			args = append(args, "--settings", v)
		default:
			if data, err := json.Marshal(v); err == nil {
				args = append(args, "--settings", string(data))
			}
		}
	}

	if len(cfg.Plugins) > 0 {
		if data, err := json.Marshal(cfg.Plugins); err == nil {
			args = append(args, "--plugins", string(data))
		}
	}

	if len(cfg.Betas) > 0 {
		args = append(args, "--betas", strings.Join(cfg.Betas, ","))
	}

	if len(cfg.AdditionalDirs) > 0 {
		args = append(args, "--additional-directories", strings.Join(cfg.AdditionalDirs, ","))
	}

	if cfg.FallbackModel != "" {
		args = append(args, "--fallback-model", cfg.FallbackModel)
	}

	if cfg.PromptSuggestions {
		args = append(args, "--prompt-suggestions")
	}

	if cfg.AgentProgressSummaries {
		args = append(args, "--agent-progress-summaries")
	}

	if cfg.StrictMCPConfig {
		args = append(args, "--strict-mcp-config")
	}

	if cfg.Sandbox != nil {
		if data, err := json.Marshal(cfg.Sandbox); err == nil {
			args = append(args, "--sandbox", string(data))
		}
	}

	if cfg.AgentName != "" {
		args = append(args, "--agent", cfg.AgentName)
	}

	// Extra args.
	for k, v := range cfg.ExtraArgs {
		if v == "" {
			args = append(args, "--"+k)
		} else {
			args = append(args, "--"+k, v)
		}
	}

	return args
}

func appendSystemPrompt(args []string, sp any) []string {
	if sp == nil {
		return args
	}

	switch v := sp.(type) {
	case string:
		if v != "" {
			args = append(args, "--system-prompt", v)
		}
	default:
		// Preset struct: {Text, Preset, Append}
		if data, err := json.Marshal(v); err == nil {
			var obj struct {
				Text   string `json:"text"`
				Preset bool   `json:"preset"`
				Append string `json:"append"`
			}
			if json.Unmarshal(data, &obj) == nil {
				if obj.Preset {
					args = append(args, "--system-prompt-preset", "claude_code")
					if obj.Append != "" {
						args = append(args, "--system-prompt-append", obj.Append)
					}
				} else if obj.Text != "" {
					args = append(args, "--system-prompt", obj.Text)
				}
			}
		}
	}

	return args
}

func appendTools(args []string, tools any) []string {
	if tools == nil {
		return args
	}

	switch v := tools.(type) {
	case []string:
		if len(v) > 0 {
			args = append(args, "--tools", strings.Join(v, ","))
		}
	default:
		if data, err := json.Marshal(v); err == nil {
			var obj struct {
				Preset bool     `json:"preset"`
				Names  []string `json:"names"`
			}
			if json.Unmarshal(data, &obj) == nil {
				if obj.Preset {
					args = append(args, "--tools-preset", "claude_code")
				} else if len(obj.Names) > 0 {
					args = append(args, "--tools", strings.Join(obj.Names, ","))
				}
			}
		}
	}

	return args
}
