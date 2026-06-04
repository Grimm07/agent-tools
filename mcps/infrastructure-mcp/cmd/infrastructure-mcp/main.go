// Command infrastructure-mcp runs the Infrastructure MCP MCP server over stdio.
package main

import (
	"context"
	"log"

	"github.com/Grimm07/agent-tools/mcps/infrastructure-mcp/internal/infrastructuremcp"
)

func main() {
	if err := infrastructuremcp.Run(context.Background()); err != nil {
		log.Fatalf("infrastructure-mcp: %v", err)
	}
}
