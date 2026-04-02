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
	TaskBudget                      *int
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

	// Note: --replay-user-messages is NOT sent (matching Python SDK).
	// The CLI echoes user messages back without this flag.

	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	args = appendSystemPrompt(args, cfg.SystemPrompt)

	// Note: cwd is set via cmd.Dir in process.Start(), not as a CLI flag.

	if len(cfg.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(cfg.AllowedTools, ","))
	}

	if len(cfg.DisallowedTools) > 0 {
		args = append(args, "--disallowedTools", strings.Join(cfg.DisallowedTools, ","))
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
		// Use --mcp-config with {"mcpServers": {...}} format (matching Python SDK).
		wrapper := map[string]any{"mcpServers": cfg.MCPServers}
		if data, err := json.Marshal(wrapper); err == nil {
			args = append(args, "--mcp-config", string(data))
		}
	}

	// Agents are always sent via initialize request (matching Python/TypeScript SDK).
	// No --agents CLI flag needed.

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

	// Only pass --setting-sources when values are configured (matching Python SDK).
	if len(cfg.SettingSources) > 0 {
		args = append(args, "--setting-sources", strings.Join(cfg.SettingSources, ","))
	}

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

	// Plugins: each local plugin gets its own --plugin-dir flag (matching Python SDK).
	for _, p := range cfg.Plugins {
		if data, err := json.Marshal(p); err == nil {
			var plugin struct {
				Type string `json:"type"`
				Path string `json:"path"`
			}
			if json.Unmarshal(data, &plugin) == nil && plugin.Type == "local" && plugin.Path != "" {
				args = append(args, "--plugin-dir", plugin.Path)
			}
		}
	}

	if len(cfg.Betas) > 0 {
		args = append(args, "--betas", strings.Join(cfg.Betas, ","))
	}

	// Each additional dir gets its own --add-dir flag (matching Python SDK).
	for _, d := range cfg.AdditionalDirs {
		args = append(args, "--add-dir", d)
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

	// Sandbox is merged into --settings (matching Python SDK).
	if cfg.Sandbox != nil {
		sandboxData, _ := json.Marshal(cfg.Sandbox)
		var sandboxMap map[string]any
		if json.Unmarshal(sandboxData, &sandboxMap) == nil {
			merged := map[string]any{"sandbox": sandboxMap}
			// If settings is a JSON string, merge sandbox into it.
			if s, ok := cfg.Settings.(string); ok && s != "" && s[0] == '{' {
				var existing map[string]any
				if json.Unmarshal([]byte(s), &existing) == nil {
					existing["sandbox"] = sandboxMap
					merged = existing
				}
			}
			if data, err := json.Marshal(merged); err == nil {
				// Replace the --settings value if already present, otherwise add it.
				found := false
				for i, a := range args {
					if a == "--settings" && i+1 < len(args) {
						args[i+1] = string(data)
						found = true
						break
					}
				}
				if !found {
					args = append(args, "--settings", string(data))
				}
			}
		}
	}

	if cfg.TaskBudget != nil {
		args = append(args, "--task-budget", fmt.Sprintf("%d", *cfg.TaskBudget))
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
		// Always pass --system-prompt (matching Python SDK).
		// Empty string means "no custom system prompt".
		args = append(args, "--system-prompt", "")
		return args
	}

	switch v := sp.(type) {
	case string:
		args = append(args, "--system-prompt", v)
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
					// Matching Python SDK: preset without append sends nothing;
					// preset with append sends only --append-system-prompt.
					if obj.Append != "" {
						args = append(args, "--append-system-prompt", obj.Append)
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
		// Empty array = "--tools ''" (disables all tools), matching Python SDK.
		args = append(args, "--tools", strings.Join(v, ","))
	default:
		if data, err := json.Marshal(v); err == nil {
			var obj struct {
				Preset bool     `json:"preset"`
				Names  []string `json:"names"`
			}
			if json.Unmarshal(data, &obj) == nil {
				if obj.Preset {
					// 'claude_code' preset maps to 'default' (matching Python SDK).
					args = append(args, "--tools", "default")
				} else if len(obj.Names) > 0 {
					args = append(args, "--tools", strings.Join(obj.Names, ","))
				}
			}
		}
	}

	return args
}
