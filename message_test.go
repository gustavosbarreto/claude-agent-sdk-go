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

	sys, ok := msg.(*SystemMessage)
	if !ok {
		t.Fatalf("expected *SystemMessage, got %T", msg)
	}

	if sys.Subtype != "task_notification" {
		t.Errorf("subtype = %q", sys.Subtype)
	}
	if sys.TaskID != "t1" {
		t.Errorf("task_id = %q", sys.TaskID)
	}
	if sys.Summary != "Done" {
		t.Errorf("summary = %q", sys.Summary)
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
