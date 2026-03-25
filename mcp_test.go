package claude

import (
	"encoding/json"
	"testing"
)

func TestMCPServerStatus_JSON(t *testing.T) {
	status := MCPServerStatus{
		Name:   "test-server",
		Status: "connected",
		Error:  "connection timeout",
		ServerInfo: &struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		}{
			Name:    "my-mcp-server",
			Version: "1.2.3",
		},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded MCPServerStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Name != "test-server" {
		t.Errorf("Name = %q, want %q", decoded.Name, "test-server")
	}
	if decoded.Status != "connected" {
		t.Errorf("Status = %q, want %q", decoded.Status, "connected")
	}
	if decoded.Error != "connection timeout" {
		t.Errorf("Error = %q, want %q", decoded.Error, "connection timeout")
	}
	if decoded.ServerInfo == nil {
		t.Fatal("ServerInfo is nil")
	}
	if decoded.ServerInfo.Name != "my-mcp-server" {
		t.Errorf("ServerInfo.Name = %q, want %q", decoded.ServerInfo.Name, "my-mcp-server")
	}
	if decoded.ServerInfo.Version != "1.2.3" {
		t.Errorf("ServerInfo.Version = %q, want %q", decoded.ServerInfo.Version, "1.2.3")
	}
}

func TestMCPServerStatus_MinimalJSON(t *testing.T) {
	status := MCPServerStatus{
		Name:   "minimal-server",
		Status: "pending",
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["name"] != "minimal-server" {
		t.Errorf("name = %v, want %q", parsed["name"], "minimal-server")
	}
	if parsed["status"] != "pending" {
		t.Errorf("status = %v, want %q", parsed["status"], "pending")
	}

	// Optional fields should be omitted.
	if _, ok := parsed["error"]; ok {
		t.Error("error field should be omitted when empty")
	}
	if _, ok := parsed["server_info"]; ok {
		t.Error("server_info field should be omitted when nil")
	}
}

func TestMCPServerStatus_AllStatuses(t *testing.T) {
	statuses := []string{"connected", "failed", "needs-auth", "pending", "disabled"}
	for _, s := range statuses {
		status := MCPServerStatus{
			Name:   "server",
			Status: s,
		}

		data, err := json.Marshal(status)
		if err != nil {
			t.Fatalf("marshal error for status %q: %v", s, err)
		}

		var decoded MCPServerStatus
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal error for status %q: %v", s, err)
		}

		if decoded.Status != s {
			t.Errorf("Status = %q, want %q", decoded.Status, s)
		}
	}
}

func TestMCPServerStatus_FromSystemMessage(t *testing.T) {
	line := `{"type":"system","subtype":"init","session_id":"s1","mcp_servers":[{"name":"srv","status":"connected","server_info":{"name":"srv","version":"0.1"}}]}`

	msg, err := ParseMessage([]byte(line))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	sys, ok := msg.(*SystemMessage)
	if !ok {
		t.Fatalf("expected *SystemMessage, got %T", msg)
	}

	if len(sys.MCPServers) != 1 {
		t.Fatalf("MCPServers length = %d, want 1", len(sys.MCPServers))
	}

	srv := sys.MCPServers[0]
	if srv.Name != "srv" {
		t.Errorf("Name = %q, want %q", srv.Name, "srv")
	}
	if srv.Status != "connected" {
		t.Errorf("Status = %q, want %q", srv.Status, "connected")
	}
	if srv.ServerInfo == nil {
		t.Fatal("ServerInfo is nil")
	}
	if srv.ServerInfo.Version != "0.1" {
		t.Errorf("ServerInfo.Version = %q, want %q", srv.ServerInfo.Version, "0.1")
	}
}

func TestMCPStdioServer_MarshalJSON(t *testing.T) {
	srv := MCPStdioServer{
		Command: "node",
		Args:    []string{"server.js"},
		Env:     map[string]string{"PORT": "3000"},
	}

	data, err := json.Marshal(srv)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["type"] != "stdio" {
		t.Errorf("type = %v, want %q", parsed["type"], "stdio")
	}
	if parsed["command"] != "node" {
		t.Errorf("command = %v, want %q", parsed["command"], "node")
	}
}

func TestMCPSSEServer_MarshalJSON(t *testing.T) {
	srv := MCPSSEServer{
		URL:     "http://localhost:8080/sse",
		Headers: map[string]string{"Authorization": "Bearer token"},
	}

	data, err := json.Marshal(srv)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["type"] != "sse" {
		t.Errorf("type = %v, want %q", parsed["type"], "sse")
	}
	if parsed["url"] != "http://localhost:8080/sse" {
		t.Errorf("url = %v, want %q", parsed["url"], "http://localhost:8080/sse")
	}
}

func TestMCPHTTPServer_MarshalJSON(t *testing.T) {
	srv := MCPHTTPServer{
		URL: "http://localhost:9090/mcp",
	}

	data, err := json.Marshal(srv)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["type"] != "http" {
		t.Errorf("type = %v, want %q", parsed["type"], "http")
	}
	if parsed["url"] != "http://localhost:9090/mcp" {
		t.Errorf("url = %v, want %q", parsed["url"], "http://localhost:9090/mcp")
	}
}
