package claude

import (
	"context"
	"encoding/json"
)

// HookEvent identifies a hook event type.
type HookEvent string

const (
	HookPreToolUse          HookEvent = "PreToolUse"
	HookPostToolUse         HookEvent = "PostToolUse"
	HookPostToolUseFailure  HookEvent = "PostToolUseFailure"
	HookNotification        HookEvent = "Notification"
	HookUserPromptSubmit    HookEvent = "UserPromptSubmit"
	HookSessionStart        HookEvent = "SessionStart"
	HookSessionEnd          HookEvent = "SessionEnd"
	HookStop                HookEvent = "Stop"
	HookStopFailure         HookEvent = "StopFailure"
	HookSubagentStart       HookEvent = "SubagentStart"
	HookSubagentStop        HookEvent = "SubagentStop"
	HookPreCompact          HookEvent = "PreCompact"
	HookPostCompact         HookEvent = "PostCompact"
	HookPermissionRequest   HookEvent = "PermissionRequest"
	HookSetup               HookEvent = "Setup"
	HookTeammateIdle        HookEvent = "TeammateIdle"
	HookTaskCompleted       HookEvent = "TaskCompleted"
	HookElicitation         HookEvent = "Elicitation"
	HookElicitationResult   HookEvent = "ElicitationResult"
	HookConfigChange        HookEvent = "ConfigChange"
	HookWorktreeCreate      HookEvent = "WorktreeCreate"
	HookWorktreeRemove      HookEvent = "WorktreeRemove"
	HookInstructionsLoaded  HookEvent = "InstructionsLoaded"
)

// HookCallback is the function signature for hook handlers.
// The input is the raw hook input from the CLI, the toolUseID is present
// for tool-related hooks.
type HookCallback func(ctx context.Context, input HookInput) (HookOutput, error)

// HookCallbackMatcher groups callbacks with an optional tool name matcher.
type HookCallbackMatcher struct {
	// Matcher is a regex pattern for tool name filtering. Nil means match all.
	Matcher *string `json:"matcher,omitempty"`

	// Hooks are the callback functions to invoke.
	Hooks []HookCallback `json:"-"`

	// Timeout in seconds for all hooks in this matcher.
	Timeout *int `json:"timeout,omitempty"`
}

// HookInput is the base input passed to all hook callbacks.
type HookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	PermissionMode string `json:"permission_mode,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	AgentType      string `json:"agent_type,omitempty"`
	HookEventName  string `json:"hook_event_name"`

	// Tool-related fields (PreToolUse, PostToolUse, PostToolUseFailure, PermissionRequest).
	ToolName   string          `json:"tool_name,omitempty"`
	ToolInput  json.RawMessage `json:"tool_input,omitempty"`
	ToolUseID  string          `json:"tool_use_id,omitempty"`

	// PostToolUse: the tool's response.
	ToolResponse json.RawMessage `json:"tool_response,omitempty"`

	// PostToolUseFailure: error info.
	Error       string `json:"error,omitempty"`
	IsInterrupt bool   `json:"is_interrupt,omitempty"`

	// Notification fields.
	Message          string `json:"message,omitempty"`
	Title            string `json:"title,omitempty"`
	NotificationType string `json:"notification_type,omitempty"`

	// PreCompact/PostCompact fields.
	Trigger            string  `json:"trigger,omitempty"`
	CustomInstructions *string `json:"custom_instructions,omitempty"`
	CompactSummary     string  `json:"compact_summary,omitempty"`

	// Stop fields.
	StopHookActive       bool   `json:"stop_hook_active,omitempty"`
	LastAssistantMessage string `json:"last_assistant_message,omitempty"`

	// SubagentStop fields.
	AgentTranscriptPath string `json:"agent_transcript_path,omitempty"`

	// UserPromptSubmit fields.
	Prompt string `json:"prompt,omitempty"`

	// PermissionRequest fields.
	PermissionSuggestions json.RawMessage `json:"permission_suggestions,omitempty"`

	// ConfigChange fields.
	Source   string `json:"source,omitempty"`
	FilePath string `json:"file_path,omitempty"`

	// InstructionsLoaded fields.
	MemoryType      string   `json:"memory_type,omitempty"`
	LoadReason      string   `json:"load_reason,omitempty"`
	Globs           []string `json:"globs,omitempty"`
	TriggerFilePath string   `json:"trigger_file_path,omitempty"`
	ParentFilePath  string   `json:"parent_file_path,omitempty"`

	// Elicitation fields.
	MCPServerName   string          `json:"mcp_server_name,omitempty"`
	Mode            string          `json:"mode,omitempty"`
	ElicitationID   string          `json:"elicitation_id,omitempty"`
	RequestedSchema json.RawMessage `json:"requested_schema,omitempty"`
	Action          string          `json:"action,omitempty"`
	ElicitContent   json.RawMessage `json:"content,omitempty"`
}

// HookOutput is returned by hook callbacks to control behavior.
type HookOutput struct {
	// Continue allows the operation to proceed (default true).
	Continue *bool `json:"continue,omitempty"`

	// Decision for PreToolUse / PermissionRequest hooks.
	Decision       string `json:"permissionDecision,omitempty"` // allow, deny, ask
	DecisionReason string `json:"permissionDecisionReason,omitempty"`

	// UpdatedInput replaces the tool input (PreToolUse).
	UpdatedInput map[string]any `json:"updatedInput,omitempty"`

	// AdditionalContext is injected into the conversation.
	AdditionalContext string `json:"additionalContext,omitempty"`

	// SystemMessage injects a system message.
	SystemMessage string `json:"systemMessage,omitempty"`

	// SuppressOutput hides the tool output from the model.
	SuppressOutput bool `json:"suppressOutput,omitempty"`

	// Stop-related fields.
	BlockStop  bool   `json:"blockStop,omitempty"`
	StopReason string `json:"stopReason,omitempty"`
	Reason     string `json:"reason,omitempty"`

	// UpdatedMCPToolOutput replaces the MCP tool output (PostToolUse).
	UpdatedMCPToolOutput any `json:"updatedMCPToolOutput,omitempty"`

	// Elicitation fields.
	ElicitAction  string         `json:"action,omitempty"`
	ElicitContent map[string]any `json:"content,omitempty"`

	// PermissionRequest decision.
	PermissionDecision *PermissionDecision `json:"-"`
}

// PermissionDecision is the structured decision for PermissionRequest hooks.
type PermissionDecision struct {
	Behavior           string         `json:"behavior"` // allow, deny
	UpdatedInput       map[string]any `json:"updatedInput,omitempty"`
	UpdatedPermissions json.RawMessage `json:"updatedPermissions,omitempty"`
	Message            string         `json:"message,omitempty"`
	Interrupt          bool           `json:"interrupt,omitempty"`
}
