package conformance_test

import (
	"encoding/json"
	"os"
	"testing"

	claude "github.com/gustavosbarreto/claude-agent-sdk-go"
)

// messageType extracts the JSON "type" field from a parsed Message.
// Used instead of the unexported messageType() method.
func messageType(msg claude.Message) string {
	raw, _ := json.Marshal(msg)
	var m struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(raw, &m)
	return m.Type
}

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
			msg, err := claude.ParseMessage(tc.Input)
			if err != nil {
				t.Fatalf("ParseMessage failed: %v", err)
			}
			if msg == nil {
				t.Fatal("ParseMessage returned nil")
			}

			gotType := messageType(msg)
			if gotType != tc.ExpectType {
				t.Errorf("type = %q, want %q", gotType, tc.ExpectType)
			}

			// Verify subtypes for system and result messages.
			if tc.ExpectSubtype != "" {
				switch m := msg.(type) {
				case *claude.SystemMessage:
					if m.Subtype != tc.ExpectSubtype {
						t.Errorf("subtype = %q, want %q", m.Subtype, tc.ExpectSubtype)
					}
				case *claude.TaskStartedMessage:
					if m.Subtype != tc.ExpectSubtype {
						t.Errorf("subtype = %q, want %q", m.Subtype, tc.ExpectSubtype)
					}
				case *claude.TaskProgressMessage:
					if m.Subtype != tc.ExpectSubtype {
						t.Errorf("subtype = %q, want %q", m.Subtype, tc.ExpectSubtype)
					}
				case *claude.TaskNotificationMessage:
					if m.Subtype != tc.ExpectSubtype {
						t.Errorf("subtype = %q, want %q", m.Subtype, tc.ExpectSubtype)
					}
				case *claude.HookEventMessage:
					if m.Subtype != tc.ExpectSubtype {
						t.Errorf("subtype = %q, want %q", m.Subtype, tc.ExpectSubtype)
					}
				case *claude.ResultMessage:
					if string(m.Subtype) != tc.ExpectSubtype {
						t.Errorf("subtype = %q, want %q", m.Subtype, tc.ExpectSubtype)
					}
				}
			}
		})
	}
}

// TestConformance_RoundTrip verifies that all fields from the input JSON
// survive parsing into Go structs. This catches missing struct fields:
// if a field exists in the CLI output but our Go struct doesn't have a
// matching json tag, it will be lost during parse → serialize round-trip.
func TestConformance_RoundTrip(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance.json")
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}

	var suite struct {
		Cases []struct {
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"cases"`
	}

	if err := json.Unmarshal(data, &suite); err != nil {
		t.Fatalf("parse fixtures: %v", err)
	}

	for _, tc := range suite.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			msg, err := claude.ParseMessage(tc.Input)
			if err != nil {
				t.Skipf("parse error (tested elsewhere): %v", err)
			}

			// Skip RawMessage — unknown types aren't round-trippable.
			if _, ok := msg.(*claude.RawMessage); ok {
				t.Skip("unknown message type, skip round-trip")
			}

			// Serialize back to JSON.
			output, err := json.Marshal(msg)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			// Parse both input and output as generic maps.
			var inputMap, outputMap map[string]any
			if err := json.Unmarshal(tc.Input, &inputMap); err != nil {
				t.Fatalf("unmarshal input: %v", err)
			}
			if err := json.Unmarshal(output, &outputMap); err != nil {
				t.Fatalf("unmarshal output: %v", err)
			}

			// Check that every key in the input exists in the output.
			checkFieldsPreserved(t, "", inputMap, outputMap)
		})
	}
}

// checkFieldsPreserved recursively checks that all keys in `input` exist in `output`.
// Skips fields with zero values (null, false, empty array) since Go's omitempty
// drops these — which is acceptable behavior matching the Python SDK.
func checkFieldsPreserved(t *testing.T, prefix string, input, output map[string]any) {
	t.Helper()

	for key, inputVal := range input {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		// Skip zero values that omitempty legitimately drops.
		if isZeroValue(inputVal) {
			continue
		}

		outputVal, exists := output[key]
		if !exists {
			t.Errorf("field %q present in input but missing after round-trip (missing json tag?)", path)
			continue
		}

		// Recurse into nested objects.
		inputObj, inputIsObj := inputVal.(map[string]any)
		outputObj, outputIsObj := outputVal.(map[string]any)
		if inputIsObj && outputIsObj {
			checkFieldsPreserved(t, path, inputObj, outputObj)
		}
	}
}

// isZeroValue returns true for null, false, empty string, empty array, and 0.
func isZeroValue(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case bool:
		return !val
	case string:
		return val == ""
	case float64:
		return val == 0
	case []any:
		return len(val) == 0
	}
	return false
}
