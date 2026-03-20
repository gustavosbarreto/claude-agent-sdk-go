#!/bin/bash
# Run e2e tests in a clean Docker container.
# This ensures no host settings/permissions leak into tests.
#
# Auth: uses Claude subscription via ~/.claude/.credentials.json
# or ANTHROPIC_API_KEY env var for API key mode.
#
# Usage:
#   ./scripts/test-e2e.sh                                      # subscription mode
#   ANTHROPIC_API_KEY=sk-... ./scripts/test-e2e.sh             # API key mode
#   ./scripts/test-e2e.sh -test.run TestE2E_Prompt             # run specific test

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

# Build test binary for linux/amd64
echo "Building test binary..."
GOOS=linux GOARCH=amd64 go test -c -o e2e.test -tags e2e . 2>/dev/null

# Build Docker image
echo "Building test container..."
docker build -f Dockerfile.test -t claude-sdk-e2e -q .

# Build docker run args
DOCKER_ARGS=(run --rm)

# Mount test binary and test data
DOCKER_ARGS+=(-v "$PROJECT_DIR/e2e.test:/workspace/e2e.test:ro")
DOCKER_ARGS+=(-v "$PROJECT_DIR/testdata:/workspace/testdata:ro")

# Auth: prefer API key env var, fall back to subscription credentials
if [ -n "$ANTHROPIC_API_KEY" ]; then
    echo "Auth: API key"
    DOCKER_ARGS+=(-e ANTHROPIC_API_KEY)
else
    CRED_FILE="$HOME/.claude/.credentials.json"
    if [ ! -f "$CRED_FILE" ]; then
        echo "Error: No ANTHROPIC_API_KEY and no $CRED_FILE found."
        echo "Either set ANTHROPIC_API_KEY or authenticate with 'claude' first."
        exit 1
    fi
    echo "Auth: subscription credentials"
    DOCKER_ARGS+=(-v "$CRED_FILE:/home/claude/.claude/.credentials.json:ro")
fi

# Always set CLAUDE_E2E to enable tests
DOCKER_ARGS+=(-e CLAUDE_E2E=1)

# Image
DOCKER_ARGS+=(claude-sdk-e2e)

# Pass any extra args (e.g. -test.run TestE2E_Prompt)
DOCKER_ARGS+=("$@")

echo "Running e2e tests in container..."
echo ""
docker "${DOCKER_ARGS[@]}"

# Clean up test binary
rm -f e2e.test
