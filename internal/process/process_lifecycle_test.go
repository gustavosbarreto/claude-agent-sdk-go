package process

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// Process lifecycle tests: verify subprocess cleanup behavior.

func TestCloseCleanTermination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal tests not supported on Windows")
	}

	proc, err := startCat(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Close should shut down cleanly (cat exits on stdin close)
	start := time.Now()
	err = proc.Close()
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Close should succeed for cat, got: %v", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("Close took too long: %v", elapsed)
	}
}

func TestKillTerminatesProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal tests not supported on Windows")
	}

	proc, err := startSleep(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	err = proc.Kill()
	if err != nil {
		t.Errorf("Kill failed: %v", err)
	}

	// Wait for process to exit
	select {
	case <-proc.Done():
		// Good
	case <-time.After(5 * time.Second):
		t.Error("process did not exit after Kill")
	}
}

func TestContextCancellationTerminatesProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal tests not supported on Windows")
	}

	ctx, cancel := context.WithCancel(context.Background())
	proc, err := startSleep(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Cancel context — should terminate process
	cancel()

	select {
	case <-proc.Done():
		// Good — process exited
	case <-time.After(5 * time.Second):
		proc.Kill()
		t.Error("process did not exit after context cancel")
	}
}

func TestCloseStdinThenClose(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal tests not supported on Windows")
	}

	proc, err := startCat(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// CloseStdin first (one-shot pattern)
	proc.CloseStdin()

	// Then Close should complete quickly
	start := time.Now()
	err = proc.Close()
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Close after CloseStdin failed: %v", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("Close took too long after CloseStdin: %v", elapsed)
	}
}

func TestDoubleCloseStdinIsSafe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal tests not supported on Windows")
	}

	proc, err := startCat(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Double CloseStdin should not panic (sync.Once)
	proc.CloseStdin()
	proc.CloseStdin()

	proc.Close()
}

func TestDoneChannelClosesOnExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal tests not supported on Windows")
	}

	proc, err := startCmd(context.Background(), "true")
	if err != nil {
		t.Fatal(err)
	}

	select {
	case <-proc.Done():
		// Good — channel closed when process exited
	case <-time.After(5 * time.Second):
		t.Error("Done channel not closed after process exit")
		proc.Kill()
	}
}

func TestErrAfterExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal tests not supported on Windows")
	}

	proc, err := startCmd(context.Background(), "true")
	if err != nil {
		t.Fatal(err)
	}

	<-proc.Done()
	// Err() should return nil for clean exit
	if proc.Err() != nil {
		t.Errorf("Err() should be nil for clean exit, got: %v", proc.Err())
	}
}

func TestErrAfterFailedExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal tests not supported on Windows")
	}

	proc, err := startCmd(context.Background(), "false")
	if err != nil {
		t.Fatal(err)
	}

	<-proc.Done()
	// Err() should return non-nil for failed exit
	if proc.Err() == nil {
		t.Error("Err() should be non-nil for failed exit")
	}
}

// startCat starts a cat process (exits on stdin close).
func startCat(ctx context.Context) (*Process, error) {
	return startCmd(ctx, "cat")
}

// startCmd starts a simple command as a Process (bypassing BuildArgs).
func startCmd(ctx context.Context, name string, args ...string) (*Process, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, path, args...)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	_ = stdout

	p := &Process{
		cmd:    cmd,
		stdin:  stdin,
		stderr: stderr,
		done:   make(chan struct{}),
	}

	go func() {
		p.err = cmd.Wait()
		close(p.done)
	}()

	return p, nil
}

// startSleep starts a long-running sleep process.
func startSleep(ctx context.Context) (*Process, error) {
	return startCmd(ctx, "sleep", "30")
}
