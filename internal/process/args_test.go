package process

import (
	"strings"
	"testing"
)

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
	if !strings.Contains(s, "--replay-user-messages") {
		t.Error("missing --replay-user-messages")
	}
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
		"--cwd /workspace",
		"--allowed-tools Bash,Read",
		"--disallowed-tools Write",
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
	if !strings.Contains(s, "--mcp-servers") {
		t.Error("missing --mcp-servers")
	}
	if !strings.Contains(s, "mydb") {
		t.Error("missing server name in --mcp-servers JSON")
	}
}
