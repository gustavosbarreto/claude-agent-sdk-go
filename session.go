package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"sync"

	"github.com/gustavosbarreto/claude-agent-sdk-go/internal/process"
	"github.com/gustavosbarreto/claude-agent-sdk-go/internal/protocol"
)

// Session manages a multi-turn conversation with the Claude CLI.
// A background goroutine continuously reads stdout and dispatches
// messages and control responses, avoiding deadlocks when sending
// control requests between turns.
type Session struct {
	proc      *process.Process
	mux       *protocol.Mux
	cfg       *Config
	sessionID string
	closed    bool
	mu        sync.Mutex

	// hookCallbacks maps callback IDs to hook functions.
	hookCallbacks map[string]HookCallback

	// sdkMcpServers holds inline MCP servers for handling mcp_message requests.
	sdkMcpServers map[string]*SdkMcpServer

	// messages is fed by the reader goroutine. Send() consumes it.
	messages chan messageOrEOF
	// readerDone is closed when the reader goroutine exits.
	readerDone chan struct{}
}

type messageOrEOF struct {
	msg Message
	err error // io.EOF means pipe closed
}

// NewSession creates a new multi-turn session.
// Sends an initialize control request to register hooks and agents with the CLI.
func NewSession(ctx context.Context, opts ...Option) (*Session, error) {
	cfg := applyOptions(opts)
	procCfg := toProcessConfig(cfg, true)

	proc, err := process.Start(ctx, cfg.CLIPath, procCfg)
	if err != nil {
		return nil, err
	}

	mux := protocol.NewMux(proc)

	// Build hook callback registry: assign IDs to each callback function
	// so the CLI can reference them in hook_callback control requests.
	hookCallbacks := make(map[string]HookCallback)
	nextID := 0

	s := &Session{
		proc:          proc,
		mux:           mux,
		cfg:           cfg,
		hookCallbacks: hookCallbacks,
		sdkMcpServers: cfg.SdkMcpServers,
		messages:      make(chan messageOrEOF, 64),
		readerDone:    make(chan struct{}),
	}

	go drainStderr(proc, cfg.StderrCallback)
	go s.readLoop()

	// Send initialize request with hooks and agents config.
	// This matches the Python SDK's initialize flow.
	initReq := map[string]any{
		"subtype": "initialize",
	}

	if len(cfg.Hooks) > 0 {
		hooksConfig := make(map[string]any)
		for event, matchers := range cfg.Hooks {
			var matcherConfigs []map[string]any
			for _, m := range matchers {
				var callbackIDs []string
				for _, hook := range m.Hooks {
					id := fmt.Sprintf("hook_%d", nextID)
					nextID++
					hookCallbacks[id] = hook
					callbackIDs = append(callbackIDs, id)
				}
				mc := map[string]any{
					"hookCallbackIds": callbackIDs,
				}
				if m.Matcher != nil {
					mc["matcher"] = *m.Matcher
				}
				if m.Timeout != nil {
					mc["timeout"] = *m.Timeout
				}
				matcherConfigs = append(matcherConfigs, mc)
			}
			hooksConfig[string(event)] = matcherConfigs
		}
		initReq["hooks"] = hooksConfig
	}

	if len(cfg.Agents) > 0 {
		initReq["agents"] = cfg.Agents
	}

	// Always send initialize to register hooks/agents with the CLI.
	// This matches the Python SDK which always calls initialize().
	if _, err := s.mux.Send("initialize", initReq); err != nil {
		_ = proc.Close()
		return nil, fmt.Errorf("claude: initialize: %w", err)
	}

	return s, nil
}

// readLoop continuously reads stdout, dispatches control responses to the mux,
// handles control requests (permissions, hooks), and forwards user-visible
// messages to the messages channel.
func (s *Session) readLoop() {
	defer close(s.readerDone)
	defer close(s.messages)

	for {
		line, err := s.proc.ReadLine()
		if err != nil {
			if err != io.EOF {
				s.messages <- messageOrEOF{err: err}
			}
			return
		}

		// Try to detect control_request / control_response before full parse.
		var peek struct {
			Type      string          `json:"type"`
			RequestID string          `json:"request_id,omitempty"`
			Request   json.RawMessage `json:"request,omitempty"`
			Response  json.RawMessage `json:"response,omitempty"`
		}
		if json.Unmarshal(line, &peek) == nil {
			switch peek.Type {
			case "control_request":
				if err := s.dispatchControlRequest(peek.RequestID, peek.Request); err != nil {
					s.messages <- messageOrEOF{err: err}
					return
				}
				continue
			case "control_response":
				var resp protocol.ControlResponseBody
				if json.Unmarshal(peek.Response, &resp) == nil {
					s.mux.HandleResponse(resp)
				}
				continue
			}
		}

		parsed, err := ParseMessage(line)
		if err != nil {
			continue // skip unparseable
		}

		// Track session ID.
		switch m := parsed.(type) {
		case *SystemMessage:
			if m.SessionID != "" {
				s.mu.Lock()
				s.sessionID = m.SessionID
				s.mu.Unlock()
			}
		case *ResultMessage:
			s.mu.Lock()
			if m.SessionID != "" && s.sessionID == "" {
				s.sessionID = m.SessionID
			}
			s.mu.Unlock()
		}

		// Forward to Send() consumer.
		s.messages <- messageOrEOF{msg: parsed}
	}
}

// dispatchControlRequest handles a control request from the CLI.
func (s *Session) dispatchControlRequest(requestID string, request json.RawMessage) error {
	var body struct {
		Subtype    string          `json:"subtype"`
		CallbackID string          `json:"callback_id,omitempty"`
		Input      json.RawMessage `json:"input,omitempty"`
		ToolUseID  string          `json:"tool_use_id,omitempty"`
	}
	if json.Unmarshal(request, &body) != nil {
		return s.mux.SendErrorResponse(requestID, "failed to parse request")
	}

	switch body.Subtype {
	case "can_use_tool":
		return handleCanUseTool(s.mux, s.cfg, requestID, request)

	case "hook_callback":
		// Look up callback by ID (registered during initialize).
		callback, ok := s.hookCallbacks[body.CallbackID]
		if !ok {
			return s.mux.SendErrorResponse(requestID,
				fmt.Sprintf("no hook callback found for ID: %s", body.CallbackID))
		}

		var input HookInput
		_ = json.Unmarshal(body.Input, &input)

		output, err := callback(context.Background(), input)
		if err != nil {
			return s.mux.SendErrorResponse(requestID, err.Error())
		}

		// Format output matching the CLI's expected format.
		// The Python SDK passes the hook output dict directly as the response.
		return s.mux.SendResponse(requestID, formatHookOutput(output, input.HookEventName))

	case "mcp_message":
		// Route MCP messages to inline SDK MCP servers.
		var mcpReq struct {
			ServerName string          `json:"server_name"`
			Message    json.RawMessage `json:"message"`
		}
		if json.Unmarshal(request, &mcpReq) != nil {
			return s.mux.SendErrorResponse(requestID, "failed to parse mcp_message")
		}

		srv, ok := s.sdkMcpServers[mcpReq.ServerName]
		if !ok {
			return s.mux.SendErrorResponse(requestID,
				fmt.Sprintf("unknown SDK MCP server: %s", mcpReq.ServerName))
		}

		resp, err := srv.HandleMessage(context.Background(), mcpReq.Message)
		if err != nil {
			return s.mux.SendErrorResponse(requestID, err.Error())
		}

		// Wrap as mcp_response (matching Python SDK line 319).
		var respObj any
		_ = json.Unmarshal(resp, &respObj)
		return s.mux.SendResponse(requestID, map[string]any{"mcp_response": respObj})

	default:
		return s.mux.SendErrorResponse(requestID, "unsupported: "+body.Subtype)
	}
}

// formatHookOutput converts a flat HookOutput to the nested format the CLI expects.
func formatHookOutput(o HookOutput, hookEventName string) map[string]any {
	result := make(map[string]any)

	if o.Continue != nil {
		result["continue"] = *o.Continue
	}
	if o.Reason != "" {
		result["reason"] = o.Reason
	}
	if o.SystemMessage != "" {
		result["systemMessage"] = o.SystemMessage
	}
	if o.SuppressOutput {
		result["suppressOutput"] = true
	}
	if o.StopReason != "" {
		result["stopReason"] = o.StopReason
	}

	// Build hookSpecificOutput if any hook-specific fields are set.
	specific := make(map[string]any)

	if o.Decision != "" {
		specific["permissionDecision"] = o.Decision
	}
	if o.DecisionReason != "" {
		specific["permissionDecisionReason"] = o.DecisionReason
	}
	if o.UpdatedInput != nil {
		specific["updatedInput"] = o.UpdatedInput
	}
	if o.AdditionalContext != "" {
		specific["additionalContext"] = o.AdditionalContext
	}
	if o.UpdatedMCPToolOutput != nil {
		specific["updatedMCPToolOutput"] = o.UpdatedMCPToolOutput
	}
	if o.BlockStop {
		specific["blockStop"] = true
	}

	if len(specific) > 0 {
		if hookEventName != "" {
			specific["hookEventName"] = hookEventName
		}
		result["hookSpecificOutput"] = specific
	}

	return result
}

// ResumeSession resumes a previous session by ID.
func ResumeSession(ctx context.Context, sessionID string, opts ...Option) (*Session, error) {
	opts = append(opts, WithResume(sessionID))
	return NewSession(ctx, opts...)
}

// Send sends a user message and returns an iterator over the response messages.
// The iterator yields all messages until a ResultMessage is received (end of turn).
func (s *Session) Send(ctx context.Context, prompt string) iter.Seq2[Message, error] {
	return func(yield func(Message, error) bool) {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			yield(nil, ErrSessionClosed)
			return
		}
		s.mu.Unlock()

		if prompt == "" {
			yield(nil, ErrEmptyPrompt)
			return
		}

		msg := map[string]any{
			"type": "user",
			"message": map[string]any{
				"role":    "user",
				"content": prompt,
			},
		}
		if err := s.proc.WriteLine(msg); err != nil {
			yield(nil, fmt.Errorf("claude: write message: %w", err))
			return
		}

		for {
			select {
			case <-ctx.Done():
				yield(nil, ctx.Err())
				return
			case me, ok := <-s.messages:
				if !ok {
					// Reader goroutine exited (process closed).
					return
				}
				if me.err != nil {
					yield(nil, me.err)
					return
				}

				if !yield(me.msg, nil) {
					return
				}

				if _, ok := me.msg.(*ResultMessage); ok {
					return
				}
			}
		}
	}
}

// SessionID returns the session ID assigned by the CLI.
func (s *Session) SessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionID
}

// Close closes the session and the CLI process.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	return s.proc.Close()
}

// Interrupt sends an interrupt request to the CLI.
func (s *Session) Interrupt() error {
	_, err := s.mux.Send("interrupt", map[string]any{})
	return err
}

// SetModel changes the model mid-session.
// Pass an empty string to reset to the default model.
func (s *Session) SetModel(model string) error {
	var modelVal any = model
	if model == "" {
		modelVal = nil
	}
	_, err := s.mux.Send("set_model", map[string]any{"model": modelVal})
	return err
}

// SetPermissionMode changes the permission mode mid-session.
func (s *Session) SetPermissionMode(mode PermissionMode) error {
	_, err := s.mux.Send("set_permission_mode", map[string]any{"permission_mode": string(mode)})
	return err
}

// SetMaxThinkingTokens changes the thinking token limit mid-session.
func (s *Session) SetMaxThinkingTokens(n *int) error {
	var val any = nil
	if n != nil {
		val = *n
	}
	_, err := s.mux.Send("set_max_thinking_tokens", map[string]any{"maxThinkingTokens": val})
	return err
}

// ApplyFlagSettings merges settings into the flag settings layer mid-session.
func (s *Session) ApplyFlagSettings(settings any) error {
	_, err := s.mux.Send("apply_flag_settings", map[string]any{"settings": settings})
	return err
}

// RewindFiles restores files to their state at a specific user message.
func (s *Session) RewindFiles(userMessageID string, dryRun bool) (*RewindFilesResult, error) {
	resp, err := s.mux.Send("rewind_files", map[string]any{
		"userMessageId": userMessageID,
		"dryRun":        dryRun,
	})
	if err != nil {
		return nil, err
	}
	var result RewindFilesResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RewindFilesResult is the response from RewindFiles.
type RewindFilesResult struct {
	CanRewind    bool     `json:"canRewind"`
	Error        string   `json:"error,omitempty"`
	FilesChanged []string `json:"filesChanged,omitempty"`
	Insertions   int      `json:"insertions,omitempty"`
	Deletions    int      `json:"deletions,omitempty"`
}

// MCPServerStatusList returns the status of all MCP servers.
func (s *Session) MCPServerStatusList() ([]MCPServerStatus, error) {
	resp, err := s.mux.Send("mcp_server_status", map[string]any{})
	if err != nil {
		return nil, err
	}
	var result []MCPServerStatus
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ReconnectMCPServer reconnects a specific MCP server.
func (s *Session) ReconnectMCPServer(name string) error {
	_, err := s.mux.Send("reconnect_mcp_server", map[string]any{"serverName": name})
	return err
}

// ToggleMCPServer enables or disables an MCP server.
func (s *Session) ToggleMCPServer(name string, enabled bool) error {
	_, err := s.mux.Send("toggle_mcp_server", map[string]any{"serverName": name, "enabled": enabled})
	return err
}

// SetMCPServers replaces the set of MCP servers.
func (s *Session) SetMCPServers(servers map[string]MCPServerConfig) (*MCPSetServersResult, error) {
	resp, err := s.mux.Send("set_mcp_servers", map[string]any{"servers": servers})
	if err != nil {
		return nil, err
	}
	var result MCPSetServersResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// MCPSetServersResult is the response from SetMCPServers.
type MCPSetServersResult struct {
	Added   []string          `json:"added"`
	Removed []string          `json:"removed"`
	Errors  map[string]string `json:"errors"`
}

// StopTask stops a background task by ID.
func (s *Session) StopTask(taskID string) error {
	_, err := s.mux.Send("stop_task", map[string]any{"taskId": taskID})
	return err
}

// InitializationResult returns the full init data (models, commands, account).
func (s *Session) InitializationResult() (*InitResult, error) {
	resp, err := s.mux.Send("get_initialization_result", map[string]any{})
	if err != nil {
		return nil, err
	}
	var result InitResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// InitResult is the full initialization data from the CLI.
type InitResult struct {
	Commands              []SlashCommand `json:"commands"`
	Agents                []AgentInfo    `json:"agents"`
	OutputStyle           string         `json:"output_style"`
	AvailableOutputStyles []string       `json:"available_output_styles"`
	Models                []ModelInfo    `json:"models"`
	Account               AccountInfo    `json:"account"`
}

// SlashCommand describes an available slash command.
type SlashCommand struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	ArgumentHint string `json:"argumentHint"`
}

// ModelInfo describes an available model.
type ModelInfo struct {
	Value                    string   `json:"value"`
	DisplayName              string   `json:"displayName"`
	Description              string   `json:"description"`
	SupportsEffort           bool     `json:"supportsEffort,omitempty"`
	SupportedEffortLevels    []string `json:"supportedEffortLevels,omitempty"`
	SupportsAdaptiveThinking bool     `json:"supportsAdaptiveThinking,omitempty"`
	SupportsFastMode         bool     `json:"supportsFastMode,omitempty"`
}

// AccountInfo describes the logged-in user's account.
type AccountInfo struct {
	Email            string `json:"email,omitempty"`
	Organization     string `json:"organization,omitempty"`
	SubscriptionType string `json:"subscriptionType,omitempty"`
	TokenSource      string `json:"tokenSource,omitempty"`
	APIKeySource     string `json:"apiKeySource,omitempty"`
	APIProvider      string `json:"apiProvider,omitempty"`
}

// StreamInput injects additional user messages into an active session.
func (s *Session) StreamInput(prompt string) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrSessionClosed
	}
	s.mu.Unlock()

	msg := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": prompt,
		},
	}
	return s.proc.WriteLine(msg)
}

// ToChan converts a message iterator to a channel for use with select.
func ToChan(seq iter.Seq2[Message, error]) <-chan MessageOrError {
	ch := make(chan MessageOrError, 16)
	go func() {
		defer close(ch)
		for msg, err := range seq {
			ch <- MessageOrError{Message: msg, Err: err}
		}
	}()
	return ch
}

// MessageOrError is a message or error pair for channel-based consumption.
type MessageOrError struct {
	Message Message
	Err     error
}
