package claude

import (
	"errors"
	"fmt"
)

var (
	// ErrCLINotFound is returned when the claude binary is not found in PATH.
	ErrCLINotFound = errors.New("claude: CLI not found in PATH")

	// ErrSessionClosed is returned when sending to a closed session.
	ErrSessionClosed = errors.New("claude: session is closed")

	// ErrEmptyPrompt is returned when an empty prompt is provided.
	ErrEmptyPrompt = errors.New("claude: prompt cannot be empty")
)

// ProcessError is returned when the CLI process exits with a non-zero code.
type ProcessError struct {
	ExitCode int
	Stderr   string
}

func (e *ProcessError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("claude: process exited with code %d: %s", e.ExitCode, e.Stderr)
	}
	return fmt.Sprintf("claude: process exited with code %d", e.ExitCode)
}

// ParseError is returned when an NDJSON line cannot be parsed.
type ParseError struct {
	Line string
	Err  error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("claude: failed to parse message: %v", e.Err)
}

func (e *ParseError) Unwrap() error { return e.Err }
