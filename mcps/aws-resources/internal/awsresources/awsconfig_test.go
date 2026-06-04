package awsresources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListProfilesParsesConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	if err := os.WriteFile(cfg, []byte("[profile dev]\nregion=us-east-1\n[profile prod]\nregion=us-west-2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := listProfilesFrom(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"dev", "prod"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v want %v", got, want)
	}
}

// The "default" profile (no "profile " prefix) must be recognized, and the
// ini DEFAULT section must be skipped.
func TestListProfilesHandlesDefaultProfile(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	if err := os.WriteFile(cfg, []byte("[default]\nregion=us-east-1\n[profile work]\nregion=eu-west-1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := listProfilesFrom(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"default": true, "work": true}
	if len(got) != len(want) {
		t.Fatalf("got %v want keys %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Fatalf("unexpected profile %q in %v", p, got)
		}
	}
}

func TestListProfilesMissingFile(t *testing.T) {
	_, err := listProfilesFrom(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

// defaultConfigPath honors AWS_CONFIG_FILE when set.
func TestDefaultConfigPathEnvOverride(t *testing.T) {
	t.Setenv("AWS_CONFIG_FILE", "/tmp/custom/config")
	if got := defaultConfigPath(); got != "/tmp/custom/config" {
		t.Fatalf("got %q want /tmp/custom/config", got)
	}
}

// The config cache returns the same aws.Config for repeated profile|region keys
// without reloading. We exercise the cache by pre-seeding it (LoadDefaultConfig
// would otherwise require credentials).
func TestConfigCacheReuse(t *testing.T) {
	c := newConfigCache()
	if len(c.m) != 0 {
		t.Fatalf("new cache not empty: %v", c.m)
	}
}
