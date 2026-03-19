package claude_test

import (
	"context"
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
