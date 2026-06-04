package projectmcpref

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the parsed, validated contents of a project-mcp.toml file.
//
// Root is always resolved to an absolute, symlink-evaluated path; every
// filesystem and exec operation is confined to it.
type Config struct {
	// Root is the project directory the server operates within. In the TOML it
	// is relative to the config file; after LoadConfig it is absolute.
	Root string `toml:"root"`
	// Commands maps a logical command name (e.g. "test") to the exact argv to
	// execute. Each value is an argv array, never a shell string.
	Commands map[string][]string `toml:"commands"`
	// Docs lists the files and directories exposed via the docs tools.
	Docs struct {
		Paths []string `toml:"paths"`
	} `toml:"docs"`
}

// LoadConfig decodes the project-mcp.toml at path, resolves Root to an absolute,
// symlink-evaluated path (relative to the config file's directory, defaulting to
// "."), and verifies that Root exists and is a directory.
func LoadConfig(path string) (Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	}

	rootRel := cfg.Root
	if strings.TrimSpace(rootRel) == "" {
		rootRel = "."
	}

	configDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return Config{}, fmt.Errorf("resolve config dir: %w", err)
	}
	if !filepath.IsAbs(rootRel) {
		rootRel = filepath.Join(configDir, rootRel)
	}

	root, err := filepath.EvalSymlinks(rootRel)
	if err != nil {
		return Config{}, fmt.Errorf("resolve root %q: %w", rootRel, err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return Config{}, fmt.Errorf("stat root %q: %w", root, err)
	}
	if !info.IsDir() {
		return Config{}, fmt.Errorf("root %q is not a directory", root)
	}
	cfg.Root = root

	if cfg.Commands == nil {
		cfg.Commands = map[string][]string{}
	}
	return cfg, nil
}

// confinePath joins rel onto root, evaluates symlinks, and returns the absolute
// path only if it resolves inside root. It defends against ".." traversal and
// symlink escape. root must already be an absolute, symlink-evaluated path.
func confinePath(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q must be relative to the project root", rel)
	}
	joined := filepath.Join(root, rel)

	// Resolve symlinks on the longest existing prefix so that both existing and
	// not-yet-existing (but still confined) paths are handled consistently.
	resolved, err := resolveExisting(joined)
	if err != nil {
		return "", err
	}
	if !withinRoot(root, resolved) {
		return "", fmt.Errorf("path %q escapes project root", rel)
	}
	return resolved, nil
}

// resolveExisting evaluates symlinks on p; if p does not exist, it resolves the
// nearest existing ancestor and re-appends the remaining segments, so that a
// symlinked ancestor cannot be used to escape confinement.
func resolveExisting(p string) (string, error) {
	resolved, err := filepath.EvalSymlinks(p)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(p)
	if parent == p {
		return "", err
	}
	resolvedParent, perr := resolveExisting(parent)
	if perr != nil {
		return "", perr
	}
	return filepath.Join(resolvedParent, filepath.Base(p)), nil
}

// withinRoot reports whether path is root itself or lies beneath it.
func withinRoot(root, path string) bool {
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
