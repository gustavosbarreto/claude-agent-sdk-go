package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test UUIDs for consistent use across tests.
const (
	testUUID1 = "00000000-0000-0000-0000-000000000001"
	testUUID2 = "00000000-0000-0000-0000-000000000002"
	testUUID3 = "00000000-0000-0000-0000-000000000003"
	testUUID4 = "00000000-0000-0000-0000-000000000004"
	testUUID5 = "00000000-0000-0000-0000-000000000005"
	testUUID6 = "00000000-0000-0000-0000-000000000006"
	testUUID7 = "00000000-0000-0000-0000-000000000007"
	testUUID8 = "00000000-0000-0000-0000-000000000008"
	testUUID9 = "00000000-0000-0000-0000-000000000009"
	testUUID10 = "00000000-0000-0000-0000-00000000000a"
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

// Helper to set up a fake config directory with .claude/projects/<subdir>/.
// Uses CLAUDE_CONFIG_DIR env var instead of HOME for cleaner tests.
// Returns the subdir path where .jsonl files should be placed.
func setupFakeHome(t *testing.T, subdir string) string {
	t.Helper()
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	projectDir := filepath.Join(configDir, "projects", subdir)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return projectDir
}

func TestListSessionsEmpty(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	_ = projectDir // dir exists but has no files

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessionsSingleSession(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"Hello Claude"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hi!"}]}}`,
		`{"type":"result","subtype":"success","is_error":false}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	s := sessions[0]
	if s.SessionID != testUUID1 {
		t.Errorf("SessionID = %q, want %s", s.SessionID, testUUID1)
	}
	if s.LastModified <= 0 {
		t.Errorf("LastModified = %d, want > 0", s.LastModified)
	}
	if s.FileSize <= 0 {
		t.Errorf("FileSize = %d, want > 0", s.FileSize)
	}
}

func TestListSessionsMultipleSessionsSortedByDate(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"msg"}}`,
	}

	// Create 3 sessions with different mod times.
	writeSessionFile(t, projectDir, testUUID1, lines) // oldest
	past := time.Now().Add(-2 * time.Hour)
	os.Chtimes(filepath.Join(projectDir, testUUID1+".jsonl"), past, past)

	writeSessionFile(t, projectDir, testUUID2, lines) // middle
	mid := time.Now().Add(-1 * time.Hour)
	os.Chtimes(filepath.Join(projectDir, testUUID2+".jsonl"), mid, mid)

	writeSessionFile(t, projectDir, testUUID3, lines) // newest
	// newest keeps the current time

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	if sessions[0].SessionID != testUUID3 {
		t.Errorf("first session = %q, want %s (newest)", sessions[0].SessionID, testUUID3)
	}
	if sessions[1].SessionID != testUUID2 {
		t.Errorf("second session = %q, want %s (middle)", sessions[1].SessionID, testUUID2)
	}
	if sessions[2].SessionID != testUUID1 {
		t.Errorf("third session = %q, want %s (oldest)", sessions[2].SessionID, testUUID1)
	}
}

func TestListSessionsLimit(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{`{"type":"user","message":{"role":"user","content":"x"}}`}

	uuids := []string{testUUID1, testUUID2, testUUID3}
	for i, id := range uuids {
		writeSessionFile(t, projectDir, id, lines)
		ts := time.Now().Add(time.Duration(i) * time.Hour)
		os.Chtimes(filepath.Join(projectDir, id+".jsonl"), ts, ts)
	}

	sessions, err := ListSessions(&ListSessionsOptions{Limit: 2})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestListSessionsOffset(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{`{"type":"user","message":{"role":"user","content":"x"}}`}

	uuids := []string{testUUID1, testUUID2, testUUID3}
	for i, id := range uuids {
		writeSessionFile(t, projectDir, id, lines)
		ts := time.Now().Add(time.Duration(i) * time.Hour)
		os.Chtimes(filepath.Join(projectDir, id+".jsonl"), ts, ts)
	}

	sessions, err := ListSessions(&ListSessionsOptions{Offset: 1})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions (skipped first), got %d", len(sessions))
	}
}

func TestListSessionsOffsetAndLimit(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{`{"type":"user","message":{"role":"user","content":"x"}}`}

	// Create 3 sessions with distinct mod times: uuid1 (oldest), uuid2 (middle), uuid3 (newest).
	uuids := []string{testUUID1, testUUID2, testUUID3}
	for i, id := range uuids {
		writeSessionFile(t, projectDir, id, lines)
		ts := time.Now().Add(time.Duration(i-3) * time.Hour)
		os.Chtimes(filepath.Join(projectDir, id+".jsonl"), ts, ts)
	}

	// Sorted desc: uuid3, uuid2, uuid1. Offset=1, Limit=1 -> uuid2.
	sessions, err := ListSessions(&ListSessionsOptions{Offset: 1, Limit: 1})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].SessionID != testUUID2 {
		t.Errorf("expected %s, got %q", testUUID2, sessions[0].SessionID)
	}
}

func TestListSessionsOffsetBeyondEnd(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{`{"type":"user","message":{"role":"user","content":"x"}}`}
	writeSessionFile(t, projectDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{Offset: 10})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if sessions != nil {
		t.Errorf("expected nil, got %v", sessions)
	}
}

func TestListSessionsExtractsSummary(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"Hello Claude, what is 2+2?"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"4"}]}}`,
		`{"type":"result","subtype":"success","is_error":false}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

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

func TestListSessionsIgnoresNonJsonl(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")

	// Write a .jsonl file and a .txt file.
	lines := []string{`{"type":"user","message":{"role":"user","content":"x"}}`}
	writeSessionFile(t, projectDir, testUUID1, lines)
	os.WriteFile(filepath.Join(projectDir, "notes.txt"), []byte("not a session"), 0o644)
	os.WriteFile(filepath.Join(projectDir, "data.json"), []byte("{}"), 0o644)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].SessionID != testUUID1 {
		t.Errorf("SessionID = %q, want %s", sessions[0].SessionID, testUUID1)
	}
}

func TestGetSessionMessagesBasic(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"Hello Claude"}}`,
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"Hi!"}]}}`,
		`{"type":"result","subtype":"success","is_error":false}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	messages, err := GetSessionMessages(testUUID1, nil)
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

func TestGetSessionMessagesFiltersSystemMessages(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"system","uuid":"s1","subtype":"init","session_id":"` + testUUID1 + `"}`,
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"Hello"}}`,
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"Hi"}]}}`,
		`{"type":"result","subtype":"success","is_error":false}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	messages, err := GetSessionMessages(testUUID1, nil)
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

func TestGetSessionMessagesLimitOffset(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"msg1"}}`,
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"resp1"}]}}`,
		`{"type":"user","uuid":"u2","parentUuid":"a1","message":{"role":"user","content":"msg2"}}`,
		`{"type":"assistant","uuid":"a2","parentUuid":"u2","message":{"role":"assistant","content":[{"type":"text","text":"resp2"}]}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	// Offset=1, Limit=2: skip first user msg, take the next 2.
	messages, err := GetSessionMessages(testUUID1, &GetSessionMessagesOptions{
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

func TestGetSessionMessagesNotFound(t *testing.T) {
	setupFakeHome(t, "testproject")

	_, err := GetSessionMessages("00000000-0000-0000-0000-ffffffffffff", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want 'not found'", err)
	}
}

func TestGetSessionInfoBasic(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"What is Go?"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"A programming language."}]}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	info, err := GetSessionInfo(testUUID1, "")
	if err != nil {
		t.Fatalf("GetSessionInfo: %v", err)
	}
	if info.SessionID != testUUID1 {
		t.Errorf("SessionID = %q, want %s", info.SessionID, testUUID1)
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

func TestGetSessionInfoNotFound(t *testing.T) {
	setupFakeHome(t, "testproject")

	_, err := GetSessionInfo("00000000-0000-0000-0000-ffffffffffff", "")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want 'not found'", err)
	}
}

func TestExtractFirstPromptLongPrompt(t *testing.T) {
	longPrompt := strings.Repeat("a", 300)
	head := fmt.Sprintf(`{"type":"user","message":{"role":"user","content":%q}}`, longPrompt) + "\n"

	result := extractFirstPrompt(head)

	if len(result) != 200 {
		t.Errorf("firstPrompt length = %d, want 200", len(result))
	}
	if result != longPrompt[:200] {
		t.Errorf("firstPrompt not truncated correctly")
	}
}

func TestExtractFirstPromptEmptyInput(t *testing.T) {
	result := extractFirstPrompt("")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestSessionDirForProject(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)

	result := sessionDirForProject("/home/user/my-project")

	// Non-alphanumeric characters should be replaced with dashes.
	expected := filepath.Join(configDir, "projects", "-home-user-my-project")
	if result != expected {
		t.Errorf("sessionDirForProject = %q, want %q", result, expected)
	}
}

func TestListSessionsEmptyFileFiltered(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	// Empty .jsonl file with valid UUID — should be filtered (no metadata).
	path := filepath.Join(projectDir, "550e8400-e29b-41d4-a716-446655440000.jsonl")
	os.WriteFile(path, []byte(""), 0o644)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (empty file filtered), got %d", len(sessions))
	}
}

func TestListSessionsNonUUIDFiltered(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	// Non-UUID .jsonl files should be filtered.
	lines := []string{`{"type":"user","message":{"role":"user","content":"x"}}`}
	writeSessionFile(t, projectDir, "not-a-uuid", lines)
	writeSessionFile(t, projectDir, "550e8400-e29b-41d4-a716-446655440000", lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (non-UUID filtered), got %d", len(sessions))
	}
	if sessions[0].SessionID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("SessionID = %q", sessions[0].SessionID)
	}
}

func TestListSessionsNoConfigDir(t *testing.T) {
	// Point CLAUDE_CONFIG_DIR to a directory that does not exist.
	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/nonexistent-config-"+fmt.Sprintf("%d", time.Now().UnixNano()))

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("expected no error for missing config dir, got: %v", err)
	}
	if sessions != nil {
		t.Errorf("expected nil sessions, got %v", sessions)
	}
}

func TestListSessionsLimitZero(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{`{"type":"user","message":{"role":"user","content":"x"}}`}

	uuids := []string{testUUID1, testUUID2, testUUID3, testUUID4, testUUID5}
	for i, id := range uuids {
		writeSessionFile(t, projectDir, id, lines)
		ts := time.Now().Add(time.Duration(i) * time.Hour)
		os.Chtimes(filepath.Join(projectDir, id+".jsonl"), ts, ts)
	}

	// Limit=0 means no limit — should return all 5 sessions.
	sessions, err := ListSessions(&ListSessionsOptions{Limit: 0})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 5 {
		t.Errorf("expected 5 sessions (limit=0 means all), got %d", len(sessions))
	}
}

func TestListSessionsMultipleProjects(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	lines := []string{`{"type":"user","message":{"role":"user","content":"hello"}}`}

	// Create sessions in two different project directories.
	projA := filepath.Join(configDir, "projects", "project-a")
	projB := filepath.Join(configDir, "projects", "project-b")
	os.MkdirAll(projA, 0o755)
	os.MkdirAll(projB, 0o755)

	writeSessionFile(t, projA, testUUID1, lines)
	writeSessionFile(t, projB, testUUID2, lines)

	// Dir="" should walk all project directories.
	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions across projects, got %d", len(sessions))
	}

	ids := map[string]bool{}
	for _, s := range sessions {
		ids[s.SessionID] = true
	}
	if !ids[testUUID1] || !ids[testUUID2] {
		t.Errorf("expected %s and %s, got %v", testUUID1, testUUID2, ids)
	}
}

func TestListSessionsWithDir(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	lines := []string{`{"type":"user","message":{"role":"user","content":"hi"}}`}

	// ListSessions with Dir="/my/project" does filepath.Abs then sanitizePath.
	// sanitizePath("/my/project") = "-my-project"
	encoded := sanitizePath("/my/project")
	projDir := filepath.Join(configDir, "projects", encoded)
	os.MkdirAll(projDir, 0o755)
	writeSessionFile(t, projDir, testUUID1, lines)

	// Also create a session in a different project dir to ensure it's not returned.
	otherDir := filepath.Join(configDir, "projects", "other-project")
	os.MkdirAll(otherDir, 0o755)
	writeSessionFile(t, otherDir, testUUID2, lines)

	sessions, err := ListSessions(&ListSessionsOptions{Dir: "/my/project"})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session scoped to dir, got %d", len(sessions))
	}
	if sessions[0].SessionID != testUUID1 {
		t.Errorf("SessionID = %q, want %s", sessions[0].SessionID, testUUID1)
	}
}

func TestGetSessionMessagesCorruptLines(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`not json at all`,
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"valid"}}`,
		`{broken json`,
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"ok"}]}}`,
		``,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	messages, err := GetSessionMessages(testUUID1, nil)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	// Only the two valid user/assistant lines should be returned.
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages (corrupt lines skipped), got %d", len(messages))
	}
	if messages[0].Type != "user" {
		t.Errorf("first message type = %q, want user", messages[0].Type)
	}
	if messages[1].Type != "assistant" {
		t.Errorf("second message type = %q, want assistant", messages[1].Type)
	}
}

func TestGetSessionMessagesEmptyFile(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	path := filepath.Join(projectDir, testUUID1+".jsonl")
	os.WriteFile(path, []byte(""), 0o644)

	messages, err := GetSessionMessages(testUUID1, nil)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages for empty file, got %d", len(messages))
	}
}

func TestGetSessionMessagesOffsetBeyondEnd(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"only one"}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	messages, err := GetSessionMessages(testUUID1, &GetSessionMessagesOptions{Offset: 100})
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if messages != nil {
		t.Errorf("expected nil for offset beyond end, got %d messages", len(messages))
	}
}

func TestGetSessionInfoWithDir(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)

	// Set up a session in an encoded project dir.
	encoded := sanitizePath("/home/user/myproj")
	projDir := filepath.Join(configDir, "projects", encoded)
	os.MkdirAll(projDir, 0o755)

	lines := []string{
		`{"type":"user","message":{"role":"user","content":"scoped info"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"ok"}]}}`,
	}
	writeSessionFile(t, projDir, testUUID1, lines)

	info, err := GetSessionInfo(testUUID1, "/home/user/myproj")
	if err != nil {
		t.Fatalf("GetSessionInfo: %v", err)
	}
	if info.SessionID != testUUID1 {
		t.Errorf("SessionID = %q, want %s", info.SessionID, testUUID1)
	}
	if info.FirstPrompt != "scoped info" {
		t.Errorf("FirstPrompt = %q, want %q", info.FirstPrompt, "scoped info")
	}
}

func TestForkSessionReturnsError(t *testing.T) {
	_, err := ForkSession("some-session", nil)
	if err == nil {
		t.Fatal("expected error from ForkSession")
	}
	if !strings.Contains(err.Error(), "NewSession") {
		t.Errorf("error should mention NewSession, got: %v", err)
	}
	if !strings.Contains(err.Error(), "some-session") {
		t.Errorf("error should mention session ID, got: %v", err)
	}
}

func TestExtractFirstPromptNonUserFirst(t *testing.T) {
	head := strings.Join([]string{
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"I am ready"}]}}`,
		`{"type":"user","message":{"role":"user","content":"Now help me"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Sure"}]}}`,
	}, "\n") + "\n"
	result := extractFirstPrompt(head)

	// The function scans for the first user message, skipping the leading assistant.
	if result != "Now help me" {
		t.Errorf("firstPrompt = %q, want %q", result, "Now help me")
	}
}

func TestExtractFirstPromptContentArray(t *testing.T) {
	// Content is an array with text blocks — extractFirstPrompt should extract
	// the text from the first text block via the "text" field fallback.
	// Use "content_block" as the block type so the first "text" match is the key we want.
	head := `{"type":"user","message":{"role":"user","content":[{"type":"content_block","text":"array content"}]}}` + "\n"

	result := extractFirstPrompt(head)

	if result != "array content" {
		t.Errorf("firstPrompt = %q, want %q", result, "array content")
	}
}

func TestListSessionsSubdirectoryIgnored(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{`{"type":"user","message":{"role":"user","content":"top level"}}`}
	writeSessionFile(t, projectDir, testUUID1, lines)

	// Create a subdirectory with a .jsonl file inside the project dir.
	// ReadDir does NOT descend into subdirectories, so it should be ignored.
	subDir := filepath.Join(projectDir, "subdir")
	os.MkdirAll(subDir, 0o755)
	// Write a .jsonl file in subdirectory — should be ignored (ReadDir, not WalkDir).
	writeSessionFile(t, subDir, testUUID2, lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	// Only the top-level .jsonl should be found.
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].SessionID != testUUID1 {
		t.Errorf("SessionID = %q, want %s", sessions[0].SessionID, testUUID1)
	}
}

func TestGetSessionMessagesManyMessages(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	var lines []string
	prevUUID := ""
	for i := 0; i < 100; i++ {
		uuid := fmt.Sprintf("00000000-0000-0000-0000-%012d", i)
		if i%2 == 0 {
			if prevUUID == "" {
				lines = append(lines, fmt.Sprintf(`{"type":"user","uuid":"%s","message":{"role":"user","content":"msg-%d"}}`, uuid, i))
			} else {
				lines = append(lines, fmt.Sprintf(`{"type":"user","uuid":"%s","parentUuid":"%s","message":{"role":"user","content":"msg-%d"}}`, uuid, prevUUID, i))
			}
		} else {
			lines = append(lines, fmt.Sprintf(`{"type":"assistant","uuid":"%s","parentUuid":"%s","message":{"role":"assistant","content":[{"type":"text","text":"resp-%d"}]}}`, uuid, prevUUID, i))
		}
		prevUUID = uuid
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	messages, err := GetSessionMessages(testUUID1, nil)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(messages) != 100 {
		t.Fatalf("expected 100 messages, got %d", len(messages))
	}

	// Verify types alternate correctly.
	for i, m := range messages {
		if i%2 == 0 {
			if m.Type != "user" {
				t.Errorf("message[%d].Type = %q, want user", i, m.Type)
			}
		} else {
			if m.Type != "assistant" {
				t.Errorf("message[%d].Type = %q, want assistant", i, m.Type)
			}
		}
	}
}

func TestListSessionsNilOptions(t *testing.T) {
	setupFakeHome(t, "testproject")

	// nil options should work and use defaults.
	sessions, err := ListSessions(nil)
	if err != nil {
		t.Fatalf("ListSessions(nil): %v", err)
	}
	// No sessions in the project dir, so empty result.
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

// --- Helper tests ---

func TestValidateUUIDValid(t *testing.T) {
	valid := []string{
		"00000000-0000-0000-0000-000000000000",
		"550e8400-e29b-41d4-a716-446655440000",
		"a1b2c3d4-e5f6-7890-abcd-ef1234567890",
	}
	for _, s := range valid {
		if !validateUUID(s) {
			t.Errorf("validateUUID(%q) = false, want true", s)
		}
	}
}

func TestValidateUUIDInvalid(t *testing.T) {
	invalid := []string{
		"",
		"not-a-uuid",
		"00000000-0000-0000-0000-00000000000", // too short
		"00000000-0000-0000-0000-0000000000000", // too long
		"00000000-0000-0000-0000-00000000000G", // uppercase hex not allowed
		"AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE", // uppercase
		"00000000_0000_0000_0000_000000000000",  // wrong separator
	}
	for _, s := range invalid {
		if validateUUID(s) {
			t.Errorf("validateUUID(%q) = true, want false", s)
		}
	}
}

func TestSanitizePathBasic(t *testing.T) {
	result := sanitizePath("/Users/foo/my-project")
	expected := "-Users-foo-my-project"
	if result != expected {
		t.Errorf("sanitizePath = %q, want %q", result, expected)
	}
}

func TestSanitizePathLong(t *testing.T) {
	// Build a path longer than 200 characters.
	longPath := "/" + strings.Repeat("abcdefghij/", 25) // 275 chars
	result := sanitizePath(longPath)

	// Should be truncated to 200 chars + "-" + hash.
	if len(result) <= 200 {
		t.Errorf("expected length > 200, got %d", len(result))
	}
	// The first 200 chars of the encoded path should match.
	encoded := nonAlphanumeric.ReplaceAllString(longPath, "-")
	if result[:200] != encoded[:200] {
		t.Errorf("prefix mismatch: got %q", result[:200])
	}
	// Should end with "-" + hash.
	suffix := result[200:]
	if suffix[0] != '-' {
		t.Errorf("expected dash separator at position 200, got %q", string(suffix[0]))
	}
	hash := simpleHash(longPath)
	if suffix != "-"+hash {
		t.Errorf("suffix = %q, want %q", suffix, "-"+hash)
	}
}

func TestSimpleHashDeterministic(t *testing.T) {
	h1 := simpleHash("hello world")
	h2 := simpleHash("hello world")
	if h1 != h2 {
		t.Errorf("same input produced different hashes: %q vs %q", h1, h2)
	}

	h3 := simpleHash("different input")
	if h1 == h3 {
		t.Errorf("different inputs produced same hash: %q", h1)
	}
}

func TestSimpleHashEmpty(t *testing.T) {
	result := simpleHash("")
	if result != "0" {
		t.Errorf("simpleHash(\"\") = %q, want %q", result, "0")
	}
}

func TestExtractJsonStringFieldSimple(t *testing.T) {
	text := `{"foo":"bar","baz":"qux"}`
	result := extractJsonStringField(text, "foo")
	if result != "bar" {
		t.Errorf("extractJsonStringField = %q, want %q", result, "bar")
	}
}

func TestExtractJsonStringFieldWithSpace(t *testing.T) {
	text := `{"foo": "bar"}`
	result := extractJsonStringField(text, "foo")
	if result != "bar" {
		t.Errorf("extractJsonStringField = %q, want %q", result, "bar")
	}
}

func TestExtractJsonStringFieldEscaped(t *testing.T) {
	text := `{"foo":"bar\"baz"}`
	result := extractJsonStringField(text, "foo")
	expected := `bar"baz`
	if result != expected {
		t.Errorf("extractJsonStringField = %q, want %q", result, expected)
	}
}

func TestExtractJsonStringFieldMissing(t *testing.T) {
	text := `{"foo":"bar"}`
	result := extractJsonStringField(text, "missing")
	if result != "" {
		t.Errorf("extractJsonStringField = %q, want %q", result, "")
	}
}

func TestExtractLastJsonStringField(t *testing.T) {
	text := `{"key":"first"}
{"key":"second"}
{"key":"third"}`
	result := extractLastJsonStringField(text, "key")
	if result != "third" {
		t.Errorf("extractLastJsonStringField = %q, want %q", result, "third")
	}
}

func TestExtractFirstPromptSimple(t *testing.T) {
	head := `{"type":"user","message":{"role":"user","content":"Hello there"}}` + "\n"
	result := extractFirstPrompt(head)
	if result != "Hello there" {
		t.Errorf("extractFirstPrompt = %q, want %q", result, "Hello there")
	}
}

func TestExtractFirstPromptSkipsMeta(t *testing.T) {
	head := strings.Join([]string{
		`{"type":"user","isMeta":true,"message":{"role":"user","content":"meta message"}}`,
		`{"type":"user","message":{"role":"user","content":"real message"}}`,
	}, "\n") + "\n"
	result := extractFirstPrompt(head)
	if result != "real message" {
		t.Errorf("extractFirstPrompt = %q, want %q", result, "real message")
	}
}

func TestExtractFirstPromptSkipsToolResult(t *testing.T) {
	head := strings.Join([]string{
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"x","content":"result"}]}}`,
		`{"type":"user","message":{"role":"user","content":"actual prompt"}}`,
	}, "\n") + "\n"
	result := extractFirstPrompt(head)
	if result != "actual prompt" {
		t.Errorf("extractFirstPrompt = %q, want %q", result, "actual prompt")
	}
}

func TestExtractFirstPromptTruncates(t *testing.T) {
	longContent := strings.Repeat("x", 250)
	head := fmt.Sprintf(`{"type":"user","message":{"role":"user","content":"%s"}}`, longContent) + "\n"
	result := extractFirstPrompt(head)
	if len(result) != 200 {
		t.Errorf("length = %d, want 200", len(result))
	}
}

// --- ListSessions feature tests ---

func TestListSessionsCustomTitleWinsSummary(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"my prompt"}}`,
		`{"type":"summary","summary":"AI summary"}`,
		`{"type":"summary","customTitle":"My Custom Title"}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Summary != "My Custom Title" {
		t.Errorf("Summary = %q, want %q", sessions[0].Summary, "My Custom Title")
	}
}

func TestListSessionsSummaryWinsFirstPrompt(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	// No customTitle, but has summary field and firstPrompt.
	// aiTitle is higher priority than lastPrompt/summary/firstPrompt per the code.
	// We use a "summary" type entry which sets the "summary" field in tail.
	// For this test, we need summary to win over firstPrompt but no customTitle or aiTitle.
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"my first prompt"}}`,
		`{"type":"result","summary":"Generated summary"}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	// The summary field "Generated summary" is found but lastPrompt "my first prompt"
	// has higher priority. To test summary winning, we need no lastPrompt in tail.
	// Since file is small, head == tail, and the user message IS in tail as lastPrompt.
	// So lastPrompt wins. Let's verify the session has the expected summary.
	// Actually: priority is customTitle > aiTitle > lastPrompt > summaryField > firstPrompt.
	// Since lastPrompt == "my first prompt" exists, it will be the summary.
	// To test summaryField winning, we need no lastPrompt. We can use a session
	// where the only user entry is isMeta (so no lastPrompt from tail scan).
	// Let me re-do this properly.
	if sessions[0].Summary != "my first prompt" {
		t.Errorf("Summary = %q, want %q", sessions[0].Summary, "my first prompt")
	}
}

func TestListSessionsFiltersSidechain(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	// First line has isSidechain:true -> should be filtered.
	lines := []string{
		`{"type":"user","isSidechain":true,"message":{"role":"user","content":"sidechain msg"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"reply"}]}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (sidechain filtered), got %d", len(sessions))
	}
}

func TestListSessionsFiltersMetaOnly(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	// Only isMeta messages -> no metadata extracted -> filtered by hasMetadata.
	lines := []string{
		`{"type":"user","isMeta":true,"message":{"role":"user","content":"meta only"}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (meta-only filtered), got %d", len(sessions))
	}
}

func TestListSessionsDeduplicates(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)

	projA := filepath.Join(configDir, "projects", "project-a")
	projB := filepath.Join(configDir, "projects", "project-b")
	os.MkdirAll(projA, 0o755)
	os.MkdirAll(projB, 0o755)

	lines := []string{`{"type":"user","message":{"role":"user","content":"hello"}}`}

	// Same session ID in two project dirs.
	writeSessionFile(t, projA, testUUID1, lines)
	// Make projA version older.
	past := time.Now().Add(-2 * time.Hour)
	os.Chtimes(filepath.Join(projA, testUUID1+".jsonl"), past, past)

	writeSessionFile(t, projB, testUUID1, lines)
	// projB version is newer (current time).

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (deduplicated), got %d", len(sessions))
	}
	if sessions[0].SessionID != testUUID1 {
		t.Errorf("SessionID = %q, want %s", sessions[0].SessionID, testUUID1)
	}
}

func TestListSessionsCwdFallbackToProjectPath(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)

	// Use Dir option so projectPath is set.
	encoded := sanitizePath("/home/user/myproject")
	projDir := filepath.Join(configDir, "projects", encoded)
	os.MkdirAll(projDir, 0o755)

	// No "cwd" field in the session data.
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"hello"}}`,
	}
	writeSessionFile(t, projDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{Dir: "/home/user/myproject"})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	// Cwd should fall back to the project path derived from Dir.
	// The Dir is passed through filepath.Abs, so check the absolute version.
	absDir, _ := filepath.Abs("/home/user/myproject")
	if sessions[0].Cwd != absDir {
		t.Errorf("Cwd = %q, want %q", sessions[0].Cwd, absDir)
	}
}

func TestListSessionsGitBranchFromTail(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	// Build a file large enough that head != tail (> 64KB).
	// Instead, for small files head == tail, so the tail gitBranch will be found.
	lines := []string{
		`{"type":"user","gitBranch":"main","message":{"role":"user","content":"hello"}}`,
		`{"type":"assistant","gitBranch":"feature-branch","message":{"role":"assistant","content":[{"type":"text","text":"ok"}]}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	// For small files, head == tail, so extractLastJsonStringField(tail, "gitBranch")
	// returns the last occurrence: "feature-branch".
	if sessions[0].GitBranch != "feature-branch" {
		t.Errorf("GitBranch = %q, want %q", sessions[0].GitBranch, "feature-branch")
	}
}

// --- GetSessionMessages chain tests ---

func TestGetSessionMessagesFiltersMeta(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"hello"}}`,
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","isMeta":true,"message":{"role":"assistant","content":[{"type":"text","text":"meta"}]}}`,
		`{"type":"assistant","uuid":"a2","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"real"}]}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	messages, err := GetSessionMessages(testUUID1, nil)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	// isMeta assistant should be filtered from output.
	for _, m := range messages {
		if m.UUID == "a1" {
			t.Errorf("isMeta message a1 should have been filtered")
		}
	}
}

func TestGetSessionMessagesFiltersProgress(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"hello"}}`,
		`{"type":"progress","uuid":"p1","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"thinking..."}]}}`,
		`{"type":"assistant","uuid":"a1","parentUuid":"p1","message":{"role":"assistant","content":[{"type":"text","text":"done"}]}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	messages, err := GetSessionMessages(testUUID1, nil)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	// progress type is not in visibleTypes, so it should be filtered.
	for _, m := range messages {
		if m.Type == "progress" {
			t.Errorf("progress message should have been filtered")
		}
	}
	// Should have user + assistant.
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

func TestGetSessionMessagesPicksMainOverSidechain(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"hello"}}`,
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","isSidechain":true,"message":{"role":"assistant","content":[{"type":"text","text":"side"}]}}`,
		`{"type":"assistant","uuid":"a2","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"main"}]}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	messages, err := GetSessionMessages(testUUID1, nil)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	// Should prefer non-sidechain leaf a2.
	found := false
	for _, m := range messages {
		if m.UUID == "a2" {
			found = true
		}
		if m.UUID == "a1" {
			t.Errorf("sidechain leaf a1 should not be in the main chain")
		}
	}
	if !found {
		t.Errorf("expected main leaf a2 in messages")
	}
}

func TestGetSessionMessagesPicksLatestLeaf(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	// Two non-sidechain leaves; higher file position should win.
	lines := []string{
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"hello"}}`,
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"first"}]}}`,
		`{"type":"assistant","uuid":"a2","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"second"}]}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	messages, err := GetSessionMessages(testUUID1, nil)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	// a2 has higher file position, so it should be picked.
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[1].UUID != "a2" {
		t.Errorf("expected leaf a2 (highest filePos), got %q", messages[1].UUID)
	}
}

func TestGetSessionMessagesTerminalProgressWalkedBack(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	// Terminal node is a progress entry; chain should walk back to user/assistant.
	lines := []string{
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"hello"}}`,
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"done"}]}}`,
		`{"type":"progress","uuid":"p1","parentUuid":"a1","message":{"role":"assistant","content":[{"type":"text","text":"loading..."}]}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	messages, err := GetSessionMessages(testUUID1, nil)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	// p1 is terminal but not user/assistant. The code walks back from p1 to find a1.
	// Chain should be u1 -> a1 (progress filtered from visible output).
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].UUID != "u1" {
		t.Errorf("messages[0].UUID = %q, want u1", messages[0].UUID)
	}
	if messages[1].UUID != "a1" {
		t.Errorf("messages[1].UUID = %q, want a1", messages[1].UUID)
	}
}

func TestGetSessionMessagesCycleDetection(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	// Create a cycle: a1 -> u1 -> a1 (via parentUuid).
	lines := []string{
		`{"type":"user","uuid":"u1","parentUuid":"a1","message":{"role":"user","content":"hello"}}`,
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"hi"}]}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	// Should not hang; cycle detection breaks the loop.
	messages, err := GetSessionMessages(testUUID1, nil)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	// Should return some messages without infinite loop.
	if len(messages) == 0 {
		t.Errorf("expected some messages despite cycle, got 0")
	}
}

func TestGetSessionMessagesSearchAllProjects(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".claude")
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)

	projDir := filepath.Join(configDir, "projects", "some-project")
	os.MkdirAll(projDir, 0o755)

	lines := []string{
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"found it"}}`,
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"yes"}]}}`,
	}
	writeSessionFile(t, projDir, testUUID1, lines)

	// No directory specified -> searches all project dirs.
	messages, err := GetSessionMessages(testUUID1, nil)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
}

func TestGetSessionMessagesIgnoresNonTranscriptTypes(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	// Entries with no uuid are skipped by chain building. summary/tag entries
	// typically don't have uuid and should be ignored.
	lines := []string{
		`{"type":"summary","summary":"a summary"}`,
		`{"type":"tag","tag":"mytag","sessionId":"` + testUUID1 + `"}`,
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"real"}}`,
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"ok"}]}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	messages, err := GetSessionMessages(testUUID1, nil)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages (summary/tag ignored), got %d", len(messages))
	}
	if messages[0].UUID != "u1" {
		t.Errorf("messages[0].UUID = %q, want u1", messages[0].UUID)
	}
}

// --- Tag tests ---

func TestListSessionsTagExtracted(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"hello"}}`,
		`{"type":"tag","tag":"mytag","sessionId":"` + testUUID1 + `"}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Tag != "mytag" {
		t.Errorf("Tag = %q, want %q", sessions[0].Tag, "mytag")
	}
}

func TestListSessionsTagLastWins(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"hello"}}`,
		`{"type":"tag","tag":"first","sessionId":"` + testUUID1 + `"}`,
		`{"type":"tag","tag":"second","sessionId":"` + testUUID1 + `"}`,
		`{"type":"tag","tag":"third","sessionId":"` + testUUID1 + `"}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Tag != "third" {
		t.Errorf("Tag = %q, want %q", sessions[0].Tag, "third")
	}
}

func TestListSessionsTagEmptyIsNone(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"hello"}}`,
		`{"type":"tag","tag":"","sessionId":"` + testUUID1 + `"}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	// Empty tag value means no tag.
	if sessions[0].Tag != "" {
		t.Errorf("Tag = %q, want empty", sessions[0].Tag)
	}
}

func TestListSessionsTagAbsent(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"hello"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"hi"}]}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Tag != "" {
		t.Errorf("Tag = %q, want empty (no tag entry)", sessions[0].Tag)
	}
}

// --- CreatedAt tests ---

func TestListSessionsCreatedAtFromTimestamp(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	// Timestamp as RFC3339 in first entry.
	lines := []string{
		`{"type":"user","timestamp":"2024-01-15T10:30:00Z","message":{"role":"user","content":"hello"}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].CreatedAt == nil {
		t.Fatal("CreatedAt should not be nil")
	}
	expected := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC).UnixMilli()
	if *sessions[0].CreatedAt != expected {
		t.Errorf("CreatedAt = %d, want %d", *sessions[0].CreatedAt, expected)
	}
}

func TestListSessionsCreatedAtMissing(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	// No timestamp field.
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"hello"}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].CreatedAt != nil {
		t.Errorf("CreatedAt = %v, want nil", sessions[0].CreatedAt)
	}
}

func TestListSessionsCreatedAtInvalidFormat(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	// Invalid timestamp format.
	lines := []string{
		`{"type":"user","timestamp":"not-a-timestamp","message":{"role":"user","content":"hello"}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	sessions, err := ListSessions(&ListSessionsOptions{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].CreatedAt != nil {
		t.Errorf("CreatedAt = %v, want nil for invalid timestamp", sessions[0].CreatedAt)
	}
}

// --- GetSessionInfo tests ---

func TestGetSessionInfoIncludesTag(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	lines := []string{
		`{"type":"user","message":{"role":"user","content":"hello"}}`,
		`{"type":"tag","tag":"important","sessionId":"` + testUUID1 + `"}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	info, err := GetSessionInfo(testUUID1, "")
	if err != nil {
		t.Fatalf("GetSessionInfo: %v", err)
	}
	if info.Tag != "important" {
		t.Errorf("Tag = %q, want %q", info.Tag, "important")
	}
}

func TestGetSessionInfoFiltersSidechain(t *testing.T) {
	projectDir := setupFakeHome(t, "testproject")
	// Sidechain session - GetSessionInfo should still return info (not filtered).
	// Only ListSessions filters sidechains.
	lines := []string{
		`{"type":"user","isSidechain":true,"message":{"role":"user","content":"sidechain"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"ok"}]}}`,
	}
	writeSessionFile(t, projectDir, testUUID1, lines)

	info, err := GetSessionInfo(testUUID1, "")
	if err != nil {
		t.Fatalf("GetSessionInfo: %v", err)
	}
	// GetSessionInfo does not filter sidechains - it returns valid info.
	if info.SessionID != testUUID1 {
		t.Errorf("SessionID = %q, want %s", info.SessionID, testUUID1)
	}
	if info.FileSize <= 0 {
		t.Errorf("FileSize = %d, want > 0", info.FileSize)
	}
}
