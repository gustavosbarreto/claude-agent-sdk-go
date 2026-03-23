package claude

import (
	"encoding/json"
	"testing"
)

func TestParseMessage_System(t *testing.T) {
	line := `{"type":"system","subtype":"init","session_id":"abc-123","model":"claude-sonnet-4-6","cwd":"/workspace","tools":["Bash"]}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sys, ok := msg.(*SystemMessage)
	if !ok {
		t.Fatalf("expected *SystemMessage, got %T", msg)
	}

	if sys.SessionID != "abc-123" {
		t.Errorf("session_id = %q, want %q", sys.SessionID, "abc-123")
	}
	if sys.Model != "claude-sonnet-4-6" {
		t.Errorf("model = %q, want %q", sys.Model, "claude-sonnet-4-6")
	}
	if len(sys.Tools) != 1 || sys.Tools[0] != "Bash" {
		t.Errorf("tools = %v, want [Bash]", sys.Tools)
	}
}

func TestParseMessage_Assistant(t *testing.T) {
	line := `{"type":"assistant","uuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"Hello!"}]}}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	asst, ok := msg.(*AssistantMessage)
	if !ok {
		t.Fatalf("expected *AssistantMessage, got %T", msg)
	}

	if asst.UUID != "u1" {
		t.Errorf("uuid = %q, want %q", asst.UUID, "u1")
	}
	if len(asst.Message.Content) != 1 {
		t.Fatalf("content blocks = %d, want 1", len(asst.Message.Content))
	}
	if asst.Message.Content[0].Text != "Hello!" {
		t.Errorf("text = %q, want %q", asst.Message.Content[0].Text, "Hello!")
	}
}

func TestParseMessage_Result(t *testing.T) {
	line := `{"type":"result","subtype":"success","result":"Done","session_id":"s1","total_cost_usd":0.05,"num_turns":3,"usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":0,"cache_read_input_tokens":80}}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, ok := msg.(*ResultMessage)
	if !ok {
		t.Fatalf("expected *ResultMessage, got %T", msg)
	}

	if res.Subtype != ResultSuccess {
		t.Errorf("subtype = %q, want %q", res.Subtype, ResultSuccess)
	}
	if res.Result != "Done" {
		t.Errorf("result = %q, want %q", res.Result, "Done")
	}
	if res.TotalCostUSD != 0.05 {
		t.Errorf("total_cost_usd = %f, want 0.05", res.TotalCostUSD)
	}
	if res.Usage == nil {
		t.Fatal("usage is nil")
	}
	if res.Usage.InputTokens != 100 {
		t.Errorf("input_tokens = %d, want 100", res.Usage.InputTokens)
	}
}

func TestParseMessage_User(t *testing.T) {
	line := `{"type":"user","message":{"role":"user","content":"hello"},"isReplay":true}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	user, ok := msg.(*UserMessage)
	if !ok {
		t.Fatalf("expected *UserMessage, got %T", msg)
	}

	if !user.IsReplay {
		t.Error("isReplay should be true")
	}
}

func TestParseMessage_ToolProgress(t *testing.T) {
	line := `{"type":"tool_progress","tool_use_id":"tu1","tool_name":"Bash","elapsed_time_seconds":5.2}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tp, ok := msg.(*ToolProgressMessage)
	if !ok {
		t.Fatalf("expected *ToolProgressMessage, got %T", msg)
	}

	if tp.ToolName != "Bash" {
		t.Errorf("tool_name = %q, want %q", tp.ToolName, "Bash")
	}
	if tp.ElapsedTimeSeconds != 5.2 {
		t.Errorf("elapsed = %f, want 5.2", tp.ElapsedTimeSeconds)
	}
}

func TestParseMessage_RateLimit(t *testing.T) {
	line := `{"type":"rate_limit_event","rate_limit_info":{"status":"allowed_warning","utilization":0.85}}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rl, ok := msg.(*RateLimitEvent)
	if !ok {
		t.Fatalf("expected *RateLimitEvent, got %T", msg)
	}

	if rl.RateLimitInfo.Status != "allowed_warning" {
		t.Errorf("status = %q, want %q", rl.RateLimitInfo.Status, "allowed_warning")
	}
}

func TestParseMessage_PromptSuggestion(t *testing.T) {
	line := `{"type":"prompt_suggestion","suggestion":"Can you also add tests?"}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ps, ok := msg.(*PromptSuggestionMessage)
	if !ok {
		t.Fatalf("expected *PromptSuggestionMessage, got %T", msg)
	}

	if ps.Suggestion != "Can you also add tests?" {
		t.Errorf("suggestion = %q, want %q", ps.Suggestion, "Can you also add tests?")
	}
}

func TestParseMessage_ToolUseSummary(t *testing.T) {
	line := `{"type":"tool_use_summary","summary":"Read 3 files","preceding_tool_use_ids":["a","b","c"]}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tus, ok := msg.(*ToolUseSummaryMessage)
	if !ok {
		t.Fatalf("expected *ToolUseSummaryMessage, got %T", msg)
	}

	if tus.Summary != "Read 3 files" {
		t.Errorf("summary = %q", tus.Summary)
	}
	if len(tus.PrecedingToolUseIDs) != 3 {
		t.Errorf("preceding ids = %d, want 3", len(tus.PrecedingToolUseIDs))
	}
}

func TestParseMessage_AuthStatus(t *testing.T) {
	line := `{"type":"auth_status","isAuthenticating":true,"output":["Waiting..."]}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	as, ok := msg.(*AuthStatusMessage)
	if !ok {
		t.Fatalf("expected *AuthStatusMessage, got %T", msg)
	}

	if !as.IsAuthenticating {
		t.Error("isAuthenticating should be true")
	}
}

func TestParseMessage_Unknown(t *testing.T) {
	line := `{"type":"some_future_type","data":"hello"}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, ok := msg.(*RawMessage)
	if !ok {
		t.Fatalf("expected *RawMessage, got %T", msg)
	}

	if raw.TypeField != "some_future_type" {
		t.Errorf("type = %q, want %q", raw.TypeField, "some_future_type")
	}
}

func TestParseMessage_Invalid(t *testing.T) {
	_, err := ParseMessage([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Line != "not json" {
		t.Errorf("line = %q", pe.Line)
	}
}

func TestParseMessage_SystemTaskNotification(t *testing.T) {
	line := `{"type":"system","subtype":"task_notification","task_id":"t1","status":"completed","summary":"Done","output_file":"/tmp/out"}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tn, ok := msg.(*TaskNotificationMessage)
	if !ok {
		t.Fatalf("expected *TaskNotificationMessage, got %T", msg)
	}

	if tn.Subtype != "task_notification" {
		t.Errorf("subtype = %q", tn.Subtype)
	}
	if tn.TaskID != "t1" {
		t.Errorf("task_id = %q", tn.TaskID)
	}
	if tn.Summary != "Done" {
		t.Errorf("summary = %q", tn.Summary)
	}
}

func TestContentBlocks(t *testing.T) {
	blocks := []ContentBlock{
		{Type: ContentBlockText, Text: "Hello "},
		{Type: ContentBlockToolUse, Name: "Bash", ID: "tu1"},
		{Type: ContentBlockText, Text: "World"},
	}

	text := CombinedText(blocks)
	if text != "Hello World" {
		t.Errorf("CombinedText = %q, want %q", text, "Hello World")
	}

	textBlocks := TextBlocks(blocks)
	if len(textBlocks) != 2 {
		t.Errorf("TextBlocks = %d, want 2", len(textBlocks))
	}

	toolBlocks := ToolUseBlocks(blocks)
	if len(toolBlocks) != 1 {
		t.Errorf("ToolUseBlocks = %d, want 1", len(toolBlocks))
	}
}

func TestMCPServerMarshal(t *testing.T) {
	s := MCPStdioServer{
		Command: "node",
		Args:    []string{"server.js"},
		Env:     map[string]string{"TOKEN": "abc"},
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	json.Unmarshal(data, &m)

	if m["type"] != "stdio" {
		t.Errorf("type = %v, want stdio", m["type"])
	}
	if m["command"] != "node" {
		t.Errorf("command = %v, want node", m["command"])
	}
}

func TestOptions(t *testing.T) {
	cfg := applyOptions([]Option{
		WithModel("claude-opus-4-6"),
		WithCwd("/workspace"),
		WithMaxTurns(10),
		WithPermissionMode(PermissionBypassPermissions),
		WithAllowDangerouslySkipPermissions(),
		WithAllowedTools("Bash", "Read"),
		WithEffort("high"),
		WithBetas("context-1m-2025-08-07"),
		WithMCPServer("myserver", MCPStdioServer{Command: "node"}),
		WithVerbose(),
	})

	if cfg.Model != "claude-opus-4-6" {
		t.Errorf("model = %q", cfg.Model)
	}
	if cfg.Cwd != "/workspace" {
		t.Errorf("cwd = %q", cfg.Cwd)
	}
	if cfg.MaxTurns == nil || *cfg.MaxTurns != 10 {
		t.Errorf("max_turns = %v", cfg.MaxTurns)
	}
	if cfg.PermissionMode != PermissionBypassPermissions {
		t.Errorf("permission_mode = %q", cfg.PermissionMode)
	}
	if !cfg.AllowDangerouslySkipPermissions {
		t.Error("AllowDangerouslySkipPermissions should be true")
	}
	if len(cfg.AllowedTools) != 2 {
		t.Errorf("allowed_tools = %v", cfg.AllowedTools)
	}
	if cfg.Effort != "high" {
		t.Errorf("effort = %q", cfg.Effort)
	}
	if len(cfg.Betas) != 1 {
		t.Errorf("betas = %v", cfg.Betas)
	}
	if len(cfg.MCPServers) != 1 {
		t.Errorf("mcp_servers = %d", len(cfg.MCPServers))
	}
	if !cfg.Verbose {
		t.Error("verbose should be true")
	}
}

func TestErrors(t *testing.T) {
	pe := &ProcessError{ExitCode: 1, Stderr: "something went wrong"}
	if pe.Error() != "claude: process exited with code 1: something went wrong" {
		t.Errorf("ProcessError = %q", pe.Error())
	}

	pe2 := &ProcessError{ExitCode: 2}
	if pe2.Error() != "claude: process exited with code 2" {
		t.Errorf("ProcessError = %q", pe2.Error())
	}

	var dummy any
	parseErr := &ParseError{Line: "bad", Err: json.Unmarshal([]byte("bad"), &dummy)}
	if parseErr.Unwrap() == nil {
		t.Error("Unwrap should return underlying error")
	}
}

// --- New edge-case and type-validation tests ---

func TestParseMessage_TaskStarted(t *testing.T) {
	line := `{"type":"system","subtype":"task_started","session_id":"s1","task_id":"t42","description":"Run tests","tool_use_id":"tu5","task_type":"local_bash"}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ts, ok := msg.(*TaskStartedMessage)
	if !ok {
		t.Fatalf("expected *TaskStartedMessage, got %T", msg)
	}

	if ts.Subtype != "task_started" {
		t.Errorf("subtype = %q, want %q", ts.Subtype, "task_started")
	}
	if ts.TaskID != "t42" {
		t.Errorf("task_id = %q, want %q", ts.TaskID, "t42")
	}
	if ts.Description != "Run tests" {
		t.Errorf("description = %q, want %q", ts.Description, "Run tests")
	}
	if ts.ToolUseID != "tu5" {
		t.Errorf("tool_use_id = %q, want %q", ts.ToolUseID, "tu5")
	}
	if ts.TaskType != "local_bash" {
		t.Errorf("task_type = %q, want %q", ts.TaskType, "local_bash")
	}
}

func TestParseMessage_TaskProgress(t *testing.T) {
	line := `{"type":"system","subtype":"task_progress","session_id":"s1","task_id":"t42","description":"Still running","last_tool_name":"Bash","usage":{"total_tokens":500,"tool_uses":3,"duration_ms":12000}}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tp, ok := msg.(*TaskProgressMessage)
	if !ok {
		t.Fatalf("expected *TaskProgressMessage, got %T", msg)
	}

	if tp.TaskID != "t42" {
		t.Errorf("task_id = %q, want %q", tp.TaskID, "t42")
	}
	if tp.Description != "Still running" {
		t.Errorf("description = %q, want %q", tp.Description, "Still running")
	}
	if tp.LastToolName != "Bash" {
		t.Errorf("last_tool_name = %q, want %q", tp.LastToolName, "Bash")
	}
	if tp.TaskUsage == nil {
		t.Fatal("usage is nil")
	}
	if tp.TaskUsage.TotalTokens != 500 {
		t.Errorf("total_tokens = %d, want 500", tp.TaskUsage.TotalTokens)
	}
	if tp.TaskUsage.ToolUses != 3 {
		t.Errorf("tool_uses = %d, want 3", tp.TaskUsage.ToolUses)
	}
	if tp.TaskUsage.DurationMS != 12000 {
		t.Errorf("duration_ms = %d, want 12000", tp.TaskUsage.DurationMS)
	}
}

func TestParseMessage_TaskNotificationCompleted(t *testing.T) {
	line := `{"type":"system","subtype":"task_notification","session_id":"s1","task_id":"t42","status":"completed","summary":"All tests passed","output_file":"/tmp/out","usage":{"total_tokens":1000,"tool_uses":5,"duration_ms":30000}}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tn, ok := msg.(*TaskNotificationMessage)
	if !ok {
		t.Fatalf("expected *TaskNotificationMessage, got %T", msg)
	}

	if tn.Subtype != "task_notification" {
		t.Errorf("subtype = %q, want %q", tn.Subtype, "task_notification")
	}
	if tn.TaskID != "t42" {
		t.Errorf("task_id = %q, want %q", tn.TaskID, "t42")
	}
	if tn.Status != "completed" {
		t.Errorf("status = %q, want %q", tn.Status, "completed")
	}
	if tn.Summary != "All tests passed" {
		t.Errorf("summary = %q, want %q", tn.Summary, "All tests passed")
	}
	if tn.OutputFile != "/tmp/out" {
		t.Errorf("output_file = %q, want %q", tn.OutputFile, "/tmp/out")
	}
	if tn.TaskUsage == nil {
		t.Fatal("usage is nil")
	}
	if tn.TaskUsage.TotalTokens != 1000 {
		t.Errorf("total_tokens = %d, want 1000", tn.TaskUsage.TotalTokens)
	}
}

func TestParseMessage_TaskNotificationFailed(t *testing.T) {
	line := `{"type":"system","subtype":"task_notification","session_id":"s1","task_id":"t99","status":"failed","summary":"Timeout exceeded"}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tn, ok := msg.(*TaskNotificationMessage)
	if !ok {
		t.Fatalf("expected *TaskNotificationMessage, got %T", msg)
	}

	if tn.Status != "failed" {
		t.Errorf("status = %q, want %q", tn.Status, "failed")
	}
	if tn.Summary != "Timeout exceeded" {
		t.Errorf("summary = %q, want %q", tn.Summary, "Timeout exceeded")
	}
	if tn.TaskUsage != nil {
		t.Errorf("usage should be nil for failed task, got %+v", tn.TaskUsage)
	}
}

func TestParseMessage_SystemUnknownSubtype(t *testing.T) {
	line := `{"type":"system","subtype":"some_future_subtype","session_id":"s1"}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sys, ok := msg.(*SystemMessage)
	if !ok {
		t.Fatalf("expected *SystemMessage, got %T", msg)
	}

	// Must NOT be dispatched to a task type.
	if _, bad := msg.(*TaskStartedMessage); bad {
		t.Fatal("should not be *TaskStartedMessage")
	}
	if _, bad := msg.(*TaskProgressMessage); bad {
		t.Fatal("should not be *TaskProgressMessage")
	}
	if _, bad := msg.(*TaskNotificationMessage); bad {
		t.Fatal("should not be *TaskNotificationMessage")
	}

	if sys.Subtype != "some_future_subtype" {
		t.Errorf("subtype = %q, want %q", sys.Subtype, "some_future_subtype")
	}
}

func TestParseMessage_AssistantWithThinking(t *testing.T) {
	line := `{"type":"assistant","uuid":"u2","message":{"role":"assistant","content":[{"type":"thinking","thinking":"Let me consider...","signature":"sig123"},{"type":"text","text":"Here is my answer."}]}}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	asst, ok := msg.(*AssistantMessage)
	if !ok {
		t.Fatalf("expected *AssistantMessage, got %T", msg)
	}

	if len(asst.Message.Content) != 2 {
		t.Fatalf("content blocks = %d, want 2", len(asst.Message.Content))
	}

	thinking := asst.Message.Content[0]
	if thinking.Type != ContentBlockThinking {
		t.Errorf("block[0].type = %q, want %q", thinking.Type, ContentBlockThinking)
	}
	if thinking.Thinking != "Let me consider..." {
		t.Errorf("thinking = %q, want %q", thinking.Thinking, "Let me consider...")
	}
	if thinking.Signature != "sig123" {
		t.Errorf("signature = %q, want %q", thinking.Signature, "sig123")
	}

	text := asst.Message.Content[1]
	if text.Type != ContentBlockText {
		t.Errorf("block[1].type = %q, want %q", text.Type, ContentBlockText)
	}
	if text.Text != "Here is my answer." {
		t.Errorf("text = %q, want %q", text.Text, "Here is my answer.")
	}
}

func TestParseMessage_AssistantWithToolUse(t *testing.T) {
	line := `{"type":"assistant","uuid":"u3","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu10","name":"Read","input":{"file_path":"/tmp/test.go"}}]}}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	asst, ok := msg.(*AssistantMessage)
	if !ok {
		t.Fatalf("expected *AssistantMessage, got %T", msg)
	}

	if len(asst.Message.Content) != 1 {
		t.Fatalf("content blocks = %d, want 1", len(asst.Message.Content))
	}

	tu := asst.Message.Content[0]
	if tu.Type != ContentBlockToolUse {
		t.Errorf("type = %q, want %q", tu.Type, ContentBlockToolUse)
	}
	if tu.ID != "tu10" {
		t.Errorf("id = %q, want %q", tu.ID, "tu10")
	}
	if tu.Name != "Read" {
		t.Errorf("name = %q, want %q", tu.Name, "Read")
	}
	if tu.Input == nil {
		t.Fatal("input is nil")
	}

	var input map[string]string
	if err := json.Unmarshal(tu.Input, &input); err != nil {
		t.Fatalf("unmarshal input: %v", err)
	}
	if input["file_path"] != "/tmp/test.go" {
		t.Errorf("file_path = %q, want %q", input["file_path"], "/tmp/test.go")
	}
}

func TestParseMessage_AssistantWithError(t *testing.T) {
	line := `{"type":"assistant","uuid":"u4","message":{"role":"assistant","content":[]},"error":"rate_limit"}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	asst, ok := msg.(*AssistantMessage)
	if !ok {
		t.Fatalf("expected *AssistantMessage, got %T", msg)
	}

	if asst.Error != "rate_limit" {
		t.Errorf("error = %q, want %q", asst.Error, "rate_limit")
	}
}

func TestParseMessage_ResultWithStructuredOutput(t *testing.T) {
	line := `{"type":"result","subtype":"success","result":"done","session_id":"s1","total_cost_usd":0.01,"structured_output":{"key":"value","count":42}}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, ok := msg.(*ResultMessage)
	if !ok {
		t.Fatalf("expected *ResultMessage, got %T", msg)
	}

	if res.StructuredOutput == nil {
		t.Fatal("structured_output is nil")
	}

	var so map[string]any
	if err := json.Unmarshal(res.StructuredOutput, &so); err != nil {
		t.Fatalf("unmarshal structured_output: %v", err)
	}
	if so["key"] != "value" {
		t.Errorf("key = %v, want %q", so["key"], "value")
	}
	if so["count"] != float64(42) {
		t.Errorf("count = %v, want 42", so["count"])
	}
}

func TestParseMessage_ResultErrorMaxTurns(t *testing.T) {
	line := `{"type":"result","subtype":"error_max_turns","result":"","session_id":"s1","is_error":true,"num_turns":25,"total_cost_usd":1.23}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, ok := msg.(*ResultMessage)
	if !ok {
		t.Fatalf("expected *ResultMessage, got %T", msg)
	}

	if res.Subtype != ResultErrorMaxTurns {
		t.Errorf("subtype = %q, want %q", res.Subtype, ResultErrorMaxTurns)
	}
	if !res.IsError {
		t.Error("is_error should be true")
	}
	if res.NumTurns != 25 {
		t.Errorf("num_turns = %d, want 25", res.NumTurns)
	}
}

func TestParseMessage_ResultWithStopReason(t *testing.T) {
	line := `{"type":"result","subtype":"success","result":"Done","session_id":"s1","stop_reason":"end_turn","total_cost_usd":0.02}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, ok := msg.(*ResultMessage)
	if !ok {
		t.Fatalf("expected *ResultMessage, got %T", msg)
	}

	if res.StopReason == nil {
		t.Fatal("stop_reason is nil, want pointer to string")
	}
	if *res.StopReason != "end_turn" {
		t.Errorf("stop_reason = %q, want %q", *res.StopReason, "end_turn")
	}
}

func TestParseMessage_ResultWithNullStopReason(t *testing.T) {
	line := `{"type":"result","subtype":"success","result":"Done","session_id":"s1","stop_reason":null,"total_cost_usd":0.02}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	res, ok := msg.(*ResultMessage)
	if !ok {
		t.Fatalf("expected *ResultMessage, got %T", msg)
	}

	if res.StopReason != nil {
		t.Errorf("stop_reason = %q, want nil", *res.StopReason)
	}
}

func TestParseMessage_StreamEvent(t *testing.T) {
	line := `{"type":"stream_event","uuid":"u5","session_id":"s1","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	se, ok := msg.(*StreamEvent)
	if !ok {
		t.Fatalf("expected *StreamEvent, got %T", msg)
	}

	if se.UUID != "u5" {
		t.Errorf("uuid = %q, want %q", se.UUID, "u5")
	}
	if se.SessionID != "s1" {
		t.Errorf("session_id = %q, want %q", se.SessionID, "s1")
	}
	if se.Event == nil {
		t.Fatal("event is nil")
	}

	var event map[string]any
	if err := json.Unmarshal(se.Event, &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if event["type"] != "content_block_delta" {
		t.Errorf("event.type = %v, want %q", event["type"], "content_block_delta")
	}
}

func TestParseMessage_UserWithToolResult(t *testing.T) {
	line := `{"type":"user","uuid":"u6","message":{"role":"user","content":"result"},"tool_use_result":{"tool_use_id":"tu1","output":"success"}}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	user, ok := msg.(*UserMessage)
	if !ok {
		t.Fatalf("expected *UserMessage, got %T", msg)
	}

	if user.ToolUseResult == nil {
		t.Fatal("tool_use_result is nil")
	}

	var result map[string]any
	if err := json.Unmarshal(user.ToolUseResult, &result); err != nil {
		t.Fatalf("unmarshal tool_use_result: %v", err)
	}
	if result["tool_use_id"] != "tu1" {
		t.Errorf("tool_use_id = %v, want %q", result["tool_use_id"], "tu1")
	}
	if result["output"] != "success" {
		t.Errorf("output = %v, want %q", result["output"], "success")
	}
}

func TestParseMessage_SystemCompactBoundary(t *testing.T) {
	line := `{"type":"system","subtype":"compact_boundary","session_id":"s1","compact_metadata":{"trigger":"auto","pre_tokens":50000}}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sys, ok := msg.(*SystemMessage)
	if !ok {
		t.Fatalf("expected *SystemMessage, got %T", msg)
	}

	if sys.Subtype != "compact_boundary" {
		t.Errorf("subtype = %q, want %q", sys.Subtype, "compact_boundary")
	}
	if sys.CompactMetadata == nil {
		t.Fatal("compact_metadata is nil")
	}
	if sys.CompactMetadata.Trigger != "auto" {
		t.Errorf("trigger = %q, want %q", sys.CompactMetadata.Trigger, "auto")
	}
	if sys.CompactMetadata.PreTokens != 50000 {
		t.Errorf("pre_tokens = %d, want 50000", sys.CompactMetadata.PreTokens)
	}
}

func TestParseMessage_EmptyLine(t *testing.T) {
	_, err := ParseMessage([]byte(""))
	if err == nil {
		t.Fatal("expected error for empty input")
	}

	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Line != "" {
		t.Errorf("line = %q, want empty string", pe.Line)
	}
}

func TestParseMessage_MissingType(t *testing.T) {
	line := `{"data":"hello","count":5}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, ok := msg.(*RawMessage)
	if !ok {
		t.Fatalf("expected *RawMessage, got %T", msg)
	}

	// Empty type string should still yield a RawMessage.
	if raw.TypeField != "" {
		t.Errorf("type = %q, want empty string", raw.TypeField)
	}
}

// --- Content block type validation tests (from test_types.py patterns) ---

func TestContentBlock_Text(t *testing.T) {
	data := `{"type":"text","text":"Hello, world!"}`

	var block ContentBlock
	if err := json.Unmarshal([]byte(data), &block); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if block.Type != ContentBlockText {
		t.Errorf("type = %q, want %q", block.Type, ContentBlockText)
	}
	if block.Text != "Hello, world!" {
		t.Errorf("text = %q, want %q", block.Text, "Hello, world!")
	}
	// Non-text fields should be zero.
	if block.Thinking != "" {
		t.Errorf("thinking should be empty, got %q", block.Thinking)
	}
	if block.ID != "" {
		t.Errorf("id should be empty, got %q", block.ID)
	}
	if block.Name != "" {
		t.Errorf("name should be empty, got %q", block.Name)
	}
}

func TestContentBlock_ToolUse(t *testing.T) {
	data := `{"type":"tool_use","id":"tu99","name":"Bash","input":{"command":"ls -la"}}`

	var block ContentBlock
	if err := json.Unmarshal([]byte(data), &block); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if block.Type != ContentBlockToolUse {
		t.Errorf("type = %q, want %q", block.Type, ContentBlockToolUse)
	}
	if block.ID != "tu99" {
		t.Errorf("id = %q, want %q", block.ID, "tu99")
	}
	if block.Name != "Bash" {
		t.Errorf("name = %q, want %q", block.Name, "Bash")
	}
	if block.Input == nil {
		t.Fatal("input is nil")
	}

	var input map[string]string
	if err := json.Unmarshal(block.Input, &input); err != nil {
		t.Fatalf("unmarshal input: %v", err)
	}
	if input["command"] != "ls -la" {
		t.Errorf("command = %q, want %q", input["command"], "ls -la")
	}
	// Non-tool-use fields should be zero.
	if block.Text != "" {
		t.Errorf("text should be empty, got %q", block.Text)
	}
	if block.Thinking != "" {
		t.Errorf("thinking should be empty, got %q", block.Thinking)
	}
}

func TestContentBlock_Thinking(t *testing.T) {
	data := `{"type":"thinking","thinking":"I need to analyze this carefully.","signature":"abc-sig"}`

	var block ContentBlock
	if err := json.Unmarshal([]byte(data), &block); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if block.Type != ContentBlockThinking {
		t.Errorf("type = %q, want %q", block.Type, ContentBlockThinking)
	}
	if block.Thinking != "I need to analyze this carefully." {
		t.Errorf("thinking = %q, want %q", block.Thinking, "I need to analyze this carefully.")
	}
	if block.Signature != "abc-sig" {
		t.Errorf("signature = %q, want %q", block.Signature, "abc-sig")
	}
	// Non-thinking fields should be zero.
	if block.Text != "" {
		t.Errorf("text should be empty, got %q", block.Text)
	}
	if block.ID != "" {
		t.Errorf("id should be empty, got %q", block.ID)
	}
}

func TestCombinedText(t *testing.T) {
	blocks := []ContentBlock{
		{Type: ContentBlockThinking, Thinking: "Let me think..."},
		{Type: ContentBlockText, Text: "First "},
		{Type: ContentBlockToolUse, Name: "Bash", ID: "tu1"},
		{Type: ContentBlockText, Text: "Second"},
		{Type: ContentBlockThinking, Thinking: "More thinking"},
	}

	text := CombinedText(blocks)
	if text != "First Second" {
		t.Errorf("CombinedText = %q, want %q", text, "First Second")
	}

	// Verify thinking and tool_use blocks are excluded from text.
	textBlocks := TextBlocks(blocks)
	if len(textBlocks) != 2 {
		t.Errorf("TextBlocks count = %d, want 2", len(textBlocks))
	}

	toolBlocks := ToolUseBlocks(blocks)
	if len(toolBlocks) != 1 {
		t.Errorf("ToolUseBlocks count = %d, want 1", len(toolBlocks))
	}

	// Empty blocks should return empty string.
	if got := CombinedText(nil); got != "" {
		t.Errorf("CombinedText(nil) = %q, want empty", got)
	}

	// Only non-text blocks should return empty string.
	nonText := []ContentBlock{
		{Type: ContentBlockThinking, Thinking: "just thinking"},
		{Type: ContentBlockToolUse, Name: "Read", ID: "tu2"},
	}
	if got := CombinedText(nonText); got != "" {
		t.Errorf("CombinedText(non-text) = %q, want empty", got)
	}
}

// --- Tests matching Python SDK's test_rate_limit_event_repro.py ---

func TestParseMessage_RateLimitAllowedWarning(t *testing.T) {
	line := `{"type":"rate_limit_event","session_id":"s1","rate_limit_info":{"status":"allowed_warning","resetsAt":1700000000,"rateLimitType":"token","utilization":0.92}}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rl, ok := msg.(*RateLimitEvent)
	if !ok {
		t.Fatalf("expected *RateLimitEvent, got %T", msg)
	}

	if rl.Type != MessageTypeRateLimit {
		t.Errorf("type = %q, want %q", rl.Type, MessageTypeRateLimit)
	}
	if rl.SessionID != "s1" {
		t.Errorf("session_id = %q, want %q", rl.SessionID, "s1")
	}
	if rl.RateLimitInfo == nil {
		t.Fatal("rate_limit_info is nil")
	}
	if rl.RateLimitInfo.Status != "allowed_warning" {
		t.Errorf("status = %q, want %q", rl.RateLimitInfo.Status, "allowed_warning")
	}
	if rl.RateLimitInfo.ResetsAt == nil {
		t.Fatal("resetsAt is nil")
	}
	if *rl.RateLimitInfo.ResetsAt != 1700000000 {
		t.Errorf("resetsAt = %d, want 1700000000", *rl.RateLimitInfo.ResetsAt)
	}
	if rl.RateLimitInfo.RateLimitType != "token" {
		t.Errorf("rateLimitType = %q, want %q", rl.RateLimitInfo.RateLimitType, "token")
	}
	if rl.RateLimitInfo.Utilization != 0.92 {
		t.Errorf("utilization = %f, want 0.92", rl.RateLimitInfo.Utilization)
	}
}

func TestParseMessage_RateLimitRejected(t *testing.T) {
	line := `{"type":"rate_limit_event","session_id":"s2","rate_limit_info":{"status":"rejected","resetsAt":1700001000,"rateLimitType":"request","utilization":1.0}}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rl, ok := msg.(*RateLimitEvent)
	if !ok {
		t.Fatalf("expected *RateLimitEvent, got %T", msg)
	}

	if rl.RateLimitInfo == nil {
		t.Fatal("rate_limit_info is nil")
	}
	if rl.RateLimitInfo.Status != "rejected" {
		t.Errorf("status = %q, want %q", rl.RateLimitInfo.Status, "rejected")
	}
	if rl.RateLimitInfo.ResetsAt == nil {
		t.Fatal("resetsAt is nil")
	}
	if *rl.RateLimitInfo.ResetsAt != 1700001000 {
		t.Errorf("resetsAt = %d, want 1700001000", *rl.RateLimitInfo.ResetsAt)
	}
	if rl.RateLimitInfo.RateLimitType != "request" {
		t.Errorf("rateLimitType = %q, want %q", rl.RateLimitInfo.RateLimitType, "request")
	}
	if rl.RateLimitInfo.Utilization != 1.0 {
		t.Errorf("utilization = %f, want 1.0", rl.RateLimitInfo.Utilization)
	}
}

func TestParseMessage_RateLimitMinimal(t *testing.T) {
	line := `{"type":"rate_limit_event","rate_limit_info":{"status":"allowed"}}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rl, ok := msg.(*RateLimitEvent)
	if !ok {
		t.Fatalf("expected *RateLimitEvent, got %T", msg)
	}

	if rl.RateLimitInfo == nil {
		t.Fatal("rate_limit_info is nil")
	}
	if rl.RateLimitInfo.Status != "allowed" {
		t.Errorf("status = %q, want %q", rl.RateLimitInfo.Status, "allowed")
	}
	if rl.RateLimitInfo.ResetsAt != nil {
		t.Errorf("resetsAt should be nil, got %d", *rl.RateLimitInfo.ResetsAt)
	}
	if rl.RateLimitInfo.RateLimitType != "" {
		t.Errorf("rateLimitType should be empty, got %q", rl.RateLimitInfo.RateLimitType)
	}
	if rl.RateLimitInfo.Utilization != 0 {
		t.Errorf("utilization should be 0, got %f", rl.RateLimitInfo.Utilization)
	}
}

func TestParseMessage_ToolProgressWithTaskID(t *testing.T) {
	line := `{"type":"tool_progress","tool_use_id":"tu20","tool_name":"Bash","elapsed_time_seconds":10.5,"task_id":"task-abc","session_id":"s1","parent_tool_use_id":"tu19"}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tp, ok := msg.(*ToolProgressMessage)
	if !ok {
		t.Fatalf("expected *ToolProgressMessage, got %T", msg)
	}

	if tp.ToolUseID != "tu20" {
		t.Errorf("tool_use_id = %q, want %q", tp.ToolUseID, "tu20")
	}
	if tp.ToolName != "Bash" {
		t.Errorf("tool_name = %q, want %q", tp.ToolName, "Bash")
	}
	if tp.ElapsedTimeSeconds != 10.5 {
		t.Errorf("elapsed_time_seconds = %f, want 10.5", tp.ElapsedTimeSeconds)
	}
	if tp.TaskID != "task-abc" {
		t.Errorf("task_id = %q, want %q", tp.TaskID, "task-abc")
	}
	if tp.SessionID != "s1" {
		t.Errorf("session_id = %q, want %q", tp.SessionID, "s1")
	}
	if tp.ParentToolUseID == nil {
		t.Fatal("parent_tool_use_id is nil")
	}
	if *tp.ParentToolUseID != "tu19" {
		t.Errorf("parent_tool_use_id = %q, want %q", *tp.ParentToolUseID, "tu19")
	}
}

func TestParseMessage_ToolUseSummaryFull(t *testing.T) {
	line := `{"type":"tool_use_summary","session_id":"s1","summary":"Edited 2 files and ran tests","preceding_tool_use_ids":["tu1","tu2"]}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tus, ok := msg.(*ToolUseSummaryMessage)
	if !ok {
		t.Fatalf("expected *ToolUseSummaryMessage, got %T", msg)
	}

	if tus.Type != MessageTypeToolUseSummary {
		t.Errorf("type = %q, want %q", tus.Type, MessageTypeToolUseSummary)
	}
	if tus.SessionID != "s1" {
		t.Errorf("session_id = %q, want %q", tus.SessionID, "s1")
	}
	if tus.Summary != "Edited 2 files and ran tests" {
		t.Errorf("summary = %q, want %q", tus.Summary, "Edited 2 files and ran tests")
	}
	if len(tus.PrecedingToolUseIDs) != 2 {
		t.Fatalf("preceding_tool_use_ids length = %d, want 2", len(tus.PrecedingToolUseIDs))
	}
	if tus.PrecedingToolUseIDs[0] != "tu1" {
		t.Errorf("preceding_tool_use_ids[0] = %q, want %q", tus.PrecedingToolUseIDs[0], "tu1")
	}
	if tus.PrecedingToolUseIDs[1] != "tu2" {
		t.Errorf("preceding_tool_use_ids[1] = %q, want %q", tus.PrecedingToolUseIDs[1], "tu2")
	}
}

func TestParseMessage_AuthStatusFull(t *testing.T) {
	line := `{"type":"auth_status","session_id":"s1","isAuthenticating":true,"output":["Waiting for browser...","Click approve"],"error":""}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	as, ok := msg.(*AuthStatusMessage)
	if !ok {
		t.Fatalf("expected *AuthStatusMessage, got %T", msg)
	}

	if as.Type != MessageTypeAuthStatus {
		t.Errorf("type = %q, want %q", as.Type, MessageTypeAuthStatus)
	}
	if as.SessionID != "s1" {
		t.Errorf("session_id = %q, want %q", as.SessionID, "s1")
	}
	if !as.IsAuthenticating {
		t.Error("isAuthenticating should be true")
	}
	if len(as.Output) != 2 {
		t.Fatalf("output length = %d, want 2", len(as.Output))
	}
	if as.Output[0] != "Waiting for browser..." {
		t.Errorf("output[0] = %q, want %q", as.Output[0], "Waiting for browser...")
	}
	if as.Output[1] != "Click approve" {
		t.Errorf("output[1] = %q, want %q", as.Output[1], "Click approve")
	}
	if as.Error != "" {
		t.Errorf("error = %q, want empty", as.Error)
	}
}
