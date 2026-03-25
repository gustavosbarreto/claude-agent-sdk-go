package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testSessionID = "a1b2c3d4-e5f6-7890-abcd-ef1234567890"

func setupMutationSession(t *testing.T) (projectDir string) {
	t.Helper()
	projectDir = setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"Hello Claude"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hi!"}]}}`,
		`{"type":"result","subtype":"success","is_error":false}`,
	}
	writeSessionFile(t, projectDir, testSessionID, lines)
	return projectDir
}

func readFileLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFileLines: %v", err)
	}
	raw := strings.TrimSuffix(string(data), "\n")
	return strings.Split(raw, "\n")
}

// --- RenameSession tests ---

func TestRenameSessionBasic(t *testing.T) {
	projectDir := setupMutationSession(t)

	err := RenameSession(testSessionID, "My Custom Title", "")
	if err != nil {
		t.Fatalf("RenameSession: %v", err)
	}

	path := filepath.Join(projectDir, testSessionID+".jsonl")
	lines := readFileLines(t, path)
	last := lines[len(lines)-1]
	if !strings.Contains(last, `"type":"custom-title"`) {
		t.Errorf("last line should contain custom-title type, got: %s", last)
	}
	if !strings.Contains(last, `"customTitle":"My Custom Title"`) {
		t.Errorf("last line should contain title, got: %s", last)
	}
	if !strings.Contains(last, `"sessionId":"`+testSessionID+`"`) {
		t.Errorf("last line should contain sessionId, got: %s", last)
	}
}

func TestRenameSessionMultiple(t *testing.T) {
	projectDir := setupMutationSession(t)

	if err := RenameSession(testSessionID, "First Title", ""); err != nil {
		t.Fatalf("RenameSession first: %v", err)
	}
	if err := RenameSession(testSessionID, "Second Title", ""); err != nil {
		t.Fatalf("RenameSession second: %v", err)
	}

	path := filepath.Join(projectDir, testSessionID+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if strings.Count(content, `"type":"custom-title"`) != 2 {
		t.Errorf("expected 2 custom-title entries, got content:\n%s", content)
	}
	if !strings.Contains(content, `"customTitle":"First Title"`) {
		t.Error("missing First Title entry")
	}
	if !strings.Contains(content, `"customTitle":"Second Title"`) {
		t.Error("missing Second Title entry")
	}
}

func TestRenameSessionInvalidUUID(t *testing.T) {
	setupMutationSession(t)

	err := RenameSession("not-a-uuid", "Title", "")
	if err == nil {
		t.Fatal("expected error for invalid UUID")
	}
	if !strings.Contains(err.Error(), "invalid session ID") {
		t.Errorf("error = %v, want 'invalid session ID'", err)
	}
}

func TestRenameSessionEmptyTitle(t *testing.T) {
	setupMutationSession(t)

	for _, title := range []string{"", "   ", "\t\n"} {
		err := RenameSession(testSessionID, title, "")
		if err == nil {
			t.Errorf("expected error for empty/whitespace title %q", title)
		}
		if err != nil && !strings.Contains(err.Error(), "title must not be empty") {
			t.Errorf("error = %v, want 'title must not be empty'", err)
		}
	}
}

func TestRenameSessionNotFound(t *testing.T) {
	setupFakeHome(t, "testproject")

	err := RenameSession("00000000-0000-0000-0000-000000000000", "Title", "")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want 'not found'", err)
	}
}

func TestRenameSessionTrimWhitespace(t *testing.T) {
	projectDir := setupMutationSession(t)

	err := RenameSession(testSessionID, "  Trimmed Title  ", "")
	if err != nil {
		t.Fatalf("RenameSession: %v", err)
	}

	path := filepath.Join(projectDir, testSessionID+".jsonl")
	lines := readFileLines(t, path)
	last := lines[len(lines)-1]
	if !strings.Contains(last, `"customTitle":"Trimmed Title"`) {
		t.Errorf("title should be trimmed, got: %s", last)
	}
}

// --- TagSession tests ---

func TestTagSessionBasic(t *testing.T) {
	projectDir := setupMutationSession(t)

	tag := "important"
	err := TagSession(testSessionID, &tag, "")
	if err != nil {
		t.Fatalf("TagSession: %v", err)
	}

	path := filepath.Join(projectDir, testSessionID+".jsonl")
	lines := readFileLines(t, path)
	last := lines[len(lines)-1]
	if !strings.Contains(last, `"type":"tag"`) {
		t.Errorf("last line should contain tag type, got: %s", last)
	}
	if !strings.Contains(last, `"tag":"important"`) {
		t.Errorf("last line should contain tag value, got: %s", last)
	}
	if !strings.Contains(last, `"sessionId":"`+testSessionID+`"`) {
		t.Errorf("last line should contain sessionId, got: %s", last)
	}
}

func TestTagSessionClearTag(t *testing.T) {
	projectDir := setupMutationSession(t)

	err := TagSession(testSessionID, nil, "")
	if err != nil {
		t.Fatalf("TagSession: %v", err)
	}

	path := filepath.Join(projectDir, testSessionID+".jsonl")
	lines := readFileLines(t, path)
	last := lines[len(lines)-1]
	if !strings.Contains(last, `"type":"tag"`) {
		t.Errorf("last line should contain tag type, got: %s", last)
	}
	if !strings.Contains(last, `"tag":""`) {
		t.Errorf("last line should contain empty tag for clear, got: %s", last)
	}
}

func TestTagSessionInvalidUUID(t *testing.T) {
	setupMutationSession(t)

	tag := "test"
	err := TagSession("bad-uuid", &tag, "")
	if err == nil {
		t.Fatal("expected error for invalid UUID")
	}
	if !strings.Contains(err.Error(), "invalid session ID") {
		t.Errorf("error = %v, want 'invalid session ID'", err)
	}
}

func TestTagSessionEmptyTag(t *testing.T) {
	setupMutationSession(t)

	tag := ""
	err := TagSession(testSessionID, &tag, "")
	if err == nil {
		t.Fatal("expected error for empty tag")
	}
	if !strings.Contains(err.Error(), "tag must not be empty") {
		t.Errorf("error = %v, want 'tag must not be empty'", err)
	}
}

func TestTagSessionUnicode(t *testing.T) {
	projectDir := setupMutationSession(t)

	// Tag with zero-width characters embedded.
	tag := "hello\u200Bworld"
	err := TagSession(testSessionID, &tag, "")
	if err != nil {
		t.Fatalf("TagSession: %v", err)
	}

	path := filepath.Join(projectDir, testSessionID+".jsonl")
	lines := readFileLines(t, path)
	last := lines[len(lines)-1]
	// Zero-width space should be stripped.
	if !strings.Contains(last, `"tag":"helloworld"`) {
		t.Errorf("tag should have zero-width chars removed, got: %s", last)
	}
}

// --- DeleteSession tests ---

func TestDeleteSessionBasic(t *testing.T) {
	projectDir := setupMutationSession(t)

	path := filepath.Join(projectDir, testSessionID+".jsonl")
	// Verify file exists before deletion.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session file should exist before delete: %v", err)
	}

	err := DeleteSession(testSessionID, "")
	if err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	// Verify file is gone.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("session file should be deleted, stat err = %v", err)
	}
}

func TestDeleteSessionNotFound(t *testing.T) {
	setupFakeHome(t, "testproject")

	err := DeleteSession("00000000-0000-0000-0000-000000000000", "")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want 'not found'", err)
	}
}

func TestDeleteSessionInvalidUUID(t *testing.T) {
	setupMutationSession(t)

	err := DeleteSession("not-valid", "")
	if err == nil {
		t.Fatal("expected error for invalid UUID")
	}
	if !strings.Contains(err.Error(), "invalid session ID") {
		t.Errorf("error = %v, want 'invalid session ID'", err)
	}
}

// --- sanitizeUnicode tests ---

func TestSanitizeUnicodeBasic(t *testing.T) {
	input := "hello world"
	got := sanitizeUnicode(input)
	if got != input {
		t.Errorf("sanitizeUnicode(%q) = %q, want %q", input, got, input)
	}
}

func TestSanitizeUnicodeZeroWidth(t *testing.T) {
	input := "hello\u200Bworld\u200Ctest\u200Dfoo"
	got := sanitizeUnicode(input)
	want := "helloworldtestfoo"
	if got != want {
		t.Errorf("sanitizeUnicode(%q) = %q, want %q", input, got, want)
	}
}

func TestSanitizeUnicodeDirectionalMarks(t *testing.T) {
	input := "hello\u202Aworld\u202Btest\u202Cfoo\u202Dbar\u202Ebaz"
	got := sanitizeUnicode(input)
	want := "helloworldtestfoobarbaz"
	if got != want {
		t.Errorf("sanitizeUnicode(%q) = %q, want %q", input, got, want)
	}
}

func TestSanitizeUnicodePrivateUse(t *testing.T) {
	input := "hello\uE000world\uF8FFtest"
	got := sanitizeUnicode(input)
	want := "helloworldtest"
	if got != want {
		t.Errorf("sanitizeUnicode(%q) = %q, want %q", input, got, want)
	}
}
