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

func TestListSessions_Empty(t *testing.T) {
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

func TestListSessions_SingleSession(t *testing.T) {
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

func TestListSessions_MultipleSessionsSortedByDate(t *testing.T) {
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

func TestListSessions_Limit(t *testing.T) {
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

func TestListSessions_Offset(t *testing.T) {
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

func TestListSessions_OffsetAndLimit(t *testing.T) {
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

func TestListSessions_OffsetBeyondEnd(t *testing.T) {
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

func TestListSessions_ExtractsSummary(t *testing.T) {
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

func TestListSessions_IgnoresNonJsonl(t *testing.T) {
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

func TestGetSessionMessages_Basic(t *testing.T) {
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

func TestGetSessionMessages_FiltersSystemMessages(t *testing.T) {
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

func TestGetSessionMessages_LimitOffset(t *testing.T) {
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

func TestGetSessionMessages_NotFound(t *testing.T) {
	setupFakeHome(t, "testproject")

	_, err := GetSessionMessages("00000000-0000-0000-0000-ffffffffffff", nil)
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

func TestGetSessionInfo_NotFound(t *testing.T) {
	setupFakeHome(t, "testproject")

	_, err := GetSessionInfo("00000000-0000-0000-0000-ffffffffffff", "")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want 'not found'", err)
	}
}

func TestExtractFirstPrompt_LongPrompt(t *testing.T) {
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

func TestExtractFirstPrompt_EmptyInput(t *testing.T) {
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

func TestListSessions_EmptyFileFiltered(t *testing.T) {
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

func TestListSessions_NonUUIDFiltered(t *testing.T) {
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

func TestListSessions_NoConfigDir(t *testing.T) {
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

func TestListSessions_LimitZero(t *testing.T) {
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

func TestListSessions_MultipleProjects(t *testing.T) {
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

func TestListSessions_WithDir(t *testing.T) {
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

func TestGetSessionMessages_CorruptLines(t *testing.T) {
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

func TestGetSessionMessages_EmptyFile(t *testing.T) {
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

func TestGetSessionMessages_OffsetBeyondEnd(t *testing.T) {
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

func TestGetSessionInfo_WithDir(t *testing.T) {
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

func TestForkSession_ReturnsError(t *testing.T) {
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

func TestExtractFirstPrompt_NonUserFirst(t *testing.T) {
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

func TestExtractFirstPrompt_ContentArray(t *testing.T) {
	// Content is an array with text blocks — extractFirstPrompt should extract
	// the text from the first text block via the "text" field fallback.
	// Use "content_block" as the block type so the first "text" match is the key we want.
	head := `{"type":"user","message":{"role":"user","content":[{"type":"content_block","text":"array content"}]}}` + "\n"

	result := extractFirstPrompt(head)

	if result != "array content" {
		t.Errorf("firstPrompt = %q, want %q", result, "array content")
	}
}

func TestListSessions_SubdirectoryIgnored(t *testing.T) {
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

func TestGetSessionMessages_ManyMessages(t *testing.T) {
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

func TestListSessions_NilOptions(t *testing.T) {
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
