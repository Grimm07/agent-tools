// Command aws-resources runs the AWS Resources MCP server over stdio.
package main

import (
	"context"
	"log"

	"github.com/Grimm07/agent-tools/mcps/aws-resources/internal/awsresources"
)

func main() {
	if err := awsresources.Run(context.Background()); err != nil {
		log.Fatalf("aws-resources: %v", err)
	}
}
