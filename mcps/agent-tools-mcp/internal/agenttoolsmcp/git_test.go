package agenttoolsmcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func newGitRepo(t *testing.T) Config {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	resolved, _ := filepath.EvalSymlinks(root)
	gitRun(t, resolved, "init")
	return Config{Root: resolved}
}

func TestGitStatus(t *testing.T) {
	cfg := newGitRepo(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := GitStatus(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected untracked entry")
	}
	if entries[0].Path != "f.txt" {
		t.Fatalf("entry = %+v", entries[0])
	}
}

func TestGitLog(t *testing.T) {
	cfg := newGitRepo(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cfg.Root, "add", ".")
	gitRun(t, cfg.Root, "commit", "-m", "first commit")

	commits, err := GitLog(context.Background(), cfg, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if commits[0].Subject != "first commit" || commits[0].SHA == "" {
		t.Fatalf("commit = %+v", commits[0])
	}
}

func TestGitDiff(t *testing.T) {
	cfg := newGitRepo(t)
	if err := os.WriteFile(filepath.Join(cfg.Root, "f.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, cfg.Root, "add", ".")
	gitRun(t, cfg.Root, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(cfg.Root, "f.txt"), []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	diff, err := GitDiff(context.Background(), cfg, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}
}

func TestGitLogCount(t *testing.T) {
	cfg := newGitRepo(t)
	for i := 0; i < 3; i++ {
		name := filepath.Join(cfg.Root, "f"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitRun(t, cfg.Root, "add", ".")
		gitRun(t, cfg.Root, "commit", "-m", "c")
	}
	commits, err := GitLog(context.Background(), cfg, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected count cap of 2, got %d", len(commits))
	}
}
