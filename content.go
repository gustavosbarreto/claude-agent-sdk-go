package claude

import "encoding/json"

// ContentBlockType identifies the kind of content block.
type ContentBlockType string

const (
	ContentBlockText       ContentBlockType = "text"
	ContentBlockThinking   ContentBlockType = "thinking"
	ContentBlockToolUse    ContentBlockType = "tool_use"
	ContentBlockToolResult ContentBlockType = "tool_result"
)

// ContentBlock is a piece of content in an assistant or user message.
// Use the Type field to determine which fields are populated.
type ContentBlock struct {
	Type ContentBlockType `json:"type"`

	// Text block fields.
	Text string `json:"text,omitempty"`

	// Thinking block fields.
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`

	// Tool use block fields.
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// Tool result block fields.
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

// TextBlocks returns all text content blocks from the message.
func TextBlocks(blocks []ContentBlock) []ContentBlock {
	var out []ContentBlock
	for _, b := range blocks {
		if b.Type == ContentBlockText {
			out = append(out, b)
		}
	}
	return out
}

// ToolUseBlocks returns all tool use content blocks.
func ToolUseBlocks(blocks []ContentBlock) []ContentBlock {
	var out []ContentBlock
	for _, b := range blocks {
		if b.Type == ContentBlockToolUse {
			out = append(out, b)
		}
	}
	return out
}

// CombinedText returns all text blocks concatenated.
func CombinedText(blocks []ContentBlock) string {
	var s string
	for _, b := range blocks {
		if b.Type == ContentBlockText {
			s += b.Text
		}
	}
	return s
}
