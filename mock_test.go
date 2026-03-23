package claude_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	claude "github.com/shellhub-io/claude-agent-sdk-go"
	"github.com/shellhub-io/claude-agent-sdk-go/internal/testutil"
)

// Mock tests use shell scripts that simulate the CLI NDJSON protocol.
// No API key or real CLI needed.

func TestMock_Prompt(t *testing.T) {
	script, err := testutil.MockStreamingCLIScript(
		nil,
		[][]string{
			{testutil.SystemInit, testutil.AssistantText, testutil.ResultOK},
		},
	)
	if err != nil {
		t.Fatalf("MockStreamingCLIScript: %v", err)
	}
	defer os.Remove(script)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := claude.Prompt(ctx, "test",
		claude.WithCLIPath(script),
	)
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	if result.Subtype != claude.ResultSuccess {
		t.Errorf("subtype = %q, want success", result.Subtype)
	}
	if result.Result != "The answer is 4." {
		t.Errorf("result = %q", result.Result)
	}
}

func TestMock_Query(t *testing.T) {
	// Query() now always uses streaming mode (sends prompt via stdin).
	// Use a streaming mock that responds to user messages.
	script, err := testutil.MockStreamingCLIScript(
		nil, // no init lines before user message
		[][]string{
			{testutil.SystemInit, testutil.AssistantText, testutil.ResultOK},
		},
	)
	if err != nil {
		t.Fatalf("MockStreamingCLIScript: %v", err)
	}
	defer os.Remove(script)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var types []string
	for msg, err := range claude.Query(ctx, "test", claude.WithCLIPath(script)) {
		if err != nil {
			break
		}
		switch msg.(type) {
		case *claude.SystemMessage:
			types = append(types, "system")
		case *claude.AssistantMessage:
			types = append(types, "assistant")
		case *claude.ResultMessage:
			types = append(types, "result")
		default:
			types = append(types, "other")
		}
	}

	if len(types) < 2 {
		t.Fatalf("got %v, want at least [assistant, result]", types)
	}
	if types[len(types)-1] != "result" {
		t.Errorf("last type = %q, want result", types[len(types)-1])
	}
}

func TestMock_Session_MultiTurn(t *testing.T) {
	script, err := testutil.MockStreamingCLIScript(
		[]string{testutil.SystemInit},
		[][]string{
			{testutil.MakeAssistant("7"), testutil.MakeResult("7")},
			{testutil.MakeAssistant("12"), testutil.MakeResult("12")},
		},
	)
	if err != nil {
		t.Fatalf("MockStreamingCLIScript: %v", err)
	}
	defer os.Remove(script)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx, claude.WithCLIPath(script))
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()

	// Turn 1
	var turn1Text string
	for msg, err := range session.Send(ctx, "pick a number") {
		if err != nil {
			t.Fatalf("turn 1: %v", err)
		}
		if a, ok := msg.(*claude.AssistantMessage); ok {
			turn1Text = claude.CombinedText(a.Message.Content)
		}
	}
	if turn1Text != "7" {
		t.Errorf("turn 1 = %q, want 7", turn1Text)
	}

	// Turn 2
	var turn2Text string
	for msg, err := range session.Send(ctx, "add 5") {
		if err != nil {
			t.Fatalf("turn 2: %v", err)
		}
		if a, ok := msg.(*claude.AssistantMessage); ok {
			turn2Text = claude.CombinedText(a.Message.Content)
		}
	}
	if turn2Text != "12" {
		t.Errorf("turn 2 = %q, want 12", turn2Text)
	}
}

func TestMock_Session_SessionID(t *testing.T) {
	// In streaming mode, the system init is emitted immediately (before any user message).
	// But our Send() only reads after writing the user message.
	// The system init is emitted as part of the first turn's response.
	script, err := testutil.MockStreamingCLIScript(
		nil, // no init lines — include system init in the first turn response
		[][]string{
			{testutil.SystemInit, testutil.AssistantText, testutil.ResultOK},
		},
	)
	if err != nil {
		t.Fatalf("MockStreamingCLIScript: %v", err)
	}
	defer os.Remove(script)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx, claude.WithCLIPath(script))
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()

	for msg, err := range session.Send(ctx, "hello") {
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		t.Logf("msg: %T", msg)
	}

	sid := session.SessionID()
	t.Logf("session_id after send: %q", sid)
	if sid != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("session_id = %q", sid)
	}
}

func TestMock_ToChan(t *testing.T) {
	script, err := testutil.MockStreamingCLIScript(
		nil,
		[][]string{
			{testutil.SystemInit, testutil.AssistantText, testutil.ResultOK},
		},
	)
	if err != nil {
		t.Fatalf("MockStreamingCLIScript: %v", err)
	}
	defer os.Remove(script)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch := claude.ToChan(claude.Query(ctx, "test", claude.WithCLIPath(script)))

	count := 0
	for me := range ch {
		if me.Err != nil {
			break // EOF after script exits is normal
		}
		count++
	}

	if count < 2 {
		t.Errorf("messages = %d, want at least 2", count)
	}
}

// --- Hook callback tests ---

func TestMock_Hook_PreToolUse(t *testing.T) {
	// Register a PreToolUse hook, simulate the CLI sending a hook_callback
	// control_request, verify the hook is called and the response reaches the CLI.
	hookCalled := false

	hookFn := func(ctx context.Context, input claude.HookInput) (claude.HookOutput, error) {
		hookCalled = true
		if input.ToolName != "Bash" {
			t.Errorf("hook input tool_name = %q, want Bash", input.ToolName)
		}
		if input.HookEventName != "PreToolUse" {
			t.Errorf("hook input hook_event_name = %q, want PreToolUse", input.HookEventName)
		}
		return claude.HookOutput{
			AdditionalContext: "approved by hook",
		}, nil
	}

	// The hook_callback request the CLI sends to the SDK. callback_id "hook_0"
	// matches the first hook registered (see session.go NewSession logic).
	hookReq := testutil.MakeHookCallbackRequest("cli_req_1", "hook_0", map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "ls"},
		"session_id":      "550e8400-e29b-41d4-a716-446655440000",
	})

	script, err := testutil.MockControlFlowCLIScript(
		[]string{hookReq},
		[][]string{
			{testutil.SystemInit, testutil.MakeAssistant("done"), testutil.MakeResult("done")},
		},
	)
	if err != nil {
		t.Fatalf("MockControlFlowCLIScript: %v", err)
	}
	defer os.Remove(script)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx,
		claude.WithCLIPath(script),
		claude.WithHook(claude.HookPreToolUse, claude.HookCallbackMatcher{
			Hooks: []claude.HookCallback{hookFn},
		}),
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()

	var resultText string
	for msg, err := range session.Send(ctx, "run ls") {
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		if r, ok := msg.(*claude.ResultMessage); ok {
			resultText = r.Result
		}
	}

	if !hookCalled {
		t.Error("hook was not called")
	}
	if resultText != "done" {
		t.Errorf("result = %q, want done", resultText)
	}
}

func TestMock_Hook_PostToolUse(t *testing.T) {
	hookCalled := false

	hookFn := func(ctx context.Context, input claude.HookInput) (claude.HookOutput, error) {
		hookCalled = true
		if input.ToolName != "Write" {
			t.Errorf("hook input tool_name = %q, want Write", input.ToolName)
		}
		if input.HookEventName != "PostToolUse" {
			t.Errorf("hook input hook_event_name = %q, want PostToolUse", input.HookEventName)
		}
		return claude.HookOutput{}, nil
	}

	hookReq := testutil.MakeHookCallbackRequest("cli_req_2", "hook_0", map[string]any{
		"hook_event_name": "PostToolUse",
		"tool_name":       "Write",
		"tool_input":      map[string]any{"file_path": "/tmp/test.txt"},
		"tool_response":   "file written",
		"session_id":      "550e8400-e29b-41d4-a716-446655440000",
	})

	script, err := testutil.MockControlFlowCLIScript(
		[]string{hookReq},
		[][]string{
			{testutil.SystemInit, testutil.MakeAssistant("written"), testutil.MakeResult("written")},
		},
	)
	if err != nil {
		t.Fatalf("MockControlFlowCLIScript: %v", err)
	}
	defer os.Remove(script)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx,
		claude.WithCLIPath(script),
		claude.WithHook(claude.HookPostToolUse, claude.HookCallbackMatcher{
			Hooks: []claude.HookCallback{hookFn},
		}),
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()

	for msg, err := range session.Send(ctx, "write file") {
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		_ = msg
	}

	if !hookCalled {
		t.Error("hook was not called")
	}
}

func TestMock_Hook_DenyDecision(t *testing.T) {
	// Hook returns a deny decision; verify the session completes (the deny
	// response is sent back to the CLI via control_response).
	hookFn := func(ctx context.Context, input claude.HookInput) (claude.HookOutput, error) {
		deny := false // continue=false means deny
		return claude.HookOutput{
			Continue:       &deny,
			Decision:       "deny",
			DecisionReason: "not allowed by policy",
		}, nil
	}

	hookReq := testutil.MakeHookCallbackRequest("cli_req_3", "hook_0", map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "rm -rf /"},
		"session_id":      "550e8400-e29b-41d4-a716-446655440000",
	})

	script, err := testutil.MockControlFlowCLIScript(
		[]string{hookReq},
		[][]string{
			{testutil.SystemInit, testutil.MakeAssistant("denied"), testutil.MakeResult("denied")},
		},
	)
	if err != nil {
		t.Fatalf("MockControlFlowCLIScript: %v", err)
	}
	defer os.Remove(script)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx,
		claude.WithCLIPath(script),
		claude.WithHook(claude.HookPreToolUse, claude.HookCallbackMatcher{
			Hooks: []claude.HookCallback{hookFn},
		}),
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()

	var resultText string
	for msg, err := range session.Send(ctx, "delete everything") {
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		if r, ok := msg.(*claude.ResultMessage); ok {
			resultText = r.Result
		}
	}

	if resultText != "denied" {
		t.Errorf("result = %q, want denied", resultText)
	}
}

// --- CanUseTool permission callback tests ---

func TestMock_CanUseTool_Allow(t *testing.T) {
	canUseToolCalled := false

	canUseTool := func(toolName string, input map[string]any, opts claude.CanUseToolOptions) (claude.PermissionResult, error) {
		canUseToolCalled = true
		if toolName != "Bash" {
			t.Errorf("tool_name = %q, want Bash", toolName)
		}
		return claude.PermissionResult{Behavior: "allow"}, nil
	}

	permReq := testutil.MakeCanUseToolRequest("cli_perm_1", "Bash", map[string]any{
		"command": "echo hello",
	})

	script, err := testutil.MockControlFlowCLIScript(
		[]string{permReq},
		[][]string{
			{testutil.SystemInit, testutil.MakeAssistant("allowed"), testutil.MakeResult("allowed")},
		},
	)
	if err != nil {
		t.Fatalf("MockControlFlowCLIScript: %v", err)
	}
	defer os.Remove(script)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx,
		claude.WithCLIPath(script),
		claude.WithCanUseTool(canUseTool),
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()

	var resultText string
	for msg, err := range session.Send(ctx, "run echo") {
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		if r, ok := msg.(*claude.ResultMessage); ok {
			resultText = r.Result
		}
	}

	if !canUseToolCalled {
		t.Error("can_use_tool callback was not called")
	}
	if resultText != "allowed" {
		t.Errorf("result = %q, want allowed", resultText)
	}
}

func TestMock_CanUseTool_Deny(t *testing.T) {
	canUseTool := func(toolName string, input map[string]any, opts claude.CanUseToolOptions) (claude.PermissionResult, error) {
		return claude.PermissionResult{
			Behavior: "deny",
			Message:  "tool not permitted",
		}, nil
	}

	permReq := testutil.MakeCanUseToolRequest("cli_perm_2", "Bash", map[string]any{
		"command": "rm -rf /",
	})

	script, err := testutil.MockControlFlowCLIScript(
		[]string{permReq},
		[][]string{
			{testutil.SystemInit, testutil.MakeAssistant("blocked"), testutil.MakeResult("blocked")},
		},
	)
	if err != nil {
		t.Fatalf("MockControlFlowCLIScript: %v", err)
	}
	defer os.Remove(script)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx,
		claude.WithCLIPath(script),
		claude.WithCanUseTool(canUseTool),
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()

	var resultText string
	for msg, err := range session.Send(ctx, "delete files") {
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		if r, ok := msg.(*claude.ResultMessage); ok {
			resultText = r.Result
		}
	}

	if resultText != "blocked" {
		t.Errorf("result = %q, want blocked", resultText)
	}
}

// --- Session lifecycle tests ---

func TestMock_Session_Close(t *testing.T) {
	script, err := testutil.MockStreamingCLIScript(
		nil,
		[][]string{
			{testutil.SystemInit, testutil.MakeAssistant("hi"), testutil.MakeResult("hi")},
		},
	)
	if err != nil {
		t.Fatalf("MockStreamingCLIScript: %v", err)
	}
	defer os.Remove(script)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx, claude.WithCLIPath(script))
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	// Use the session first to confirm it works.
	for msg, err := range session.Send(ctx, "hello") {
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		_ = msg
	}

	// Close should succeed.
	if err := session.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// After close, Send should return ErrSessionClosed.
	for _, err := range session.Send(ctx, "after close") {
		if err != nil && errors.Is(err, claude.ErrSessionClosed) {
			return // expected
		}
		if err != nil {
			t.Fatalf("Send after close: unexpected error: %v", err)
		}
	}
	t.Error("expected ErrSessionClosed after Close()")
}

func TestMock_Session_CloseTwice(t *testing.T) {
	script, err := testutil.MockStreamingCLIScript(
		nil,
		[][]string{
			{testutil.SystemInit, testutil.MakeAssistant("hi"), testutil.MakeResult("hi")},
		},
	)
	if err != nil {
		t.Fatalf("MockStreamingCLIScript: %v", err)
	}
	defer os.Remove(script)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx, claude.WithCLIPath(script))
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	// First close.
	if err := session.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	// Second close should be idempotent (return nil).
	if err := session.Close(); err != nil {
		t.Fatalf("second Close: %v (expected nil)", err)
	}
}

func TestMock_Session_SendAfterClose(t *testing.T) {
	script, err := testutil.MockStreamingCLIScript(
		nil,
		[][]string{
			{testutil.SystemInit, testutil.MakeAssistant("hi"), testutil.MakeResult("hi")},
		},
	)
	if err != nil {
		t.Fatalf("MockStreamingCLIScript: %v", err)
	}
	defer os.Remove(script)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx, claude.WithCLIPath(script))
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	if err := session.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	gotErr := false
	for _, err := range session.Send(ctx, "should fail") {
		if err != nil {
			if !errors.Is(err, claude.ErrSessionClosed) {
				t.Fatalf("Send after close: got %v, want ErrSessionClosed", err)
			}
			gotErr = true
			break
		}
	}

	if !gotErr {
		t.Error("expected ErrSessionClosed from Send after Close")
	}
}

func TestMock_Session_EmptyPrompt(t *testing.T) {
	script, err := testutil.MockStreamingCLIScript(
		nil,
		[][]string{
			{testutil.SystemInit, testutil.MakeAssistant("hi"), testutil.MakeResult("hi")},
		},
	)
	if err != nil {
		t.Fatalf("MockStreamingCLIScript: %v", err)
	}
	defer os.Remove(script)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx, claude.WithCLIPath(script))
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()

	gotErr := false
	for _, err := range session.Send(ctx, "") {
		if err != nil {
			if !errors.Is(err, claude.ErrEmptyPrompt) {
				t.Fatalf("Send empty: got %v, want ErrEmptyPrompt", err)
			}
			gotErr = true
			break
		}
	}

	if !gotErr {
		t.Error("expected ErrEmptyPrompt from Send with empty string")
	}
}
