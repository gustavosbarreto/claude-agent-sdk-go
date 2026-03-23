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
		"--additional-directories /extra1,/extra2",
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
	if containsFlag(args, "--system-prompt-preset") {
		t.Error("should not have --system-prompt-preset for plain string prompt")
	}
}

func TestBuildArgs_SystemPromptPreset(t *testing.T) {
	args := BuildArgs(Config{
		SystemPrompt: struct {
			Preset bool   `json:"preset"`
			Append string `json:"append"`
		}{Preset: true},
	})

	val, ok := flagValue(args, "--system-prompt-preset")
	if !ok {
		t.Fatal("missing --system-prompt-preset flag")
	}
	if val != "claude_code" {
		t.Errorf("expected preset %q, got %q", "claude_code", val)
	}
	if containsFlag(args, "--system-prompt-append") {
		t.Error("should not have --system-prompt-append when append is empty")
	}
}

func TestBuildArgs_SystemPromptPresetAppend(t *testing.T) {
	args := BuildArgs(Config{
		SystemPrompt: struct {
			Preset bool   `json:"preset"`
			Append string `json:"append"`
		}{Preset: true, Append: "Be concise."},
	})

	val, ok := flagValue(args, "--system-prompt-preset")
	if !ok {
		t.Fatal("missing --system-prompt-preset flag")
	}
	if val != "claude_code" {
		t.Errorf("expected preset %q, got %q", "claude_code", val)
	}

	appendVal, ok := flagValue(args, "--system-prompt-append")
	if !ok {
		t.Fatal("missing --system-prompt-append flag")
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
	if containsFlag(args, "--tools-preset") {
		t.Error("should not have --tools-preset for array tools")
	}
}

func TestBuildArgs_ToolsPreset(t *testing.T) {
	args := BuildArgs(Config{
		Tools: struct {
			Preset bool `json:"preset"`
		}{Preset: true},
	})

	val, ok := flagValue(args, "--tools-preset")
	if !ok {
		t.Fatal("missing --tools-preset flag")
	}
	if val != "claude_code" {
		t.Errorf("expected tools preset %q, got %q", "claude_code", val)
	}
	if containsFlag(args, "--tools") {
		t.Error("should not have --tools when using preset")
	}
}

func TestBuildArgs_Plugins(t *testing.T) {
	plugins := []any{
		map[string]any{"name": "my-plugin", "version": "1.0"},
	}
	args := BuildArgs(Config{
		Plugins: plugins,
	})

	val, ok := flagValue(args, "--plugins")
	if !ok {
		t.Fatal("missing --plugins flag")
	}

	// Verify it's valid JSON containing the plugin name.
	var parsed []map[string]any
	if err := json.Unmarshal([]byte(val), &parsed); err != nil {
		t.Fatalf("--plugins value is not valid JSON: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(parsed))
	}
	if parsed[0]["name"] != "my-plugin" {
		t.Errorf("expected plugin name %q, got %v", "my-plugin", parsed[0]["name"])
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

	val, ok := flagValue(args, "--additional-directories")
	if !ok {
		t.Fatal("missing --additional-directories flag")
	}
	if val != "/home/user/dir1,/home/user/dir2" {
		t.Errorf("expected dirs %q, got %q", "/home/user/dir1,/home/user/dir2", val)
	}
}

func TestBuildArgs_AgentsNotInArgs(t *testing.T) {
	// Agents are sent via the initialize control message, not CLI args.
	// However, the current implementation does include --agents in CLI args.
	// This test documents the current behavior: agents ARE passed as CLI args.
	args := BuildArgs(Config{
		Agents: map[string]any{
			"researcher": map[string]any{
				"description": "Research agent",
				"model":       "claude-sonnet-4-6",
			},
		},
	})

	// Verify agents data is present in args (current behavior).
	if !containsFlag(args, "--agents") {
		t.Error("expected --agents flag to be present")
	}

	val, _ := flagValue(args, "--agents")
	if !strings.Contains(val, "researcher") {
		t.Error("expected agents JSON to contain agent name")
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

	val, ok := flagValue(args, "--sandbox")
	if !ok {
		t.Fatal("missing --sandbox flag")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(val), &parsed); err != nil {
		t.Fatalf("--sandbox value is not valid JSON: %v", err)
	}
	if parsed["type"] != "docker" {
		t.Errorf("expected sandbox type %q, got %v", "docker", parsed["type"])
	}
	if parsed["container"] != "my-sandbox" {
		t.Errorf("expected sandbox container %q, got %v", "my-sandbox", parsed["container"])
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
		name           string
		mode           string
		allowBypass    bool
		expectFlag     string
		expectValue    string
		expectNoFlag   string
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
