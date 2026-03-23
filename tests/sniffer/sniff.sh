#!/bin/bash
# Sniff and compare protocol traces per test case.
#
# For each test pair (Python + Go equivalent), runs both in containers
# with the sniffer, then compares the traces.
#
# Usage: ./tests/sniffer/sniff.sh
#
# Requires: Docker, ~/.claude/.credentials.json or ANTHROPIC_API_KEY

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
TRACE_DIR="/tmp/claude-traces"
CRED_FILE="$HOME/.claude/.credentials.json"

cd "$PROJECT_DIR"

# --- Build ---
echo "Building sniffer..."
GOOS=linux GOARCH=amd64 go build -o tests/sniffer/claude-sniffer tests/sniffer/main.go

echo "Building containers..."
docker build -f tests/e2e/Dockerfile.sniffer -t claude-sniff-python -q .
docker build -f tests/e2e/Dockerfile -t claude-sniff-go -q .

# --- Auth ---
DOCKER_AUTH=()
if [ -n "$ANTHROPIC_API_KEY" ]; then
    DOCKER_AUTH+=(-e ANTHROPIC_API_KEY)
elif [ -n "$CLAUDE_CODE_OAUTH_TOKEN" ]; then
    DOCKER_AUTH+=(-e CLAUDE_CODE_OAUTH_TOKEN)
elif [ -f "$CRED_FILE" ]; then
    DOCKER_AUTH+=(-v "$CRED_FILE:/home/claude/.claude/.credentials.json:ro")
else
    echo "Error: no auth"; exit 1
fi

# --- Test pairs: Python test → Go test ---
declare -A TEST_PAIRS
TEST_PAIRS[set_model]="e2e-tests/test_dynamic_control.py::test_set_model|TestE2E_Session_SetModel"
TEST_PAIRS[hook_deny]="e2e-tests/test_hooks.py::test_hook_with_permission_decision_and_reason|TestE2E_Hook_PermissionDeny"

run_python() {
    local tag=$1 test=$2
    local dir="$TRACE_DIR/$tag/python"
    rm -rf "$dir"; mkdir -p "$dir"

    docker run --rm \
        "${DOCKER_AUTH[@]}" \
        -v "$dir:/traces" \
        -e CLAUDE_SNIFFER_TAG="$tag" \
        claude-sniff-python \
        python -m pytest "$test" -v -m e2e \
        2>&1 | tail -5
}

run_go() {
    local tag=$1 test=$2
    local dir="$TRACE_DIR/$tag/go"
    rm -rf "$dir"; mkdir -p "$dir"

    docker run --rm \
        "${DOCKER_AUTH[@]}" \
        -v "$dir:/traces" \
        -v "$PROJECT_DIR/tests/sniffer/claude-sniffer:/tmp/sniffer:ro" \
        -e CLAUDE_SNIFFER_TAG="$tag" \
        --user root \
        --entrypoint sh \
        claude-sniff-go \
        -c ' \
            cp /usr/local/bin/claude /usr/local/bin/claude-real && \
            cp /tmp/sniffer /usr/local/bin/claude && \
            chown claude:claude /traces && \
            su -s /bin/sh claude -c " \
                CLAUDE_SNIFFER_DIR=/traces \
                CLAUDE_SNIFFER_REAL=/usr/local/bin/claude-real \
                CLAUDE_SNIFFER_TAG='"$tag"' \
                CLAUDE_E2E=1 \
                INSIDE_CONTAINER=1 \
                /workspace/e2e.test \
                    -test.v \
                    -test.run '"$test"' \
                    -test.timeout 3m \
            "' \
        2>&1 | tail -5
}

# --- Run each pair ---
for tag in "${!TEST_PAIRS[@]}"; do
    IFS='|' read -r py_test go_test <<< "${TEST_PAIRS[$tag]}"

    echo ""
    echo "========================================"
    echo "  $tag"
    echo "========================================"

    echo "--- Python: $py_test ---"
    run_python "$tag" "$py_test"

    echo "--- Go: $go_test ---"
    run_go "$tag" "$go_test"

    echo "--- Compare ---"
    python3 tests/sniffer/compare-traces.py "$TRACE_DIR/$tag/python" "$TRACE_DIR/$tag/go"
done

# Cleanup
rm -f tests/sniffer/claude-sniffer
