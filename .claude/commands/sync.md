You are syncing this Go SDK with the official Python SDK at `anthropics/claude-agent-sdk-python`.

## Context

The Python SDK has been cloned to `/tmp/claude-agent-sdk-python`.
New conformance fixtures have been extracted to `testdata/conformance.json`.

## What to do

1. First, run `go test ./...` and check what fails.

2. If all tests pass, report "No changes needed" and stop.

3. If tests fail, analyze the failures:
   - Read the failing test output carefully
   - Compare our Go types with the Python SDK's `src/claude_agent_sdk/types.py`
   - Compare our parser with `src/claude_agent_sdk/_internal/message_parser.py`
   - Compare our CLI args with `src/claude_agent_sdk/_internal/transport/subprocess_cli.py`

4. Implement the fixes:
   - Update message types in `message.go` to match Python SDK
   - Update content blocks in `content.go` if needed
   - Update options/args in `option.go` and `internal/process/args.go`
   - Update hooks in `hook.go` if new events were added
   - Add conformance test assertions in `conformance_test.go` for new fixtures

5. After fixing, run `go test ./...` again to confirm all tests pass.

6. Run `go vet ./...` to ensure code quality.

## Rules

- Zero external dependencies
- Match Python SDK's behavior exactly
- Every new message type needs a conformance test
- Use idiomatic Go patterns
- Don't break existing API — add, don't remove
- JSON tags must match the CLI's NDJSON protocol (snake_case)

## Reference files in the Python SDK (/tmp/claude-agent-sdk-python/)

- `src/claude_agent_sdk/types.py` — all type definitions
- `src/claude_agent_sdk/_internal/message_parser.py` — how messages are parsed
- `src/claude_agent_sdk/_internal/transport/subprocess_cli.py` — how CLI args are built
- `tests/test_message_parser.py` — parsing contract tests
- `tests/test_transport.py` — CLI arg tests
- `CHANGELOG.md` — what changed recently
