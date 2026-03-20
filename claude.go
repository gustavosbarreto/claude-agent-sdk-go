package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	"github.com/shellhub-io/claude-agent-sdk-go/internal/process"
	"github.com/shellhub-io/claude-agent-sdk-go/internal/protocol"
)

// Prompt runs a one-shot prompt and returns the result.
// For multi-turn conversations, use NewSession instead.
func Prompt(ctx context.Context, prompt string, opts ...Option) (*ResultMessage, error) {
	if prompt == "" {
		return nil, ErrEmptyPrompt
	}

	session, err := NewSession(ctx, opts...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = session.Close() }()

	var result *ResultMessage
	for msg, err := range session.Send(ctx, prompt) {
		if err != nil {
			return nil, err
		}
		if r, ok := msg.(*ResultMessage); ok {
			result = r
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

		session, err := NewSession(ctx, opts...)
		if err != nil {
			yield(nil, err)
			return
		}
		defer func() { _ = session.Close() }()

		for msg, err := range session.Send(ctx, prompt) {
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(msg, nil) {
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

func handleCanUseTool(mux *protocol.Mux, cfg *Config, requestID string, request json.RawMessage) error {
	if cfg.CanUseTool == nil {
		return mux.SendResponse(requestID, map[string]any{"behavior": "allow"})
	}

	var req struct {
		ToolName string         `json:"tool_name"`
		Input    map[string]any `json:"input"`
	}
	if json.Unmarshal(request, &req) != nil {
		return mux.SendResponse(requestID, map[string]any{"behavior": "allow"})
	}

	result, err := cfg.CanUseTool(req.ToolName, req.Input, CanUseToolOptions{})
	if err != nil {
		return mux.SendErrorResponse(requestID, err.Error())
	}

	return mux.SendResponse(requestID, result)
}

func handleHookCallback(mux *protocol.Mux, cfg *Config, requestID string, request json.RawMessage) error {
	var req struct {
		HookEventName string          `json:"hook_event_name"`
		Body          json.RawMessage `json:"body"`
	}
	if json.Unmarshal(request, &req) != nil {
		return mux.SendErrorResponse(requestID, "failed to parse hook callback")
	}

	event := HookEvent(req.HookEventName)
	matchers, ok := cfg.Hooks[event]
	if !ok || len(matchers) == 0 {
		return mux.SendResponse(requestID, map[string]any{})
	}

	var input HookInput
	_ = json.Unmarshal(req.Body, &input)

	ctx := context.Background()
	var lastOutput HookOutput
	for _, m := range matchers {
		for _, hook := range m.Hooks {
			output, err := hook(ctx, input)
			if err != nil {
				return mux.SendErrorResponse(requestID, err.Error())
			}
			lastOutput = output
		}
	}

	return mux.SendResponse(requestID, lastOutput)
}

// drainStderr reads stderr line by line and calls the callback per line.
// Matches the Python SDK behavior where the callback receives one line at a time
// with trailing whitespace stripped.
func drainStderr(proc *process.Process, callback func(string)) {
	scanner := bufio.NewScanner(proc.Stderr())
	scanner.Buffer(make([]byte, 0, 64*1024), 1*1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), " \t\r\n")
		if line == "" {
			continue
		}
		if callback != nil {
			callback(line)
		}
	}
}
