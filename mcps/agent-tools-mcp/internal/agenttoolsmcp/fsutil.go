package agenttoolsmcp

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

// maxFileBytes caps the bytes returned by ReadFile so a single read cannot
// produce an unbounded response.
const maxFileBytes = 256 * 1024

// maxSearchLineBytes bounds the length of any single matched line returned by
// SearchCode.
const maxSearchLineBytes = 512

// SearchHit is one regex match: the file path (relative to root), 1-based line
// number, and the matching line (trimmed and length-capped).
type SearchHit struct {
	Path  string `json:"path"`
	Line  int    `json:"line"`
	Match string `json:"match"`
}

// ReadFile reads one file located under cfg.Root. The path is confined to the
// root (rejecting ".." traversal and symlink escape) and the returned content is
// capped to maxFileBytes.
func ReadFile(cfg Config, rel string) (string, error) {
	abs, err := confinePath(cfg.Root, rel)
	if err != nil {
		return "", err
	}
	f, err := os.Open(abs)
	if err != nil {
		return "", fmt.Errorf("open %q: %w", rel, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%q is a directory", rel)
	}

	limited := io.LimitReader(f, maxFileBytes)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	out := string(data)
	if info.Size() > maxFileBytes {
		out += "\n... [file truncated]\n"
	}
	return out, nil
}

// SearchCode walks cfg.Root, skipping .git, and returns up to limit lines matching
// the given regular expression. Each hit's Path is relative to the root.
func SearchCode(cfg Config, pattern string, limit int) ([]SearchHit, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}
	if limit <= 0 {
		limit = 100
	}

	var hits []SearchHit
	walkErr := filepath.WalkDir(cfg.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if len(hits) >= limit {
			return filepath.SkipAll
		}
		rel, relErr := filepath.Rel(cfg.Root, path)
		if relErr != nil {
			return nil
		}
		hits = searchFile(path, rel, re, hits, limit)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return hits, nil
}

// searchFile appends matching lines from path (capped) to hits and returns it.
func searchFile(path, rel string, re *regexp.Regexp, hits []SearchHit, limit int) []SearchHit {
	f, err := os.Open(path)
	if err != nil {
		return hits
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		if len(hits) >= limit {
			break
		}
		text := scanner.Text()
		if re.MatchString(text) {
			if len(text) > maxSearchLineBytes {
				text = text[:maxSearchLineBytes] + "…"
			}
			hits = append(hits, SearchHit{Path: rel, Line: line, Match: text})
		}
	}
	return hits
}
