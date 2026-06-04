// Command project-mcp-ref runs the Project MCP Ref MCP server over stdio.
package main

import (
	"context"
	"log"

	"github.com/Grimm07/agent-tools/mcps/project-mcp-ref/internal/projectmcpref"
)

func main() {
	if err := projectmcpref.Run(context.Background()); err != nil {
		log.Fatalf("project-mcp-ref: %v", err)
	}
}
