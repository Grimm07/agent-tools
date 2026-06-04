package portfoliomcp

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// ListDocs expands cfg.Docs.Paths into the set of doc files exposed by the docs
// tools. Each configured path may be a file or a directory (expanded
// recursively); paths are confined to cfg.Root, and missing entries are
// skipped. Returned paths are relative to cfg.Root and sorted.
func ListDocs(cfg Config) ([]string, error) {
	seen := map[string]struct{}{}
	for _, p := range cfg.Docs.Paths {
		abs, err := confinePath(cfg.Root, p)
		if err != nil {
			// A misconfigured docs entry should not break listing the rest.
			continue
		}
		info, err := os.Stat(abs)
		if err != nil {
			continue // missing entry: skip
		}
		if !info.IsDir() {
			if rel, ok := relUnder(cfg.Root, abs); ok {
				seen[rel] = struct{}{}
			}
			continue
		}
		_ = filepath.WalkDir(abs, func(path string, d fs.DirEntry, werr error) error {
			if werr != nil {
				return nil
			}
			if d.IsDir() {
				if d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if rel, ok := relUnder(cfg.Root, path); ok {
				seen[rel] = struct{}{}
			}
			return nil
		})
	}

	out := make([]string, 0, len(seen))
	for rel := range seen {
		out = append(out, rel)
	}
	sort.Strings(out)
	return out, nil
}

// ReadDoc reads one doc file. The path must be confined to cfg.Root and must
// resolve to a file listed by ListDocs (i.e. under a configured docs path);
// otherwise it returns an error.
func ReadDoc(cfg Config, rel string) (string, error) {
	abs, err := confinePath(cfg.Root, rel)
	if err != nil {
		return "", err
	}
	canon, ok := relUnder(cfg.Root, abs)
	if !ok {
		return "", fmt.Errorf("path %q escapes project root", rel)
	}

	allowed, err := ListDocs(cfg)
	if err != nil {
		return "", err
	}
	for _, d := range allowed {
		if d == canon {
			return ReadFile(cfg, canon)
		}
	}
	return "", fmt.Errorf("%q is not a configured doc path", rel)
}

// relUnder returns abs relative to root, reporting false if it is not under root.
func relUnder(root, abs string) (string, bool) {
	if !withinRoot(root, abs) {
		return "", false
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", false
	}
	return rel, true
}
