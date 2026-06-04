// Command portfolio-mcp runs the Portfolio MCP MCP server over stdio.
package main

import (
	"context"
	"log"

	"github.com/Grimm07/agent-tools/mcps/portfolio-mcp/internal/portfoliomcp"
)

func main() {
	if err := portfoliomcp.Run(context.Background()); err != nil {
		log.Fatalf("portfolio-mcp: %v", err)
	}
}
