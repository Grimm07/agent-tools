package agenttoolsmcp

import (
	"os"
	"path/filepath"
	"testing"
)

func newDocsConfig(t *testing.T, paths ...string) Config {
	t.Helper()
	root := t.TempDir()
	resolved, _ := filepath.EvalSymlinks(root)
	cfg := Config{Root: resolved}
	cfg.Docs.Paths = paths
	return cfg
}

func TestListDocs(t *testing.T) {
	cfg := newDocsConfig(t, "README.md", "docs")
	mustWrite(t, filepath.Join(cfg.Root, "README.md"), "readme")
	if err := os.MkdirAll(filepath.Join(cfg.Root, "docs", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(cfg.Root, "docs", "a.md"), "a")
	mustWrite(t, filepath.Join(cfg.Root, "docs", "sub", "b.md"), "b")

	docs, err := ListDocs(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// README.md + docs/a.md + docs/sub/b.md = 3 files (dir expanded recursively).
	if len(docs) != 3 {
		t.Fatalf("expected 3 docs, got %d: %+v", len(docs), docs)
	}
}

func TestListDocsSkipsMissing(t *testing.T) {
	cfg := newDocsConfig(t, "README.md", "MISSING.md")
	mustWrite(t, filepath.Join(cfg.Root, "README.md"), "x")
	docs, err := ListDocs(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %+v", docs)
	}
}

func TestReadDoc(t *testing.T) {
	cfg := newDocsConfig(t, "docs")
	if err := os.MkdirAll(filepath.Join(cfg.Root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(cfg.Root, "docs", "guide.md"), "the guide")
	got, err := ReadDoc(cfg, "docs/guide.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "the guide" {
		t.Fatalf("got %q", got)
	}
}

func TestReadDocNotConfigured(t *testing.T) {
	cfg := newDocsConfig(t, "docs")
	mustWrite(t, filepath.Join(cfg.Root, "secret.txt"), "s")
	// secret.txt is under root but not under any configured docs path.
	if _, err := ReadDoc(cfg, "secret.txt"); err == nil {
		t.Fatal("expected error reading non-doc file")
	}
}

func TestReadDocEscape(t *testing.T) {
	cfg := newDocsConfig(t, "docs")
	if _, err := ReadDoc(cfg, "../escape"); err == nil {
		t.Fatal("expected error for path escape")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
