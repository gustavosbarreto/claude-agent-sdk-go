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
├── protocol/
│   └── control.go            ← Control request/response multiplexer
└── testutil/
    └── mock.go               ← Mock CLI helpers for unit tests
tests/
├── e2e/
│   ├── e2e_test.go           ← E2E tests against real Claude CLI
│   ├── container_test.go     ← Docker container orchestrator (testcontainers-go)
│   ├── Dockerfile            ← Clean container for e2e tests
│   └── Dockerfile.sniffer    ← Python SDK + sniffer container
├── conformance/
│   ├── conformance_test.go   ← Python SDK compatibility tests
│   └── testdata/
│       └── conformance.json  ← Fixtures from Python SDK test_message_parser.py
└── sniffer/
    ├── main.go               ← CLI proxy that logs NDJSON traffic
    ├── sniff.sh              ← Orchestrates Python vs Go trace comparison
    └── compare-traces.py     ← Compares NDJSON traces between SDKs
scripts/
├── extract-fixtures.py       ← Extracts conformance fixtures from Python SDK
└── check-coverage.py         ← Detects type/hook/option coverage gaps
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

## Testing

### Unit tests

Test parsing, options, errors, and SDK logic using mock CLI scripts (no API key, no real CLI needed).

```bash
go test . -v -count=1                    # Root package: message parsing, options, mock sessions
go test ./internal/process/ -v -count=1  # CLI argument builder
go test ./... -v -count=1 -race          # Everything (unit + conformance, e2e skipped)
```

Files: `message_test.go`, `mock_test.go` (root), `internal/process/args_test.go`.

### Conformance tests

Validates that `ParseMessage()` produces the same types and fields as the Python SDK. Fixtures in `tests/conformance/testdata/conformance.json` are auto-extracted from `anthropics/claude-agent-sdk-python tests/test_message_parser.py`.

Two tests:
- `TestConformance` — type and subtype match for every fixture
- `TestConformance_RoundTrip` — all JSON fields survive parse → serialize (catches missing struct fields)

```bash
go test ./tests/conformance/ -v -count=1
```

To regenerate fixtures from a fresh Python SDK clone:
```bash
python3 scripts/extract-fixtures.py /path/to/claude-agent-sdk-python > tests/conformance/testdata/conformance.json
```

### E2E tests

Run against the **real Claude CLI** inside a clean Docker container (no host settings, no host CLI). Always run via the container — both locally and in CI.

```bash
go test ./tests/e2e/ -v -count=1 -run TestE2E_Container -timeout 10m
```

`TestE2E_Container` builds `tests/e2e/Dockerfile`, installs Claude CLI, copies the test binary, and runs all `TestE2E_*` tests inside. Auth is injected via env vars (`CLAUDE_CODE_OAUTH_TOKEN` or `ANTHROPIC_API_KEY`) or `~/.claude/.credentials.json` bind mount.

Individual e2e tests check `INSIDE_CONTAINER=1` (set by the Dockerfile) and skip when run on the host. `TestE2E_Container` skips itself inside the container to prevent recursion.

Covers: `Prompt()`, `Query()`, multi-turn sessions, `SetModel`, structured output, tool use, hooks (pre/post, deny, stop, multiple), `can_use_tool` callback, stderr callbacks, partial messages / thinking deltas, interrupt, permission mode switching, setting sources, SDK MCP tools.

### Protocol sniffer (Python vs Go comparison)

Runs the same e2e test in both the Python SDK and Go SDK, each with a transparent CLI proxy (sniffer) that logs all NDJSON traffic. Then compares CLI args, message types, and control protocol sequences.

```bash
./tests/sniffer/sniff.sh
```

Requires Docker and auth (`~/.claude/.credentials.json` or `ANTHROPIC_API_KEY`). What it does:

1. Builds the sniffer binary (`tests/sniffer/main.go`) — a CLI proxy that tees stdin/stdout/stderr to trace files
2. Builds two Docker images: Python SDK (`tests/e2e/Dockerfile.sniffer`) and Go SDK (`tests/e2e/Dockerfile`)
3. For each test pair (e.g. `test_set_model` ↔ `TestE2E_Session_SetModel`), runs both and saves traces
4. `tests/sniffer/compare-traces.py` diffs CLI args, stdin/stdout message types, and control protocol messages

### Coverage check

Detects missing types, hook events, or options compared to the Python SDK:

```bash
python3 scripts/check-coverage.py /path/to/claude-agent-sdk-python .
```

## Sync Workflow

The `.github/workflows/sync.yml` workflow runs daily to keep this SDK in sync with the official Python SDK:

1. Clones `anthropics/claude-agent-sdk-python` at latest
2. Extracts conformance fixtures — checks if they changed
3. Runs unit + conformance tests — checks for failures
4. Runs coverage check — detects type/hook/option gaps
5. If sync needed: invokes Claude Code to fix, then verifies all tests pass
6. Opens a PR with changes

## Rules

- Zero external dependencies — stdlib only
- All message types must match the official Python SDK's `types.py`
- Every new message type needs a conformance test fixture in `tests/conformance/testdata/conformance.json`
- CLI args must match the Python SDK's `subprocess_cli.py` `_build_command()`
- Use idiomatic Go: functional options, `iter.Seq2`, `context.Context`, `json.RawMessage` for flexible fields
- JSON tags use snake_case matching the CLI's NDJSON protocol
- Optional fields use pointer types or omitempty
