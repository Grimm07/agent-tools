package githubaccount

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestListReposOverTransport wires a client and server through in-memory
// transports, exercising tool registration + dispatch without stdio. The
// GitHub client is pointed at an httptest server, so it stays fully offline.
func TestListReposOverTransport(t *testing.T) {
	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"name":"app","full_name":"me/app","private":true,
			"description":"d","default_branch":"main","html_url":"u"}]`))
	}))
	defer ghSrv.Close()

	ctx := context.Background()
	clientT, serverT := mcp.NewInMemoryTransports()

	gh := newClient("t", ghSrv.URL+"/")
	serverSession, err := newServerWithClient(gh).Connect(ctx, serverT, nil)
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
		Name:      "list_repos",
		Arguments: map[string]any{"per_page": 30},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("CallTool returned IsError: %+v", res.Content)
	}
	if len(res.Content) == 0 {
		t.Fatal("expected content")
	}
}
