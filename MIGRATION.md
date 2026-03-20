# Python → Go Translation Rules

Reference implementation: [anthropics/claude-agent-sdk-python](https://github.com/anthropics/claude-agent-sdk-python)

This document encodes the translation rules used to port the Python SDK to Go, and guides the automated sync bot when implementing new features.

## Type Mapping

| Python | Go |
|--------|-----|
| `str` | `string` |
| `int` | `int` / `int64` (for token counts) |
| `float` | `float64` |
| `bool` | `bool` |
| `None` | `nil` / pointer type |
| `dict[str, Any]` | `map[string]any` |
| `list[T]` | `[]T` |
| `Optional[T]` / `T \| None` | `*T` or omitempty |
| `Any` | `any` or `json.RawMessage` |
| `bytes` | `[]byte` |
| `Literal["a", "b"]` | `string` with constants |
| `TypedDict` | Go struct with json tags |
| `@dataclass` | Go struct |
| `Callable[..., Awaitable[T]]` | `func(ctx context.Context, ...) (T, error)` |
| `AsyncIterable[T]` | `iter.Seq2[T, error]` or `chan T` |

## Naming Conventions

| Python | Go |
|--------|-----|
| `snake_case` fields | `CamelCase` exported fields with `json:"snake_case"` tags |
| `_private` fields | unexported fields (lowercase) |
| `ClaudeAgentOptions` | `Config` + `Option` functional pattern |
| `can_use_tool` param | `WithCanUseTool(fn)` option |
| `permission_mode` param | `WithPermissionMode(mode)` option |
| `include_partial_messages` | `WithIncludePartialMessages()` option |

## Async → Go Patterns

| Python | Go |
|--------|-----|
| `async def` | regular `func` with `context.Context` |
| `await` | blocking call or channel receive |
| `async for msg in stream` | `for msg, err := range iter` |
| `anyio.create_task_group()` | goroutine + `sync.WaitGroup` or channels |
| `anyio.Event` | `chan struct{}` |
| `anyio.create_memory_object_stream` | buffered `chan T` |
| `anyio.fail_after(timeout)` | `context.WithTimeout` or `time.After` |
| `async with client` | `defer session.Close()` |

## SDK Architecture Mapping

| Python | Go | Notes |
|--------|-----|-------|
| `ClaudeSDKClient` | `Session` | Long-lived, multi-turn |
| `query()` function | `Query()` / `Prompt()` | One-shot, uses Session internally |
| `Transport` (subprocess) | `internal/process` | CLI subprocess management |
| `Query._read_messages()` | `Session.readLoop()` | Background goroutine |
| `Query._send_control_request()` | `protocol.Mux.Send()` | With timeout |
| `Query._handle_control_request()` | `Session.dispatchControlRequest()` | Handles can_use_tool, hook_callback, mcp_message |
| `_convert_hook_output_for_cli()` | `formatHookOutput()` | Flat struct → nested dict |

## CLI Argument Mapping

| Python option | CLI flag | Notes |
|---------------|----------|-------|
| `system_prompt=str` | `--system-prompt <text>` | |
| `system_prompt={"type":"preset",...}` | `--append-system-prompt <text>` | When preset with append |
| `cwd=path` | `cmd.Dir` (not a flag!) | Python uses `Popen(cwd=...)` |
| `setting_sources=["user"]` | `--setting-sources user` | Always pass, empty = isolation |
| `setting_sources=None` | `--setting-sources ""` | SDK isolation mode |
| `can_use_tool=callback` | `--permission-prompt-tool stdio` | Auto-set when callback provided |
| `mcp_servers={"name": config}` | `--mcp-config '{"mcpServers":{...}}'` | Wrapper format |
| SDK MCP (`type:"sdk"`) | `--mcp-config '{"mcpServers":{"name":{"type":"sdk","name":"..."}}}'` | Must include `name` field |
| `hooks={...}` | via `initialize` control request | Not CLI flags |
| `agents={...}` | via `initialize` control request | Not CLI flags |

## Control Protocol

| Python method | Go method | Control subtype |
|---------------|-----------|-----------------|
| `_send_control_request({"subtype":"initialize",...})` | `mux.Send("initialize", ...)` | `initialize` |
| `can_use_tool(name, input, ctx)` | `handleCanUseTool(mux, cfg, reqID, req)` | `can_use_tool` |
| `hook_callbacks[id](input, toolUseId, ctx)` | `hookCallbacks[id](ctx, input)` | `hook_callback` |
| `_handle_sdk_mcp_request(name, msg)` | `srv.HandleMessage(ctx, msg)` | `mcp_message` |
| `set_permission_mode(mode)` | `session.SetPermissionMode(mode)` | `set_permission_mode` |
| `set_model(model)` | `session.SetModel(model)` | `set_model` |
| `interrupt()` | `session.Interrupt()` | `interrupt` |

## Hook Output Format

Python returns a flat dict that goes directly as the control response. Go uses a `HookOutput` struct that `formatHookOutput()` converts to the nested format:

```
Python:                              Go struct → wire format:
{                                    HookOutput{
  "reason": "...",                     Reason: "...",
  "systemMessage": "...",              SystemMessage: "...",
  "hookSpecificOutput": {              Decision: "allow",
    "hookEventName": "PreToolUse",     DecisionReason: "...",
    "permissionDecision": "allow",     AdditionalContext: "...",
    "additionalContext": "...",       }
  },                                  ↓ formatHookOutput() →
}                                    {"reason":"...","hookSpecificOutput":{"hookEventName":"...","permissionDecision":"allow",...}}
```

## Message Type Mapping

| Python class | Go type | Notes |
|-------------|---------|-------|
| `SystemMessage` | `SystemMessage` | Generic: `data` dict in Python, explicit fields in Go |
| `TaskStartedMessage(SystemMessage)` | `SystemMessage` with `Subtype="task_started"` | Subclass in Python, subtype in Go |
| `TaskProgressMessage(SystemMessage)` | `SystemMessage` with `Subtype="task_progress"` | |
| `TaskNotificationMessage(SystemMessage)` | `SystemMessage` with `Subtype="task_notification"` | |
| `AssistantMessage` | `AssistantMessage` | `model` inside `Message` sub-struct |
| `UserMessage` | `UserMessage` | |
| `ResultMessage` | `ResultMessage` | `is_error` without omitempty, `stop_reason` as `*string` |
| `StreamEvent` | `StreamEvent` | |
| `RateLimitEvent` | `RateLimitEvent` | |

## Testing Strategy

| Layer | What it catches | How |
|-------|----------------|-----|
| Conformance fixtures | Parse errors, type mismatches | Extracted from Python `test_message_parser.py` |
| Round-trip test | Missing struct fields (json tags) | Parse → serialize → compare keys |
| Mock tests | SDK logic (Session, Send, hooks) | Shell scripts simulating CLI NDJSON |
| E2E in container | Real behavior, CLI compatibility | testcontainers-go, clean Docker env |
| Coverage check | Missing types, hooks, options | Python AST → Go source comparison |

## Known Differences

1. **SystemMessage.data**: Python stores all init fields in a generic `data: dict[str, Any]`. Go uses explicit struct fields (`Model`, `Tools`, `Agents`, etc.). New fields in the CLI require adding to the Go struct.

2. **Error field**: Python's `AssistantMessage.error` is typed as `AssistantMessageError | None`. Go uses `string` because the CLI sends it as a plain string (`"authentication_failed"`).

3. **Null handling**: Go's `omitempty` drops `false`, `0`, `""`, and `nil`. Fields that the CLI sends explicitly as `null` or `false` use pointer types or no omitempty.

4. **MCP inline tools**: Python uses `@modelcontextprotocol/sdk` (MCP SDK). Go uses `mark3labs/mcp-go`. The wire protocol is identical (JSON-RPC over control messages).

5. **Setting sources**: Must always pass `--setting-sources` (even empty). Without it, the CLI loads all settings from disk, breaking isolation.
