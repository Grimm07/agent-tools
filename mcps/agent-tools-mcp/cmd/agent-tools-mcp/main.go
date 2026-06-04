// Command agent-tools-mcp runs the Agent Tools MCP MCP server over stdio.
package main

import (
	"context"
	"log"

	"github.com/Grimm07/agent-tools/mcps/agent-tools-mcp/internal/agenttoolsmcp"
)

func main() {
	if err := agenttoolsmcp.Run(context.Background()); err != nil {
		log.Fatalf("agent-tools-mcp: %v", err)
	}
}
