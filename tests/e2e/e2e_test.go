package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	claude "github.com/gustavosbarreto/claude-agent-sdk-go"
)

// E2E tests run against the real Claude CLI inside a Docker container.
// They only execute when INSIDE_CONTAINER=1 is set (by the Dockerfile).
//
// From the host, run TestE2E_Container which builds the image and runs
// all tests inside:
//
//   go test ./tests/e2e/ -v -count=1 -run TestE2E_Container -timeout 10m
//
// These tests follow the patterns from the official Python SDK e2e tests:
// - SystemMessage(init) must be the first message
// - ResultMessage must be the last message
// - Init message fields are inspected (session_id, model, tools, cwd)
// - Structured output fields are parsed and validated individually
// - Hook invocations are tracked and asserted

func skipIfNotInContainer(t *testing.T) {
	t.Helper()
	if os.Getenv("INSIDE_CONTAINER") == "" {
		t.Skip("e2e tests only run inside the container (use TestE2E_Container)")
	}
}

// collectMessages runs a Query and returns all messages in order.
func collectMessages(t *testing.T, ctx context.Context, prompt string, opts ...claude.Option) []claude.Message {
	t.Helper()
	var messages []claude.Message
	for msg, err := range claude.Query(ctx, prompt, opts...) {
		if err != nil {
			t.Fatalf("Query error: %v", err)
		}
		messages = append(messages, msg)
	}
	return messages
}

// assertMessageOrder verifies SystemMessage first, ResultMessage last.
func assertMessageOrder(t *testing.T, messages []claude.Message) {
	t.Helper()
	if len(messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(messages))
	}

	// First message must be SystemMessage(init)
	sys, ok := messages[0].(*claude.SystemMessage)
	if !ok {
		t.Errorf("first message should be SystemMessage, got %T", messages[0])
	} else if sys.Subtype != "init" {
		t.Errorf("first message subtype = %q, want init", sys.Subtype)
	}

	// Last message must be ResultMessage
	if _, ok := messages[len(messages)-1].(*claude.ResultMessage); !ok {
		t.Errorf("last message should be ResultMessage, got %T", messages[len(messages)-1])
	}
}

// assertInitMessage verifies SystemMessage(init) fields.
func assertInitMessage(t *testing.T, messages []claude.Message) *claude.SystemMessage {
	t.Helper()
	for _, msg := range messages {
		sys, ok := msg.(*claude.SystemMessage)
		if !ok || sys.Subtype != "init" {
			continue
		}
		if sys.SessionID == "" {
			t.Error("init message missing session_id")
		}
		if sys.Model == "" {
			t.Error("init message missing model")
		}
		if len(sys.Tools) == 0 {
			t.Error("init message missing tools")
		}
		if sys.Cwd == "" {
			t.Error("init message missing cwd")
		}
		t.Logf("init: session=%s model=%s tools=%d cwd=%s",
			sys.SessionID, sys.Model, len(sys.Tools), sys.Cwd)
		return sys
	}
	t.Error("no SystemMessage(init) found")
	return nil
}

// findResult returns the ResultMessage from a message list.
func findResult(t *testing.T, messages []claude.Message) *claude.ResultMessage {
	t.Helper()
	for _, msg := range messages {
		if r, ok := msg.(*claude.ResultMessage); ok {
			return r
		}
	}
	t.Fatal("no ResultMessage found")
	return nil
}

// assertResultOK verifies that the result is successful and not an error.
func assertResultOK(t *testing.T, result *claude.ResultMessage) {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.IsError {
		t.Errorf("result is error: %s", result.Result)
	}
	if result.Subtype != claude.ResultSuccess {
		t.Errorf("result subtype = %q, want success", result.Subtype)
	}
}

// defaultOpts returns options common to all e2e tests.
func defaultOpts(extra ...claude.Option) []claude.Option {
	opts := make([]claude.Option, 0, 3+len(extra))
	opts = append(opts,
		claude.WithPermissionMode(claude.PermissionBypassPermissions),
		claude.WithAllowDangerouslySkipPermissions(),
		claude.WithNoPersistSession(),
	)
	return append(opts, extra...)
}

func TestE2E_Prompt(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := claude.Prompt(ctx, "What is 2+2? Reply with just the number.",
		defaultOpts(claude.WithMaxTurns(1))...,
	)
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	assertResultOK(t, result)
	if result.Result == "" {
		t.Error("empty result")
	}
	if result.SessionID == "" {
		t.Error("missing session_id in result")
	}
	t.Logf("result=%q cost=$%.4f turns=%d session=%s",
		result.Result, result.TotalCostUSD, result.NumTurns, result.SessionID)
}

func TestE2E_Query_MessageTypes(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	messages := collectMessages(t, ctx, "Say hello in one word.",
		defaultOpts(claude.WithMaxTurns(1))...,
	)

	// Verify ordering: SystemMessage first, ResultMessage last.
	assertMessageOrder(t, messages)

	// Verify init message fields.
	assertInitMessage(t, messages)

	// Verify we got an AssistantMessage.
	gotAssistant := false
	for _, msg := range messages {
		if a, ok := msg.(*claude.AssistantMessage); ok {
			gotAssistant = true
			text := claude.CombinedText(a.Message.Content)
			t.Logf("assistant: %s", text)
		}
	}
	if !gotAssistant {
		t.Error("missing assistant message")
	}

	// Log all message types for debugging.
	types := make([]string, 0, len(messages))
	for _, msg := range messages {
		types = append(types, typeName(msg))
	}
	t.Logf("message types: %v", types)
}

func TestE2E_Session_MultiTurn(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx, defaultOpts(claude.WithMaxTurns(3))...)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = session.Close() }()

	// Turn 1
	var turn1 string
	var turn1Result *claude.ResultMessage
	for msg, err := range session.Send(ctx, "Pick a number between 1 and 10. Reply with just the number.") {
		if err != nil {
			t.Fatalf("turn 1: %v", err)
		}
		if a, ok := msg.(*claude.AssistantMessage); ok {
			turn1 = claude.CombinedText(a.Message.Content)
		}
		if r, ok := msg.(*claude.ResultMessage); ok {
			turn1Result = r
		}
	}
	t.Logf("turn 1: %s", turn1)
	assertResultOK(t, turn1Result)

	if session.SessionID() == "" {
		t.Error("session ID empty after turn 1")
	}

	// Turn 2 — references turn 1
	var turn2 string
	var turn2Result *claude.ResultMessage
	for msg, err := range session.Send(ctx, "Add 5 to the number you just said. Reply with just the result.") {
		if err != nil {
			t.Fatalf("turn 2: %v", err)
		}
		if a, ok := msg.(*claude.AssistantMessage); ok {
			turn2 = claude.CombinedText(a.Message.Content)
		}
		if r, ok := msg.(*claude.ResultMessage); ok {
			turn2Result = r
		}
	}
	t.Logf("turn 2: %s", turn2)
	assertResultOK(t, turn2Result)

	if turn2 == "" {
		t.Error("turn 2 empty — multi-turn context may not work")
	}
}

func TestE2E_Session_SetModel(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// No extra options — matching Python: ClaudeAgentOptions()
	session, err := claude.NewSession(ctx)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = session.Close() }()

	// Turn 1 with default model
	for msg, err := range session.Send(ctx, "What is 1+1? Just the number.") {
		if err != nil {
			t.Fatalf("turn 1: %v", err)
		}
		if r, ok := msg.(*claude.ResultMessage); ok {
			t.Logf("turn 1 (default model): %s", r.Result)
			if r.IsError {
				t.Errorf("turn 1 returned error: %s", r.Result)
			}
		}
	}

	// Switch to haiku
	if err := session.SetModel("claude-3-5-haiku-20241022"); err != nil {
		t.Fatalf("SetModel to haiku: %v", err)
	}

	// Turn 2 with haiku — don't assert on error, model may not be available
	// on all accounts (matching Python SDK which just consumes messages).
	for msg, err := range session.Send(ctx, "What is 2+2? Just the number.") {
		if err != nil {
			t.Fatalf("turn 2: %v", err)
		}
		if r, ok := msg.(*claude.ResultMessage); ok {
			t.Logf("turn 2 (haiku): %s", r.Result)
		}
	}

	// Switch back to default (empty string = default)
	if err := session.SetModel(""); err != nil {
		t.Fatalf("SetModel to default: %v", err)
	}

	// Turn 3 with default model again
	for msg, err := range session.Send(ctx, "What is 3+3? Just the number.") {
		if err != nil {
			t.Fatalf("turn 3: %v", err)
		}
		if r, ok := msg.(*claude.ResultMessage); ok {
			t.Logf("turn 3 (default again): %s", r.Result)
			if r.IsError {
				t.Errorf("turn 3 returned error: %s", r.Result)
			}
		}
	}
}

func TestE2E_StructuredOutput(t *testing.T) {
	skipIfNotInContainer(t)

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
		defaultOpts(
			claude.WithOutputFormat(claude.OutputFormat{Type: "json_schema", Schema: schema}),
		)...,
	)
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	assertResultOK(t, result)

	// Parse and validate structured output fields individually.
	if len(result.StructuredOutput) == 0 {
		t.Fatal("structured_output field is empty")
	}

	var output map[string]any
	if err := json.Unmarshal(result.StructuredOutput, &output); err != nil {
		t.Fatalf("failed to parse structured_output: %v", err)
	}

	// Verify "answer" field exists and is a number.
	answer, ok := output["answer"]
	if !ok {
		t.Error("structured_output missing 'answer' field")
	} else {
		answerNum, ok := answer.(float64)
		if !ok {
			t.Errorf("answer should be a number, got %T", answer)
		} else {
			t.Logf("answer: %g", answerNum)
		}
	}

	// Verify "confidence" field exists and is a valid enum value.
	confidence, ok := output["confidence"]
	if !ok {
		t.Error("structured_output missing 'confidence' field")
	} else if confStr, isStr := confidence.(string); !isStr {
		t.Errorf("confidence should be a string, got %T", confidence)
	} else {
		validValues := map[string]bool{"high": true, "medium": true, "low": true}
		if !validValues[confStr] {
			t.Errorf("confidence = %q, want high/medium/low", confStr)
		}
		t.Logf("confidence: %s", confStr)
	}
}

func TestE2E_ToolUse(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	messages := collectMessages(t, ctx, "List the files in /tmp using Bash. Just run: ls /tmp | head -5",
		defaultOpts(
			claude.WithMaxTurns(5),
			claude.WithAllowedTools("Bash"),
		)...,
	)

	assertMessageOrder(t, messages)

	// Verify a tool_use block appeared.
	var toolNames []string
	for _, msg := range messages {
		if a, ok := msg.(*claude.AssistantMessage); ok {
			for _, block := range a.Message.Content {
				if block.Type == claude.ContentBlockToolUse {
					toolNames = append(toolNames, block.Name)
				}
			}
		}
	}

	if len(toolNames) == 0 {
		t.Error("no tool_use blocks — expected Bash usage")
	} else {
		t.Logf("tools used: %v", toolNames)
	}

	assertResultOK(t, findResult(t, messages))
}

func TestE2E_Hook_PreToolUse(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	type hookInvocation struct {
		toolName  string
		toolUseID string
	}

	var invocations []hookInvocation

	matcher := "Bash"
	messages := collectMessages(t, ctx, "Run: echo 'test hook'",
		claude.WithPermissionMode(claude.PermissionAcceptEdits),
		claude.WithAllowedTools("Bash"),
		claude.WithNoPersistSession(),
		claude.WithMaxTurns(5),
		claude.WithHook(claude.HookPreToolUse, claude.HookCallbackMatcher{
			Matcher: &matcher,
			Hooks: []claude.HookCallback{
				func(ctx context.Context, input claude.HookInput) (claude.HookOutput, error) {
					invocations = append(invocations, hookInvocation{
						toolName:  input.ToolName,
						toolUseID: input.ToolUseID,
					})
					t.Logf("hook fired: tool=%s tool_use_id=%s", input.ToolName, input.ToolUseID)

					return claude.HookOutput{
						Decision:          "allow",
						DecisionReason:    "Approved with context",
						AdditionalContext: "This command is running in a test environment",
					}, nil
				},
			},
		}),
	)

	assertMessageOrder(t, messages)

	result := findResult(t, messages)
	assertResultOK(t, result)
	t.Logf("result=%q hooks_fired=%d", result.Result, len(invocations))

	// Hook should have been invoked for Bash.
	if len(invocations) == 0 {
		t.Error("PreToolUse hook should have been invoked")
	}
	for i, inv := range invocations {
		t.Logf("invocation[%d]: tool=%s tool_use_id=%s", i, inv.toolName, inv.toolUseID)
		if inv.toolUseID == "" {
			t.Errorf("invocation[%d]: tool_use_id should not be empty", i)
		}
	}
}

func TestE2E_StderrCallback(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var stderrOutput []string

	// Enable debug mode to get stderr output, like the Python test does.
	result, err := claude.Prompt(ctx, "What is 1+1? Just the number.",
		defaultOpts(
			claude.WithMaxTurns(1),
			claude.WithExtraArgs(map[string]string{"debug-to-stderr": ""}),
			claude.WithStderrCallback(func(s string) {
				stderrOutput = append(stderrOutput, s)
			}),
		)...,
	)
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	assertResultOK(t, result)

	// Should capture stderr output with debug enabled.
	if len(stderrOutput) == 0 {
		t.Error("should capture stderr output with debug enabled")
	}

	// Should contain [DEBUG] messages.
	hasDebug := false
	for _, line := range stderrOutput {
		if strings.Contains(line, "[DEBUG]") {
			hasDebug = true
			break
		}
	}
	if !hasDebug {
		t.Error("should contain [DEBUG] messages in stderr")
	}

	t.Logf("stderr lines: %d", len(stderrOutput))
}

func TestE2E_StderrCallback_WithoutDebug(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var stderrOutput []string

	// No debug mode — should capture no output.
	result, err := claude.Prompt(ctx, "What is 1+1? Just the number.",
		defaultOpts(
			claude.WithMaxTurns(1),
			claude.WithStderrCallback(func(s string) {
				stderrOutput = append(stderrOutput, s)
			}),
		)...,
	)
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	assertResultOK(t, result)

	if len(stderrOutput) != 0 {
		t.Errorf("should not capture stderr output without debug mode, got %d lines", len(stderrOutput))
	}
}

func TestE2E_IncludePartialMessages(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Use sonnet with thinking tokens to get ThinkingBlock + TextBlock,
	// matching the Python SDK test.
	messages := collectMessages(t, ctx, "Think of three jokes, then tell one",
		claude.WithModel("claude-sonnet-4-5"),
		claude.WithMaxTurns(2),
		claude.WithIncludePartialMessages(),
		claude.WithEnv(map[string]string{"MAX_THINKING_TOKENS": "8000"}),
		claude.WithPermissionMode(claude.PermissionBypassPermissions),
		claude.WithAllowDangerouslySkipPermissions(),
		claude.WithNoPersistSession(),
	)

	// Verify ordering and result.
	assertMessageOrder(t, messages)
	assertResultOK(t, findResult(t, messages))

	// Collect stream events.
	var streamEvents []*claude.StreamEvent
	for _, msg := range messages {
		if se, ok := msg.(*claude.StreamEvent); ok {
			streamEvents = append(streamEvents, se)
		}
	}

	if len(streamEvents) == 0 {
		t.Fatal("no StreamEvent messages with includePartialMessages enabled")
	}
	t.Logf("stream events: %d", len(streamEvents))

	// Check for expected StreamEvent types (matching Python SDK assertions).
	eventTypes := make(map[string]bool)
	for _, se := range streamEvents {
		var evt struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(se.Event, &evt) == nil && evt.Type != "" {
			eventTypes[evt.Type] = true
		}
	}

	for _, expected := range []string{
		"message_start",
		"content_block_start",
		"content_block_delta",
		"content_block_stop",
		"message_stop",
	} {
		if !eventTypes[expected] {
			t.Errorf("missing StreamEvent type %q", expected)
		}
	}
	t.Logf("event types: %v", eventTypes)

	// Verify AssistantMessage has ThinkingBlock and TextBlock content.
	var hasThinking, hasText bool
	for _, msg := range messages {
		a, ok := msg.(*claude.AssistantMessage)
		if !ok {
			continue
		}
		for _, block := range a.Message.Content {
			if block.Type == claude.ContentBlockThinking && block.Thinking != "" {
				hasThinking = true
			}
			if block.Type == claude.ContentBlockText && block.Text != "" {
				hasText = true
			}
		}
	}
	if !hasThinking {
		t.Error("no ThinkingBlock found in AssistantMessages")
	}
	if !hasText {
		t.Error("no TextBlock found in AssistantMessages")
	}

	// Verify we still got the regular messages alongside stream events.
	var gotAssistant, gotResult bool
	for _, msg := range messages {
		switch msg.(type) {
		case *claude.AssistantMessage:
			gotAssistant = true
		case *claude.ResultMessage:
			gotResult = true
		}
	}
	if !gotAssistant {
		t.Error("missing AssistantMessage alongside stream events")
	}
	if !gotResult {
		t.Error("missing ResultMessage alongside stream events")
	}
}

func TestE2E_IncludePartialMessages_ThinkingDeltas(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var thinkingDeltas []string

	for msg, err := range claude.Query(ctx, "Think step by step about what 2 + 2 equals",
		claude.WithModel("claude-sonnet-4-5"),
		claude.WithMaxTurns(2),
		claude.WithIncludePartialMessages(),
		claude.WithEnv(map[string]string{"MAX_THINKING_TOKENS": "8000"}),
		claude.WithPermissionMode(claude.PermissionBypassPermissions),
		claude.WithAllowDangerouslySkipPermissions(),
		claude.WithNoPersistSession(),
	) {
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if se, ok := msg.(*claude.StreamEvent); ok {
			var evt struct {
				Type  string `json:"type"`
				Delta struct {
					Type     string `json:"type"`
					Thinking string `json:"thinking"`
				} `json:"delta"`
			}
			if json.Unmarshal(se.Event, &evt) == nil &&
				evt.Type == "content_block_delta" &&
				evt.Delta.Type == "thinking_delta" {
				thinkingDeltas = append(thinkingDeltas, evt.Delta.Thinking)
			}
		}
	}

	if len(thinkingDeltas) == 0 {
		t.Error("no thinking deltas received")
	}

	combined := strings.Join(thinkingDeltas, "")
	t.Logf("thinking deltas: %d, combined length: %d", len(thinkingDeltas), len(combined))

	if len(combined) < 10 {
		t.Error("thinking content too short")
	}
	if !strings.Contains(strings.ToLower(combined), "2") {
		t.Error("thinking doesn't mention the numbers")
	}
}

func TestE2E_PartialMessages_DisabledByDefault(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Without WithIncludePartialMessages, no StreamEvents should appear.
	messages := collectMessages(t, ctx, "What is 1+1? Just the number.",
		defaultOpts(claude.WithMaxTurns(1))...,
	)

	assertMessageOrder(t, messages)
	assertResultOK(t, findResult(t, messages))

	for _, msg := range messages {
		if _, ok := msg.(*claude.StreamEvent); ok {
			t.Error("StreamEvent present when partial messages not enabled")
			break
		}
	}

	// Should still have regular messages.
	var gotSystem, gotAssistant, gotResult bool
	for _, msg := range messages {
		switch msg.(type) {
		case *claude.SystemMessage:
			gotSystem = true
		case *claude.AssistantMessage:
			gotAssistant = true
		case *claude.ResultMessage:
			gotResult = true
		}
	}
	if !gotSystem {
		t.Error("missing SystemMessage")
	}
	if !gotAssistant {
		t.Error("missing AssistantMessage")
	}
	if !gotResult {
		t.Error("missing ResultMessage")
	}
}

func TestE2E_Hook_PermissionDeny(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var hookInvocations []string

	matcher := "Bash"
	messages := collectMessages(t, ctx, "Run this bash command: echo 'hello'",
		// Matching Python: ClaudeAgentOptions(allowed_tools=["Bash","Write"], hooks=...)
		claude.WithAllowedTools("Bash", "Write"),
		claude.WithHook(claude.HookPreToolUse, claude.HookCallbackMatcher{
			Matcher: &matcher,
			Hooks: []claude.HookCallback{
				func(ctx context.Context, input claude.HookInput) (claude.HookOutput, error) {
					hookInvocations = append(hookInvocations, input.ToolName)
					t.Logf("hook: deny %s", input.ToolName)

					if input.ToolName == "Bash" {
						return claude.HookOutput{
							Reason:         "Bash blocked by test hook",
							SystemMessage:  "Command blocked by hook",
							Decision:       "deny",
							DecisionReason: "Security policy: Bash blocked",
						}, nil
					}

					return claude.HookOutput{
						Decision:       "allow",
						DecisionReason: "Tool passed security checks",
					}, nil
				},
			},
		}),
	)

	assertMessageOrder(t, messages)

	// Hook should have been invoked for Bash.
	hasBash := false
	for _, name := range hookInvocations {
		if name == "Bash" {
			hasBash = true
		}
	}
	if !hasBash {
		t.Error("hook should have been invoked for Bash tool")
	}

	t.Logf("hook invocations: %v", hookInvocations)
}

func TestE2E_Hook_StopReason(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var hookInvocations []string

	matcher := "Bash"
	messages := collectMessages(t, ctx, "Run: echo 'test message'",
		claude.WithPermissionMode(claude.PermissionAcceptEdits),
		claude.WithAllowedTools("Bash"),
		claude.WithNoPersistSession(),
		claude.WithHook(claude.HookPostToolUse, claude.HookCallbackMatcher{
			Matcher: &matcher,
			Hooks: []claude.HookCallback{
				func(ctx context.Context, input claude.HookInput) (claude.HookOutput, error) {
					hookInvocations = append(hookInvocations, input.ToolName)
					t.Logf("hook: stop after %s", input.ToolName)

					cont := false
					return claude.HookOutput{
						Continue:      &cont,
						StopReason:    "Execution halted by test hook for validation",
						Reason:        "Testing continue and stopReason fields",
						SystemMessage: "Test hook stopped execution",
					}, nil
				},
			},
		}),
	)

	assertMessageOrder(t, messages)

	hasBash := false
	for _, name := range hookInvocations {
		if name == "Bash" {
			hasBash = true
		}
	}
	if !hasBash {
		t.Error("PostToolUse hook should have been invoked for Bash")
	}

	t.Logf("hook invocations: %v", hookInvocations)
}

func TestE2E_Hook_AdditionalContext(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var hookInvoked bool

	matcher := "Bash"
	messages := collectMessages(t, ctx, "Run: echo 'testing hooks'",
		claude.WithPermissionMode(claude.PermissionAcceptEdits),
		claude.WithAllowedTools("Bash"),
		claude.WithNoPersistSession(),
		claude.WithHook(claude.HookPostToolUse, claude.HookCallbackMatcher{
			Matcher: &matcher,
			Hooks: []claude.HookCallback{
				func(ctx context.Context, input claude.HookInput) (claude.HookOutput, error) {
					hookInvoked = true
					t.Logf("hook: additionalContext for %s", input.ToolName)

					return claude.HookOutput{
						SystemMessage:    "Additional context provided by hook",
						Reason:           "Hook providing monitoring feedback",
						AdditionalContext: "The command executed successfully with hook monitoring",
					}, nil
				},
			},
		}),
	)

	assertMessageOrder(t, messages)

	if !hookInvoked {
		t.Error("PostToolUse hook with additionalContext should have been invoked")
	}
}

func TestE2E_Hook_MultipleHooks(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var invocations []string

	trackHook := func(ctx context.Context, input claude.HookInput) (claude.HookOutput, error) {
		invocations = append(invocations, input.HookEventName)
		t.Logf("hook: %s", input.HookEventName)
		return claude.HookOutput{}, nil
	}

	matcher := "Bash"
	messages := collectMessages(t, ctx, "Run: echo 'multi-hook test'",
		claude.WithPermissionMode(claude.PermissionAcceptEdits),
		claude.WithAllowedTools("Bash"),
		claude.WithNoPersistSession(),
		claude.WithHook(claude.HookNotification, claude.HookCallbackMatcher{
			Hooks: []claude.HookCallback{trackHook},
		}),
		claude.WithHook(claude.HookPreToolUse, claude.HookCallbackMatcher{
			Matcher: &matcher,
			Hooks:   []claude.HookCallback{trackHook},
		}),
		claude.WithHook(claude.HookPostToolUse, claude.HookCallbackMatcher{
			Matcher: &matcher,
			Hooks:   []claude.HookCallback{trackHook},
		}),
	)

	assertMessageOrder(t, messages)

	eventNames := make(map[string]bool)
	for _, name := range invocations {
		eventNames[name] = true
	}

	t.Logf("hook invocations: %v", invocations)

	// At minimum, PreToolUse and PostToolUse should fire for the Bash command.
	if !eventNames["PreToolUse"] {
		t.Error("PreToolUse hook should have fired")
	}
	if !eventNames["PostToolUse"] {
		t.Error("PostToolUse hook should have fired")
	}
}

func TestE2E_CanUseTool(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	type invocation struct {
		toolName string
		input    map[string]any
	}
	var invocations []invocation

	testFile := "/tmp/sdk_permission_test.txt"

	// No permission_mode set — uses default, which consults can_use_tool callback.
	// Matching the Python SDK test_permission_callback_gets_called exactly.
	session, err := claude.NewSession(ctx,
		claude.WithNoPersistSession(),
		claude.WithCanUseTool(func(toolName string, input map[string]any, opts claude.CanUseToolOptions) (claude.PermissionResult, error) {
			t.Logf("can_use_tool: %s", toolName)
			invocations = append(invocations, invocation{toolName: toolName, input: input})
			return claude.PermissionResult{Behavior: "allow"}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = session.Close() }()

	for msg, err := range session.Send(ctx, "Run the command: touch "+testFile) {
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		if r, ok := msg.(*claude.ResultMessage); ok {
			assertResultOK(t, r)
		}
	}

	// Verify callback was invoked for Bash (touch is not auto-allowed).
	var gotBash bool
	for _, inv := range invocations {
		if inv.toolName == "Bash" {
			gotBash = true
		}
	}
	if !gotBash {
		toolNames := make([]string, 0, len(invocations))
		for _, inv := range invocations {
			toolNames = append(toolNames, inv.toolName)
		}
		t.Errorf("permission callback should have been invoked for Bash, got: %v", toolNames)
	}

	t.Logf("callback invocations: %d", len(invocations))
}

func TestE2E_Interrupt(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx, defaultOpts()...)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = session.Close() }()

	// Start a long query via ToChan so we can interrupt mid-stream.
	ch := claude.ToChan(session.Send(ctx, "Count from 1 to 100, one number per line."))

	// Read a few messages then send interrupt.
	msgCount := 0
	for me := range ch {
		if me.Err != nil {
			t.Logf("error after %d messages: %v", msgCount, me.Err)
			break
		}
		msgCount++

		// After receiving some messages, interrupt.
		if msgCount >= 2 {
			t.Log("sending interrupt")
			if err := session.Interrupt(); err != nil {
				t.Logf("interrupt: %v", err)
			}
			// Drain remaining messages.
			for me := range ch {
				if me.Err != nil {
					break
				}
				msgCount++
			}
			break
		}
	}

	t.Logf("total messages after interrupt: %d", msgCount)
	// No strong assertions — just verify interrupt doesn't crash.
	// Matching Python SDK test_interrupt pattern.
}

func TestE2E_SetPermissionMode(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx,
		claude.WithPermissionMode(claude.PermissionDefault),
		claude.WithNoPersistSession(),
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = session.Close() }()

	// Change to acceptEdits
	if err := session.SetPermissionMode(claude.PermissionAcceptEdits); err != nil {
		t.Fatalf("SetPermissionMode to acceptEdits: %v", err)
	}

	// Turn 1
	for msg, err := range session.Send(ctx, "What is 2+2? Just the number.") {
		if err != nil {
			t.Fatalf("turn 1: %v", err)
		}
		if r, ok := msg.(*claude.ResultMessage); ok {
			assertResultOK(t, r)
			t.Logf("turn 1 (acceptEdits): %s", r.Result)
		}
	}

	// Change back to default
	if err := session.SetPermissionMode(claude.PermissionDefault); err != nil {
		t.Fatalf("SetPermissionMode to default: %v", err)
	}

	// Turn 2
	for msg, err := range session.Send(ctx, "What is 3+3? Just the number.") {
		if err != nil {
			t.Fatalf("turn 2: %v", err)
		}
		if r, ok := msg.(*claude.ResultMessage); ok {
			assertResultOK(t, r)
			t.Logf("turn 2 (default): %s", r.Result)
		}
	}
}

func TestE2E_SettingSources_Default(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create temp project with local settings.
	projectDir := t.TempDir()
	claudeDir := filepath.Join(projectDir, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	os.WriteFile(filepath.Join(claudeDir, "settings.local.json"),
		[]byte(`{"outputStyle": "local-test-style"}`), 0o644)

	// No setting_sources — CLI uses its default (loads all settings).
	// This matches Python SDK: when setting_sources is None, the flag
	// is omitted and the CLI decides which settings to load.
	// To explicitly disable settings, pass an empty setting_sources.
	session, err := claude.NewSession(ctx,
		claude.WithCwd(projectDir),
		claude.WithMaxTurns(1),
		claude.WithNoPersistSession(),
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = session.Close() }()

	var initMsg *claude.SystemMessage
	for msg, err := range session.Send(ctx, "What is 2+2? Just the number.") {
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		if sys, ok := msg.(*claude.SystemMessage); ok && sys.Subtype == "init" {
			initMsg = sys
		}
	}

	// When setting_sources is not set, CLI loads all settings by default,
	// so local settings WILL be loaded. This matches Python SDK behavior.
	if initMsg != nil && initMsg.OutputStyle != "local-test-style" {
		t.Errorf("outputStyle should be from local settings when setting_sources not set (CLI default loads all), got %q", initMsg.OutputStyle)
	}
	if initMsg != nil {
		t.Logf("output_style: %q", initMsg.OutputStyle)
	}
}

func TestE2E_SettingSources_ProjectIncluded(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create temp project with local settings.
	projectDir := t.TempDir()
	claudeDir := filepath.Join(projectDir, ".claude")
	os.MkdirAll(claudeDir, 0o755)
	os.WriteFile(filepath.Join(claudeDir, "settings.local.json"),
		[]byte(`{"outputStyle": "local-test-style"}`), 0o644)

	// Include user + project + local — should load local settings.
	// Matching Python: ClaudeAgentOptions(setting_sources=["user","project","local"], cwd=project_dir, max_turns=1)
	session, err := claude.NewSession(ctx,
		claude.WithCwd(projectDir),
		claude.WithMaxTurns(1),
		claude.WithNoPersistSession(),
		claude.WithSettingSources(
			claude.SettingSourceUser,
			claude.SettingSourceProject,
			claude.SettingSourceLocal,
		),
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = session.Close() }()

	var initMsg *claude.SystemMessage
	for msg, err := range session.Send(ctx, "What is 2+2? Just the number.") {
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		if sys, ok := msg.(*claude.SystemMessage); ok && sys.Subtype == "init" {
			initMsg = sys
		}
	}

	if initMsg == nil {
		t.Fatal("no init message")
	}
	if initMsg.OutputStyle != "local-test-style" {
		t.Errorf("outputStyle should be 'local-test-style', got %q", initMsg.OutputStyle)
	}
	t.Logf("output_style: %q", initMsg.OutputStyle)
}

func TestE2E_SdkMcpTool(t *testing.T) {
	skipIfNotInContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var executions []string

	srv := claude.NewSdkMcpServer("test",
		claude.SdkMcpTool{
			Name:        "echo",
			Description: "Echo back the input text",
			InputSchema: map[string]any{
				"text": map[string]any{"type": "string"},
			},
			Handler: func(ctx context.Context, args map[string]any) ([]claude.ToolContent, error) {
				executions = append(executions, "echo")
				text, _ := args["text"].(string)
				t.Logf("echo tool called: %s", text)
				return []claude.ToolContent{{Type: "text", Text: "Echo: " + text}}, nil
			},
		},
	)

	session, err := claude.NewSession(ctx,
		claude.WithSdkMcpServer("test", srv),
		claude.WithAllowedTools("mcp__test__echo"),
		claude.WithNoPersistSession(),
	)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer func() { _ = session.Close() }()

	for msg, err := range session.Send(ctx, "Call the mcp__test__echo tool with text 'hello world'") {
		if err != nil {
			t.Fatalf("Send: %v", err)
		}
		if r, ok := msg.(*claude.ResultMessage); ok {
			assertResultOK(t, r)
		}
	}

	// Verify the Go function was actually called.
	found := false
	for _, e := range executions {
		if e == "echo" {
			found = true
		}
	}
	if !found {
		t.Error("echo tool function was not executed")
	}
	t.Logf("executions: %v", executions)
}

// typeName returns a short name for a message type.
func typeName(msg claude.Message) string {
	switch msg.(type) {
	case *claude.SystemMessage:
		return "SystemMessage"
	case *claude.AssistantMessage:
		return "AssistantMessage"
	case *claude.UserMessage:
		return "UserMessage"
	case *claude.ResultMessage:
		return "ResultMessage"
	case *claude.StreamEvent:
		return "StreamEvent"
	case *claude.ToolProgressMessage:
		return "ToolProgressMessage"
	case *claude.RateLimitEvent:
		return "RateLimitEvent"
	default:
		return "Unknown"
	}
}
