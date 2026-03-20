package claude

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// SdkMcpTool defines an inline MCP tool that runs in the SDK process.
// The handler is called directly when Claude invokes the tool — no subprocess.
type SdkMcpTool struct {
	// Name is the tool name (e.g. "echo").
	// Claude sees it as mcp__<server>__<name>.
	Name string

	// Description explains when to use the tool (shown to Claude).
	Description string

	// InputSchema defines the tool's parameters as JSON Schema properties.
	// Example: map[string]any{"text": map[string]any{"type": "string"}}
	InputSchema map[string]any

	// Handler is called when Claude invokes the tool.
	// Receives the input arguments, returns content blocks.
	Handler func(ctx context.Context, args map[string]any) ([]ToolContent, error)
}

// ToolContent is a content block returned by a tool handler.
type ToolContent struct {
	Type string `json:"type"` // "text" or "image"
	Text string `json:"text,omitempty"`
}

// SdkMcpServer holds an in-process MCP server with its tools.
type SdkMcpServer struct {
	name    string
	version string
	server  *mcpserver.MCPServer
	tools   []SdkMcpTool
}

// NewSdkMcpServer creates an inline MCP server with the given tools.
// Pass this to WithMCPServer to register it with a session.
func NewSdkMcpServer(name string, tools ...SdkMcpTool) *SdkMcpServer {
	srv := mcpserver.NewMCPServer(name, "1.0.0")

	for _, t := range tools {
		// Build JSON Schema for the tool.
		schema := mcp.ToolInputSchema{
			Type:       "object",
			Properties: make(map[string]interface{}),
		}
		for k, v := range t.InputSchema {
			schema.Properties[k] = v
		}

		tool := mcp.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		}

		// Capture handler for closure.
		handler := t.Handler
		srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := make(map[string]any)
			if req.Params.Arguments != nil {
				if m, ok := req.Params.Arguments.(map[string]any); ok {
					args = m
				}
			}

			content, err := handler(ctx, args)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						mcp.NewTextContent(err.Error()),
					},
					IsError: true,
				}, nil
			}

			var mcpContent []mcp.Content
			for _, c := range content {
				mcpContent = append(mcpContent, mcp.NewTextContent(c.Text))
			}

			return &mcp.CallToolResult{
				Content: mcpContent,
			}, nil
		})
	}

	return &SdkMcpServer{
		name:    name,
		version: "1.0.0",
		server:  srv,
		tools:   tools,
	}
}

// HandleMessage processes a JSON-RPC message and returns the response.
func (s *SdkMcpServer) HandleMessage(ctx context.Context, msg json.RawMessage) (json.RawMessage, error) {
	resp := s.server.HandleMessage(ctx, msg)
	return json.Marshal(resp)
}
