package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"
)

var uuidRegexp = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// dangerousUnicodeRanges defines Unicode ranges that should be stripped from
// user-supplied strings. These include zero-width characters, directional
// formatting marks, directional isolates, the BOM, and private-use characters.
var dangerousUnicodeRanges = []*unicode.RangeTable{
	// Zero-width spaces and marks (U+200B–U+200F).
	{R16: []unicode.Range16{{Lo: 0x200B, Hi: 0x200F, Stride: 1}}},
	// Directional formatting (U+202A–U+202E).
	{R16: []unicode.Range16{{Lo: 0x202A, Hi: 0x202E, Stride: 1}}},
	// Directional isolates (U+2066–U+2069).
	{R16: []unicode.Range16{{Lo: 0x2066, Hi: 0x2069, Stride: 1}}},
	// BOM / ZWNBSP (U+FEFF).
	{R16: []unicode.Range16{{Lo: 0xFEFF, Hi: 0xFEFF, Stride: 1}}},
	// Private Use Area (U+E000–U+F8FF).
	{R16: []unicode.Range16{{Lo: 0xE000, Hi: 0xF8FF, Stride: 1}}},
	// Format characters (Cf) and unassigned/private-use not already covered.
	unicode.Cf,
	unicode.Co,
}

// sanitizeUnicode strips dangerous Unicode characters from s.
func sanitizeUnicode(s string) string {
	return strings.Map(func(r rune) rune {
		for _, rt := range dangerousUnicodeRanges {
			if unicode.Is(rt, r) {
				return -1
			}
		}
		return r
	}, s)
}

func validateSessionID(sessionID string) error {
	if !uuidRegexp.MatchString(sessionID) {
		return fmt.Errorf("claude: invalid session ID: %q", sessionID)
	}
	return nil
}

// RenameSession renames a session by appending a custom-title entry to its JSONL file.
// The most recent custom-title entry wins when listing sessions.
func RenameSession(sessionID, title, dir string) error {
	if err := validateSessionID(sessionID); err != nil {
		return err
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("claude: title must not be empty")
	}

	path, err := findSessionFile(sessionID, dir)
	if err != nil {
		return err
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("claude: stat session file: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("claude: session file is empty")
	}

	entry := struct {
		Type        string `json:"type"`
		CustomTitle string `json:"customTitle"`
		SessionID   string `json:"sessionId"`
	}{
		Type:        "custom-title",
		CustomTitle: title,
		SessionID:   sessionID,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("claude: marshal custom-title: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("claude: open session file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("claude: write custom-title: %w", err)
	}

	return nil
}

// TagSession tags a session. Pass nil to clear the tag.
// Appends a tag entry to the session's JSONL file.
func TagSession(sessionID string, tag *string, dir string) error {
	if err := validateSessionID(sessionID); err != nil {
		return err
	}

	tagValue := ""
	if tag != nil {
		sanitized := sanitizeUnicode(*tag)
		sanitized = strings.TrimSpace(sanitized)
		if sanitized == "" {
			return fmt.Errorf("claude: tag must not be empty")
		}
		tagValue = sanitized
	}

	path, err := findSessionFile(sessionID, dir)
	if err != nil {
		return err
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("claude: stat session file: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("claude: session file is empty")
	}

	entry := struct {
		Type      string `json:"type"`
		Tag       string `json:"tag"`
		SessionID string `json:"sessionId"`
	}{
		Type:      "tag",
		Tag:       tagValue,
		SessionID: sessionID,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("claude: marshal tag: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("claude: open session file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("claude: write tag: %w", err)
	}

	return nil
}

// DeleteSession removes a session's JSONL file.
func DeleteSession(sessionID, dir string) error {
	if err := validateSessionID(sessionID); err != nil {
		return err
	}

	path, err := findSessionFile(sessionID, dir)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("claude: delete session: %w", err)
	}

	return nil
}
