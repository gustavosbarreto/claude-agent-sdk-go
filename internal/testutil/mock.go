// Package testutil provides mock CLI helpers and fixtures for testing.
package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// MockCLIScript creates a temp shell script that echoes NDJSON lines and exits.
// For one-shot mode testing. Caller must os.Remove the returned path.
func MockCLIScript(lines []string) (string, error) {
	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	for _, line := range lines {
		escaped := strings.ReplaceAll(line, "'", "'\\''")
		fmt.Fprintf(&sb, "echo '%s'\n", escaped)
	}
	return writeTempScript(sb.String())
}

// MockStreamingCLIScript creates a temp shell script that simulates
// the streaming NDJSON protocol. Emits initLines immediately, then
// for each user message on stdin, emits the next turn's response lines.
func MockStreamingCLIScript(initLines []string, responseTurns [][]string) (string, error) {
	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")

	for _, line := range initLines {
		escaped := strings.ReplaceAll(line, "'", "'\\''")
		fmt.Fprintf(&sb, "echo '%s'\n", escaped)
	}

	sb.WriteString("TURN=0\n")
	sb.WriteString("while IFS= read -r input; do\n")
	// Skip control_response messages (sent by SDK in response to our requests).
	sb.WriteString("  case \"$input\" in *control_response*) continue ;; esac\n")
	// Respond to control_request messages (e.g. initialize) with success.
	sb.WriteString("  case \"$input\" in *control_request*)\n")
	sb.WriteString("    REQ_ID=$(echo \"$input\" | sed 's/.*request_id\":\"\\([^\"]*\\).*/\\1/')\n")
	sb.WriteString("    echo \"{\\\"type\\\":\\\"control_response\\\",\\\"response\\\":{\\\"subtype\\\":\\\"success\\\",\\\"request_id\\\":\\\"$REQ_ID\\\",\\\"response\\\":{}}}\"\n")
	sb.WriteString("    continue\n")
	sb.WriteString("  ;; esac\n")
	// Only process user messages for turn responses.
	sb.WriteString("  case \"$input\" in *'\"type\":\"user\"'*) ;; *) continue ;; esac\n")

	for i, turn := range responseTurns {
		fmt.Fprintf(&sb, "  if [ \"$TURN\" -eq %d ]; then\n", i)
		for _, line := range turn {
			escaped := strings.ReplaceAll(line, "'", "'\\''")
			fmt.Fprintf(&sb, "    echo '%s'\n", escaped)
		}
		sb.WriteString("    TURN=$((TURN + 1))\n")
		sb.WriteString("    continue\n")
		sb.WriteString("  fi\n")
	}

	sb.WriteString("done\n")
	return writeTempScript(sb.String())
}

func writeTempScript(content string) (string, error) {
	f, err := os.CreateTemp("", "mock-claude-*.sh")
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	_ = f.Close()
	if err := os.Chmod(f.Name(), 0o755); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// --- Fixtures ---

const SystemInit = `{"type":"system","subtype":"init","session_id":"550e8400-e29b-41d4-a716-446655440000","uuid":"sys-001","cwd":"/workspace","model":"claude-sonnet-4-6","tools":["Read","Write","Edit","Bash","Glob","Grep"],"mcp_servers":[],"permissionMode":"default","claude_code_version":"2.1.39"}`

const AssistantText = `{"type":"assistant","uuid":"msg_01","session_id":"550e8400-e29b-41d4-a716-446655440000","message":{"role":"assistant","content":[{"type":"text","text":"The answer is 4."}]}}`

const ResultOK = `{"type":"result","subtype":"success","session_id":"550e8400-e29b-41d4-a716-446655440000","is_error":false,"result":"The answer is 4.","num_turns":1,"duration_ms":100,"duration_api_ms":80,"total_cost_usd":0.001,"usage":{"input_tokens":100,"output_tokens":20,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}`

// MakeAssistant creates an assistant message with custom text.
func MakeAssistant(text string) string {
	data, _ := json.Marshal(map[string]any{
		"type":       "assistant",
		"uuid":       "msg_auto",
		"session_id": "550e8400-e29b-41d4-a716-446655440000",
		"message": map[string]any{
			"role":    "assistant",
			"content": []map[string]any{{"type": "text", "text": text}},
		},
	})
	return string(data)
}

// MakeResult creates a result message with custom text.
func MakeResult(text string) string {
	data, _ := json.Marshal(map[string]any{
		"type":            "result",
		"subtype":         "success",
		"session_id":      "550e8400-e29b-41d4-a716-446655440000",
		"is_error":        false,
		"result":          text,
		"num_turns":       1,
		"duration_ms":     100,
		"duration_api_ms": 80,
		"total_cost_usd":  0.001,
	})
	return string(data)
}
