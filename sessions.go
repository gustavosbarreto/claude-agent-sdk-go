package claude

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SessionInfo describes a stored session.
type SessionInfo struct {
	SessionID    string `json:"sessionId"`
	Summary      string `json:"summary"`
	LastModified int64  `json:"lastModified"`  // epoch milliseconds
	FileSize     int64  `json:"fileSize"`
	CustomTitle  string `json:"customTitle,omitempty"`
	FirstPrompt  string `json:"firstPrompt,omitempty"`
	GitBranch    string `json:"gitBranch,omitempty"`
	Cwd          string `json:"cwd,omitempty"`
	Tag          string `json:"tag,omitempty"`
	CreatedAt    *int64 `json:"createdAt,omitempty"` // epoch milliseconds
}

// SessionMessage is a message from a session transcript.
type SessionMessage struct {
	Type            string          `json:"type"` // user, assistant
	UUID            string          `json:"uuid"`
	SessionID       string          `json:"session_id"`
	Message         json.RawMessage `json:"message"`
	ParentToolUseID *string         `json:"parent_tool_use_id"`
}

// ListSessionsOptions configures session listing.
type ListSessionsOptions struct {
	Dir              string
	Limit            int
	Offset           int
	IncludeWorktrees bool
}

// GetSessionMessagesOptions configures message retrieval.
type GetSessionMessagesOptions struct {
	Dir    string
	Limit  int
	Offset int
}

// ForkSessionOptions configures session forking.
type ForkSessionOptions struct {
	Dir           string
	UpToMessageID string
	Title         string
}

// --- config dir ---

// getClaudeConfigDir returns the Claude config directory, respecting
// CLAUDE_CONFIG_DIR env var with fallback to ~/.claude.
func getClaudeConfigDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// --- simple hash (port from Python/JS) ---

// simpleHash computes a 32-bit hash of s and returns it in base36.
// This matches the JavaScript/Python simpleHash used for long path encoding.
func simpleHash(s string) string {
	var hash uint32
	for i := 0; i < len(s); i++ {
		hash = (hash<<5 - hash) + uint32(s[i])
		hash &= 0xFFFFFFFF // keep 32-bit
	}
	return strconv.FormatUint(uint64(hash), 36)
}

// --- path sanitization ---

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]`)

// sanitizePath encodes a project path for use as a directory name.
// Paths longer than 200 chars get a hash suffix and are truncated.
func sanitizePath(p string) string {
	encoded := nonAlphanumeric.ReplaceAllString(p, "-")
	if len(encoded) > 200 {
		h := simpleHash(p)
		// Truncate to 200 and append hash.
		encoded = encoded[:200] + "-" + h
	}
	return encoded
}

// sessionDirForProject returns the session directory for a given project path.
func sessionDirForProject(projectDir string) string {
	configDir := getClaudeConfigDir()
	encoded := sanitizePath(projectDir)
	return filepath.Join(configDir, "projects", encoded)
}

// --- UUID validation ---

var uuidFileRegexp = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// validateUUID checks if s is a valid lowercase UUID.
func validateUUID(s string) bool {
	return uuidFileRegexp.MatchString(s)
}

// --- lite session reading ---

const liteChunkSize = 64 * 1024 // 64 KB

// sessionLite holds the head/tail text and stat info of a session file.
type sessionLite struct {
	head     string
	tail     string
	modTime  time.Time
	fileSize int64
}

// readSessionLite reads the first 64KB and last 64KB of a session file
// along with its stat info, without reading the entire file.
func readSessionLite(path string) (*sessionLite, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := info.Size()
	lite := &sessionLite{
		modTime:  info.ModTime(),
		fileSize: size,
	}

	// Read head.
	headSize := int64(liteChunkSize)
	if headSize > size {
		headSize = size
	}
	headBuf := make([]byte, headSize)
	n, err := f.Read(headBuf)
	if err != nil && err != io.EOF {
		return nil, err
	}
	lite.head = string(headBuf[:n])

	// Read tail (may overlap with head for small files).
	if size > int64(liteChunkSize) {
		tailOffset := size - int64(liteChunkSize)
		tailBuf := make([]byte, liteChunkSize)
		n, err = f.ReadAt(tailBuf, tailOffset)
		if err != nil && err != io.EOF {
			return nil, err
		}
		lite.tail = string(tailBuf[:n])
	} else {
		lite.tail = lite.head
	}

	return lite, nil
}

// --- JSON field extraction without full parse ---

// extractJsonStringField extracts the first occurrence of a JSON string field
// value from text using simple string scanning. Returns "" if not found.
func extractJsonStringField(text, key string) string {
	needle := `"` + key + `"`
	idx := strings.Index(text, needle)
	if idx < 0 {
		return ""
	}
	return extractJsonValueAfterKey(text[idx+len(needle):])
}

// extractLastJsonStringField extracts the last occurrence of a JSON string
// field value from text. Returns "" if not found.
func extractLastJsonStringField(text, key string) string {
	needle := `"` + key + `"`
	idx := strings.LastIndex(text, needle)
	if idx < 0 {
		return ""
	}
	return extractJsonValueAfterKey(text[idx+len(needle):])
}

// extractJsonValueAfterKey extracts a JSON string value after the key portion.
// Expects the text to start right after the key's closing quote, e.g. `: "value"...`.
func extractJsonValueAfterKey(after string) string {
	// Skip whitespace and colon.
	i := 0
	for i < len(after) && (after[i] == ' ' || after[i] == '\t' || after[i] == ':') {
		i++
	}
	if i >= len(after) {
		return ""
	}
	// Check for opening quote.
	if after[i] != '"' {
		return ""
	}
	i++ // skip opening quote
	var sb strings.Builder
	for i < len(after) {
		ch := after[i]
		if ch == '\\' && i+1 < len(after) {
			next := after[i+1]
			switch next {
			case '"', '\\', '/':
				sb.WriteByte(next)
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			default:
				sb.WriteByte('\\')
				sb.WriteByte(next)
			}
			i += 2
			continue
		}
		if ch == '"' {
			return sb.String()
		}
		sb.WriteByte(ch)
		i++
	}
	return ""
}

// --- first prompt extraction ---

// commandNamePattern matches lines like `{"type":"user","message":{"role":"user","content":"/something ..."}}`.
var commandNamePattern = regexp.MustCompile(`^/[a-zA-Z]`)

// extractFirstPrompt extracts the first user prompt from the head text,
// skipping tool_result, isMeta, isCompactSummary, and command-name entries.
func extractFirstPrompt(head string) string {
	lines := strings.Split(head, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Quick type check.
		typeVal := extractJsonStringField(line, "type")
		if typeVal != "user" {
			continue
		}

		// Skip tool_result entries.
		if strings.Contains(line, `"tool_result"`) {
			continue
		}

		// Skip isMeta entries.
		if strings.Contains(line, `"isMeta"`) && strings.Contains(line, "true") {
			continue
		}

		// Skip isCompactSummary entries.
		if strings.Contains(line, `"isCompactSummary"`) && strings.Contains(line, "true") {
			continue
		}

		// Extract the content.
		content := extractJsonStringField(line, "content")
		if content == "" {
			// Try to extract from array content (content blocks).
			// Look for "text" field inside content blocks.
			roleIdx := strings.Index(line, `"content"`)
			if roleIdx >= 0 {
				rest := line[roleIdx:]
				textVal := extractJsonStringField(rest, "text")
				if textVal != "" {
					content = textVal
				}
			}
		}

		if content == "" {
			continue
		}

		// Skip command-name patterns (slash commands).
		if commandNamePattern.MatchString(content) {
			continue
		}

		// Truncate to 200 chars.
		if len(content) > 200 {
			content = content[:200]
		}
		return content
	}
	return ""
}

// --- sidechain detection ---

// isSidechainSession checks if the first line of text has "isSidechain":true.
func isSidechainSession(head string) bool {
	// Check only the first line.
	firstLine := head
	if idx := strings.IndexByte(head, '\n'); idx >= 0 {
		firstLine = head[:idx]
	}
	return strings.Contains(firstLine, `"isSidechain"`) && strings.Contains(firstLine, "true")
}

// --- session info parsing from lite data ---

// parseSessionInfoFromLite extracts all metadata from head/tail lite reads.
func parseSessionInfoFromLite(sessionID string, lite *sessionLite, projectPath string) *SessionInfo {
	head := lite.head
	tail := lite.tail

	si := &SessionInfo{
		SessionID:    sessionID,
		LastModified: lite.modTime.UnixMilli(),
		FileSize:     lite.fileSize,
	}

	// Custom title: last occurrence in tail (preferred), fallback to head.
	if ct := extractLastJsonStringField(tail, "customTitle"); ct != "" {
		si.CustomTitle = ct
	} else if ct := extractLastJsonStringField(head, "customTitle"); ct != "" {
		si.CustomTitle = ct
	}

	// AI title (used as summary fallback): from tail then head.
	aiTitle := extractLastJsonStringField(tail, "aiTitle")
	if aiTitle == "" {
		aiTitle = extractLastJsonStringField(head, "aiTitle")
	}

	// First prompt.
	si.FirstPrompt = extractFirstPrompt(head)

	// Git branch: tail (preferred), fallback to head.
	if gb := extractLastJsonStringField(tail, "gitBranch"); gb != "" {
		si.GitBranch = gb
	} else if gb := extractJsonStringField(head, "gitBranch"); gb != "" {
		si.GitBranch = gb
	}

	// Cwd: from head, fallback to project path.
	if cwd := extractJsonStringField(head, "cwd"); cwd != "" {
		si.Cwd = cwd
	} else if projectPath != "" {
		si.Cwd = projectPath
	}

	// Tag: last occurrence in tail.
	if t := extractLastJsonStringField(tail, "tag"); t != "" {
		si.Tag = t
	}

	// Created at: from first entry's timestamp.
	if ts := extractJsonStringField(head, "timestamp"); ts != "" {
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			ms := t.UnixMilli()
			si.CreatedAt = &ms
		} else if t, err := time.Parse(time.RFC3339, ts); err == nil {
			ms := t.UnixMilli()
			si.CreatedAt = &ms
		}
	}

	// Build summary with priority: customTitle > aiTitle > lastPrompt > summary > firstPrompt.
	// "lastPrompt" comes from tail user messages.
	lastPrompt := ""
	// Scan tail for last user prompt.
	tailLines := strings.Split(tail, "\n")
	for i := len(tailLines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(tailLines[i])
		if line == "" {
			continue
		}
		if extractJsonStringField(line, "type") == "user" {
			if !strings.Contains(line, `"tool_result"`) && !strings.Contains(line, `"isMeta"`) {
				c := extractJsonStringField(line, "content")
				if c != "" && !commandNamePattern.MatchString(c) {
					lastPrompt = c
					if len(lastPrompt) > 200 {
						lastPrompt = lastPrompt[:200]
					}
					break
				}
			}
		}
	}

	// Summary from head/tail.
	summaryField := extractLastJsonStringField(tail, "summary")
	if summaryField == "" {
		summaryField = extractJsonStringField(head, "summary")
	}

	// Apply priority.
	switch {
	case si.CustomTitle != "":
		si.Summary = si.CustomTitle
	case aiTitle != "":
		si.Summary = aiTitle
	case lastPrompt != "":
		si.Summary = lastPrompt
	case summaryField != "":
		si.Summary = summaryField
	case si.FirstPrompt != "":
		si.Summary = si.FirstPrompt
	}

	return si
}

// --- meta-only filtering ---

// hasMetadata checks if a session has any meaningful metadata (title, summary, or prompt).
func hasMetadata(si *SessionInfo) bool {
	return si.CustomTitle != "" || si.Summary != "" || si.FirstPrompt != ""
}

// --- ListSessions ---

// ListSessions lists stored sessions.
func ListSessions(opts *ListSessionsOptions) ([]SessionInfo, error) {
	if opts == nil {
		opts = &ListSessionsOptions{}
	}

	configDir := getClaudeConfigDir()
	projectsBase := filepath.Join(configDir, "projects")

	type sessionCandidate struct {
		info        SessionInfo
		projectPath string
	}

	// dedup map: sessionID -> best candidate (newest).
	dedup := make(map[string]sessionCandidate)

	addSessionsFromDir := func(dir, projectPath string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".jsonl") {
				continue
			}

			// UUID filename validation.
			sessionID := strings.TrimSuffix(name, ".jsonl")
			if !validateUUID(sessionID) {
				continue
			}

			path := filepath.Join(dir, name)

			// Lite read.
			lite, err := readSessionLite(path)
			if err != nil {
				continue
			}

			// Sidechain filtering.
			if isSidechainSession(lite.head) {
				continue
			}

			// Parse info.
			si := parseSessionInfoFromLite(sessionID, lite, projectPath)

			// Meta-only filtering: skip sessions with no metadata.
			if !hasMetadata(si) {
				continue
			}

			// Dedup: keep newest.
			if existing, ok := dedup[sessionID]; ok {
				if si.LastModified > existing.info.LastModified {
					dedup[sessionID] = sessionCandidate{info: *si, projectPath: projectPath}
				}
			} else {
				dedup[sessionID] = sessionCandidate{info: *si, projectPath: projectPath}
			}
		}
		return nil
	}

	if opts.Dir != "" {
		// Canonicalize path.
		absDir, err := filepath.Abs(opts.Dir)
		if err != nil {
			absDir = opts.Dir
		}
		absDir = filepath.Clean(absDir)

		dir := sessionDirForProject(absDir)
		if err := addSessionsFromDir(dir, absDir); err != nil {
			return nil, err
		}
	} else {
		// Scan all project dirs.
		projectDirs, err := os.ReadDir(projectsBase)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}

		for _, pd := range projectDirs {
			if !pd.IsDir() {
				continue
			}
			dir := filepath.Join(projectsBase, pd.Name())
			if err := addSessionsFromDir(dir, ""); err != nil {
				continue // skip errored directories
			}
		}
	}

	// Collect results.
	sessions := make([]SessionInfo, 0, len(dedup))
	for _, c := range dedup {
		sessions = append(sessions, c.info)
	}

	// Sort by last modified descending.
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastModified > sessions[j].LastModified
	})

	// Apply offset and limit.
	if opts.Offset > 0 && opts.Offset < len(sessions) {
		sessions = sessions[opts.Offset:]
	} else if opts.Offset >= len(sessions) && opts.Offset > 0 {
		return nil, nil
	}

	if opts.Limit > 0 && opts.Limit < len(sessions) {
		sessions = sessions[:opts.Limit]
	}

	return sessions, nil
}

// --- conversation chain building for GetSessionMessages ---

// chainEntry holds a parsed JSONL entry for chain building.
type chainEntry struct {
	Type       string          `json:"type"`
	UUID       string          `json:"uuid"`
	ParentUUID string          `json:"parentUuid"`
	IsMeta     bool            `json:"isMeta"`
	IsSidechain bool           `json:"isSidechain"`
	TeamName   string          `json:"teamName"`
	IsCompactSummary bool      `json:"isCompactSummary"`
	Message    json.RawMessage `json:"message"`
	SessionID  string          `json:"session_id"`
	ParentToolUseID *string    `json:"parent_tool_use_id"`
	filePos    int             // position in file for tie-breaking
}

// validChainTypes are the message types included in chain building.
var validChainTypes = map[string]bool{
	"user":       true,
	"assistant":  true,
	"progress":   true,
	"system":     true,
	"attachment": true,
}

// visibleTypes are the message types visible in final output.
var visibleTypes = map[string]bool{
	"user":      true,
	"assistant": true,
}

// buildConversationChain builds the conversation chain from a JSONL file
// by walking parentUuid links from the best leaf to root.
func buildConversationChain(path string) ([]SessionMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("claude: open session: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Read all bytes and split by newlines (handles large files with scanner buffer).
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("claude: read session: %w", err)
	}

	lines := strings.Split(string(data), "\n")

	// Parse entries.
	var entries []chainEntry
	byUUID := make(map[string]*chainEntry)
	childCount := make(map[string]int) // parentUUID -> count of children

	for pos, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var e chainEntry
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		if e.UUID == "" {
			continue
		}
		if !validChainTypes[e.Type] {
			continue
		}

		e.filePos = pos
		entries = append(entries, e)
		stored := &entries[len(entries)-1]
		byUUID[e.UUID] = stored

		if e.ParentUUID != "" {
			childCount[e.ParentUUID]++
		}
	}

	if len(entries) == 0 {
		return nil, nil
	}

	// Find terminal entries (no children).
	var terminals []*chainEntry
	for i := range entries {
		e := &entries[i]
		if childCount[e.UUID] == 0 {
			terminals = append(terminals, e)
		}
	}

	if len(terminals) == 0 {
		// Fallback: use last entry.
		terminals = append(terminals, &entries[len(entries)-1])
	}

	// Find user/assistant leaf terminals.
	var leaves []*chainEntry
	for _, t := range terminals {
		if t.Type == "user" || t.Type == "assistant" {
			leaves = append(leaves, t)
		}
	}

	if len(leaves) == 0 {
		// Walk back from any terminal to find a user/assistant.
		for _, t := range terminals {
			cur := t
			for cur != nil {
				if cur.Type == "user" || cur.Type == "assistant" {
					leaves = append(leaves, cur)
					break
				}
				if cur.ParentUUID == "" {
					break
				}
				cur = byUUID[cur.ParentUUID]
			}
		}
	}

	if len(leaves) == 0 {
		// No conversation found, return empty.
		return nil, nil
	}

	// Pick best leaf: prefer non-sidechain, non-meta, highest file position.
	bestLeaf := leaves[0]
	for _, l := range leaves[1:] {
		lBetter := false
		if bestLeaf.IsSidechain && !l.IsSidechain {
			lBetter = true
		} else if !bestLeaf.IsSidechain && l.IsSidechain {
			lBetter = false
		} else if bestLeaf.IsMeta && !l.IsMeta {
			lBetter = true
		} else if !bestLeaf.IsMeta && l.IsMeta {
			lBetter = false
		} else if l.filePos > bestLeaf.filePos {
			lBetter = true
		}
		if lBetter {
			bestLeaf = l
		}
	}

	// Walk parentUuid chain from leaf to root.
	var chain []*chainEntry
	seen := make(map[string]bool)
	cur := bestLeaf
	for cur != nil {
		if seen[cur.UUID] {
			break // cycle protection
		}
		seen[cur.UUID] = true
		chain = append(chain, cur)
		if cur.ParentUUID == "" {
			break
		}
		cur = byUUID[cur.ParentUUID]
	}

	// Reverse to get root-to-leaf order.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	// Filter to visible messages.
	var messages []SessionMessage
	for _, e := range chain {
		if !visibleTypes[e.Type] {
			continue
		}
		if e.IsMeta || e.IsSidechain || e.TeamName != "" || e.IsCompactSummary {
			continue
		}

		msg := SessionMessage{
			Type:            e.Type,
			UUID:            e.UUID,
			SessionID:       e.SessionID,
			Message:         e.Message,
			ParentToolUseID: e.ParentToolUseID,
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// GetSessionMessages reads messages from a session transcript using
// conversation chain building (parentUuid chain walking).
func GetSessionMessages(sessionID string, opts *GetSessionMessagesOptions) ([]SessionMessage, error) {
	if opts == nil {
		opts = &GetSessionMessagesOptions{}
	}

	path, err := findSessionFile(sessionID, opts.Dir)
	if err != nil {
		return nil, err
	}

	messages, err := buildConversationChain(path)
	if err != nil {
		return nil, err
	}

	// Apply offset and limit.
	if opts.Offset > 0 && opts.Offset < len(messages) {
		messages = messages[opts.Offset:]
	} else if opts.Offset >= len(messages) && opts.Offset > 0 {
		return nil, nil
	}

	if opts.Limit > 0 && opts.Limit < len(messages) {
		messages = messages[:opts.Limit]
	}

	return messages, nil
}

// GetSessionInfo reads metadata for a single session using lite reads.
func GetSessionInfo(sessionID, dir string) (*SessionInfo, error) {
	path, err := findSessionFile(sessionID, dir)
	if err != nil {
		return nil, err
	}

	lite, err := readSessionLite(path)
	if err != nil {
		return nil, err
	}

	// Determine project path from dir if available.
	projectPath := ""
	if dir != "" {
		if abs, err := filepath.Abs(dir); err == nil {
			projectPath = abs
		}
	}

	si := parseSessionInfoFromLite(sessionID, lite, projectPath)
	return si, nil
}

// ForkSession forks a session at a specific point.
// Returns the new session ID. This is done by the CLI, not by the SDK directly.
func ForkSession(sessionID string, opts *ForkSessionOptions) (string, error) {
	// Forking is implemented by starting a new session with --resume + --fork-session.
	// The actual fork happens when the CLI starts. This function is a convenience
	// that documents the pattern.
	return "", fmt.Errorf("claude: use NewSession with WithResume(%q) and WithForkSession() instead", sessionID)
}

// --- helpers used by session_mutations.go ---

// findSessionFile locates a session's JSONL file.
func findSessionFile(sessionID, dir string) (string, error) {
	configDir := getClaudeConfigDir()

	if dir != "" {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			absDir = dir
		}
		absDir = filepath.Clean(absDir)

		path := filepath.Join(sessionDirForProject(absDir), sessionID+".jsonl")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Search all projects.
	projectsDir := filepath.Join(configDir, "projects")
	var found string
	var foundModTime time.Time

	projectDirs, err := os.ReadDir(projectsDir)
	if err != nil {
		return "", fmt.Errorf("claude: session %s not found", sessionID)
	}

	for _, pd := range projectDirs {
		if !pd.IsDir() {
			continue
		}
		path := filepath.Join(projectsDir, pd.Name(), sessionID+".jsonl")
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		// Keep newest in case of duplicates across project dirs.
		if found == "" || info.ModTime().After(foundModTime) {
			found = path
			foundModTime = info.ModTime()
		}
	}

	if found == "" {
		return "", fmt.Errorf("claude: session %s not found", sessionID)
	}
	return found, nil
}
