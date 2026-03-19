package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SessionInfo describes a stored session.
type SessionInfo struct {
	SessionID    string `json:"sessionId"`
	Summary      string `json:"summary"`
	LastModified int64  `json:"lastModified"`
	FileSize     int64  `json:"fileSize"`
	CustomTitle  string `json:"customTitle,omitempty"`
	FirstPrompt  string `json:"firstPrompt,omitempty"`
	GitBranch    string `json:"gitBranch,omitempty"`
	Cwd          string `json:"cwd,omitempty"`
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

// ListSessions lists stored sessions.
func ListSessions(opts *ListSessionsOptions) ([]SessionInfo, error) {
	if opts == nil {
		opts = &ListSessionsOptions{}
	}

	dir := opts.Dir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("claude: home dir: %w", err)
		}
		dir = filepath.Join(home, ".claude", "projects")
	} else {
		dir = sessionDirForProject(dir)
	}

	var sessions []SessionInfo

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible dirs
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		sessionID := strings.TrimSuffix(d.Name(), ".jsonl")
		si := SessionInfo{
			SessionID:    sessionID,
			LastModified: info.ModTime().Unix(),
			FileSize:     info.Size(),
		}

		// Try to extract summary from first few lines.
		si.Summary, si.FirstPrompt = extractSessionSummary(path)

		sessions = append(sessions, si)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Sort by last modified descending.
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastModified > sessions[j].LastModified
	})

	// Apply offset and limit.
	if opts.Offset > 0 && opts.Offset < len(sessions) {
		sessions = sessions[opts.Offset:]
	} else if opts.Offset >= len(sessions) {
		return nil, nil
	}

	if opts.Limit > 0 && opts.Limit < len(sessions) {
		sessions = sessions[:opts.Limit]
	}

	return sessions, nil
}

// GetSessionMessages reads messages from a session transcript.
func GetSessionMessages(sessionID string, opts *GetSessionMessagesOptions) ([]SessionMessage, error) {
	if opts == nil {
		opts = &GetSessionMessagesOptions{}
	}

	path, err := findSessionFile(sessionID, opts.Dir)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("claude: open session: %w", err)
	}
	defer f.Close()

	var messages []SessionMessage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		var raw struct {
			Type string `json:"type"`
		}
		line := scanner.Bytes()
		if json.Unmarshal(line, &raw) != nil {
			continue
		}

		if raw.Type != "user" && raw.Type != "assistant" {
			continue
		}

		var msg SessionMessage
		if json.Unmarshal(line, &msg) == nil {
			messages = append(messages, msg)
		}
	}

	// Apply offset and limit.
	if opts.Offset > 0 && opts.Offset < len(messages) {
		messages = messages[opts.Offset:]
	} else if opts.Offset >= len(messages) {
		return nil, nil
	}

	if opts.Limit > 0 && opts.Limit < len(messages) {
		messages = messages[:opts.Limit]
	}

	return messages, nil
}

// GetSessionMessagesOptions configures message retrieval.
type GetSessionMessagesOptions struct {
	Dir    string
	Limit  int
	Offset int
}

// GetSessionInfo reads metadata for a single session.
func GetSessionInfo(sessionID string, dir string) (*SessionInfo, error) {
	path, err := findSessionFile(sessionID, dir)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	summary, firstPrompt := extractSessionSummary(path)

	return &SessionInfo{
		SessionID:    sessionID,
		LastModified: info.ModTime().Unix(),
		FileSize:     info.Size(),
		Summary:      summary,
		FirstPrompt:  firstPrompt,
	}, nil
}

// ForkSessionOptions configures session forking.
type ForkSessionOptions struct {
	Dir            string
	UpToMessageID  string
	Title          string
}

// ForkSession forks a session at a specific point.
// Returns the new session ID. This is done by the CLI, not by the SDK directly.
func ForkSession(sessionID string, opts *ForkSessionOptions) (string, error) {
	// Forking is implemented by starting a new session with --resume + --fork-session.
	// The actual fork happens when the CLI starts. This function is a convenience
	// that documents the pattern.
	return "", fmt.Errorf("claude: use NewSession with WithResume(%q) and WithForkSession() instead", sessionID)
}

// --- helpers ---

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]`)

func sessionDirForProject(projectDir string) string {
	home, _ := os.UserHomeDir()
	encoded := nonAlphanumeric.ReplaceAllString(projectDir, "-")
	return filepath.Join(home, ".claude", "projects", encoded)
}

func findSessionFile(sessionID, dir string) (string, error) {
	if dir != "" {
		path := filepath.Join(sessionDirForProject(dir), sessionID+".jsonl")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Search all projects.
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("claude: home dir: %w", err)
	}

	projectsDir := filepath.Join(home, ".claude", "projects")
	var found string

	filepath.WalkDir(projectsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if d.Name() == sessionID+".jsonl" {
			found = path
			return filepath.SkipAll
		}
		return nil
	})

	if found == "" {
		return "", fmt.Errorf("claude: session %s not found", sessionID)
	}
	return found, nil
}

func extractSessionSummary(path string) (summary, firstPrompt string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1*1024*1024)

	for scanner.Scan() {
		var raw struct {
			Type    string `json:"type"`
			Message struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(scanner.Bytes(), &raw) != nil {
			continue
		}

		if raw.Type == "user" && raw.Message.Role == "user" && firstPrompt == "" {
			switch v := raw.Message.Content.(type) {
			case string:
				firstPrompt = v
			}
			if firstPrompt != "" {
				if len(firstPrompt) > 200 {
					firstPrompt = firstPrompt[:200]
				}
				summary = firstPrompt
				return
			}
		}
	}

	return "", ""
}
