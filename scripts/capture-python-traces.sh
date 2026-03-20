#!/bin/bash
# Capture NDJSON traces from the official Python SDK e2e tests.
#
# Usage:
#   ./scripts/capture-python-traces.sh /path/to/claude-agent-sdk-python
#
# Output: /tmp/claude-traces/python/

set -e

PYTHON_SDK="${1:?Usage: $0 /path/to/claude-agent-sdk-python}"
TRACE_DIR="/tmp/claude-traces/python"
SNIFFER="$(cd "$(dirname "$0")" && pwd)/claude-sniffer.sh"

rm -rf "$TRACE_DIR"
mkdir -p "$TRACE_DIR"

echo "Capturing Python SDK traces..."
echo "  SDK: $PYTHON_SDK"
echo "  Traces: $TRACE_DIR"
echo "  Sniffer: $SNIFFER"
echo ""

cd "$PYTHON_SDK"

# Install SDK in dev mode if needed
pip install -e ".[dev]" -q 2>/dev/null || true

# Run e2e tests with the sniffer as CLI path
CLAUDE_SNIFFER_DIR="$TRACE_DIR" \
  python -m pytest e2e-tests/ -v -m e2e \
  --override-ini="cli_path=$SNIFFER" \
  -x \
  2>&1 | tee "$TRACE_DIR/pytest_output.txt"

echo ""
echo "Traces captured:"
ls -la "$TRACE_DIR"/*.ndjson 2>/dev/null | wc -l
echo " sessions in $TRACE_DIR"
