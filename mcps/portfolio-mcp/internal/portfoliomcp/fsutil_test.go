package portfoliomcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newFSConfig(t *testing.T) Config {
	t.Helper()
	root := t.TempDir()
	resolved, _ := filepath.EvalSymlinks(root)
	return Config{Root: resolved}
}

func TestReadFile(t *testing.T) {
	cfg := newFSConfig(t)
	if err := os.MkdirAll(filepath.Join(cfg.Root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Root, "sub", "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(cfg, "sub/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestReadFileEscape(t *testing.T) {
	cfg := newFSConfig(t)
	if _, err := ReadFile(cfg, "../escape"); err == nil {
		t.Fatal("expected error for path escape")
	}
}

func TestReadFileSizeCap(t *testing.T) {
	cfg := newFSConfig(t)
	big := strings.Repeat("a", maxFileBytes+1000)
	if err := os.WriteFile(filepath.Join(cfg.Root, "big.txt"), []byte(big), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(cfg, "big.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > maxFileBytes+200 {
		t.Fatalf("file not capped: %d bytes", len(got))
	}
}

func TestSearchCode(t *testing.T) {
	cfg := newFSConfig(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "a.go"), []byte("package x\n// needle here\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.Root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Root, ".git", "config"), []byte("needle"), 0o600); err != nil {
		t.Fatal(err)
	}
	hits, err := SearchCode(cfg, "needle", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit (.git skipped), got %d: %+v", len(hits), hits)
	}
	h := hits[0]
	if h.Path != "a.go" || h.Line != 2 || !strings.Contains(h.Match, "needle") {
		t.Fatalf("hit = %+v", h)
	}
}

func TestSearchCodeCap(t *testing.T) {
	cfg := newFSConfig(t)
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString("needle\n")
	}
	if err := os.WriteFile(filepath.Join(cfg.Root, "many.txt"), []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	hits, err := SearchCode(cfg, "needle", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 10 {
		t.Fatalf("expected cap of 10 hits, got %d", len(hits))
	}
}

func TestSearchCodeBadRegex(t *testing.T) {
	cfg := newFSConfig(t)
	if _, err := SearchCode(cfg, "(", 10); err == nil {
		t.Fatal("expected error for invalid regex")
	}
}
