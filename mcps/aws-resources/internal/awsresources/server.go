// Package awsresources implements the AWS Resources MCP server.
//
// A read-only Model Context Protocol server that lets an agent view live AWS
// resources (compute, storage & databases, networking, identity & cost) across
// the owner's accounts, targeting multiple named profiles from ~/.aws/config.
//
// Every tool calls only read-only AWS APIs (Describe*/List*/Get*). No mutating
// client method is referenced anywhere in this unit.
package awsresources

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is reported to MCP clients during initialization.
const version = "0.1.0"

// NewServer builds the MCP server with all tools registered. Schemas are
// derived automatically from each handler's input/output struct.
//
// Service clients are constructed lazily inside each handler from an aws.Config
// cached per profile|region, so no AWS calls happen until a tool is invoked.
func NewServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "aws-resources",
		Version: version,
	}, nil)

	cache := newConfigCache()
	registerHelperTools(s, cache)
	registerComputeTools(s, cache)
	registerStorageTools(s, cache)
	registerNetworkTools(s, cache)
	registerIdentityTools(s, cache)
	registerCostTools(s, cache)
	return s
}

// Run serves the MCP server over stdio until ctx is cancelled or the client
// disconnects.
func Run(ctx context.Context) error {
	return NewServer().Run(ctx, &mcp.StdioTransport{})
}
