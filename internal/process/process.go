package process

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

const maxLineSize = 10 * 1024 * 1024 // 10 MB

// Process wraps the Claude Code CLI subprocess.
type Process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	stderr io.ReadCloser

	stdinMu sync.Mutex
	done    chan struct{}
	err     error
	once    sync.Once
}

// Start spawns the claude CLI as a subprocess.
func Start(ctx context.Context, cliPath string, cfg Config) (*Process, error) {
	if cliPath == "" {
		cliPath = "claude"
	}

	path, err := exec.LookPath(cliPath)
	if err != nil {
		return nil, fmt.Errorf("claude: CLI not found at %q: %w", cliPath, err)
	}

	args := BuildArgs(cfg)
	cmd := exec.CommandContext(ctx, path, args...)

	// Inherit env, then overlay.
	cmd.Env = os.Environ()
	if cfg.Cwd != "" {
		cmd.Dir = cfg.Cwd
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("claude: stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("claude: stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("claude: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("claude: start: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	p := &Process{
		cmd:    cmd,
		stdin:  stdin,
		stdout: scanner,
		stderr: stderr,
		done:   make(chan struct{}),
	}

	// Wait for process exit in background.
	go func() {
		p.err = cmd.Wait()
		close(p.done)
	}()

	return p, nil
}

// WriteLine JSON-marshals v and writes it as a single NDJSON line to stdin.
func (p *Process) WriteLine(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("claude: marshal: %w", err)
	}

	p.stdinMu.Lock()
	defer p.stdinMu.Unlock()

	data = append(data, '\n')
	_, err = p.stdin.Write(data)
	return err
}

// ReadLine reads the next NDJSON line from stdout.
// Returns io.EOF when the stdout pipe is closed.
func (p *Process) ReadLine() ([]byte, error) {
	if p.stdout.Scan() {
		// Return a copy so the caller owns the slice.
		line := p.stdout.Bytes()
		cp := make([]byte, len(line))
		copy(cp, line)
		return cp, nil
	}
	if err := p.stdout.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

// Stderr returns the stderr reader for the process.
func (p *Process) Stderr() io.Reader {
	return p.stderr
}

// CloseStdin closes the stdin pipe without waiting for exit.
// Used for one-shot mode: send prompt, close stdin, then read response.
func (p *Process) CloseStdin() {
	p.once.Do(func() {
		p.stdin.Close()
	})
}

// Close closes stdin and waits for the process to exit.
func (p *Process) Close() error {
	p.CloseStdin()
	<-p.done
	return p.err
}

// Kill sends SIGKILL to the process.
func (p *Process) Kill() error {
	if p.cmd.Process != nil {
		return p.cmd.Process.Kill()
	}
	return nil
}

// Done returns a channel that's closed when the process exits.
func (p *Process) Done() <-chan struct{} {
	return p.done
}

// Err returns the process exit error (only valid after Done is closed).
func (p *Process) Err() error {
	return p.err
}
