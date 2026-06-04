// Command github-account runs the GitHub Account MCP server over stdio.
package main

import (
	"context"
	"log"

	"github.com/Grimm07/agent-tools/mcps/github-account/internal/githubaccount"
)

func main() {
	if err := githubaccount.Run(context.Background()); err != nil {
		log.Fatalf("github-account: %v", err)
	}
}
