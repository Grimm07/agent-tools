package githubaccount

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestGreet exercises the handler directly — no transport, fully offline.
func TestGreet(t *testing.T) {
	res, out, err := Greet(context.Background(), nil, GreetInput{Name: "Ada"})
	if err != nil {
		t.Fatalf("Greet returned error: %v", err)
	}
	if out.Message != "Hello, Ada!" {
		t.Errorf("Message = %q, want %q", out.Message, "Hello, Ada!")
	}
	if len(res.Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(res.Content))
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Content[0] is %T, want *mcp.TextContent", res.Content[0])
	}
	if text.Text != "Hello, Ada!" {
		t.Errorf("Text = %q, want %q", text.Text, "Hello, Ada!")
	}
}

// TestGreetOverTransport wires a client and server through in-memory transports,
// exercising registration + dispatch without stdio. Still fully offline.
func TestGreetOverTransport(t *testing.T) {
	ctx := context.Background()
	clientT, serverT := mcp.NewInMemoryTransports()

	serverSession, err := NewServer().Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer clientSession.Close()

	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "greet",
		Arguments: map[string]any{"name": "Ada"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatal("CallTool returned IsError")
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Content[0] is %T, want *mcp.TextContent", res.Content[0])
	}
	if text.Text != "Hello, Ada!" {
		t.Errorf("Text = %q, want %q", text.Text, "Hello, Ada!")
	}
}
