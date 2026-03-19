package claude_test

import (
	"context"
	"os"
	"testing"
	"time"

	claude "github.com/shellhub-io/claude-agent-sdk-go"
)

// E2E tests run against the real Claude CLI using subscription credits.
// Skipped unless CLAUDE_E2E=1 is set.
//
// Run: CLAUDE_E2E=1 go test -v -run TestE2E -count=1

func skipIfNoE2E(t *testing.T) {
	t.Helper()
	if os.Getenv("CLAUDE_E2E") == "" {
		t.Skip("set CLAUDE_E2E=1 to run e2e tests")
	}
}

func TestE2E_Prompt(t *testing.T) {
	skipIfNoE2E(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := claude.Prompt(ctx, "What is 2+2? Reply with just the number.",
		claude.WithMaxTurns(1),
		claude.WithPermissionMode(claude.PermissionBypassPermissions),
		claude.WithAllowDangerouslySkipPermissions(),
		claude.WithNoPersistSession(),
		claude.WithStderrCallback(func(s string) {
			t.Logf("[stderr] %s", s)
		}),
	)
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	if result.Subtype != claude.ResultSuccess {
		t.Errorf("subtype = %q, want success", result.Subtype)
	}
	if result.Result == "" {
		t.Error("empty result")
	}
	t.Logf("result=%q cost=$%.4f turns=%d", result.Result, result.TotalCostUSD, result.NumTurns)
}

func TestE2E_Query_MessageTypes(t *testing.T) {
	skipIfNoE2E(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var gotSystem, gotAssistant, gotResult bool

	for msg, err := range claude.Query(ctx, "Say hello in one word.",
		claude.WithMaxTurns(1),
		claude.WithPermissionMode(claude.PermissionBypassPermissions),
		claude.WithAllowDangerouslySkipPermissions(),
		claude.WithNoPersistSession(),
	) {
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		switch m := msg.(type) {
		case *claude.SystemMessage:
			gotSystem = true
			if m.SessionID == "" {
				t.Error("system message missing session_id")
			}
		case *claude.AssistantMessage:
			gotAssistant = true
			text := claude.CombinedText(m.Message.Content)
			t.Logf("assistant: %s", text)
		case *claude.ResultMessage:
			gotResult = true
			t.Logf("result: subtype=%s cost=$%.4f", m.Subtype, m.TotalCostUSD)
		}
	}

	if !gotSystem {
		t.Error("missing system message")
	}
	if !gotAssistant {
		t.Error("missing assistant message")
	}
	if !gotResult {
		t.Error("missing result message")
	}
}

func TestE2E_Session_MultiTurn(t *testing.T) {
	skipIfNoE2E(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx,
		claude.WithMaxTurns(3),
		claude.WithPermissionMode(claude.PermissionBypassPermissions),
		claude.WithAllowDangerouslySkipPermissions(),
		claude.WithNoPersistSession(),
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()

	// Turn 1
	var turn1 string
	for msg, err := range session.Send(ctx, "Pick a number between 1 and 10. Reply with just the number.") {
		if err != nil {
			t.Fatalf("turn 1: %v", err)
		}
		if a, ok := msg.(*claude.AssistantMessage); ok {
			turn1 = claude.CombinedText(a.Message.Content)
		}
	}
	t.Logf("turn 1: %s", turn1)

	if session.SessionID() == "" {
		t.Error("session ID empty after turn 1")
	}

	// Turn 2 — references turn 1
	var turn2 string
	for msg, err := range session.Send(ctx, "Add 5 to the number you just said. Reply with just the result.") {
		if err != nil {
			t.Fatalf("turn 2: %v", err)
		}
		if a, ok := msg.(*claude.AssistantMessage); ok {
			turn2 = claude.CombinedText(a.Message.Content)
		}
	}
	t.Logf("turn 2: %s", turn2)

	if turn2 == "" {
		t.Error("turn 2 empty — multi-turn context may not work")
	}
}

func TestE2E_Session_SetModel(t *testing.T) {
	skipIfNoE2E(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx,
		claude.WithPermissionMode(claude.PermissionBypassPermissions),
		claude.WithAllowDangerouslySkipPermissions(),
		claude.WithNoPersistSession(),
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()

	// Turn 1
	for _, err := range session.Send(ctx, "What is 1+1? Just the number.") {
		if err != nil {
			t.Fatalf("turn 1: %v", err)
		}
	}

	// Switch model
	if err := session.SetModel("claude-haiku-4-5"); err != nil {
		t.Logf("SetModel: %v (may not be supported)", err)
	}

	// Turn 2 with new model
	for msg, err := range session.Send(ctx, "What is 2+2? Just the number.") {
		if err != nil {
			t.Fatalf("turn 2: %v", err)
		}
		if r, ok := msg.(*claude.ResultMessage); ok {
			t.Logf("turn 2 result: %s", r.Result)
		}
	}
}

func TestE2E_StructuredOutput(t *testing.T) {
	skipIfNoE2E(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"answer":     map[string]any{"type": "number"},
			"confidence": map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}},
		},
		"required": []string{"answer", "confidence"},
	}

	result, err := claude.Prompt(ctx, "What is 7*8?",
		claude.WithOutputFormat(claude.OutputFormat{Type: "json_schema", Schema: schema}),
		claude.WithPermissionMode(claude.PermissionBypassPermissions),
		claude.WithAllowDangerouslySkipPermissions(),
		claude.WithNoPersistSession(),
	)
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	if result.Subtype != claude.ResultSuccess {
		t.Errorf("subtype = %q", result.Subtype)
	}
	if len(result.StructuredOutput) == 0 {
		t.Error("structured_output field is empty")
	} else {
		t.Logf("structured_output: %s", string(result.StructuredOutput))
	}
}

func TestE2E_ToolUse(t *testing.T) {
	skipIfNoE2E(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var sawToolUse bool

	for msg, err := range claude.Query(ctx, "List the files in /tmp using Bash. Just run: ls /tmp | head -5",
		claude.WithMaxTurns(3),
		claude.WithAllowedTools("Bash"),
		claude.WithPermissionMode(claude.PermissionBypassPermissions),
		claude.WithAllowDangerouslySkipPermissions(),
		claude.WithNoPersistSession(),
	) {
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if a, ok := msg.(*claude.AssistantMessage); ok {
			for _, block := range a.Message.Content {
				if block.Type == claude.ContentBlockToolUse {
					sawToolUse = true
					t.Logf("tool_use: %s", block.Name)
				}
			}
		}
	}

	if !sawToolUse {
		t.Error("no tool_use block seen — expected Bash usage")
	}
}

func TestE2E_Hook_PreToolUse(t *testing.T) {
	skipIfNoE2E(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	hookCalls := 0

	result, err := claude.Prompt(ctx, "Read the file /etc/hostname",
		claude.WithMaxTurns(3),
		claude.WithPermissionMode(claude.PermissionBypassPermissions),
		claude.WithAllowDangerouslySkipPermissions(),
		claude.WithNoPersistSession(),
		claude.WithHook(claude.HookPreToolUse, claude.HookCallbackMatcher{
			Hooks: []claude.HookCallback{
				func(ctx context.Context, input claude.HookInput) (claude.HookOutput, error) {
					hookCalls++
					t.Logf("hook: %s on %s", input.HookEventName, input.ToolName)
					return claude.HookOutput{}, nil
				},
			},
		}),
	)
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	t.Logf("result=%q hooks_fired=%d", result.Result, hookCalls)
}

func TestE2E_StderrCallback(t *testing.T) {
	skipIfNoE2E(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var stderrOutput []string

	_, err := claude.Prompt(ctx, "What is 1+1? Just the number.",
		claude.WithMaxTurns(1),
		claude.WithPermissionMode(claude.PermissionBypassPermissions),
		claude.WithAllowDangerouslySkipPermissions(),
		claude.WithNoPersistSession(),
		claude.WithStderrCallback(func(s string) {
			stderrOutput = append(stderrOutput, s)
		}),
	)
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	t.Logf("stderr lines: %d", len(stderrOutput))
}

func TestE2E_IncludePartialMessages(t *testing.T) {
	skipIfNoE2E(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var streamEvents int

	for msg, err := range claude.Query(ctx, "Write a haiku about Go programming.",
		claude.WithMaxTurns(1),
		claude.WithPermissionMode(claude.PermissionBypassPermissions),
		claude.WithAllowDangerouslySkipPermissions(),
		claude.WithNoPersistSession(),
		claude.WithIncludePartialMessages(),
	) {
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if _, ok := msg.(*claude.StreamEvent); ok {
			streamEvents++
		}
	}

	t.Logf("stream events: %d", streamEvents)
	if streamEvents == 0 {
		t.Error("no stream events with includePartialMessages")
	}
}
