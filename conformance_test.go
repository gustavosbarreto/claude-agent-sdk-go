package claude

import (
	"encoding/json"
	"os"
	"testing"
)

// TestConformance runs test cases extracted from the official Python SDK
// (anthropics/claude-agent-sdk-python tests/test_message_parser.py).
// This ensures our Go parser produces compatible results.
func TestConformance(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance.json")
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}

	var suite struct {
		Cases []struct {
			Name          string          `json:"name"`
			Input         json.RawMessage `json:"input"`
			ExpectType    string          `json:"expect_type"`
			ExpectSubtype string          `json:"expect_subtype,omitempty"`
		} `json:"cases"`
	}

	if err := json.Unmarshal(data, &suite); err != nil {
		t.Fatalf("parse fixtures: %v", err)
	}

	for _, tc := range suite.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			msg, err := ParseMessage(tc.Input)
			if err != nil {
				t.Fatalf("ParseMessage failed: %v", err)
			}
			if msg == nil {
				t.Fatal("ParseMessage returned nil")
			}

			gotType := string(msg.messageType())
			if gotType != tc.ExpectType {
				t.Errorf("type = %q, want %q", gotType, tc.ExpectType)
			}

			// Verify subtypes for system and result messages.
			if tc.ExpectSubtype != "" {
				switch m := msg.(type) {
				case *SystemMessage:
					if m.Subtype != tc.ExpectSubtype {
						t.Errorf("subtype = %q, want %q", m.Subtype, tc.ExpectSubtype)
					}
				case *ResultMessage:
					if string(m.Subtype) != tc.ExpectSubtype {
						t.Errorf("subtype = %q, want %q", m.Subtype, tc.ExpectSubtype)
					}
				}
			}

			// Verify specific fields for key message types.
			switch tc.Name {
			case "user_message_with_uuid":
				u := msg.(*UserMessage)
				// UserMessage should parse without error — uuid is in the raw JSON.
				if u == nil {
					t.Fatal("nil user message")
				}

			case "assistant_message_with_thinking":
				a := msg.(*AssistantMessage)
				if len(a.Message.Content) < 2 {
					t.Fatalf("expected >= 2 content blocks, got %d", len(a.Message.Content))
				}
				if a.Message.Content[0].Type != ContentBlockThinking {
					t.Errorf("first block type = %q, want thinking", a.Message.Content[0].Type)
				}
				if a.Message.Content[0].Thinking != "I'm thinking about the answer..." {
					t.Errorf("thinking = %q", a.Message.Content[0].Thinking)
				}
				if a.Message.Content[0].Signature != "sig-123" {
					t.Errorf("signature = %q", a.Message.Content[0].Signature)
				}

			case "assistant_message_with_auth_error":
				a := msg.(*AssistantMessage)
				if a.Error != "authentication_failed" {
					t.Errorf("error = %q, want authentication_failed", a.Error)
				}

			case "result_with_stop_reason":
				r := msg.(*ResultMessage)
				if r.StopReason != "end_turn" {
					t.Errorf("stop_reason = %q, want end_turn", r.StopReason)
				}
				if r.Result != "Done" {
					t.Errorf("result = %q, want Done", r.Result)
				}

			case "task_started":
				s := msg.(*SystemMessage)
				if s.TaskID != "task-abc" {
					t.Errorf("task_id = %q", s.TaskID)
				}
				if s.Description != "Reticulating splines" {
					t.Errorf("description = %q", s.Description)
				}
				if s.TaskType != "background" {
					t.Errorf("task_type = %q", s.TaskType)
				}

			case "task_notification_completed":
				s := msg.(*SystemMessage)
				if s.TaskID != "task-abc" {
					t.Errorf("task_id = %q", s.TaskID)
				}
				if s.Summary != "All done" {
					t.Errorf("summary = %q", s.Summary)
				}
				if s.OutputFile != "/tmp/out.md" {
					t.Errorf("output_file = %q", s.OutputFile)
				}

			case "rate_limit_event":
				r := msg.(*RateLimitEvent)
				if r.RateLimitInfo == nil {
					t.Fatal("rate_limit_info is nil")
				}
				if r.RateLimitInfo.Status != "allowed_warning" {
					t.Errorf("status = %q", r.RateLimitInfo.Status)
				}
				if r.UUID != "abc-123" {
					t.Errorf("uuid = %q", r.UUID)
				}

			case "unknown_type":
				raw, ok := msg.(*RawMessage)
				if !ok {
					t.Fatalf("expected *RawMessage for unknown type, got %T", msg)
				}
				if raw.TypeField != "unknown_future_type" {
					t.Errorf("type = %q", raw.TypeField)
				}
			}
		})
	}
}
