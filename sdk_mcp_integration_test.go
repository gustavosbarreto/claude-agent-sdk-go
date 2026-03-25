package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestNewSdkMcpServer(t *testing.T) {
	tools := []SdkMcpTool{
		{
			Name:        "echo",
			Description: "Echoes back the input text",
			InputSchema: map[string]any{
				"text": map[string]any{"type": "string"},
			},
			Handler: func(ctx context.Context, args map[string]any) ([]ToolContent, error) {
				return []ToolContent{{Type: "text", Text: args["text"].(string)}}, nil
			},
		},
		{
			Name:        "add",
			Description: "Adds two numbers",
			InputSchema: map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			Handler: func(ctx context.Context, args map[string]any) ([]ToolContent, error) {
				return []ToolContent{{Type: "text", Text: "result"}}, nil
			},
		},
	}

	srv := NewSdkMcpServer("test-server", tools...)

	if srv.name != "test-server" {
		t.Errorf("name = %q, want %q", srv.name, "test-server")
	}
	if srv.version != "1.0.0" {
		t.Errorf("version = %q, want %q", srv.version, "1.0.0")
	}
	if srv.server == nil {
		t.Fatal("server is nil")
	}
	if len(srv.tools) != 2 {
		t.Errorf("tools count = %d, want 2", len(srv.tools))
	}
	if srv.tools[0].Name != "echo" {
		t.Errorf("tools[0].name = %q, want %q", srv.tools[0].Name, "echo")
	}
	if srv.tools[1].Name != "add" {
		t.Errorf("tools[1].name = %q, want %q", srv.tools[1].Name, "add")
	}
}

// initializeServer sends the JSON-RPC initialize request so the server
// transitions to an initialized state, which is required before tools/call
// or tools/list can succeed.
func initializeServer(t *testing.T, srv *SdkMcpServer) {
	t.Helper()

	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      0,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-11-25",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}
	initMsg, err := json.Marshal(initReq)
	if err != nil {
		t.Fatalf("marshal initialize request: %v", err)
	}

	_, err = srv.HandleMessage(context.Background(), initMsg)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// Send initialized notification (no id, so no response expected).
	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	notifMsg, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("marshal initialized notification: %v", err)
	}
	_, err = srv.HandleMessage(context.Background(), notifMsg)
	if err != nil {
		t.Fatalf("initialized notification: %v", err)
	}
}

func TestSdkMcpServerHandleMessageCallTool(t *testing.T) {
	srv := NewSdkMcpServer("test-server", SdkMcpTool{
		Name:        "echo",
		Description: "Echoes back the input text",
		InputSchema: map[string]any{
			"text": map[string]any{"type": "string"},
		},
		Handler: func(ctx context.Context, args map[string]any) ([]ToolContent, error) {
			text, _ := args["text"].(string)
			return []ToolContent{{Type: "text", Text: text}}, nil
		},
	})

	initializeServer(t, srv)

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "echo",
			"arguments": map[string]any{"text": "hello world"},
		},
	}
	reqMsg, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	respMsg, err := srv.HandleMessage(context.Background(), reqMsg)
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(respMsg, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want %q", resp["jsonrpc"], "2.0")
	}
	// id may be float64 after JSON round-trip.
	if id, ok := resp["id"].(float64); !ok || id != 1 {
		t.Errorf("id = %v, want 1", resp["id"])
	}

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result is not a map, got %T: %v", resp["result"], resp["result"])
	}

	content, ok := result["content"].([]any)
	if !ok {
		t.Fatalf("content is not an array, got %T", result["content"])
	}
	if len(content) != 1 {
		t.Fatalf("content length = %d, want 1", len(content))
	}

	block, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("content[0] is not a map, got %T", content[0])
	}
	if block["type"] != "text" {
		t.Errorf("content[0].type = %v, want %q", block["type"], "text")
	}
	if block["text"] != "hello world" {
		t.Errorf("content[0].text = %v, want %q", block["text"], "hello world")
	}
}

func TestSdkMcpServerHandleMessageToolError(t *testing.T) {
	srv := NewSdkMcpServer("test-server", SdkMcpTool{
		Name:        "fail",
		Description: "Always fails",
		InputSchema: map[string]any{},
		Handler: func(ctx context.Context, args map[string]any) ([]ToolContent, error) {
			return nil, fmt.Errorf("something went wrong")
		},
	})

	initializeServer(t, srv)

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "fail",
			"arguments": map[string]any{},
		},
	}
	reqMsg, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	respMsg, err := srv.HandleMessage(context.Background(), reqMsg)
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(respMsg, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result is not a map, got %T: %v", resp["result"], resp["result"])
	}

	// The handler error is wrapped as isError=true with the error message as content.
	isError, _ := result["isError"].(bool)
	if !isError {
		t.Error("isError should be true")
	}

	content, ok := result["content"].([]any)
	if !ok {
		t.Fatalf("content is not an array, got %T", result["content"])
	}
	if len(content) != 1 {
		t.Fatalf("content length = %d, want 1", len(content))
	}

	block, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("content[0] is not a map, got %T", content[0])
	}
	if block["text"] != "something went wrong" {
		t.Errorf("content[0].text = %v, want %q", block["text"], "something went wrong")
	}
}

func TestSdkMcpServerHandleMessageMultipleTools(t *testing.T) {
	srv := NewSdkMcpServer("multi-server",
		SdkMcpTool{
			Name:        "greet",
			Description: "Returns a greeting",
			InputSchema: map[string]any{
				"name": map[string]any{"type": "string"},
			},
			Handler: func(ctx context.Context, args map[string]any) ([]ToolContent, error) {
				name, _ := args["name"].(string)
				return []ToolContent{{Type: "text", Text: "Hello, " + name + "!"}}, nil
			},
		},
		SdkMcpTool{
			Name:        "farewell",
			Description: "Returns a farewell",
			InputSchema: map[string]any{
				"name": map[string]any{"type": "string"},
			},
			Handler: func(ctx context.Context, args map[string]any) ([]ToolContent, error) {
				name, _ := args["name"].(string)
				return []ToolContent{{Type: "text", Text: "Goodbye, " + name + "!"}}, nil
			},
		},
	)

	initializeServer(t, srv)

	// Call "greet".
	req1 := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "greet",
			"arguments": map[string]any{"name": "Alice"},
		},
	}
	reqMsg1, _ := json.Marshal(req1)
	respMsg1, err := srv.HandleMessage(context.Background(), reqMsg1)
	if err != nil {
		t.Fatalf("HandleMessage greet: %v", err)
	}

	var resp1 map[string]any
	json.Unmarshal(respMsg1, &resp1)
	result1, _ := resp1["result"].(map[string]any)
	content1, _ := result1["content"].([]any)
	if len(content1) != 1 {
		t.Fatalf("greet content length = %d, want 1", len(content1))
	}
	block1, _ := content1[0].(map[string]any)
	if block1["text"] != "Hello, Alice!" {
		t.Errorf("greet text = %v, want %q", block1["text"], "Hello, Alice!")
	}

	// Call "farewell".
	req2 := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "farewell",
			"arguments": map[string]any{"name": "Bob"},
		},
	}
	reqMsg2, _ := json.Marshal(req2)
	respMsg2, err := srv.HandleMessage(context.Background(), reqMsg2)
	if err != nil {
		t.Fatalf("HandleMessage farewell: %v", err)
	}

	var resp2 map[string]any
	json.Unmarshal(respMsg2, &resp2)
	result2, _ := resp2["result"].(map[string]any)
	content2, _ := result2["content"].([]any)
	if len(content2) != 1 {
		t.Fatalf("farewell content length = %d, want 1", len(content2))
	}
	block2, _ := content2[0].(map[string]any)
	if block2["text"] != "Goodbye, Bob!" {
		t.Errorf("farewell text = %v, want %q", block2["text"], "Goodbye, Bob!")
	}
}

func TestSdkMcpServerHandleMessageInitialize(t *testing.T) {
	srv := NewSdkMcpServer("init-server", SdkMcpTool{
		Name:        "dummy",
		Description: "Dummy tool",
		InputSchema: map[string]any{},
		Handler: func(ctx context.Context, args map[string]any) ([]ToolContent, error) {
			return nil, nil
		},
	})

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-11-25",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}
	reqMsg, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	respMsg, err := srv.HandleMessage(context.Background(), reqMsg)
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(respMsg, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want %q", resp["jsonrpc"], "2.0")
	}
	if id, ok := resp["id"].(float64); !ok || id != 1 {
		t.Errorf("id = %v, want 1", resp["id"])
	}

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result is not a map, got %T: %v", resp["result"], resp["result"])
	}

	// The response should contain protocolVersion and serverInfo.
	if _, ok := result["protocolVersion"].(string); !ok {
		t.Error("protocolVersion missing or not a string")
	}
	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok {
		t.Fatalf("serverInfo is not a map, got %T", result["serverInfo"])
	}
	if serverInfo["name"] != "init-server" {
		t.Errorf("serverInfo.name = %v, want %q", serverInfo["name"], "init-server")
	}

	// Capabilities should be present.
	if _, ok := result["capabilities"].(map[string]any); !ok {
		t.Error("capabilities missing or not a map")
	}
}

func TestSdkMcpServerHandleMessageToolsList(t *testing.T) {
	srv := NewSdkMcpServer("list-server",
		SdkMcpTool{
			Name:        "alpha",
			Description: "First tool",
			InputSchema: map[string]any{
				"x": map[string]any{"type": "string"},
			},
			Handler: func(ctx context.Context, args map[string]any) ([]ToolContent, error) {
				return nil, nil
			},
		},
		SdkMcpTool{
			Name:        "beta",
			Description: "Second tool",
			InputSchema: map[string]any{
				"y": map[string]any{"type": "number"},
			},
			Handler: func(ctx context.Context, args map[string]any) ([]ToolContent, error) {
				return nil, nil
			},
		},
	)

	initializeServer(t, srv)

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/list",
	}
	reqMsg, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	respMsg, err := srv.HandleMessage(context.Background(), reqMsg)
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(respMsg, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want %q", resp["jsonrpc"], "2.0")
	}

	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result is not a map, got %T: %v", resp["result"], resp["result"])
	}

	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools is not an array, got %T", result["tools"])
	}
	if len(tools) != 2 {
		t.Fatalf("tools length = %d, want 2", len(tools))
	}

	// Collect tool names from the response.
	names := make(map[string]bool)
	for _, tool := range tools {
		toolMap, ok := tool.(map[string]any)
		if !ok {
			t.Fatalf("tool is not a map, got %T", tool)
		}
		name, _ := toolMap["name"].(string)
		names[name] = true

		// Each tool should have a description and inputSchema.
		if _, ok := toolMap["description"].(string); !ok {
			t.Errorf("tool %q missing description", name)
		}
		if _, ok := toolMap["inputSchema"].(map[string]any); !ok {
			t.Errorf("tool %q missing inputSchema", name)
		}
	}

	if !names["alpha"] {
		t.Error("tool 'alpha' not found in tools/list response")
	}
	if !names["beta"] {
		t.Error("tool 'beta' not found in tools/list response")
	}
}
