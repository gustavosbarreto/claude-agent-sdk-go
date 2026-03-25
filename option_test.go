package claude

import (
	"context"
	"testing"
)

func TestWithModel(t *testing.T) {
	cfg := applyOptions([]Option{WithModel("claude-sonnet-4-20250514")})
	if cfg.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", cfg.Model, "claude-sonnet-4-20250514")
	}
}

func TestWithSystemPrompt(t *testing.T) {
	cfg := applyOptions([]Option{WithSystemPrompt("You are a helpful assistant.")})
	if cfg.SystemPrompt == nil {
		t.Fatal("SystemPrompt is nil")
	}
	if cfg.SystemPrompt.Text != "You are a helpful assistant." {
		t.Errorf("SystemPrompt.Text = %q, want %q", cfg.SystemPrompt.Text, "You are a helpful assistant.")
	}
	if cfg.SystemPrompt.Preset {
		t.Error("SystemPrompt.Preset = true, want false")
	}
}

func TestWithAllowedTools(t *testing.T) {
	cfg := applyOptions([]Option{WithAllowedTools("Read", "Edit", "Bash")})
	if len(cfg.AllowedTools) != 3 {
		t.Fatalf("len(AllowedTools) = %d, want 3", len(cfg.AllowedTools))
	}
	want := []string{"Read", "Edit", "Bash"}
	for i, v := range want {
		if cfg.AllowedTools[i] != v {
			t.Errorf("AllowedTools[%d] = %q, want %q", i, cfg.AllowedTools[i], v)
		}
	}
}

func TestWithDisallowedTools(t *testing.T) {
	cfg := applyOptions([]Option{WithDisallowedTools("Bash", "Write")})
	if len(cfg.DisallowedTools) != 2 {
		t.Fatalf("len(DisallowedTools) = %d, want 2", len(cfg.DisallowedTools))
	}
	if cfg.DisallowedTools[0] != "Bash" {
		t.Errorf("DisallowedTools[0] = %q, want %q", cfg.DisallowedTools[0], "Bash")
	}
	if cfg.DisallowedTools[1] != "Write" {
		t.Errorf("DisallowedTools[1] = %q, want %q", cfg.DisallowedTools[1], "Write")
	}
}

func TestWithPermissionMode(t *testing.T) {
	tests := []struct {
		mode PermissionMode
	}{
		{PermissionDefault},
		{PermissionAcceptEdits},
		{PermissionBypassPermissions},
		{PermissionPlan},
		{PermissionDontAsk},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			cfg := applyOptions([]Option{WithPermissionMode(tt.mode)})
			if cfg.PermissionMode != tt.mode {
				t.Errorf("PermissionMode = %q, want %q", cfg.PermissionMode, tt.mode)
			}
		})
	}
}

func TestWithMaxTurns(t *testing.T) {
	cfg := applyOptions([]Option{WithMaxTurns(10)})
	if cfg.MaxTurns == nil {
		t.Fatal("MaxTurns is nil")
	}
	if *cfg.MaxTurns != 10 {
		t.Errorf("MaxTurns = %d, want 10", *cfg.MaxTurns)
	}
}

func TestWithMaxBudgetUSD(t *testing.T) {
	cfg := applyOptions([]Option{WithMaxBudgetUSD(5.50)})
	if cfg.MaxBudgetUSD == nil {
		t.Fatal("MaxBudgetUSD is nil")
	}
	if *cfg.MaxBudgetUSD != 5.50 {
		t.Errorf("MaxBudgetUSD = %f, want 5.50", *cfg.MaxBudgetUSD)
	}
}

func TestWithCwd(t *testing.T) {
	cfg := applyOptions([]Option{WithCwd("/tmp/test-dir")})
	if cfg.Cwd != "/tmp/test-dir" {
		t.Errorf("Cwd = %q, want %q", cfg.Cwd, "/tmp/test-dir")
	}
}

func TestWithEnv(t *testing.T) {
	env := map[string]string{
		"FOO": "bar",
		"BAZ": "qux",
	}
	cfg := applyOptions([]Option{WithEnv(env)})
	if cfg.Env == nil {
		t.Fatal("Env is nil")
	}
	if len(cfg.Env) != 2 {
		t.Fatalf("len(Env) = %d, want 2", len(cfg.Env))
	}
	if cfg.Env["FOO"] != "bar" {
		t.Errorf("Env[FOO] = %q, want %q", cfg.Env["FOO"], "bar")
	}
	if cfg.Env["BAZ"] != "qux" {
		t.Errorf("Env[BAZ] = %q, want %q", cfg.Env["BAZ"], "qux")
	}
}

func TestWithNoPersistSession(t *testing.T) {
	cfg := applyOptions([]Option{WithNoPersistSession()})
	if !cfg.NoPersistSession {
		t.Error("NoPersistSession = false, want true")
	}
}

func TestWithIncludePartialMessages(t *testing.T) {
	cfg := applyOptions([]Option{WithIncludePartialMessages()})
	if !cfg.IncludePartialMessages {
		t.Error("IncludePartialMessages = false, want true")
	}
}

func TestWithOutputFormat(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}
	fmt := OutputFormat{
		Type:   "json_schema",
		Schema: schema,
	}
	cfg := applyOptions([]Option{WithOutputFormat(fmt)})
	if cfg.OutputFormat == nil {
		t.Fatal("OutputFormat is nil")
	}
	if cfg.OutputFormat.Type != "json_schema" {
		t.Errorf("OutputFormat.Type = %q, want %q", cfg.OutputFormat.Type, "json_schema")
	}
	if cfg.OutputFormat.Schema == nil {
		t.Fatal("OutputFormat.Schema is nil")
	}
	if cfg.OutputFormat.Schema["type"] != "object" {
		t.Errorf("OutputFormat.Schema[type] = %v, want %q", cfg.OutputFormat.Schema["type"], "object")
	}
}

func TestWithHook(t *testing.T) {
	called := false
	cb := func(ctx context.Context, input HookInput) (HookOutput, error) {
		called = true
		return HookOutput{}, nil
	}

	matcher := HookCallbackMatcher{
		Hooks: []HookCallback{cb},
	}
	cfg := applyOptions([]Option{WithHook(HookPreToolUse, matcher)})

	if cfg.Hooks == nil {
		t.Fatal("Hooks is nil")
	}
	hooks, ok := cfg.Hooks[HookPreToolUse]
	if !ok {
		t.Fatal("Hooks[PreToolUse] not found")
	}
	if len(hooks) != 1 {
		t.Fatalf("len(Hooks[PreToolUse]) = %d, want 1", len(hooks))
	}
	if len(hooks[0].Hooks) != 1 {
		t.Fatalf("len(Hooks[PreToolUse][0].Hooks) = %d, want 1", len(hooks[0].Hooks))
	}

	// Invoke the callback to verify it was registered correctly.
	_, _ = hooks[0].Hooks[0](context.Background(), HookInput{})
	if !called {
		t.Error("hook callback was not invoked")
	}
}

func TestWithCanUseTool(t *testing.T) {
	called := false
	fn := func(toolName string, input map[string]any, opts CanUseToolOptions) (PermissionResult, error) {
		called = true
		return PermissionResult{Behavior: "allow"}, nil
	}

	cfg := applyOptions([]Option{WithCanUseTool(fn)})
	if cfg.CanUseTool == nil {
		t.Fatal("CanUseTool is nil")
	}

	result, err := cfg.CanUseTool("Read", nil, CanUseToolOptions{})
	if err != nil {
		t.Fatalf("CanUseTool returned error: %v", err)
	}
	if !called {
		t.Error("CanUseTool callback was not invoked")
	}
	if result.Behavior != "allow" {
		t.Errorf("Behavior = %q, want %q", result.Behavior, "allow")
	}
}

func TestWithSettingSources(t *testing.T) {
	cfg := applyOptions([]Option{WithSettingSources(SettingSourceUser, SettingSourceProject, SettingSourceLocal)})
	if len(cfg.SettingSources) != 3 {
		t.Fatalf("len(SettingSources) = %d, want 3", len(cfg.SettingSources))
	}
	want := []SettingSource{SettingSourceUser, SettingSourceProject, SettingSourceLocal}
	for i, v := range want {
		if cfg.SettingSources[i] != v {
			t.Errorf("SettingSources[%d] = %q, want %q", i, cfg.SettingSources[i], v)
		}
	}
}

func TestWithMultipleOptionsCombined(t *testing.T) {
	cfg := applyOptions([]Option{
		WithModel("claude-sonnet-4-20250514"),
		WithSystemPrompt("Be concise."),
		WithCwd("/home/user/project"),
		WithMaxTurns(5),
		WithMaxBudgetUSD(1.00),
		WithPermissionMode(PermissionAcceptEdits),
		WithAllowedTools("Read", "Edit"),
		WithNoPersistSession(),
		WithIncludePartialMessages(),
		WithEnv(map[string]string{"KEY": "value"}),
	})

	if cfg.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", cfg.Model, "claude-sonnet-4-20250514")
	}
	if cfg.SystemPrompt == nil || cfg.SystemPrompt.Text != "Be concise." {
		t.Error("SystemPrompt not set correctly")
	}
	if cfg.Cwd != "/home/user/project" {
		t.Errorf("Cwd = %q, want %q", cfg.Cwd, "/home/user/project")
	}
	if cfg.MaxTurns == nil || *cfg.MaxTurns != 5 {
		t.Error("MaxTurns not set correctly")
	}
	if cfg.MaxBudgetUSD == nil || *cfg.MaxBudgetUSD != 1.00 {
		t.Error("MaxBudgetUSD not set correctly")
	}
	if cfg.PermissionMode != PermissionAcceptEdits {
		t.Errorf("PermissionMode = %q, want %q", cfg.PermissionMode, PermissionAcceptEdits)
	}
	if len(cfg.AllowedTools) != 2 {
		t.Errorf("len(AllowedTools) = %d, want 2", len(cfg.AllowedTools))
	}
	if !cfg.NoPersistSession {
		t.Error("NoPersistSession = false, want true")
	}
	if !cfg.IncludePartialMessages {
		t.Error("IncludePartialMessages = false, want true")
	}
	if cfg.Env == nil || cfg.Env["KEY"] != "value" {
		t.Error("Env not set correctly")
	}
}
