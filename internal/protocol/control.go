package protocol

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/shellhub-io/claude-agent-sdk-go/internal/process"
)

// ControlRequest is a request from the CLI to the SDK (or vice versa).
type ControlRequest struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	Request   json.RawMessage `json:"request"`
}

// ControlRequestBody is the inner payload of a control request.
type ControlRequestBody struct {
	Subtype string          `json:"subtype"`
	Data    json.RawMessage `json:"-"`
}

// ParseControlRequest parses a ControlRequest from raw JSON.
func ParseControlRequest(data json.RawMessage) (*ControlRequest, error) {
	var req ControlRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// ControlResponse is a response to a control request.
type ControlResponse struct {
	Type     string              `json:"type"`
	Response ControlResponseBody `json:"response"`
}

// ControlResponseBody is the inner payload of a control response.
type ControlResponseBody struct {
	Subtype   string          `json:"subtype"` // success or error
	RequestID string          `json:"request_id"`
	Response  json.RawMessage `json:"response,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// Mux multiplexes control requests and responses, matching them by request ID.
type Mux struct {
	proc    *process.Process
	mu      sync.Mutex
	pending map[string]chan ControlResponseBody
	counter atomic.Int64
}

// NewMux creates a new control multiplexer.
func NewMux(proc *process.Process) *Mux {
	return &Mux{
		proc:    proc,
		pending: make(map[string]chan ControlResponseBody),
	}
}

// nextID generates a unique request ID.
func (m *Mux) nextID() string {
	n := m.counter.Add(1)
	return fmt.Sprintf("sdk_%d", n)
}

// Send sends a control request to the CLI and waits for the response.
func (m *Mux) Send(subtype string, payload any) (json.RawMessage, error) {
	id := m.nextID()

	ch := make(chan ControlResponseBody, 1)
	m.mu.Lock()
	m.pending[id] = ch
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.pending, id)
		m.mu.Unlock()
	}()

	reqPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request payload: %w", err)
	}

	inner := map[string]any{"subtype": subtype}
	// Merge payload fields into inner.
	var payloadMap map[string]any
	if json.Unmarshal(reqPayload, &payloadMap) == nil {
		for k, v := range payloadMap {
			inner[k] = v
		}
	}

	innerData, _ := json.Marshal(inner)

	req := ControlRequest{
		Type:      "control_request",
		RequestID: id,
		Request:   innerData,
	}

	if err := m.proc.WriteLine(req); err != nil {
		return nil, fmt.Errorf("write control request: %w", err)
	}

	resp := <-ch

	if resp.Subtype == "error" {
		return nil, fmt.Errorf("control error: %s", resp.Error)
	}

	return resp.Response, nil
}

// HandleResponse routes a control response to the waiting sender.
func (m *Mux) HandleResponse(resp ControlResponseBody) {
	m.mu.Lock()
	ch, ok := m.pending[resp.RequestID]
	m.mu.Unlock()

	if ok {
		ch <- resp
	}
}

// SendResponse sends a control response back to the CLI.
func (m *Mux) SendResponse(requestID string, payload any) error {
	respData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp := ControlResponse{
		Type: "control_response",
		Response: ControlResponseBody{
			Subtype:   "success",
			RequestID: requestID,
			Response:  respData,
		},
	}

	return m.proc.WriteLine(resp)
}

// SendErrorResponse sends an error control response back to the CLI.
func (m *Mux) SendErrorResponse(requestID string, errMsg string) error {
	resp := ControlResponse{
		Type: "control_response",
		Response: ControlResponseBody{
			Subtype:   "error",
			RequestID: requestID,
			Error:     errMsg,
		},
	}

	return m.proc.WriteLine(resp)
}
