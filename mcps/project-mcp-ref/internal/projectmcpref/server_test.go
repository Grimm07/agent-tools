package projectmcpref

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// writeConfig writes a project-mcp.toml in a fresh temp dir and returns the
// config path and the (resolved) root dir.
func writeConfig(t *testing.T, body string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	resolved, _ := filepath.EvalSymlinks(dir)
	path := filepath.Join(resolved, "project-mcp.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, resolved
}

func TestRunNoConfig(t *testing.T) {
	t.Setenv("PROJECT_MCP_CONFIG", "")
	dir := t.TempDir()
	t.Chdir(dir)
	if err := Run(context.Background()); err == nil {
		t.Fatal("expected error when no config present")
	}
}

func TestFindConfigFromEnv(t *testing.T) {
	path, _ := writeConfig(t, "root = \".\"\n")
	t.Setenv("PROJECT_MCP_CONFIG", path)
	got, err := findConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("got %q, want %q", got, path)
	}
}

func TestNewServerLoads(t *testing.T) {
	path, _ := writeConfig(t, `
root = "."
[commands]
test = ["printf", "ok"]
[docs]
paths = ["README.md"]
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(cfg)
	if srv == nil {
		t.Fatal("nil server")
	}
}

// TestToolsOverTransport wires a client and server through in-memory transports
// and confirms enabled tools are listed and a command tool runs. Fully offline.
func TestToolsOverTransport(t *testing.T) {
	path, root := writeConfig(t, `
root = "."
[commands]
test = ["printf", "ok"]
[docs]
paths = ["README.md"]
`)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	clientT, serverT := mcp.NewInMemoryTransports()
	serverSession, err := NewServer(cfg).Connect(ctx, serverT, nil)
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

	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := map[string]bool{}
	for _, tl := range tools.Tools {
		names[tl.Name] = true
	}
	for _, want := range []string{"run_tests", "search_code", "read_file", "list_docs", "read_doc", "git_status", "git_diff", "git_log"} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
	// Lint/build are unconfigured, so they must NOT be registered.
	if names["run_lint"] || names["run_build"] {
		t.Errorf("disabled command tools should not be registered: %v", names)
	}

	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "run_tests",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call run_tests: %v", err)
	}
	if res.IsError {
		t.Fatalf("run_tests returned IsError: %+v", res)
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok || text.Text != "ok" {
		t.Fatalf("unexpected result: %+v", res.Content)
	}
}

// TestReadFileEscapeOverTransport confirms path confinement surfaces as a tool
// error through the transport.
func TestReadFileEscapeOverTransport(t *testing.T) {
	path, _ := writeConfig(t, "root = \".\"\n")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	clientT, serverT := mcp.NewInMemoryTransports()
	serverSession, err := NewServer(cfg).Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil)
	clientSession, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "read_file",
		Arguments: map[string]any{"path": "../escape"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError for path escape")
	}
}
