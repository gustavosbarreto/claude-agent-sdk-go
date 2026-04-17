package claude

import "encoding/json"

// MessageType identifies the kind of NDJSON message from the CLI.
type MessageType string

const (
	MessageTypeSystem          MessageType = "system"
	MessageTypeAssistant       MessageType = "assistant"
	MessageTypeUser            MessageType = "user"
	MessageTypeResult          MessageType = "result"
	MessageTypeStream          MessageType = "stream_event"
	MessageTypeToolProgress    MessageType = "tool_progress"
	MessageTypeToolUseSummary  MessageType = "tool_use_summary"
	MessageTypeAuthStatus      MessageType = "auth_status"
	MessageTypeRateLimit       MessageType = "rate_limit_event"
	MessageTypePromptSuggest   MessageType = "prompt_suggestion"
)

// ResultSubtype identifies how a turn ended.
type ResultSubtype string

const (
	ResultSuccess        ResultSubtype = "success"
	ResultErrorMaxTurns  ResultSubtype = "error_max_turns"
	ResultErrorExecution ResultSubtype = "error_during_execution"
	ResultErrorBudget    ResultSubtype = "error_max_budget_usd"
)

// Message is the interface implemented by all SDK message types.
type Message interface {
	messageType() MessageType
}

// SystemMessage is emitted once at session start (subtype "init") and for
// various system events (compact_boundary, status, hook_*, task_*, files_persisted).
type SystemMessage struct {
	Type      MessageType `json:"type"`
	Subtype   string      `json:"subtype"`
	UUID      string      `json:"uuid,omitempty"`
	SessionID string      `json:"session_id"`
	Model     string      `json:"model,omitempty"`
	Cwd       string      `json:"cwd,omitempty"`
	Tools     []string    `json:"tools,omitempty"`
	MCPServers []MCPServerStatus `json:"mcp_servers,omitempty"`
	Betas     []string    `json:"betas,omitempty"`
	ClaudeCodeVersion string `json:"claude_code_version,omitempty"`
	PermMode  string      `json:"permissionMode,omitempty"`
	SlashCommands []string `json:"slash_commands,omitempty"`
	OutputStyle string    `json:"output_style,omitempty"`
	Skills    []string    `json:"skills,omitempty"`
	Agents    []string    `json:"agents,omitempty"`

	// Status subtype fields.
	Status string `json:"status,omitempty"` // "compacting", null

	// CompactBoundary subtype fields.
	CompactMetadata *CompactMetadata `json:"compact_metadata,omitempty"`

	// Hook subtype fields (hook_started, hook_progress, hook_response).
	HookID    string `json:"hook_id,omitempty"`
	HookName  string `json:"hook_name,omitempty"`
	HookEvent string `json:"hook_event,omitempty"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
	Output    string `json:"output,omitempty"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	Outcome   string `json:"outcome,omitempty"` // success, error, canceled

	// Task subtype fields (task_started, task_progress, task_notification).
	TaskID          string     `json:"task_id,omitempty"`
	ToolUseID       string     `json:"tool_use_id,omitempty"`
	Description     string     `json:"description,omitempty"`
	TaskType        string     `json:"task_type,omitempty"` // local_bash, local_agent, remote_agent
	TaskStatus      string     `json:"task_status,omitempty"`
	OutputFile      string     `json:"output_file,omitempty"`
	Summary         string     `json:"summary,omitempty"`
	LastToolName    string     `json:"last_tool_name,omitempty"`
	TaskUsage       *TaskUsage `json:"usage,omitempty"`

	// FilesPersisted subtype fields.
	Files     []PersistedFile      `json:"files,omitempty"`
	Failed    []PersistedFileFail  `json:"failed,omitempty"`
	ProcessedAt string             `json:"processed_at,omitempty"`
}

func (m *SystemMessage) messageType() MessageType { return MessageTypeSystem }

// TaskStartedMessage is emitted when a background task starts.
// Parsed from system messages with subtype "task_started".
type TaskStartedMessage struct {
	SystemMessage
}

func (m *TaskStartedMessage) messageType() MessageType { return MessageTypeSystem }

// TaskProgressMessage is emitted while a background task is in progress.
// Parsed from system messages with subtype "task_progress".
type TaskProgressMessage struct {
	SystemMessage
}

func (m *TaskProgressMessage) messageType() MessageType { return MessageTypeSystem }

// TaskNotificationMessage is emitted when a background task completes, fails, or is stopped.
// Parsed from system messages with subtype "task_notification".
type TaskNotificationMessage struct {
	SystemMessage
}

func (m *TaskNotificationMessage) messageType() MessageType { return MessageTypeSystem }

// CompactMetadata is attached to compact_boundary system messages.
type CompactMetadata struct {
	Trigger   string `json:"trigger"` // manual, auto
	PreTokens int    `json:"pre_tokens"`
}

// TaskUsage reports token usage for a task.
type TaskUsage struct {
	TotalTokens int   `json:"total_tokens"`
	ToolUses    int   `json:"tool_uses"`
	DurationMS  int64 `json:"duration_ms"`
}

// PersistedFile is a file that was persisted.
type PersistedFile struct {
	Filename string `json:"filename"`
	FileID   string `json:"file_id"`
}

// PersistedFileFail is a file that failed to persist.
type PersistedFileFail struct {
	Filename string `json:"filename"`
	Error    string `json:"error"`
}

// MCPServerStatus reports the connection state of an MCP server.
type MCPServerStatus struct {
	Name       string `json:"name"`
	Status     string `json:"status"` // connected, failed, needs-auth, pending, disabled
	Error      string `json:"error,omitempty"`
	ServerInfo *struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"server_info,omitempty"`
}

// AssistantMessage is Claude's response containing content blocks.
type AssistantMessage struct {
	Type      MessageType `json:"type"`
	UUID      string      `json:"uuid,omitempty"`
	SessionID string      `json:"session_id,omitempty"`
	Message   struct {
		Role       string          `json:"role,omitempty"`
		Content    []ContentBlock  `json:"content"`
		Model      string          `json:"model,omitempty"`
		ID         string          `json:"id,omitempty"`
		StopReason string          `json:"stop_reason,omitempty"`
		Usage      json.RawMessage `json:"usage,omitempty"`
	} `json:"message"`
	ParentToolUseID *string `json:"parent_tool_use_id,omitempty"`
	// Error is the error type string (e.g. "authentication_failed", "rate_limit", "unknown").
	Error string `json:"error,omitempty"`
}

func (m *AssistantMessage) messageType() MessageType { return MessageTypeAssistant }

// UserMessage is a user message echoed back by the CLI.
type UserMessage struct {
	Type    MessageType `json:"type"`
	UUID    string      `json:"uuid,omitempty"`
	Message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	ParentToolUseID *string         `json:"parent_tool_use_id,omitempty"`
	ToolUseResult   json.RawMessage `json:"tool_use_result,omitempty"`
	IsSynthetic     bool            `json:"isSynthetic,omitempty"`
	IsReplay        bool            `json:"isReplay,omitempty"`
}

func (m *UserMessage) messageType() MessageType { return MessageTypeUser }

// ResultMessage marks the end of a turn.
type ResultMessage struct {
	Type                     MessageType      `json:"type"`
	Subtype                  ResultSubtype    `json:"subtype"`
	UUID                     string           `json:"uuid,omitempty"`
	SessionID                string           `json:"session_id,omitempty"`
	Result                   string           `json:"result,omitempty"`
	IsError                  bool             `json:"is_error"`
	StopReason               *string          `json:"stop_reason"`
	TotalCostUSD             float64          `json:"total_cost_usd,omitempty"`
	DurationMS               float64          `json:"duration_ms,omitempty"`
	DurationAPIMS            float64          `json:"duration_api_ms,omitempty"`
	NumTurns                 int              `json:"num_turns,omitempty"`
	Usage                    *Usage           `json:"usage,omitempty"`
	ModelUsage               map[string]ModelUsage `json:"modelUsage,omitempty"`
	PermissionDenials        []PermissionDenial `json:"permission_denials,omitempty"`
	StructuredOutput         json.RawMessage  `json:"structured_output,omitempty"`
	Errors                   []string         `json:"errors,omitempty"`

	// Legacy fields (flat token counts for backward compat).
	CostUSD                  float64 `json:"cost_usd,omitempty"`
	InputTokens              int64   `json:"input_tokens,omitempty"`
	OutputTokens             int64   `json:"output_tokens,omitempty"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens,omitempty"`
}

func (m *ResultMessage) messageType() MessageType { return MessageTypeResult }

// Usage holds token usage stats.
type Usage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

// ModelUsage holds per-model usage stats.
type ModelUsage struct {
	InputTokens              int64   `json:"inputTokens"`
	OutputTokens             int64   `json:"outputTokens"`
	CacheReadInputTokens     int64   `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int64   `json:"cacheCreationInputTokens"`
	WebSearchRequests        int     `json:"webSearchRequests"`
	CostUSD                  float64 `json:"costUSD"`
	ContextWindow            int     `json:"contextWindow"`
	MaxOutputTokens          int     `json:"maxOutputTokens"`
}

// PermissionDenial records a denied tool call.
type PermissionDenial struct {
	ToolName  string         `json:"tool_name"`
	ToolUseID string         `json:"tool_use_id"`
	ToolInput map[string]any `json:"tool_input"`
}

// StreamEvent carries token-level streaming deltas (partial messages).
type StreamEvent struct {
	Type            MessageType     `json:"type"`
	UUID            string          `json:"uuid,omitempty"`
	SessionID       string          `json:"session_id,omitempty"`
	ParentToolUseID *string         `json:"parent_tool_use_id,omitempty"`
	Event           json.RawMessage `json:"event,omitempty"`
}

func (m *StreamEvent) messageType() MessageType { return MessageTypeStream }

// ToolProgressMessage reports progress of a running tool.
type ToolProgressMessage struct {
	Type                MessageType `json:"type"`
	UUID                string      `json:"uuid,omitempty"`
	SessionID           string      `json:"session_id,omitempty"`
	ToolUseID           string      `json:"tool_use_id"`
	ToolName            string      `json:"tool_name"`
	ParentToolUseID     *string     `json:"parent_tool_use_id,omitempty"`
	ElapsedTimeSeconds  float64     `json:"elapsed_time_seconds"`
	TaskID              string      `json:"task_id,omitempty"`
}

func (m *ToolProgressMessage) messageType() MessageType { return MessageTypeToolProgress }

// ToolUseSummaryMessage summarizes tool usage.
type ToolUseSummaryMessage struct {
	Type                  MessageType `json:"type"`
	UUID                  string      `json:"uuid,omitempty"`
	SessionID             string      `json:"session_id,omitempty"`
	Summary               string      `json:"summary"`
	PrecedingToolUseIDs   []string    `json:"preceding_tool_use_ids"`
}

func (m *ToolUseSummaryMessage) messageType() MessageType { return MessageTypeToolUseSummary }

// AuthStatusMessage reports authentication state.
type AuthStatusMessage struct {
	Type             MessageType `json:"type"`
	UUID             string      `json:"uuid,omitempty"`
	SessionID        string      `json:"session_id,omitempty"`
	IsAuthenticating bool        `json:"isAuthenticating"`
	Output           []string    `json:"output"`
	Error            string      `json:"error,omitempty"`
}

func (m *AuthStatusMessage) messageType() MessageType { return MessageTypeAuthStatus }

// RateLimitEvent reports rate limiting information.
type RateLimitEvent struct {
	Type          MessageType    `json:"type"`
	UUID          string         `json:"uuid,omitempty"`
	SessionID     string         `json:"session_id,omitempty"`
	RateLimitInfo *RateLimitInfo `json:"rate_limit_info"`
}

func (m *RateLimitEvent) messageType() MessageType { return MessageTypeRateLimit }

// RateLimitInfo contains rate limit details.
type RateLimitInfo struct {
	Status        string  `json:"status"` // allowed, allowed_warning, rejected
	ResetsAt      *int64  `json:"resetsAt,omitempty"`
	RateLimitType string  `json:"rateLimitType,omitempty"`
	Utilization   float64 `json:"utilization,omitempty"`
}

// PromptSuggestionMessage carries a predicted next user prompt.
type PromptSuggestionMessage struct {
	Type       MessageType `json:"type"`
	UUID       string      `json:"uuid,omitempty"`
	SessionID  string      `json:"session_id,omitempty"`
	Suggestion string      `json:"suggestion"`
}

func (m *PromptSuggestionMessage) messageType() MessageType { return MessageTypePromptSuggest }

// ParseMessage parses a raw NDJSON line into a typed Message.
func ParseMessage(line []byte) (Message, error) {
	var raw struct {
		Type    MessageType `json:"type"`
		Subtype string      `json:"subtype,omitempty"`
	}
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil, &ParseError{Line: string(line), Err: err}
	}

	switch raw.Type {
	case MessageTypeSystem:
		var m SystemMessage
		if err := json.Unmarshal(line, &m); err != nil {
			return nil, &ParseError{Line: string(line), Err: err}
		}
		// Dispatch task subtypes to dedicated types (matching Python SDK).
		switch m.Subtype {
		case "task_started":
			return &TaskStartedMessage{SystemMessage: m}, nil
		case "task_progress":
			return &TaskProgressMessage{SystemMessage: m}, nil
		case "task_notification":
			return &TaskNotificationMessage{SystemMessage: m}, nil
		}
		return &m, nil

	case MessageTypeAssistant:
		var m AssistantMessage
		if err := json.Unmarshal(line, &m); err != nil {
			return nil, &ParseError{Line: string(line), Err: err}
		}
		return &m, nil

	case MessageTypeUser:
		var m UserMessage
		if err := json.Unmarshal(line, &m); err != nil {
			return nil, &ParseError{Line: string(line), Err: err}
		}
		return &m, nil

	case MessageTypeResult:
		var m ResultMessage
		if err := json.Unmarshal(line, &m); err != nil {
			return nil, &ParseError{Line: string(line), Err: err}
		}
		return &m, nil

	case MessageTypeStream:
		var m StreamEvent
		if err := json.Unmarshal(line, &m); err != nil {
			return nil, &ParseError{Line: string(line), Err: err}
		}
		return &m, nil

	case MessageTypeToolProgress:
		var m ToolProgressMessage
		if err := json.Unmarshal(line, &m); err != nil {
			return nil, &ParseError{Line: string(line), Err: err}
		}
		return &m, nil

	case MessageTypeToolUseSummary:
		var m ToolUseSummaryMessage
		if err := json.Unmarshal(line, &m); err != nil {
			return nil, &ParseError{Line: string(line), Err: err}
		}
		return &m, nil

	case MessageTypeAuthStatus:
		var m AuthStatusMessage
		if err := json.Unmarshal(line, &m); err != nil {
			return nil, &ParseError{Line: string(line), Err: err}
		}
		return &m, nil

	case MessageTypeRateLimit:
		var m RateLimitEvent
		if err := json.Unmarshal(line, &m); err != nil {
			return nil, &ParseError{Line: string(line), Err: err}
		}
		return &m, nil

	case MessageTypePromptSuggest:
		var m PromptSuggestionMessage
		if err := json.Unmarshal(line, &m); err != nil {
			return nil, &ParseError{Line: string(line), Err: err}
		}
		return &m, nil

	default:
		// Unknown message type — return raw for forward compatibility.
		return &RawMessage{TypeField: string(raw.Type), Data: json.RawMessage(line)}, nil
	}
}

// RawMessage is returned for unrecognized message types.
type RawMessage struct {
	TypeField string          `json:"type"`
	Data      json.RawMessage `json:"-"`
}

func (m *RawMessage) messageType() MessageType { return MessageType(m.TypeField) }
