package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"

	"github.com/shellhub-io/claude-agent-sdk-go/internal/process"
	"github.com/shellhub-io/claude-agent-sdk-go/internal/protocol"
)

// Prompt runs a one-shot prompt and returns the result.
// For multi-turn conversations, use NewSession instead.
func Prompt(ctx context.Context, prompt string, opts ...Option) (*ResultMessage, error) {
	if prompt == "" {
		return nil, ErrEmptyPrompt
	}

	cfg := applyOptions(opts)
	// Always use streaming mode — prompt is sent via stdin, not CLI args.
	// This matches the official Python SDK's behavior.
	procCfg := toProcessConfig(cfg, true)

	proc, err := process.Start(ctx, cfg.CLIPath, procCfg)
	if err != nil {
		return nil, err
	}
	defer proc.Close()

	// Drain stderr in background.
	go drainStderr(proc, cfg.StderrCallback)

	// Send the prompt as a user message via stdin, then close stdin
	// to signal that no more messages are coming.
	msg := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": prompt,
		},
	}
	if err := proc.WriteLine(msg); err != nil {
		return nil, fmt.Errorf("claude: write prompt: %w", err)
	}

	var result *ResultMessage
	for {
		line, err := proc.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		parsed, err := ParseMessage(line)
		if err != nil {
			continue // skip unparseable lines
		}

		if r, ok := parsed.(*ResultMessage); ok {
			result = r
			proc.CloseStdin()
			break
		}

		// Handle control requests (permissions, hooks).
		if raw, ok := parsed.(*RawMessage); ok {
			handleControlRequest(proc, cfg, raw.Data)
		}
	}

	if result == nil {
		return nil, fmt.Errorf("claude: no result received")
	}

	return result, nil
}

// Query runs a prompt and returns an iterator over all messages.
// This gives full access to assistant messages, tool use, etc.
func Query(ctx context.Context, prompt string, opts ...Option) iter.Seq2[Message, error] {
	return func(yield func(Message, error) bool) {
		if prompt == "" {
			yield(nil, ErrEmptyPrompt)
			return
		}

		cfg := applyOptions(opts)
		procCfg := toProcessConfig(cfg, true)

		proc, err := process.Start(ctx, cfg.CLIPath, procCfg)
		if err != nil {
			yield(nil, err)
			return
		}
		defer proc.Close()

		go drainStderr(proc, cfg.StderrCallback)

		// Send prompt via stdin, then close to signal one-shot.
		msg := map[string]any{
			"type": "user",
			"message": map[string]any{
				"role":    "user",
				"content": prompt,
			},
		}
		if err := proc.WriteLine(msg); err != nil {
			yield(nil, fmt.Errorf("claude: write prompt: %w", err))
			return
		}

		for {
			line, err := proc.ReadLine()
			if err != nil {
				if err == io.EOF {
					return
				}
				yield(nil, err)
				return
			}

			parsed, err := ParseMessage(line)
			if err != nil {
				continue
			}

			// Handle control requests transparently.
			if raw, ok := parsed.(*RawMessage); ok {
				handleControlRequest(proc, cfg, raw.Data)
				continue
			}

			if !yield(parsed, nil) {
				return
			}

			// Close stdin after result to signal we're done (one-shot).
			if _, ok := parsed.(*ResultMessage); ok {
				proc.CloseStdin()
				return
			}
		}
	}
}

// toProcessConfig converts the public Config to the internal process.Config.
func toProcessConfig(cfg *Config, streaming bool) process.Config {
	pc := process.Config{
		Streaming:                       streaming,
		Model:                           cfg.Model,
		Cwd:                             cfg.Cwd,
		AllowedTools:                    cfg.AllowedTools,
		DisallowedTools:                 cfg.DisallowedTools,
		PermissionMode:                  string(cfg.PermissionMode),
		AllowDangerouslySkipPermissions: cfg.AllowDangerouslySkipPermissions,
		MaxTurns:                        cfg.MaxTurns,
		MaxBudgetUSD:                    cfg.MaxBudgetUSD,
		Effort:                          cfg.Effort,
		MaxThinkingTokens:               cfg.MaxThinkingTokens,
		IncludePartialMessages:          cfg.IncludePartialMessages,
		Resume:                          cfg.Resume,
		ResumeAt:                        cfg.ResumeAt,
		SessionID:                       cfg.SessionID,
		ForkSession:                     cfg.ForkSession,
		Continue:                        cfg.Continue,
		NoPersistSession:                cfg.NoPersistSession,
		Verbose:                         cfg.Verbose,
		Debug:                           cfg.Debug,
		DebugFile:                       cfg.DebugFile,
		FallbackModel:                   cfg.FallbackModel,
		PromptSuggestions:               cfg.PromptSuggestions,
		AgentProgressSummaries:          cfg.AgentProgressSummaries,
		StrictMCPConfig:                 cfg.StrictMCPConfig,
		AgentName:                       cfg.AgentName,
		ExtraArgs:                       cfg.ExtraArgs,
	}

	if cfg.SystemPrompt != nil {
		if cfg.SystemPrompt.Preset {
			pc.SystemPrompt = cfg.SystemPrompt
		} else {
			pc.SystemPrompt = cfg.SystemPrompt.Text
		}
	}

	if cfg.Tools != nil {
		if cfg.Tools.Preset {
			pc.Tools = cfg.Tools
		} else {
			pc.Tools = cfg.Tools.Names
		}
	}

	if cfg.Thinking != nil {
		pc.Thinking = cfg.Thinking
	}

	if len(cfg.MCPServers) > 0 {
		pc.MCPServers = make(map[string]any)
		for k, v := range cfg.MCPServers {
			pc.MCPServers[k] = v
		}
	}

	if len(cfg.Agents) > 0 {
		pc.Agents = make(map[string]any)
		for k, v := range cfg.Agents {
			pc.Agents[k] = v
		}
	}

	if cfg.OutputFormat != nil {
		pc.OutputFormat = cfg.OutputFormat
	}

	if len(cfg.SettingSources) > 0 {
		for _, s := range cfg.SettingSources {
			pc.SettingSources = append(pc.SettingSources, string(s))
		}
	}

	if cfg.Settings != nil {
		pc.Settings = cfg.Settings
	}

	if len(cfg.Plugins) > 0 {
		for _, p := range cfg.Plugins {
			pc.Plugins = append(pc.Plugins, p)
		}
	}

	if len(cfg.Betas) > 0 {
		pc.Betas = cfg.Betas
	}

	if len(cfg.AdditionalDirectories) > 0 {
		pc.AdditionalDirs = cfg.AdditionalDirectories
	}

	if cfg.Sandbox != nil {
		pc.Sandbox = cfg.Sandbox
	}

	return pc
}

// handleControlRequest processes a control request from the CLI.
func handleControlRequest(proc *process.Process, cfg *Config, data json.RawMessage) {
	var raw struct {
		Type      string          `json:"type"`
		RequestID string          `json:"request_id"`
		Request   json.RawMessage `json:"request"`
	}
	if json.Unmarshal(data, &raw) != nil || raw.Type != "control_request" {
		return
	}

	var body struct {
		Subtype string `json:"subtype"`
	}
	if json.Unmarshal(raw.Request, &body) != nil {
		return
	}

	mux := protocol.NewMux(proc)

	switch body.Subtype {
	case "can_use_tool":
		handleCanUseTool(mux, cfg, raw.RequestID, raw.Request)
	case "hook_callback":
		handleHookCallback(mux, cfg, raw.RequestID, raw.Request)
	default:
		// Unknown control request — send error response.
		mux.SendErrorResponse(raw.RequestID, "unsupported request: "+body.Subtype)
	}
}

func handleCanUseTool(mux *protocol.Mux, cfg *Config, requestID string, request json.RawMessage) {
	if cfg.CanUseTool == nil {
		// No handler — allow by default.
		mux.SendResponse(requestID, map[string]any{"behavior": "allow"})
		return
	}

	var req struct {
		ToolName string         `json:"tool_name"`
		Input    map[string]any `json:"input"`
	}
	if json.Unmarshal(request, &req) != nil {
		mux.SendResponse(requestID, map[string]any{"behavior": "allow"})
		return
	}

	result, err := cfg.CanUseTool(req.ToolName, req.Input, CanUseToolOptions{})
	if err != nil {
		mux.SendErrorResponse(requestID, err.Error())
		return
	}

	mux.SendResponse(requestID, result)
}

func handleHookCallback(mux *protocol.Mux, cfg *Config, requestID string, request json.RawMessage) {
	var req struct {
		HookEventName string          `json:"hook_event_name"`
		Body          json.RawMessage `json:"body"`
	}
	if json.Unmarshal(request, &req) != nil {
		mux.SendErrorResponse(requestID, "failed to parse hook callback")
		return
	}

	event := HookEvent(req.HookEventName)
	matchers, ok := cfg.Hooks[event]
	if !ok || len(matchers) == 0 {
		mux.SendResponse(requestID, map[string]any{})
		return
	}

	var input HookInput
	json.Unmarshal(req.Body, &input)

	ctx := context.Background()
	var lastOutput HookOutput
	for _, m := range matchers {
		for _, hook := range m.Hooks {
			output, err := hook(ctx, input)
			if err != nil {
				mux.SendErrorResponse(requestID, err.Error())
				return
			}
			lastOutput = output
		}
	}

	mux.SendResponse(requestID, lastOutput)
}

// drainStderr reads stderr and optionally calls the callback.
func drainStderr(proc *process.Process, callback func(string)) {
	buf := make([]byte, 4096)
	for {
		n, err := proc.Stderr().Read(buf)
		if n > 0 && callback != nil {
			callback(string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}
