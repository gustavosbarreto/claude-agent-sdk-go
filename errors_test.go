package claude

import (
	"errors"
	"fmt"
	"testing"
)

func TestParseError(t *testing.T) {
	inner := fmt.Errorf("invalid JSON")
	line := `{"broken":}`

	pe := &ParseError{
		Line: line,
		Err:  inner,
	}

	if pe.Line != line {
		t.Errorf("Line = %q, want %q", pe.Line, line)
	}
	if pe.Err != inner {
		t.Errorf("Err = %v, want %v", pe.Err, inner)
	}
}

func TestParseErrorMessage(t *testing.T) {
	inner := fmt.Errorf("unexpected EOF")
	pe := &ParseError{
		Line: `{"truncated`,
		Err:  inner,
	}

	got := pe.Error()
	want := "claude: failed to parse message: unexpected EOF"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestParseErrorUnwrap(t *testing.T) {
	inner := fmt.Errorf("bad token")
	pe := &ParseError{
		Line: `not json`,
		Err:  inner,
	}

	unwrapped := pe.Unwrap()
	if unwrapped != inner {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, inner)
	}

	// Also verify errors.Is works through Unwrap.
	if !errors.Is(pe, inner) {
		t.Error("errors.Is(pe, inner) = false, want true")
	}
}

func TestSessionClosedError(t *testing.T) {
	if ErrSessionClosed == nil {
		t.Fatal("ErrSessionClosed is nil")
	}
	want := "claude: session is closed"
	if got := ErrSessionClosed.Error(); got != want {
		t.Errorf("ErrSessionClosed.Error() = %q, want %q", got, want)
	}
}

func TestEmptyPromptError(t *testing.T) {
	if ErrEmptyPrompt == nil {
		t.Fatal("ErrEmptyPrompt is nil")
	}
	want := "claude: prompt cannot be empty"
	if got := ErrEmptyPrompt.Error(); got != want {
		t.Errorf("ErrEmptyPrompt.Error() = %q, want %q", got, want)
	}
}

func TestProcessError(t *testing.T) {
	t.Run("with stderr", func(t *testing.T) {
		pe := &ProcessError{
			ExitCode: 1,
			Stderr:   "something went wrong",
		}
		want := "claude: process exited with code 1: something went wrong"
		if got := pe.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
		if pe.ExitCode != 1 {
			t.Errorf("ExitCode = %d, want 1", pe.ExitCode)
		}
		if pe.Stderr != "something went wrong" {
			t.Errorf("Stderr = %q, want %q", pe.Stderr, "something went wrong")
		}
	})

	t.Run("without stderr", func(t *testing.T) {
		pe := &ProcessError{
			ExitCode: 2,
			Stderr:   "",
		}
		want := "claude: process exited with code 2"
		if got := pe.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("implements error interface", func(t *testing.T) {
		pe := &ProcessError{ExitCode: 1}
		// Verify ProcessError satisfies the error interface at compile time.
		var _ error = pe
		if pe.Error() == "" {
			t.Error("ProcessError.Error() should not be empty")
		}
	})
}
