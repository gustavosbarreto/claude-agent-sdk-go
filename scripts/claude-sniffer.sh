#!/bin/bash
# Claude CLI sniffer — sits between the SDK and the real claude binary,
# logging all stdin/stdout NDJSON traffic.
#
# Usage:
#   CLAUDE_SNIFFER_DIR=/tmp/traces ./scripts/claude-sniffer.sh [claude args...]
#
# The SDK should use this script as the CLI path:
#   Go:     claude.WithCLIPath("./scripts/claude-sniffer.sh")
#   Python: ClaudeAgentOptions(cli_path="./scripts/claude-sniffer.sh")
#
# Output:
#   $CLAUDE_SNIFFER_DIR/<timestamp>_args.txt    — CLI arguments
#   $CLAUDE_SNIFFER_DIR/<timestamp>_stdin.ndjson — what the SDK sent
#   $CLAUDE_SNIFFER_DIR/<timestamp>_stdout.ndjson — what the CLI returned
#   $CLAUDE_SNIFFER_DIR/<timestamp>_stderr.txt   — stderr

set -e

SNIFFER_DIR="${CLAUDE_SNIFFER_DIR:-/tmp/claude-traces}"
mkdir -p "$SNIFFER_DIR"

# Timestamp for this session
TS=$(date +%s%N)

ARGS_FILE="$SNIFFER_DIR/${TS}_args.txt"
STDIN_FILE="$SNIFFER_DIR/${TS}_stdin.ndjson"
STDOUT_FILE="$SNIFFER_DIR/${TS}_stdout.ndjson"
STDERR_FILE="$SNIFFER_DIR/${TS}_stderr.txt"

# Find the real claude binary (skip ourselves)
REAL_CLAUDE=$(which -a claude | grep -v "claude-sniffer" | head -1)
if [ -z "$REAL_CLAUDE" ]; then
    echo "Error: real claude binary not found in PATH" >&2
    exit 1
fi

# Log args
echo "$REAL_CLAUDE $*" > "$ARGS_FILE"

# Use tee to split stdin/stdout through the real CLI while logging
# stdin: SDK → tee(log) → claude
# stdout: claude → tee(log) → SDK
tee "$STDIN_FILE" | "$REAL_CLAUDE" "$@" 2>"$STDERR_FILE" | tee "$STDOUT_FILE"
