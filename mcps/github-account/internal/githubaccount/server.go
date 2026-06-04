// Package githubaccount implements the GitHub Account MCP server.
//
// A go-mcp unit.
package githubaccount

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is reported to MCP clients during initialization.
const version = "0.1.0"

// GreetInput is the argument schema for the "greet" tool. Struct tags drive the
// JSON Schema that the SDK advertises to clients: `json` names the field and
// `jsonschema` documents it.
type GreetInput struct {
	Name string `json:"name" jsonschema:"the name to greet"`
}

// GreetOutput is the structured result of the "greet" tool.
type GreetOutput struct {
	Message string `json:"message"`
}

// Greet handles the "greet" tool call. It is a plain function (not a closure
// over server state) so it can be unit-tested directly, without a transport.
func Greet(ctx context.Context, req *mcp.CallToolRequest, in GreetInput) (*mcp.CallToolResult, GreetOutput, error) {
	msg := "Hello, " + in.Name + "!"
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}, GreetOutput{Message: msg}, nil
}

// NewServer builds the MCP server with all tools registered. Schemas are
// derived automatically from each handler's input/output struct.
func NewServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "github-account",
		Version: version,
	}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "greet",
		Description: "Greet someone by name.",
	}, Greet)

	return s
}

// Run serves the MCP server over stdio until ctx is cancelled or the client
// disconnects.
func Run(ctx context.Context) error {
	return NewServer().Run(ctx, &mcp.StdioTransport{})
}
