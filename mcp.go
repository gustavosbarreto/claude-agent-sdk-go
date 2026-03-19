package claude

// MCPServerConfig is the interface for MCP server configurations.
type MCPServerConfig interface {
	mcpType() string
}

// MCPStdioServer configures an MCP server that communicates via stdio.
type MCPStdioServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

func (s MCPStdioServer) mcpType() string { return "stdio" }

// MarshalJSON implements custom JSON marshaling to include the type field.
func (s MCPStdioServer) MarshalJSON() ([]byte, error) {
	type alias MCPStdioServer
	return marshalWithType("stdio", alias(s))
}

// MCPSSEServer configures an MCP server using Server-Sent Events.
type MCPSSEServer struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (s MCPSSEServer) mcpType() string { return "sse" }

func (s MCPSSEServer) MarshalJSON() ([]byte, error) {
	type alias MCPSSEServer
	return marshalWithType("sse", alias(s))
}

// MCPHTTPServer configures an MCP server using HTTP.
type MCPHTTPServer struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (s MCPHTTPServer) mcpType() string { return "http" }

func (s MCPHTTPServer) MarshalJSON() ([]byte, error) {
	type alias MCPHTTPServer
	return marshalWithType("http", alias(s))
}
