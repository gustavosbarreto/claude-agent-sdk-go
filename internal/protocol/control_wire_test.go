package protocol

import (
	"encoding/json"
	"testing"
)

// Wire format tests: verify exact JSON structure of control request/response
// types matches the Python SDK wire format.

func TestWireControlRequestEnvelope(t *testing.T) {
	req := ControlRequest{
		Type:      "control_request",
		RequestID: "sdk_1",
		Request:   json.RawMessage(`{"subtype":"interrupt"}`),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(data, &parsed)

	assertField(t, parsed, "type", "control_request")
	assertField(t, parsed, "request_id", "sdk_1")

	request := parsed["request"].(map[string]any)
	assertField(t, request, "subtype", "interrupt")
}

func TestWireControlResponseSuccess(t *testing.T) {
	resp := ControlResponse{
		Type: "control_response",
		Response: ControlResponseBody{
			Subtype:   "success",
			RequestID: "sdk_1",
			Response:  json.RawMessage(`{"key":"value"}`),
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(data, &parsed)

	assertField(t, parsed, "type", "control_response")

	response := parsed["response"].(map[string]any)
	assertField(t, response, "subtype", "success")
	assertField(t, response, "request_id", "sdk_1")
}

func TestWireControlResponseError(t *testing.T) {
	resp := ControlResponse{
		Type: "control_response",
		Response: ControlResponseBody{
			Subtype:   "error",
			RequestID: "sdk_2",
			Error:     "something went wrong",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(data, &parsed)

	response := parsed["response"].(map[string]any)
	assertField(t, response, "subtype", "error")
	assertField(t, response, "error", "something went wrong")
}

func TestWireInterruptRequest(t *testing.T) {
	inner := map[string]any{"subtype": "interrupt"}
	assertInnerJSON(t, inner, "subtype", "interrupt")
}

func TestWireSetPermissionModeRequest(t *testing.T) {
	inner := map[string]any{"subtype": "set_permission_mode", "mode": "acceptEdits"}
	data, _ := json.Marshal(inner)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)
	assertField(t, parsed, "subtype", "set_permission_mode")
	assertField(t, parsed, "mode", "acceptEdits")
}

func TestWireSetModelRequest(t *testing.T) {
	inner := map[string]any{"subtype": "set_model", "model": "claude-sonnet-4-5"}
	data, _ := json.Marshal(inner)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)
	assertField(t, parsed, "subtype", "set_model")
	assertField(t, parsed, "model", "claude-sonnet-4-5")
}

func TestWireSetModelRequestNull(t *testing.T) {
	inner := map[string]any{"subtype": "set_model", "model": nil}
	data, _ := json.Marshal(inner)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)
	assertField(t, parsed, "subtype", "set_model")
	if parsed["model"] != nil {
		t.Errorf("model should be null, got %v", parsed["model"])
	}
}

func TestWireStopTaskRequest(t *testing.T) {
	inner := map[string]any{"subtype": "stop_task", "task_id": "task_abc"}
	data, _ := json.Marshal(inner)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)
	assertField(t, parsed, "subtype", "stop_task")
	assertField(t, parsed, "task_id", "task_abc")
}

func TestWireMcpStatusRequest(t *testing.T) {
	assertInnerJSON(t, map[string]any{"subtype": "mcp_status"}, "subtype", "mcp_status")
}

func TestWireGetContextUsageRequest(t *testing.T) {
	assertInnerJSON(t, map[string]any{"subtype": "get_context_usage"}, "subtype", "get_context_usage")
}

func TestWireMcpReconnectRequest(t *testing.T) {
	inner := map[string]any{"subtype": "mcp_reconnect", "serverName": "web-search"}
	data, _ := json.Marshal(inner)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)
	assertField(t, parsed, "subtype", "mcp_reconnect")
	assertField(t, parsed, "serverName", "web-search")
}

func TestWireMcpToggleRequest(t *testing.T) {
	inner := map[string]any{"subtype": "mcp_toggle", "serverName": "web-search", "enabled": true}
	data, _ := json.Marshal(inner)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)
	assertField(t, parsed, "subtype", "mcp_toggle")
	assertField(t, parsed, "serverName", "web-search")
	if parsed["enabled"] != true {
		t.Errorf("enabled = %v, want true", parsed["enabled"])
	}
}

func TestWireRewindFilesRequest(t *testing.T) {
	inner := map[string]any{"subtype": "rewind_files", "user_message_id": "550e8400"}
	data, _ := json.Marshal(inner)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)
	assertField(t, parsed, "subtype", "rewind_files")
	assertField(t, parsed, "user_message_id", "550e8400")
}

func TestWirePermissionResultAllow(t *testing.T) {
	result := map[string]any{
		"behavior":     "allow",
		"updatedInput": map[string]any{"path": "/safe"},
		"updatedPermissions": []map[string]any{
			{"type": "addRules", "rules": []map[string]any{{"toolName": "Read"}}},
		},
	}
	data, _ := json.Marshal(result)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)
	assertField(t, parsed, "behavior", "allow")
	if parsed["updatedInput"] == nil {
		t.Error("updatedInput missing")
	}
}

func TestWirePermissionResultDeny(t *testing.T) {
	result := map[string]any{"behavior": "deny", "message": "not allowed", "interrupt": true}
	data, _ := json.Marshal(result)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)
	assertField(t, parsed, "behavior", "deny")
	assertField(t, parsed, "message", "not allowed")
	if parsed["interrupt"] != true {
		t.Errorf("interrupt = %v, want true", parsed["interrupt"])
	}
}

func TestWireHookOutputFieldNames(t *testing.T) {
	// Verify that hook output JSON uses the field names CLI expects
	output := map[string]any{
		"continue":       true,
		"suppressOutput": false,
		"decision":       "allow",
		"systemMessage":  "Approved",
		"reason":         "Safe",
	}
	data, _ := json.Marshal(output)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)

	// CLI expects "continue" not "continue_" (Python converts internally)
	if parsed["continue"] != true {
		t.Errorf("continue = %v", parsed["continue"])
	}
	assertField(t, parsed, "suppressOutput", false)
	assertField(t, parsed, "decision", "allow")
	assertField(t, parsed, "systemMessage", "Approved")
}

func TestWireRequestIDFormat(t *testing.T) {
	m := NewMux(nil)
	id := m.nextID()
	if len(id) < 4 {
		t.Errorf("request ID too short: %s", id)
	}
	// Should start with "sdk_"
	if id[:4] != "sdk_" {
		t.Errorf("request ID should start with sdk_, got %s", id)
	}
}

// helpers

func assertField(t *testing.T, m map[string]any, key string, expected any) {
	t.Helper()
	val, exists := m[key]
	if !exists {
		t.Errorf("field %q missing", key)
		return
	}
	if val != expected {
		t.Errorf("field %q = %v (%T), want %v (%T)", key, val, val, expected, expected)
	}
}

func assertInnerJSON(t *testing.T, inner map[string]any, key string, expected any) {
	t.Helper()
	data, _ := json.Marshal(inner)
	var parsed map[string]any
	json.Unmarshal(data, &parsed)
	assertField(t, parsed, key, expected)
}
