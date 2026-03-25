package claude

import (
	"encoding/json"
	"testing"
)

func TestAgentDefinition_MinimalJSON(t *testing.T) {
	agent := AgentDefinition{
		Description: "A helpful agent",
		Prompt:      "You are helpful.",
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["description"] != "A helpful agent" {
		t.Errorf("description = %v, want %q", parsed["description"], "A helpful agent")
	}
	if parsed["prompt"] != "You are helpful." {
		t.Errorf("prompt = %v, want %q", parsed["prompt"], "You are helpful.")
	}

	// Optional fields should be omitted.
	for _, key := range []string{"tools", "disallowedTools", "model", "mcpServers", "skills", "maxTurns", "criticalSystemReminder_EXPERIMENTAL"} {
		if _, ok := parsed[key]; ok {
			t.Errorf("field %q should be omitted in minimal JSON", key)
		}
	}
}

func TestAgentDefinition_AllFieldsJSON(t *testing.T) {
	maxTurns := 5
	agent := AgentDefinition{
		Description:            "Full agent",
		Prompt:                 "You do everything.",
		Tools:                  []string{"Bash", "Read"},
		DisallowedTools:        []string{"Write"},
		Model:                  "opus",
		MCPServers:             []any{"server1", map[string]any{"name": "server2"}},
		Skills:                 []string{"coding", "review"},
		MaxTurns:               &maxTurns,
		CriticalSystemReminder: "Always verify",
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["description"] != "Full agent" {
		t.Errorf("description = %v, want %q", parsed["description"], "Full agent")
	}
	if parsed["prompt"] != "You do everything." {
		t.Errorf("prompt = %v, want %q", parsed["prompt"], "You do everything.")
	}
	if parsed["model"] != "opus" {
		t.Errorf("model = %v, want %q", parsed["model"], "opus")
	}

	tools, ok := parsed["tools"].([]any)
	if !ok {
		t.Fatal("tools missing or wrong type")
	}
	if len(tools) != 2 || tools[0] != "Bash" || tools[1] != "Read" {
		t.Errorf("tools = %v, want [Bash Read]", tools)
	}

	disallowed, ok := parsed["disallowedTools"].([]any)
	if !ok {
		t.Fatal("disallowedTools missing or wrong type")
	}
	if len(disallowed) != 1 || disallowed[0] != "Write" {
		t.Errorf("disallowedTools = %v, want [Write]", disallowed)
	}

	mcpServers, ok := parsed["mcpServers"].([]any)
	if !ok {
		t.Fatal("mcpServers missing or wrong type")
	}
	if len(mcpServers) != 2 {
		t.Errorf("mcpServers length = %d, want 2", len(mcpServers))
	}

	skills, ok := parsed["skills"].([]any)
	if !ok {
		t.Fatal("skills missing or wrong type")
	}
	if len(skills) != 2 || skills[0] != "coding" {
		t.Errorf("skills = %v, want [coding review]", skills)
	}

	// JSON numbers are float64.
	if parsed["maxTurns"] != float64(5) {
		t.Errorf("maxTurns = %v, want 5", parsed["maxTurns"])
	}

	if parsed["criticalSystemReminder_EXPERIMENTAL"] != "Always verify" {
		t.Errorf("criticalSystemReminder_EXPERIMENTAL = %v, want %q", parsed["criticalSystemReminder_EXPERIMENTAL"], "Always verify")
	}
}

func TestAgentDefinition_OmitsNilFields(t *testing.T) {
	agent := AgentDefinition{
		Description: "Minimal",
		Prompt:      "Prompt only",
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	// Only description and prompt should be present.
	if len(parsed) != 2 {
		t.Errorf("expected 2 fields in JSON, got %d: %v", len(parsed), parsed)
	}
	if _, ok := parsed["description"]; !ok {
		t.Error("description should be present")
	}
	if _, ok := parsed["prompt"]; !ok {
		t.Error("prompt should be present")
	}
}

func TestAgentDefinition_RoundTrip(t *testing.T) {
	maxTurns := 10
	original := AgentDefinition{
		Description:            "Round trip test",
		Prompt:                 "System prompt",
		Tools:                  []string{"Bash"},
		Model:                  "sonnet",
		MaxTurns:               &maxTurns,
		CriticalSystemReminder: "Important",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded AgentDefinition
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Description != original.Description {
		t.Errorf("Description = %q, want %q", decoded.Description, original.Description)
	}
	if decoded.Prompt != original.Prompt {
		t.Errorf("Prompt = %q, want %q", decoded.Prompt, original.Prompt)
	}
	if decoded.Model != original.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, original.Model)
	}
	if len(decoded.Tools) != 1 || decoded.Tools[0] != "Bash" {
		t.Errorf("Tools = %v, want [Bash]", decoded.Tools)
	}
	if decoded.MaxTurns == nil || *decoded.MaxTurns != 10 {
		t.Errorf("MaxTurns = %v, want 10", decoded.MaxTurns)
	}
	if decoded.CriticalSystemReminder != "Important" {
		t.Errorf("CriticalSystemReminder = %q, want %q", decoded.CriticalSystemReminder, "Important")
	}
}

func TestAgentInfo_Fields(t *testing.T) {
	info := AgentInfo{
		Name:        "test-agent",
		Description: "A test agent",
		Model:       "sonnet",
	}

	if info.Name != "test-agent" {
		t.Errorf("Name = %q, want %q", info.Name, "test-agent")
	}
	if info.Description != "A test agent" {
		t.Errorf("Description = %q, want %q", info.Description, "A test agent")
	}
	if info.Model != "sonnet" {
		t.Errorf("Model = %q, want %q", info.Model, "sonnet")
	}
}
