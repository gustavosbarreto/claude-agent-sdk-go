package e2e_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestE2E_Container builds the test image and runs all e2e tests inside
// a clean Docker container with no host settings or permissions.
//
// Run: go test ./tests/e2e/ -v -count=1 -run TestE2E_Container -timeout 10m
func TestE2E_Container(t *testing.T) {
	if os.Getenv("INSIDE_CONTAINER") != "" {
		t.Skip("already inside container")
	}
	if testing.Short() {
		t.Skip("container e2e skipped in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}
	credFile := filepath.Join(home, ".claude", ".credentials.json")

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       "../..",
			Dockerfile:    "tests/e2e/Dockerfile",
			PrintBuildLog: true,
			KeepImage:     true,
		},
		Env: map[string]string{},
		WaitingFor: wait.ForExit().WithExitTimeout(10 * time.Minute),
	}

	// Auth: prefer env vars (for CI), fall back to credentials bind mount (local).
	if oauthToken := os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"); oauthToken != "" {
		req.Env["CLAUDE_CODE_OAUTH_TOKEN"] = oauthToken
	} else if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		req.Env["ANTHROPIC_API_KEY"] = apiKey
	} else if _, err := os.Stat(credFile); err == nil {
		req.HostConfigModifier = func(hc *container.HostConfig) {
			hc.Binds = append(hc.Binds,
				credFile+":/home/claude/.claude/.credentials.json:ro",
			)
		}
	} else {
		t.Fatal("no CLAUDE_CODE_OAUTH_TOKEN, ANTHROPIC_API_KEY, or ~/.claude/.credentials.json")
	}

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start container: %v", err)
	}
	defer func() {
		_ = ctr.Terminate(ctx)
	}()

	// Container has exited (WaitForExit). Read all logs.
	logs, err := ctr.Logs(ctx)
	if err != nil {
		t.Fatalf("container logs: %v", err)
	}
	defer logs.Close()

	output, err := io.ReadAll(logs)
	if err != nil {
		t.Fatalf("read logs: %v", err)
	}
	t.Logf("\n%s", string(output))

	state, err := ctr.State(ctx)
	if err != nil {
		t.Fatalf("container state: %v", err)
	}

	if state.ExitCode != 0 {
		t.Fatalf("e2e tests failed in container (exit code %d)", state.ExitCode)
	}
}
