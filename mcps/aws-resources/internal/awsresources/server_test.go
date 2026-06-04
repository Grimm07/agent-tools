package awsresources

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// listRegisteredTools connects a client to the server over in-memory transports
// and returns the set of advertised tool names. Fully offline: it never invokes
// a tool, so no AWS call happens.
func listRegisteredTools(t *testing.T) map[string]bool {
	t.Helper()
	ctx := context.Background()
	clientT, serverT := mcp.NewInMemoryTransports()

	serverSession, err := NewServer().Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { clientSession.Close() })

	res, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	return got
}

// TestServerBuilds verifies NewServer constructs and serves over a transport.
func TestServerBuilds(t *testing.T) {
	if NewServer() == nil {
		t.Fatal("NewServer returned nil")
	}
	_ = listRegisteredTools(t)
}
