package projectmcpref

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "project-mcp.toml"), []byte(`
root = "."
[commands]
test = ["go", "test", "./..."]
[docs]
paths = ["README.md"]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(filepath.Join(dir, "project-mcp.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Commands["test"][0] != "go" {
		t.Fatalf("got %+v", cfg.Commands)
	}
	if !filepath.IsAbs(cfg.Root) {
		t.Fatalf("root must resolve absolute, got %q", cfg.Root)
	}
	if len(cfg.Docs.Paths) != 1 || cfg.Docs.Paths[0] != "README.md" {
		t.Fatalf("docs paths = %+v", cfg.Docs.Paths)
	}
}

func TestLoadConfigDefaultRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "project-mcp.toml"), []byte("[commands]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(filepath.Join(dir, "project-mcp.toml"))
	if err != nil {
		t.Fatal(err)
	}
	// Default root "." resolves to the config file's directory.
	want, _ := filepath.EvalSymlinks(dir)
	if cfg.Root != want {
		t.Fatalf("Root = %q, want %q", cfg.Root, want)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	if _, err := LoadConfig(filepath.Join(t.TempDir(), "nope.toml")); err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestConfinePath(t *testing.T) {
	root := t.TempDir()
	resolvedRoot, _ := filepath.EvalSymlinks(root)
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := confinePath(resolvedRoot, "ok.txt")
	if err != nil {
		t.Fatalf("confinePath(ok.txt) error: %v", err)
	}
	if got != filepath.Join(resolvedRoot, "ok.txt") {
		t.Fatalf("got %q", got)
	}

	if _, err := confinePath(resolvedRoot, "../etc/passwd"); err == nil {
		t.Fatal("expected error for ../ traversal")
	}
}

func TestConfinePathSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	resolvedRoot, _ := filepath.EvalSymlinks(root)
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("s"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if _, err := confinePath(resolvedRoot, "link/secret.txt"); err == nil {
		t.Fatal("expected error for symlink escape")
	}
}
