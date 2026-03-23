//go:build ignore

// Claude CLI sniffer — transparent proxy between the SDK and the real claude binary.
// Logs all stdin/stdout NDJSON traffic with zero buffering.
//
// Build: go build -o tests/sniffer/claude-sniffer tests/sniffer/main.go
// Usage: CLAUDE_SNIFFER_DIR=/tmp/traces ./claude-sniffer [claude args...]
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	dir := os.Getenv("CLAUDE_SNIFFER_DIR")
	if dir == "" {
		dir = "/tmp/claude-traces"
	}
	os.MkdirAll(dir, 0o755)

	tag := os.Getenv("CLAUDE_SNIFFER_TAG")
	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	if tag != "" {
		ts = tag + "_" + ts
	}

	// Find real claude binary: prefer CLAUDE_SNIFFER_REAL env, then search PATH.
	realClaude := os.Getenv("CLAUDE_SNIFFER_REAL")
	if realClaude == "" {
		realClaude = findRealClaude()
	}
	if realClaude == "" {
		fmt.Fprintln(os.Stderr, "sniffer: real claude not found (set CLAUDE_SNIFFER_REAL)")
		os.Exit(1)
	}

	// Log args
	os.WriteFile(filepath.Join(dir, ts+"_args.txt"),
		[]byte(realClaude+" "+strings.Join(os.Args[1:], " ")+"\n"), 0o644)

	// Open log files
	stdinLog, _ := os.Create(filepath.Join(dir, ts+"_stdin.ndjson"))
	defer stdinLog.Close()
	stdoutLog, _ := os.Create(filepath.Join(dir, ts+"_stdout.ndjson"))
	defer stdoutLog.Close()
	stderrLog, _ := os.Create(filepath.Join(dir, ts+"_stderr.txt"))
	defer stderrLog.Close()

	// Start real claude
	cmd := exec.Command(realClaude, os.Args[1:]...)

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "sniffer: start: %v\n", err)
		os.Exit(1)
	}

	// Copy stdin: SDK → tee(log) → claude
	go func() {
		w := io.MultiWriter(stdin, stdinLog)
		io.Copy(w, os.Stdin)
		stdin.Close()
	}()

	// Copy stderr: claude → tee(log) → SDK stderr
	go func() {
		w := io.MultiWriter(os.Stderr, stderrLog)
		io.Copy(w, stderr)
	}()

	// Copy stdout: claude → tee(log) → SDK stdout (main goroutine, no buffering)
	w := io.MultiWriter(os.Stdout, stdoutLog)
	io.Copy(w, stdout)

	// Wait for exit
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}

func findRealClaude() string {
	self, _ := os.Executable()
	selfBase := filepath.Base(self)

	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		candidate := filepath.Join(dir, "claude")
		if candidate == self || filepath.Base(candidate) == selfBase {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}
