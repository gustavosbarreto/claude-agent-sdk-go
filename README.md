# Claude Agent SDK for Go

Go SDK for the Claude Code CLI. Spawns `claude` as a subprocess, communicates via NDJSON over stdin/stdout. Zero external dependencies.

## Install

```bash
go get github.com/gustavosbarreto/claude-agent-sdk-go
```

Requires the [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) installed and authenticated.

## Usage

### One-shot prompt

```go
result, err := claude.Prompt(ctx, "What is 2+2?",
    claude.WithModel("claude-sonnet-4-6"),
)
fmt.Println(result.Result) // "4"
```

### Streaming all messages

```go
for msg, err := range claude.Query(ctx, "Explain Go interfaces") {
    switch m := msg.(type) {
    case *claude.AssistantMessage:
        fmt.Print(claude.CombinedText(m.Message.Content))
    case *claude.ResultMessage:
        fmt.Printf("\nCost: $%.4f\n", m.TotalCostUSD)
    }
}
```

### Multi-turn session

```go
session, _ := claude.NewSession(ctx,
    claude.WithPermissionMode(claude.PermissionBypassPermissions),
    claude.WithAllowDangerouslySkipPermissions(),
)
defer session.Close()

// Turn 1
for msg, _ := range session.Send(ctx, "Pick a number between 1 and 10") {
    if a, ok := msg.(*claude.AssistantMessage); ok {
        fmt.Println(claude.CombinedText(a.Message.Content))
    }
}

// Turn 2 — same session, context preserved
for msg, _ := range session.Send(ctx, "Double it") {
    if a, ok := msg.(*claude.AssistantMessage); ok {
        fmt.Println(claude.CombinedText(a.Message.Content))
    }
}
```

### Structured output

```go
schema := map[string]any{
    "type": "object",
    "properties": map[string]any{
        "answer": map[string]any{"type": "number"},
    },
    "required": []string{"answer"},
}

result, _ := claude.Prompt(ctx, "What is 7*8?",
    claude.WithOutputFormat(claude.OutputFormat{
        Type:   "json_schema",
        Schema: schema,
    }),
)
fmt.Println(string(result.StructuredOutput)) // {"answer": 56}
```

### MCP servers

```go
result, _ := claude.Prompt(ctx, "Query the users table",
    claude.WithMCPServer("postgres", claude.MCPStdioServer{
        Command: "npx",
        Args:    []string{"-y", "@modelcontextprotocol/server-postgres"},
        Env:     map[string]string{"DATABASE_URL": "postgresql://..."},
    }),
)
```

### Hooks

```go
result, _ := claude.Prompt(ctx, "Read /etc/hostname",
    claude.WithHook(claude.HookPreToolUse, claude.HookCallbackMatcher{
        Hooks: []claude.HookCallback{
            func(ctx context.Context, input claude.HookInput) (claude.HookOutput, error) {
                log.Printf("Tool: %s", input.ToolName)
                return claude.HookOutput{}, nil
            },
        },
    }),
)
```

### Subagents

```go
result, _ := claude.Prompt(ctx, "Review this code for bugs",
    claude.WithAgent("reviewer", claude.AgentDefinition{
        Description: "Reviews code for bugs and security issues",
        Prompt:      "You are a code reviewer. Focus on bugs and security.",
        Tools:       []string{"Read", "Glob", "Grep"},
    }),
)
```

### Mid-session control

```go
session, _ := claude.NewSession(ctx)

// Change model between turns
session.SetModel("claude-haiku-4-5")

// Change permissions
session.SetPermissionMode(claude.PermissionAcceptEdits)

// Interrupt a running query
session.Interrupt()
```

### Channel adapter

```go
ch := claude.ToChan(claude.Query(ctx, "Hello"))
for me := range ch {
    if me.Err != nil {
        break
    }
    fmt.Printf("%T\n", me.Message)
}
```

## Options

| Option | Description |
|--------|-------------|
| `WithModel` | Model ID or alias |
| `WithSystemPrompt` | Custom system prompt |
| `WithSystemPromptPreset` | Claude Code default prompt + append |
| `WithCwd` | Working directory |
| `WithMaxTurns` | Max conversation turns |
| `WithMaxBudgetUSD` | Cost limit |
| `WithPermissionMode` | default, acceptEdits, bypassPermissions, plan, dontAsk |
| `WithAllowedTools` | Auto-allowed tools |
| `WithDisallowedTools` | Blocked tools |
| `WithMCPServer` | Add MCP server |
| `WithHook` | Register hook callback |
| `WithAgent` | Define subagent |
| `WithOutputFormat` | Structured output schema |
| `WithThinking` | Thinking mode (adaptive, enabled, disabled) |
| `WithEffort` | low, medium, high, max |
| `WithResume` | Resume session by ID |
| `WithIncludePartialMessages` | Enable streaming deltas |
| `WithNoPersistSession` | Don't save session to disk |

See [option.go](option.go) for the full list (40+ options).

## Message types

| Type | Description |
|------|-------------|
| `SystemMessage` | Session init, status, hooks, tasks, files |
| `AssistantMessage` | Claude's response with content blocks |
| `UserMessage` | Echoed user messages |
| `ResultMessage` | End of turn with cost/usage |
| `StreamEvent` | Token-level streaming deltas |
| `ToolProgressMessage` | Tool execution progress |
| `ToolUseSummaryMessage` | Tool usage summary |
| `AuthStatusMessage` | Authentication state |
| `RateLimitEvent` | Rate limit info |
| `PromptSuggestionMessage` | Predicted next prompt |

## Compatibility

This SDK tracks the official [Python SDK](https://github.com/anthropics/claude-agent-sdk-python). A daily [sync workflow](.github/workflows/sync.yml) detects changes and uses Claude Code to keep the Go implementation in sync.

Three detection layers:
1. **Conformance fixtures** — test cases extracted from the Python SDK
2. **Test suite** — all tests must pass
3. **Type coverage** — all Python message types must be implemented in Go

## License

MIT
