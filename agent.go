package claude

// AgentDefinition defines a custom subagent invocable via the Agent tool.
type AgentDefinition struct {
	// Description is when to use this agent (required).
	Description string `json:"description"`

	// Prompt is the agent's system prompt (required).
	Prompt string `json:"prompt"`

	// Tools is the list of allowed tool names. If nil, inherits from parent.
	Tools []string `json:"tools,omitempty"`

	// DisallowedTools is the list of explicitly disallowed tool names.
	DisallowedTools []string `json:"disallowedTools,omitempty"`

	// Model alias (e.g. "sonnet", "opus") or full model ID. If empty, inherits.
	Model string `json:"model,omitempty"`

	// MCPServers for this agent. Each entry is a server name string or a
	// map[string]MCPServerConfig.
	MCPServers []any `json:"mcpServers,omitempty"`

	// Skills to preload into the agent context.
	Skills []string `json:"skills,omitempty"`

	// MaxTurns limits the number of agentic turns.
	MaxTurns *int `json:"maxTurns,omitempty"`

	// CriticalSystemReminder is an experimental reminder added to the system prompt.
	CriticalSystemReminder string `json:"criticalSystemReminder_EXPERIMENTAL,omitempty"`
}

// AgentInfo describes an available subagent reported by the CLI.
type AgentInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Model       string `json:"model,omitempty"`
}
