package process

import (
	"encoding/json"
	"strings"
	"testing"
)

// containsFlag checks whether the flag (e.g. "--model") appears in the args slice.
func containsFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// flagValue returns the value immediately following the flag in the args slice.
// Returns ("", false) if the flag is not found or has no following value.
func flagValue(args []string, flag string) (string, bool) {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

// flagValues returns all values following occurrences of the flag in the args slice.
func flagValues(args []string, flag string) []string {
	var vals []string
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			vals = append(vals, args[i+1])
		}
	}
	return vals
}

func TestBuildArgs_OneShot(t *testing.T) {
	maxTurns := 5
	args := BuildArgs(Config{
		Streaming: false,
		Model:     "claude-sonnet-4-6",
		MaxTurns:  &maxTurns,
	})

	s := strings.Join(args, " ")

	// Always uses streaming format (prompt sent via stdin).
	if !strings.Contains(s, "--output-format stream-json") {
		t.Error("missing --output-format")
	}
	if !strings.Contains(s, "--input-format stream-json") {
		t.Error("missing --input-format")
	}
	if !strings.Contains(s, "--verbose") {
		t.Error("missing --verbose")
	}
	if !strings.Contains(s, "--model claude-sonnet-4-6") {
		t.Error("missing --model")
	}
	if !strings.Contains(s, "--max-turns 5") {
		t.Error("missing --max-turns")
	}
	if strings.Contains(s, "--print") {
		t.Error("should not use --print (deprecated for stream-json)")
	}
}

func TestBuildArgs_Streaming(t *testing.T) {
	args := BuildArgs(Config{
		Streaming: true,
		Model:     "claude-opus-4-6",
	})

	s := strings.Join(args, " ")

	if !strings.Contains(s, "--input-format stream-json") {
		t.Error("missing --input-format")
	}
	if !strings.Contains(s, "--output-format stream-json") {
		t.Error("missing --output-format")
	}
	if !strings.Contains(s, "--verbose") {
		t.Error("missing --verbose")
	}
	// --replay-user-messages is NOT sent (matching Python SDK).
}

func TestBuildArgs_BypassPermissions(t *testing.T) {
	args := BuildArgs(Config{
		PermissionMode:                  "bypassPermissions",
		AllowDangerouslySkipPermissions: true,
	})

	s := strings.Join(args, " ")

	if !strings.Contains(s, "--dangerously-skip-permissions") {
		t.Error("missing --dangerously-skip-permissions")
	}
	if strings.Contains(s, "--permission-mode") {
		t.Error("should not have --permission-mode when bypassing")
	}
}

func TestBuildArgs_AllOptions(t *testing.T) {
	maxTurns := 10
	maxBudget := 5.0
	maxThink := 8000

	args := BuildArgs(Config{
		Streaming:              true,
		Model:                  "claude-opus-4-6",
		Cwd:                    "/workspace",
		AllowedTools:           []string{"Bash", "Read"},
		DisallowedTools:        []string{"Write"},
		PermissionMode:         "acceptEdits",
		MaxTurns:               &maxTurns,
		MaxBudgetUSD:           &maxBudget,
		MaxThinkingTokens:      &maxThink,
		Effort:                 "high",
		IncludePartialMessages: true,
		Resume:                 "session-123",
		ResumeAt:               "msg-456",
		SessionID:              "sid-789",
		ForkSession:            true,
		NoPersistSession:       true,
		Verbose:                true,
		Debug:                  true,
		DebugFile:              "/tmp/debug.log",
		SettingSources:         []string{"user", "project"},
		Betas:                  []string{"context-1m-2025-08-07"},
		AdditionalDirs:         []string{"/extra1", "/extra2"},
		FallbackModel:          "claude-sonnet-4-6",
		PromptSuggestions:      true,
		AgentProgressSummaries: true,
		StrictMCPConfig:        true,
		AgentName:              "test-agent",
		ExtraArgs:              map[string]string{"custom-flag": "val"},
	})

	s := strings.Join(args, " ")

	checks := []string{
		"--model claude-opus-4-6",
		"--allowedTools Bash,Read",
		"--disallowedTools Write",
		"--permission-mode acceptEdits",
		"--max-turns 10",
		"--max-budget-usd 5",
		"--max-thinking-tokens 8000",
		"--effort high",
		"--include-partial-messages",
		"--resume session-123",
		"--resume-session-at msg-456",
		"--session-id sid-789",
		"--fork-session",
		"--no-session-persistence",
		"--verbose",
		"--debug-file /tmp/debug.log",
		"--setting-sources user,project",
		"--betas context-1m-2025-08-07",
		"--fallback-model claude-sonnet-4-6",
		"--prompt-suggestions",
		"--agent-progress-summaries",
		"--strict-mcp-config",
		"--agent test-agent",
		"--custom-flag val",
	}

	for _, check := range checks {
		if !strings.Contains(s, check) {
			t.Errorf("missing %q in args: %s", check, s)
		}
	}

	// Additional dirs use --add-dir per directory (not comma-separated).
	addDirVals := flagValues(args, "--add-dir")
	if len(addDirVals) != 2 {
		t.Errorf("expected 2 --add-dir flags, got %d", len(addDirVals))
	}
}

func TestBuildArgs_MCPServers(t *testing.T) {
	args := BuildArgs(Config{
		MCPServers: map[string]any{
			"mydb": map[string]any{
				"command": "npx",
				"args":    []string{"-y", "@mcp/postgres"},
			},
		},
	})

	s := strings.Join(args, " ")
	if !strings.Contains(s, "--mcp-config") {
		t.Error("missing --mcp-config")
	}
	if !strings.Contains(s, "mydb") {
		t.Error("missing server name in --mcp-config JSON")
	}
}

func TestBuildArgs_SystemPromptString(t *testing.T) {
	args := BuildArgs(Config{
		SystemPrompt: "Be helpful",
	})

	val, ok := flagValue(args, "--system-prompt")
	if !ok {
		t.Fatal("missing --system-prompt flag")
	}
	if val != "Be helpful" {
		t.Errorf("expected system prompt %q, got %q", "Be helpful", val)
	}
	if containsFlag(args, "--append-system-prompt") {
		t.Error("should not have --append-system-prompt for plain string prompt")
	}
}

func TestBuildArgs_SystemPromptPreset(t *testing.T) {
	// Matching Python SDK: preset without append sends no system prompt flags.
	args := BuildArgs(Config{
		SystemPrompt: struct {
			Preset bool   `json:"preset"`
			Append string `json:"append"`
		}{Preset: true},
	})

	if containsFlag(args, "--system-prompt") {
		t.Error("should not have --system-prompt for preset")
	}
	if containsFlag(args, "--append-system-prompt") {
		t.Error("should not have --append-system-prompt (not used by Python SDK)")
	}
	if containsFlag(args, "--append-system-prompt") {
		t.Error("should not have --append-system-prompt when append is empty")
	}
}

func TestBuildArgs_SystemPromptPresetAppend(t *testing.T) {
	// Matching Python SDK: preset with append sends only --append-system-prompt.
	args := BuildArgs(Config{
		SystemPrompt: struct {
			Preset bool   `json:"preset"`
			Append string `json:"append"`
		}{Preset: true, Append: "Be concise."},
	})

	if containsFlag(args, "--system-prompt") {
		t.Error("should not have --system-prompt for preset")
	}

	appendVal, ok := flagValue(args, "--append-system-prompt")
	if !ok {
		t.Fatal("missing --append-system-prompt flag")
	}
	if appendVal != "Be concise." {
		t.Errorf("expected append %q, got %q", "Be concise.", appendVal)
	}
}

func TestBuildArgs_SystemPromptEmpty(t *testing.T) {
	// When SystemPrompt is nil, BuildArgs should still emit --system-prompt ""
	args := BuildArgs(Config{})

	val, ok := flagValue(args, "--system-prompt")
	if !ok {
		t.Fatal("missing --system-prompt flag when no system prompt configured")
	}
	if val != "" {
		t.Errorf("expected empty system prompt, got %q", val)
	}
}

func TestBuildArgs_FallbackModel(t *testing.T) {
	args := BuildArgs(Config{
		FallbackModel: "sonnet",
	})

	val, ok := flagValue(args, "--fallback-model")
	if !ok {
		t.Fatal("missing --fallback-model flag")
	}
	if val != "sonnet" {
		t.Errorf("expected fallback model %q, got %q", "sonnet", val)
	}
}

func TestBuildArgs_MaxThinkingTokens(t *testing.T) {
	maxThink := 5000
	args := BuildArgs(Config{
		MaxThinkingTokens: &maxThink,
	})

	val, ok := flagValue(args, "--max-thinking-tokens")
	if !ok {
		t.Fatal("missing --max-thinking-tokens flag")
	}
	if val != "5000" {
		t.Errorf("expected max thinking tokens %q, got %q", "5000", val)
	}
}

func TestBuildArgs_SessionContinuation(t *testing.T) {
	args := BuildArgs(Config{
		Continue: true,
		Resume:   "session-123",
	})

	if !containsFlag(args, "--continue") {
		t.Error("missing --continue flag")
	}

	val, ok := flagValue(args, "--resume")
	if !ok {
		t.Fatal("missing --resume flag")
	}
	if val != "session-123" {
		t.Errorf("expected resume %q, got %q", "session-123", val)
	}
}

func TestBuildArgs_SettingsFile(t *testing.T) {
	args := BuildArgs(Config{
		Settings: "/path/to/settings.json",
	})

	val, ok := flagValue(args, "--settings")
	if !ok {
		t.Fatal("missing --settings flag")
	}
	if val != "/path/to/settings.json" {
		t.Errorf("expected settings path %q, got %q", "/path/to/settings.json", val)
	}
}

func TestBuildArgs_ExtraArgs(t *testing.T) {
	args := BuildArgs(Config{
		ExtraArgs: map[string]string{
			"custom-value-flag": "myval",
			"custom-bool-flag":  "",
		},
	})

	// Value flag.
	val, ok := flagValue(args, "--custom-value-flag")
	if !ok {
		t.Fatal("missing --custom-value-flag")
	}
	if val != "myval" {
		t.Errorf("expected %q, got %q", "myval", val)
	}

	// Boolean flag (empty value → flag only, no value).
	if !containsFlag(args, "--custom-bool-flag") {
		t.Error("missing --custom-bool-flag")
	}
	// Ensure boolean flag does NOT consume next arg as value.
	boolVal, _ := flagValue(args, "--custom-bool-flag")
	// If the next arg starts with "--", it's not a value for this flag.
	if boolVal != "" && !strings.HasPrefix(boolVal, "--") {
		t.Errorf("boolean flag should not have a value, got %q", boolVal)
	}
}

func TestBuildArgs_ToolsArray(t *testing.T) {
	args := BuildArgs(Config{
		Tools: []string{"Read", "Edit", "Bash"},
	})

	val, ok := flagValue(args, "--tools")
	if !ok {
		t.Fatal("missing --tools flag")
	}
	if val != "Read,Edit,Bash" {
		t.Errorf("expected tools %q, got %q", "Read,Edit,Bash", val)
	}
}

func TestBuildArgs_ToolsPreset(t *testing.T) {
	args := BuildArgs(Config{
		Tools: struct {
			Preset bool `json:"preset"`
		}{Preset: true},
	})

	val, ok := flagValue(args, "--tools")
	if !ok {
		t.Fatal("missing --tools flag")
	}
	if val != "default" {
		t.Errorf("expected tools %q, got %q", "default", val)
	}
}

func TestBuildArgs_ToolsEmptyArray(t *testing.T) {
	// Empty tools array emits --tools "" (disables all tools, matching Python SDK).
	args := BuildArgs(Config{
		Tools: []string{},
	})

	val, ok := flagValue(args, "--tools")
	if !ok {
		t.Fatal("missing --tools flag for empty tools array")
	}
	if val != "" {
		t.Errorf("expected empty tools value, got %q", val)
	}
}

func TestBuildArgs_ToolsNil(t *testing.T) {
	// Nil tools emits nothing.
	args := BuildArgs(Config{
		Tools: nil,
	})

	if containsFlag(args, "--tools") {
		t.Error("should not have --tools flag when tools is nil")
	}
}

func TestBuildArgs_Plugins(t *testing.T) {
	plugins := []any{
		map[string]any{"type": "local", "path": "/home/user/plugins/my-plugin"},
	}
	args := BuildArgs(Config{
		Plugins: plugins,
	})

	val, ok := flagValue(args, "--plugin-dir")
	if !ok {
		t.Fatal("missing --plugin-dir flag")
	}
	if val != "/home/user/plugins/my-plugin" {
		t.Errorf("expected plugin dir %q, got %q", "/home/user/plugins/my-plugin", val)
	}
	if containsFlag(args, "--plugins") {
		t.Error("should not have --plugins flag (use --plugin-dir per plugin)")
	}
}

func TestBuildArgs_Betas(t *testing.T) {
	args := BuildArgs(Config{
		Betas: []string{"context-1m-2025-08-07"},
	})

	val, ok := flagValue(args, "--betas")
	if !ok {
		t.Fatal("missing --betas flag")
	}
	if val != "context-1m-2025-08-07" {
		t.Errorf("expected betas %q, got %q", "context-1m-2025-08-07", val)
	}
}

func TestBuildArgs_AdditionalDirs(t *testing.T) {
	args := BuildArgs(Config{
		AdditionalDirs: []string{"/home/user/dir1", "/home/user/dir2"},
	})

	// Each dir gets its own --add-dir flag (matching Python SDK).
	vals := flagValues(args, "--add-dir")
	if len(vals) != 2 {
		t.Fatalf("expected 2 --add-dir flags, got %d", len(vals))
	}
	if vals[0] != "/home/user/dir1" {
		t.Errorf("expected first dir %q, got %q", "/home/user/dir1", vals[0])
	}
	if vals[1] != "/home/user/dir2" {
		t.Errorf("expected second dir %q, got %q", "/home/user/dir2", vals[1])
	}
	if containsFlag(args, "--additional-directories") {
		t.Error("should not have --additional-directories flag (use --add-dir per dir)")
	}
}

func TestBuildArgs_AgentsNotInArgs(t *testing.T) {
	// Agents are sent via the initialize control message, not CLI args.
	args := BuildArgs(Config{
		Agents: map[string]any{
			"researcher": map[string]any{
				"description": "Research agent",
				"model":       "claude-sonnet-4-6",
			},
		},
	})

	// Verify agents data is NOT present in args (matching Python/TypeScript SDK).
	if containsFlag(args, "--agents") {
		t.Error("--agents flag should not be present; agents are sent via initialize request")
	}
}

func TestBuildArgs_AlwaysStreaming(t *testing.T) {
	// Whether Streaming is true or false, args should always include
	// --input-format stream-json and --output-format stream-json, never --print.
	for _, streaming := range []bool{true, false} {
		args := BuildArgs(Config{Streaming: streaming})

		if !containsFlag(args, "--input-format") {
			t.Errorf("streaming=%v: missing --input-format", streaming)
		}
		val, _ := flagValue(args, "--input-format")
		if val != "stream-json" {
			t.Errorf("streaming=%v: expected --input-format stream-json, got %q", streaming, val)
		}

		if !containsFlag(args, "--output-format") {
			t.Errorf("streaming=%v: missing --output-format", streaming)
		}
		oval, _ := flagValue(args, "--output-format")
		if oval != "stream-json" {
			t.Errorf("streaming=%v: expected --output-format stream-json, got %q", streaming, oval)
		}

		if containsFlag(args, "--print") {
			t.Errorf("streaming=%v: should never use --print", streaming)
		}
	}
}

func TestBuildArgs_SettingSources(t *testing.T) {
	// With specific sources.
	args := BuildArgs(Config{
		SettingSources: []string{"user", "project"},
	})
	val, ok := flagValue(args, "--setting-sources")
	if !ok {
		t.Fatal("missing --setting-sources flag")
	}
	if val != "user,project" {
		t.Errorf("expected setting sources %q, got %q", "user,project", val)
	}

	// With no sources (empty slice → empty string, meaning no settings loaded).
	argsEmpty := BuildArgs(Config{})
	valEmpty, ok := flagValue(argsEmpty, "--setting-sources")
	if !ok {
		t.Fatal("--setting-sources should always be present even when empty")
	}
	if valEmpty != "" {
		t.Errorf("expected empty setting sources, got %q", valEmpty)
	}
}

func TestBuildArgs_Sandbox(t *testing.T) {
	sandbox := map[string]any{
		"type":      "docker",
		"container": "my-sandbox",
	}
	args := BuildArgs(Config{
		Sandbox: sandbox,
	})

	// Sandbox is merged into --settings (matching Python SDK).
	val, ok := flagValue(args, "--settings")
	if !ok {
		t.Fatal("missing --settings flag (sandbox should be merged into settings)")
	}
	if containsFlag(args, "--sandbox") {
		t.Error("should not have --sandbox flag (sandbox is merged into --settings)")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(val), &parsed); err != nil {
		t.Fatalf("--settings value is not valid JSON: %v", err)
	}
	sandboxVal, ok := parsed["sandbox"]
	if !ok {
		t.Fatal("--settings JSON missing 'sandbox' key")
	}
	sandboxMap, ok := sandboxVal.(map[string]any)
	if !ok {
		t.Fatal("sandbox value is not a map")
	}
	if sandboxMap["type"] != "docker" {
		t.Errorf("expected sandbox type %q, got %v", "docker", sandboxMap["type"])
	}
	if sandboxMap["container"] != "my-sandbox" {
		t.Errorf("expected sandbox container %q, got %v", "my-sandbox", sandboxMap["container"])
	}
}

func TestBuildArgs_SandboxMergedWithSettings(t *testing.T) {
	// When both Settings (as JSON string) and Sandbox are provided,
	// sandbox is merged into the existing settings object.
	args := BuildArgs(Config{
		Settings: `{"theme":"dark","fontSize":14}`,
		Sandbox: map[string]any{
			"enabled": true,
			"type":    "docker",
		},
	})

	val, ok := flagValue(args, "--settings")
	if !ok {
		t.Fatal("missing --settings flag")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(val), &parsed); err != nil {
		t.Fatalf("--settings value is not valid JSON: %v", err)
	}

	// Original settings keys should be preserved.
	if parsed["theme"] != "dark" {
		t.Errorf("expected theme %q, got %v", "dark", parsed["theme"])
	}
	if parsed["fontSize"] != float64(14) {
		t.Errorf("expected fontSize 14, got %v", parsed["fontSize"])
	}

	// Sandbox should be merged in.
	sandboxVal, ok := parsed["sandbox"]
	if !ok {
		t.Fatal("--settings JSON missing 'sandbox' key after merge")
	}
	sandboxMap, ok := sandboxVal.(map[string]any)
	if !ok {
		t.Fatal("sandbox value is not a map")
	}
	if sandboxMap["enabled"] != true {
		t.Errorf("expected sandbox enabled=true, got %v", sandboxMap["enabled"])
	}
	if sandboxMap["type"] != "docker" {
		t.Errorf("expected sandbox type %q, got %v", "docker", sandboxMap["type"])
	}
}

func TestBuildArgs_SandboxMinimal(t *testing.T) {
	// Minimal sandbox config {enabled: true} should produce --settings {"sandbox":{"enabled":true}}.
	args := BuildArgs(Config{
		Sandbox: map[string]any{
			"enabled": true,
		},
	})

	val, ok := flagValue(args, "--settings")
	if !ok {
		t.Fatal("missing --settings flag for minimal sandbox")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(val), &parsed); err != nil {
		t.Fatalf("--settings value is not valid JSON: %v", err)
	}

	sandboxVal, ok := parsed["sandbox"]
	if !ok {
		t.Fatal("--settings JSON missing 'sandbox' key")
	}
	sandboxMap, ok := sandboxVal.(map[string]any)
	if !ok {
		t.Fatal("sandbox value is not a map")
	}
	if sandboxMap["enabled"] != true {
		t.Errorf("expected sandbox enabled=true, got %v", sandboxMap["enabled"])
	}
	// Should only have the sandbox key at the top level.
	if len(parsed) != 1 {
		t.Errorf("expected only 'sandbox' key in settings, got %d keys", len(parsed))
	}
}

func TestBuildArgs_OutputFormat(t *testing.T) {
	// When OutputFormat contains a schema wrapper, --json-schema should extract the inner schema.
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"answer": map[string]any{"type": "string"},
		},
	}
	args := BuildArgs(Config{
		OutputFormat: map[string]any{
			"type":   "json",
			"schema": schema,
		},
	})

	val, ok := flagValue(args, "--json-schema")
	if !ok {
		t.Fatal("missing --json-schema flag")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(val), &parsed); err != nil {
		t.Fatalf("--json-schema value is not valid JSON: %v", err)
	}
	// Should be the inner schema, not the wrapper.
	if parsed["type"] != "object" {
		t.Errorf("expected schema type %q, got %v", "object", parsed["type"])
	}
	if _, hasSchema := parsed["schema"]; hasSchema {
		t.Error("--json-schema should contain the inner schema, not the wrapper")
	}
}

func TestBuildArgs_PermissionModes(t *testing.T) {
	tests := []struct {
		name         string
		mode         string
		allowBypass  bool
		expectFlag   string
		expectValue  string
		expectNoFlag string
	}{
		{
			name:         "default mode",
			mode:         "default",
			expectFlag:   "--permission-mode",
			expectValue:  "default",
			expectNoFlag: "--dangerously-skip-permissions",
		},
		{
			name:         "acceptEdits mode",
			mode:         "acceptEdits",
			expectFlag:   "--permission-mode",
			expectValue:  "acceptEdits",
			expectNoFlag: "--dangerously-skip-permissions",
		},
		{
			name:         "plan mode",
			mode:         "plan",
			expectFlag:   "--permission-mode",
			expectValue:  "plan",
			expectNoFlag: "--dangerously-skip-permissions",
		},
		{
			name:         "bypass without allow flag uses permission-mode",
			mode:         "bypassPermissions",
			allowBypass:  false,
			expectFlag:   "--permission-mode",
			expectValue:  "bypassPermissions",
			expectNoFlag: "--dangerously-skip-permissions",
		},
		{
			name:         "bypass with allow flag uses dangerously-skip",
			mode:         "bypassPermissions",
			allowBypass:  true,
			expectFlag:   "--dangerously-skip-permissions",
			expectNoFlag: "--permission-mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := BuildArgs(Config{
				PermissionMode:                  tt.mode,
				AllowDangerouslySkipPermissions: tt.allowBypass,
			})

			if !containsFlag(args, tt.expectFlag) {
				t.Errorf("expected flag %q to be present", tt.expectFlag)
			}
			if tt.expectValue != "" {
				val, ok := flagValue(args, tt.expectFlag)
				if !ok {
					t.Errorf("expected flag %q to have a value", tt.expectFlag)
				}
				if val != tt.expectValue {
					t.Errorf("expected %q=%q, got %q", tt.expectFlag, tt.expectValue, val)
				}
			}
			if tt.expectNoFlag != "" && containsFlag(args, tt.expectNoFlag) {
				t.Errorf("did not expect flag %q to be present", tt.expectNoFlag)
			}
		})
	}
}
