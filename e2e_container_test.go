package claude_test

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
// Run: CLAUDE_E2E=1 go test -v -run TestE2E_Container -timeout 10m
func TestE2E_Container(t *testing.T) {
	if os.Getenv("CLAUDE_E2E") == "" {
		t.Skip("set CLAUDE_E2E=1 to run e2e tests")
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
			Context:    ".",
			Dockerfile: "Dockerfile.test",
		},
		Env: map[string]string{
			"CLAUDE_E2E": "1",
		},
		WaitingFor: wait.ForExit().WithExitTimeout(10 * time.Minute),
	}

	// Auth: prefer API key env var, fall back to credentials bind mount.
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		req.Env["ANTHROPIC_API_KEY"] = apiKey
	} else if _, err := os.Stat(credFile); err == nil {
		req.HostConfigModifier = func(hc *container.HostConfig) {
			hc.Binds = append(hc.Binds,
				credFile+":/home/claude/.claude/.credentials.json:ro",
			)
		}
	} else {
		t.Fatal("no ANTHROPIC_API_KEY and no ~/.claude/.credentials.json")
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
