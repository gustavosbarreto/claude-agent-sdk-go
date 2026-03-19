# Claude Agent SDK for Go

Go SDK for the Claude Code CLI. Wraps the `claude` binary as a subprocess, communicating via NDJSON over stdin/stdout.

## Architecture

```
claude.go                     ← Prompt() one-shot, Query() iterator
session.go                    ← NewSession, Send, StreamInput (multi-turn)
sessions.go                   ← ListSessions, GetSessionMessages (discovery)
message.go                    ← All message types + ParseMessage()
content.go                    ← ContentBlock (text, thinking, tool_use, tool_result)
option.go                     ← Config + 40+ With* functional options
hook.go                       ← 23 hook events + HookCallback
agent.go                      ← AgentDefinition for subagents
mcp.go                        ← MCP server configs (stdio/SSE/HTTP)
error.go                      ← Typed errors
internal/
├── process/
│   ├── process.go            ← CLI subprocess lifecycle
│   └── args.go               ← CLI argument builder
└── protocol/
    └── control.go            ← Control request/response multiplexer
```

## Protocol

The SDK communicates with `claude --print --input-format stream-json --output-format stream-json`:

- **stdin**: SDK sends `{"type":"user","message":{"role":"user","content":"..."}}` NDJSON lines
- **stdout**: CLI sends system, assistant, user, result, stream_event, control_request NDJSON lines
- **Control protocol**: bidirectional request/response for permissions and hooks

## Reference SDK

The **official Python SDK** at `github.com/anthropics/claude-agent-sdk-python` is the reference implementation. This Go SDK must remain compatible with it.

Key reference files:
- `tests/test_message_parser.py` — message parsing contract tests
- `tests/test_transport.py` — CLI argument construction tests
- `tests/test_types.py` — type definitions
- `src/claude_agent_sdk/types.py` — canonical type definitions
- `src/claude_agent_sdk/_internal/message_parser.py` — message parser
- `src/claude_agent_sdk/_internal/transport/subprocess_cli.py` — subprocess transport

## Conformance Tests

`testdata/conformance.json` contains test fixtures extracted from the Python SDK's `tests/test_message_parser.py`. These are the **source of truth** for message parsing compatibility.

`conformance_test.go` runs every fixture against `ParseMessage()` and validates types, subtypes, and key fields.

## Sync Workflow

The `.github/workflows/sync.yml` workflow runs automatically to keep this SDK in sync with the official Python SDK:

1. Clones `anthropics/claude-agent-sdk-python` at latest
2. Compares Python types/tests with our Go implementation
3. Uses Claude Code to implement any missing features
4. Runs all tests including conformance
5. Opens a PR if changes are needed

## Development

```bash
go build ./...           # Build
go test ./... -v         # Run all tests
go test -run Conformance # Run only conformance tests
go vet ./...             # Lint
```

## Rules

- Zero external dependencies — stdlib only
- All message types must match the official Python SDK's `types.py`
- Every new message type needs a conformance test fixture in `testdata/conformance.json`
- CLI args must match the Python SDK's `subprocess_cli.py` `_build_command()`
- Use idiomatic Go: functional options, `iter.Seq2`, `context.Context`, `json.RawMessage` for flexible fields
- JSON tags use snake_case matching the CLI's NDJSON protocol
- Optional fields use pointer types or omitempty
