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

// MakeControlRequest creates a control_request JSON line (sent from CLI to SDK).
func MakeControlRequest(requestID string, request map[string]any) string {
	reqData, _ := json.Marshal(request)
	data, _ := json.Marshal(map[string]any{
		"type":       "control_request",
		"request_id": requestID,
		"request":    json.RawMessage(reqData),
	})
	return string(data)
}

// MakeHookCallbackRequest creates a control_request for a hook_callback.
func MakeHookCallbackRequest(requestID, callbackID string, input map[string]any) string {
	req := map[string]any{
		"subtype":     "hook_callback",
		"callback_id": callbackID,
	}
	if input != nil {
		inputData, _ := json.Marshal(input)
		req["input"] = json.RawMessage(inputData)
	}
	return MakeControlRequest(requestID, req)
}

// MakeCanUseToolRequest creates a control_request for a can_use_tool callback.
func MakeCanUseToolRequest(requestID, toolName string, input map[string]any) string {
	req := map[string]any{
		"subtype":   "can_use_tool",
		"tool_name": toolName,
	}
	if input != nil {
		req["input"] = input
	}
	return MakeControlRequest(requestID, req)
}

// MockControlFlowCLIScript creates a mock CLI script that simulates the control
// protocol flow for hooks and permissions. On each user message, it sends a
// control_request to the SDK, reads the control_response, then emits response lines.
//
// controlRequests: for each turn, the control_request JSON to send to the SDK.
// An empty string means no control_request for that turn.
// responseTurns: for each turn, the response lines to emit after the control exchange.
func MockControlFlowCLIScript(controlRequests []string, responseTurns [][]string) (string, error) {
	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")

	sb.WriteString("TURN=0\n")
	sb.WriteString("while IFS= read -r input; do\n")

	// Skip control_response messages except when we are waiting for one.
	// We use a flag WAITING_RESP to know when to capture the response.
	sb.WriteString("  if [ \"$WAITING_RESP\" = \"1\" ]; then\n")
	sb.WriteString("    case \"$input\" in *control_response*)\n")
	sb.WriteString("      LAST_RESP=\"$input\"\n")
	sb.WriteString("      WAITING_RESP=0\n")
	sb.WriteString("      continue\n")
	sb.WriteString("    ;; esac\n")
	sb.WriteString("  fi\n")

	// Respond to control_request messages from SDK (e.g. initialize) with success.
	sb.WriteString("  case \"$input\" in *control_request*)\n")
	sb.WriteString("    REQ_ID=$(echo \"$input\" | sed 's/.*request_id\":\"\\([^\"]*\\).*/\\1/')\n")
	sb.WriteString("    echo \"{\\\"type\\\":\\\"control_response\\\",\\\"response\\\":{\\\"subtype\\\":\\\"success\\\",\\\"request_id\\\":\\\"$REQ_ID\\\",\\\"response\\\":{}}}\"\n")
	sb.WriteString("    continue\n")
	sb.WriteString("  ;; esac\n")

	// Skip non-user messages.
	sb.WriteString("  case \"$input\" in *'\"type\":\"user\"'*) ;; *) continue ;; esac\n")

	for i := range responseTurns {
		fmt.Fprintf(&sb, "  if [ \"$TURN\" -eq %d ]; then\n", i)

		// If there is a control_request for this turn, send it and wait for response.
		if i < len(controlRequests) && controlRequests[i] != "" {
			escaped := strings.ReplaceAll(controlRequests[i], "'", "'\\''")
			fmt.Fprintf(&sb, "    echo '%s'\n", escaped)
			sb.WriteString("    WAITING_RESP=1\n")
			// Read lines until we get the control_response.
			sb.WriteString("    while IFS= read -r resp_input; do\n")
			sb.WriteString("      case \"$resp_input\" in *control_response*)\n")
			sb.WriteString("        LAST_RESP=\"$resp_input\"\n")
			sb.WriteString("        break\n")
			sb.WriteString("      ;; esac\n")
			sb.WriteString("    done\n")
			sb.WriteString("    WAITING_RESP=0\n")
		}

		for _, line := range responseTurns[i] {
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
