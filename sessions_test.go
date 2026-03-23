package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Helper to write a .jsonl file with the given NDJSON lines.
func writeSessionFile(t *testing.T, dir, sessionID string, lines []string) string {
	t.Helper()
	path := filepath.Join(dir, sessionID+".jsonl")
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeSessionFile: %v", err)
	}
	return path
}

// Helper to set up a fake home directory with .claude/projects/<subdir>/.
// Returns the subdir path where .jsonl files should be placed.
func setupFakeHome(t *testing.T, subdir string) string {
	t.Helper()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	projectDir := filepath.Join(tmpHome, ".claude", "projects", subdir)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return projectDir
}

func TestListSessions_Empty(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	_ = projectDir // dir exists but has no files

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if sessions != nil {
		t.Errorf("expected nil, got %v", sessions)
	}
}

func TestListSessions_SingleSession(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"Hello Claude"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hi!"}]}}`,
		`{"type":"result","subtype":"success","is_error":false}`,
	}
	writeSessionFile(t, projectDir, "sess-001", lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	s := sessions[0]
	if s.SessionID != "sess-001" {
		t.Errorf("SessionID = %q, want sess-001", s.SessionID)
	}
	if s.LastModified <= 0 {
		t.Errorf("LastModified = %d, want > 0", s.LastModified)
	}
	if s.FileSize <= 0 {
		t.Errorf("FileSize = %d, want > 0", s.FileSize)
	}
}

func TestListSessions_MultipleSessionsSortedByDate(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"msg"}}`,
	}

	// Create 3 sessions with different mod times.
	writeSessionFile(t, projectDir, "oldest", lines)
	// Ensure distinct modification times.
	past := time.Now().Add(-2 * time.Hour)
	os.Chtimes(filepath.Join(projectDir, "oldest.jsonl"), past, past)

	writeSessionFile(t, projectDir, "middle", lines)
	mid := time.Now().Add(-1 * time.Hour)
	os.Chtimes(filepath.Join(projectDir, "middle.jsonl"), mid, mid)

	writeSessionFile(t, projectDir, "newest", lines)
	// newest keeps the current time

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	if sessions[0].SessionID != "newest" {
		t.Errorf("first session = %q, want newest", sessions[0].SessionID)
	}
	if sessions[1].SessionID != "middle" {
		t.Errorf("second session = %q, want middle", sessions[1].SessionID)
	}
	if sessions[2].SessionID != "oldest" {
		t.Errorf("third session = %q, want oldest", sessions[2].SessionID)
	}
}

func TestListSessions_Limit(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{`{"type":"user","message":{"role":"user","content":"x"}}`}

	for i := 0; i < 3; i++ {
		writeSessionFile(t, projectDir, fmt.Sprintf("sess-%d", i), lines)
		ts := time.Now().Add(time.Duration(i) * time.Hour)
		os.Chtimes(filepath.Join(projectDir, fmt.Sprintf("sess-%d.jsonl", i)), ts, ts)
	}

	sessions, err := ListSessions(&ListSessionsOptions{Limit: 2})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestListSessions_Offset(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{`{"type":"user","message":{"role":"user","content":"x"}}`}

	for i := 0; i < 3; i++ {
		writeSessionFile(t, projectDir, fmt.Sprintf("sess-%d", i), lines)
		ts := time.Now().Add(time.Duration(i) * time.Hour)
		os.Chtimes(filepath.Join(projectDir, fmt.Sprintf("sess-%d.jsonl", i)), ts, ts)
	}

	sessions, err := ListSessions(&ListSessionsOptions{Offset: 1})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions (skipped first), got %d", len(sessions))
	}
}

func TestListSessions_OffsetAndLimit(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{`{"type":"user","message":{"role":"user","content":"x"}}`}

	// Create 3 sessions with distinct mod times: sess-0 (oldest), sess-1 (middle), sess-2 (newest).
	for i := 0; i < 3; i++ {
		writeSessionFile(t, projectDir, fmt.Sprintf("sess-%d", i), lines)
		ts := time.Now().Add(time.Duration(i-3) * time.Hour)
		os.Chtimes(filepath.Join(projectDir, fmt.Sprintf("sess-%d.jsonl", i)), ts, ts)
	}

	// Sorted desc: sess-2, sess-1, sess-0. Offset=1, Limit=1 -> sess-1.
	sessions, err := ListSessions(&ListSessionsOptions{Offset: 1, Limit: 1})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].SessionID != "sess-1" {
		t.Errorf("expected sess-1, got %q", sessions[0].SessionID)
	}
}

func TestListSessions_OffsetBeyondEnd(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{`{"type":"user","message":{"role":"user","content":"x"}}`}
	writeSessionFile(t, projectDir, "sess-0", lines)

	sessions, err := ListSessions(&ListSessionsOptions{Offset: 10})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if sessions != nil {
		t.Errorf("expected nil, got %v", sessions)
	}
}

func TestListSessions_ExtractsSummary(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"Hello Claude, what is 2+2?"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"4"}]}}`,
		`{"type":"result","subtype":"success","is_error":false}`,
	}
	writeSessionFile(t, projectDir, "sess-summary", lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	s := sessions[0]
	if s.FirstPrompt != "Hello Claude, what is 2+2?" {
		t.Errorf("FirstPrompt = %q, want %q", s.FirstPrompt, "Hello Claude, what is 2+2?")
	}
	if s.Summary != "Hello Claude, what is 2+2?" {
		t.Errorf("Summary = %q, want %q", s.Summary, "Hello Claude, what is 2+2?")
	}
}

func TestListSessions_IgnoresNonJsonl(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")

	// Write a .jsonl file and a .txt file.
	lines := []string{`{"type":"user","message":{"role":"user","content":"x"}}`}
	writeSessionFile(t, projectDir, "real-session", lines)
	os.WriteFile(filepath.Join(projectDir, "notes.txt"), []byte("not a session"), 0o644)
	os.WriteFile(filepath.Join(projectDir, "data.json"), []byte("{}"), 0o644)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].SessionID != "real-session" {
		t.Errorf("SessionID = %q, want real-session", sessions[0].SessionID)
	}
}

func TestGetSessionMessages_Basic(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"Hello Claude"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hi!"}]}}`,
		`{"type":"result","subtype":"success","is_error":false}`,
	}
	writeSessionFile(t, projectDir, "sess-msg", lines)

	messages, err := GetSessionMessages("sess-msg", nil)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages (user+assistant), got %d", len(messages))
	}
	if messages[0].Type != "user" {
		t.Errorf("first message type = %q, want user", messages[0].Type)
	}
	if messages[1].Type != "assistant" {
		t.Errorf("second message type = %q, want assistant", messages[1].Type)
	}
}

func TestGetSessionMessages_FiltersSystemMessages(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"system","subtype":"init","session_id":"sess-filter"}`,
		`{"type":"user","message":{"role":"user","content":"Hello"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hi"}]}}`,
		`{"type":"result","subtype":"success","is_error":false}`,
	}
	writeSessionFile(t, projectDir, "sess-filter", lines)

	messages, err := GetSessionMessages("sess-filter", nil)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	// Only user and assistant should be included; system and result are filtered.
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	for _, m := range messages {
		if m.Type != "user" && m.Type != "assistant" {
			t.Errorf("unexpected message type: %q", m.Type)
		}
	}
}

func TestGetSessionMessages_LimitOffset(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"msg1"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"resp1"}]}}`,
		`{"type":"user","message":{"role":"user","content":"msg2"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"resp2"}]}}`,
	}
	writeSessionFile(t, projectDir, "sess-lo", lines)

	// Offset=1, Limit=2: skip first user msg, take the next 2.
	messages, err := GetSessionMessages("sess-lo", &GetSessionMessagesOptions{
		Offset: 1,
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Type != "assistant" {
		t.Errorf("first message type = %q, want assistant", messages[0].Type)
	}
	if messages[1].Type != "user" {
		t.Errorf("second message type = %q, want user", messages[1].Type)
	}
}

func TestGetSessionMessages_NotFound(t *testing.T) {
	setupFakeHome(t, "testproject")

	_, err := GetSessionMessages("nonexistent-session", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want 'not found'", err)
	}
}

func TestGetSessionInfo_Basic(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"What is Go?"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"A programming language."}]}}`,
	}
	writeSessionFile(t, projectDir, "sess-info", lines)

	info, err := GetSessionInfo("sess-info", "")
	if err != nil {
		t.Fatalf("GetSessionInfo: %v", err)
	}
	if info.SessionID != "sess-info" {
		t.Errorf("SessionID = %q, want sess-info", info.SessionID)
	}
	if info.LastModified <= 0 {
		t.Errorf("LastModified = %d, want > 0", info.LastModified)
	}
	if info.FileSize <= 0 {
		t.Errorf("FileSize = %d, want > 0", info.FileSize)
	}
	if info.FirstPrompt != "What is Go?" {
		t.Errorf("FirstPrompt = %q, want %q", info.FirstPrompt, "What is Go?")
	}
	if info.Summary != "What is Go?" {
		t.Errorf("Summary = %q, want %q", info.Summary, "What is Go?")
	}
}

func TestGetSessionInfo_NotFound(t *testing.T) {
	setupFakeHome(t, "testproject")

	_, err := GetSessionInfo("nonexistent-session", "")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want 'not found'", err)
	}
}

func TestExtractSessionSummary_LongPrompt(t *testing.T) {
	dir := t.TempDir()
	longPrompt := strings.Repeat("a", 300)
	lines := []string{
		fmt.Sprintf(`{"type":"user","message":{"role":"user","content":"%s"}}`, longPrompt),
	}
	path := filepath.Join(dir, "long.jsonl")
	os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)

	summary, firstPrompt := extractSessionSummary(path)

	if len(firstPrompt) != 200 {
		t.Errorf("firstPrompt length = %d, want 200", len(firstPrompt))
	}
	if firstPrompt != longPrompt[:200] {
		t.Errorf("firstPrompt not truncated correctly")
	}
	if summary != firstPrompt {
		t.Errorf("summary should equal truncated firstPrompt")
	}
}

func TestExtractSessionSummary_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	os.WriteFile(path, []byte(""), 0o644)

	summary, firstPrompt := extractSessionSummary(path)

	if summary != "" {
		t.Errorf("summary = %q, want empty", summary)
	}
	if firstPrompt != "" {
		t.Errorf("firstPrompt = %q, want empty", firstPrompt)
	}
}

func TestSessionDirForProject(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	result := sessionDirForProject("/home/user/my-project")

	// Non-alphanumeric characters should be replaced with dashes.
	expected := filepath.Join(tmpHome, ".claude", "projects", "-home-user-my-project")
	if result != expected {
		t.Errorf("sessionDirForProject = %q, want %q", result, expected)
	}
}
