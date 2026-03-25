package claude

import (
	"encoding/json"
	"testing"
)

func TestHookInput_Fields(t *testing.T) {
	input := HookInput{
		SessionID:      "sess-123",
		TranscriptPath: "/tmp/transcript.json",
		Cwd:            "/workspace",
		PermissionMode: "default",
		HookEventName:  "PreToolUse",
		ToolName:       "Bash",
		ToolInput:      json.RawMessage(`{"command":"ls"}`),
		ToolUseID:      "tool-456",
	}

	if input.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", input.SessionID, "sess-123")
	}
	if input.TranscriptPath != "/tmp/transcript.json" {
		t.Errorf("TranscriptPath = %q, want %q", input.TranscriptPath, "/tmp/transcript.json")
	}
	if input.Cwd != "/workspace" {
		t.Errorf("Cwd = %q, want %q", input.Cwd, "/workspace")
	}
	if input.PermissionMode != "default" {
		t.Errorf("PermissionMode = %q, want %q", input.PermissionMode, "default")
	}
	if input.HookEventName != "PreToolUse" {
		t.Errorf("HookEventName = %q, want %q", input.HookEventName, "PreToolUse")
	}
	if input.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want %q", input.ToolName, "Bash")
	}
	if string(input.ToolInput) != `{"command":"ls"}` {
		t.Errorf("ToolInput = %s, want %s", input.ToolInput, `{"command":"ls"}`)
	}
	if input.ToolUseID != "tool-456" {
		t.Errorf("ToolUseID = %q, want %q", input.ToolUseID, "tool-456")
	}
}

func TestHookInput_AgentFields(t *testing.T) {
	input := HookInput{
		SessionID:     "sess-789",
		HookEventName: "SubagentStart",
		AgentID:       "agent-001",
		AgentType:     "custom",
		Cwd:           "/workspace",
	}

	if input.AgentID != "agent-001" {
		t.Errorf("AgentID = %q, want %q", input.AgentID, "agent-001")
	}
	if input.AgentType != "custom" {
		t.Errorf("AgentType = %q, want %q", input.AgentType, "custom")
	}
}

func TestHookInput_JSON(t *testing.T) {
	raw := `{
		"session_id": "s1",
		"transcript_path": "/tmp/t.json",
		"cwd": "/home",
		"hook_event_name": "PostToolUse",
		"tool_name": "Edit",
		"tool_use_id": "tu-1",
		"tool_response": {"status":"ok"},
		"agent_id": "a1",
		"agent_type": "subagent"
	}`

	var input HookInput
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if input.SessionID != "s1" {
		t.Errorf("SessionID = %q, want %q", input.SessionID, "s1")
	}
	if input.ToolName != "Edit" {
		t.Errorf("ToolName = %q, want %q", input.ToolName, "Edit")
	}
	if input.AgentID != "a1" {
		t.Errorf("AgentID = %q, want %q", input.AgentID, "a1")
	}
	if string(input.ToolResponse) != `{"status":"ok"}` {
		t.Errorf("ToolResponse = %s, want %s", input.ToolResponse, `{"status":"ok"}`)
	}
}

func TestHookOutput_AllFields(t *testing.T) {
	cont := true
	output := HookOutput{
		Continue:             &cont,
		Decision:             "allow",
		DecisionReason:       "safe command",
		UpdatedInput:         map[string]any{"command": "echo hi"},
		AdditionalContext:    "extra context",
		SystemMessage:        "system note",
		SuppressOutput:       true,
		BlockStop:            true,
		StopReason:           "hook stopped",
		Reason:               "testing",
		UpdatedMCPToolOutput: map[string]any{"result": "modified"},
	}

	if *output.Continue != true {
		t.Error("Continue should be true")
	}
	if output.Decision != "allow" {
		t.Errorf("Decision = %q, want %q", output.Decision, "allow")
	}
	if output.DecisionReason != "safe command" {
		t.Errorf("DecisionReason = %q, want %q", output.DecisionReason, "safe command")
	}
	if output.UpdatedInput["command"] != "echo hi" {
		t.Errorf("UpdatedInput[command] = %v, want %q", output.UpdatedInput["command"], "echo hi")
	}
	if output.AdditionalContext != "extra context" {
		t.Errorf("AdditionalContext = %q, want %q", output.AdditionalContext, "extra context")
	}
	if output.SystemMessage != "system note" {
		t.Errorf("SystemMessage = %q, want %q", output.SystemMessage, "system note")
	}
	if !output.SuppressOutput {
		t.Error("SuppressOutput should be true")
	}
	if !output.BlockStop {
		t.Error("BlockStop should be true")
	}
	if output.StopReason != "hook stopped" {
		t.Errorf("StopReason = %q, want %q", output.StopReason, "hook stopped")
	}
	if output.Reason != "testing" {
		t.Errorf("Reason = %q, want %q", output.Reason, "testing")
	}
}

func TestHookOutput_EmptyIsValid(t *testing.T) {
	output := HookOutput{}

	if output.Continue != nil {
		t.Error("Continue should be nil for empty output")
	}
	if output.Decision != "" {
		t.Error("Decision should be empty")
	}
	if output.Reason != "" {
		t.Error("Reason should be empty")
	}

	// formatHookOutput with empty output should produce an empty map.
	result := formatHookOutput(output, "")
	if len(result) != 0 {
		t.Errorf("formatHookOutput(empty) = %v, want empty map", result)
	}
}

func TestFormatHookOutput_PreToolUse(t *testing.T) {
	output := HookOutput{
		Decision:       "deny",
		DecisionReason: "dangerous command",
		UpdatedInput:   map[string]any{"command": "safe"},
	}

	result := formatHookOutput(output, "PreToolUse")

	specific, ok := result["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("hookSpecificOutput missing or wrong type")
	}
	if specific["permissionDecision"] != "deny" {
		t.Errorf("permissionDecision = %v, want %q", specific["permissionDecision"], "deny")
	}
	if specific["permissionDecisionReason"] != "dangerous command" {
		t.Errorf("permissionDecisionReason = %v, want %q", specific["permissionDecisionReason"], "dangerous command")
	}
	if specific["hookEventName"] != "PreToolUse" {
		t.Errorf("hookEventName = %v, want %q", specific["hookEventName"], "PreToolUse")
	}
	updatedInput, ok := specific["updatedInput"].(map[string]any)
	if !ok {
		t.Fatal("updatedInput missing or wrong type")
	}
	if updatedInput["command"] != "safe" {
		t.Errorf("updatedInput[command] = %v, want %q", updatedInput["command"], "safe")
	}
}

func TestFormatHookOutput_PostToolUse(t *testing.T) {
	output := HookOutput{
		UpdatedMCPToolOutput: map[string]any{"content": "modified result"},
	}

	result := formatHookOutput(output, "PostToolUse")

	specific, ok := result["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("hookSpecificOutput missing or wrong type")
	}
	mcpOutput, ok := specific["updatedMCPToolOutput"].(map[string]any)
	if !ok {
		t.Fatal("updatedMCPToolOutput missing or wrong type")
	}
	if mcpOutput["content"] != "modified result" {
		t.Errorf("updatedMCPToolOutput[content] = %v, want %q", mcpOutput["content"], "modified result")
	}
	if specific["hookEventName"] != "PostToolUse" {
		t.Errorf("hookEventName = %v, want %q", specific["hookEventName"], "PostToolUse")
	}
}

func TestFormatHookOutput_Notification(t *testing.T) {
	output := HookOutput{}

	result := formatHookOutput(output, "Notification")

	// Empty output with notification event should produce no hookSpecificOutput.
	if _, ok := result["hookSpecificOutput"]; ok {
		t.Error("hookSpecificOutput should not be present for empty notification output")
	}
	if len(result) != 0 {
		t.Errorf("result should be empty map, got %v", result)
	}
}

func TestFormatHookOutput_StopReason(t *testing.T) {
	cont := false
	output := HookOutput{
		Continue:   &cont,
		StopReason: "user requested stop",
		Reason:     "hook decided to stop",
	}

	result := formatHookOutput(output, "Stop")

	if result["continue"] != false {
		t.Errorf("continue = %v, want false", result["continue"])
	}
	if result["stopReason"] != "user requested stop" {
		t.Errorf("stopReason = %v, want %q", result["stopReason"], "user requested stop")
	}
	if result["reason"] != "hook decided to stop" {
		t.Errorf("reason = %v, want %q", result["reason"], "hook decided to stop")
	}
}

func TestFormatHookOutput_AdditionalContext(t *testing.T) {
	output := HookOutput{
		AdditionalContext: "Remember to be safe",
	}

	result := formatHookOutput(output, "PreToolUse")

	specific, ok := result["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("hookSpecificOutput missing or wrong type")
	}
	if specific["additionalContext"] != "Remember to be safe" {
		t.Errorf("additionalContext = %v, want %q", specific["additionalContext"], "Remember to be safe")
	}
}

func TestHookEvents_AllDefined(t *testing.T) {
	// All 23 hook event constants must exist with the expected string values.
	events := map[HookEvent]string{
		HookPreToolUse:         "PreToolUse",
		HookPostToolUse:        "PostToolUse",
		HookPostToolUseFailure: "PostToolUseFailure",
		HookNotification:       "Notification",
		HookUserPromptSubmit:   "UserPromptSubmit",
		HookSessionStart:       "SessionStart",
		HookSessionEnd:         "SessionEnd",
		HookStop:               "Stop",
		HookStopFailure:        "StopFailure",
		HookSubagentStart:      "SubagentStart",
		HookSubagentStop:       "SubagentStop",
		HookPreCompact:         "PreCompact",
		HookPostCompact:        "PostCompact",
		HookPermissionRequest:  "PermissionRequest",
		HookSetup:              "Setup",
		HookTeammateIdle:       "TeammateIdle",
		HookTaskCompleted:      "TaskCompleted",
		HookElicitation:        "Elicitation",
		HookElicitationResult:  "ElicitationResult",
		HookConfigChange:       "ConfigChange",
		HookWorktreeCreate:     "WorktreeCreate",
		HookWorktreeRemove:     "WorktreeRemove",
		HookInstructionsLoaded: "InstructionsLoaded",
	}

	if len(events) != 23 {
		t.Fatalf("expected 23 hook events, got %d", len(events))
	}

	for event, want := range events {
		if string(event) != want {
			t.Errorf("HookEvent %v = %q, want %q", event, string(event), want)
		}
	}
}

func TestHookInput_NotificationFields(t *testing.T) {
	input := HookInput{
		SessionID:        "s1",
		HookEventName:    "Notification",
		Message:          "Task completed",
		Title:            "Done",
		NotificationType: "info",
	}

	if input.Message != "Task completed" {
		t.Errorf("Message = %q, want %q", input.Message, "Task completed")
	}
	if input.Title != "Done" {
		t.Errorf("Title = %q, want %q", input.Title, "Done")
	}
	if input.NotificationType != "info" {
		t.Errorf("NotificationType = %q, want %q", input.NotificationType, "info")
	}
}

func TestHookInput_CompactFields(t *testing.T) {
	custom := "Be concise"
	input := HookInput{
		SessionID:          "s1",
		HookEventName:      "PostCompact",
		Trigger:            "auto",
		CustomInstructions: &custom,
		CompactSummary:     "Summarized conversation",
	}

	if input.Trigger != "auto" {
		t.Errorf("Trigger = %q, want %q", input.Trigger, "auto")
	}
	if *input.CustomInstructions != "Be concise" {
		t.Errorf("CustomInstructions = %q, want %q", *input.CustomInstructions, "Be concise")
	}
	if input.CompactSummary != "Summarized conversation" {
		t.Errorf("CompactSummary = %q, want %q", input.CompactSummary, "Summarized conversation")
	}
}

func TestHookInput_StopFields(t *testing.T) {
	input := HookInput{
		SessionID:            "s1",
		HookEventName:        "Stop",
		StopHookActive:       true,
		LastAssistantMessage: "I have completed the task.",
	}

	if !input.StopHookActive {
		t.Error("StopHookActive should be true")
	}
	if input.LastAssistantMessage != "I have completed the task." {
		t.Errorf("LastAssistantMessage = %q, want %q", input.LastAssistantMessage, "I have completed the task.")
	}
}

func TestHookInput_ElicitationFields(t *testing.T) {
	input := HookInput{
		SessionID:       "s1",
		HookEventName:   "Elicitation",
		MCPServerName:   "my-server",
		Mode:            "interactive",
		ElicitationID:   "elicit-1",
		RequestedSchema: json.RawMessage(`{"type":"object"}`),
		Action:          "confirm",
		ElicitContent:   json.RawMessage(`{"confirmed":true}`),
	}

	if input.MCPServerName != "my-server" {
		t.Errorf("MCPServerName = %q, want %q", input.MCPServerName, "my-server")
	}
	if input.ElicitationID != "elicit-1" {
		t.Errorf("ElicitationID = %q, want %q", input.ElicitationID, "elicit-1")
	}
	if string(input.RequestedSchema) != `{"type":"object"}` {
		t.Errorf("RequestedSchema = %s, want %s", input.RequestedSchema, `{"type":"object"}`)
	}
}

func TestHookOutput_ElicitationFields(t *testing.T) {
	output := HookOutput{
		ElicitAction:  "submit",
		ElicitContent: map[string]any{"name": "test"},
	}

	if output.ElicitAction != "submit" {
		t.Errorf("ElicitAction = %q, want %q", output.ElicitAction, "submit")
	}
	if output.ElicitContent["name"] != "test" {
		t.Errorf("ElicitContent[name] = %v, want %q", output.ElicitContent["name"], "test")
	}
}

func TestHookOutput_PermissionDecision(t *testing.T) {
	output := HookOutput{
		PermissionDecision: &PermissionDecision{
			Behavior:     "deny",
			UpdatedInput: map[string]any{"command": "safe"},
			Message:      "rejected",
			Interrupt:    true,
		},
	}

	if output.PermissionDecision.Behavior != "deny" {
		t.Errorf("Behavior = %q, want %q", output.PermissionDecision.Behavior, "deny")
	}
	if !output.PermissionDecision.Interrupt {
		t.Error("Interrupt should be true")
	}
	if output.PermissionDecision.Message != "rejected" {
		t.Errorf("Message = %q, want %q", output.PermissionDecision.Message, "rejected")
	}
}

func TestFormatHookOutput_BlockStop(t *testing.T) {
	output := HookOutput{
		BlockStop: true,
	}

	result := formatHookOutput(output, "Stop")

	specific, ok := result["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatal("hookSpecificOutput missing or wrong type")
	}
	if specific["blockStop"] != true {
		t.Errorf("blockStop = %v, want true", specific["blockStop"])
	}
}

func TestFormatHookOutput_SystemMessage(t *testing.T) {
	output := HookOutput{
		SystemMessage: "Remember the rules",
	}

	result := formatHookOutput(output, "PreToolUse")

	if result["systemMessage"] != "Remember the rules" {
		t.Errorf("systemMessage = %v, want %q", result["systemMessage"], "Remember the rules")
	}
}

func TestFormatHookOutput_SuppressOutput(t *testing.T) {
	output := HookOutput{
		SuppressOutput: true,
	}

	result := formatHookOutput(output, "PostToolUse")

	if result["suppressOutput"] != true {
		t.Errorf("suppressOutput = %v, want true", result["suppressOutput"])
	}
}
