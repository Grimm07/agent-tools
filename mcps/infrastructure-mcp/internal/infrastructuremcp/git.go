package infrastructuremcp

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// gitTimeout bounds every git invocation.
const gitTimeout = 30 * time.Second

// maxGitLogCount caps how many commits git_log will return regardless of the
// requested count.
const maxGitLogCount = 100

// gitLogFormat is a record/field-delimited pretty format parsed by GitLog.
// %x1f is the unit separator, %x1e the record separator.
const gitLogFormat = "%H%x1f%an%x1f%aI%x1f%s%x1e"

// StatusEntry is one line of `git status --porcelain`.
type StatusEntry struct {
	// Status is the two-character porcelain status code (e.g. "??", " M").
	Status string `json:"status"`
	Path   string `json:"path"`
}

// Commit is one entry from git_log.
type Commit struct {
	SHA     string `json:"sha"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
}

// GitStatus runs `git status --porcelain` in cfg.Root (read-only) and parses the
// result into entries.
func GitStatus(ctx context.Context, cfg Config) ([]StatusEntry, error) {
	res, err := runGit(ctx, cfg, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	var entries []StatusEntry
	for _, line := range strings.Split(res, "\n") {
		if len(line) < 4 {
			continue
		}
		entries = append(entries, StatusEntry{
			Status: line[:2],
			Path:   strings.TrimSpace(line[3:]),
		})
	}
	return entries, nil
}

// GitDiff runs a read-only `git diff`. When staged is true it diffs the index
// (`--staged`); when path is non-empty the diff is restricted to that pathspec.
// Output is capped.
func GitDiff(ctx context.Context, cfg Config, staged bool, path string) (string, error) {
	args := []string{"diff"}
	if staged {
		args = append(args, "--staged")
	}
	if strings.TrimSpace(path) != "" {
		// Confine the requested path to the project root before passing it on.
		if _, err := confinePath(cfg.Root, path); err != nil {
			return "", err
		}
		args = append(args, "--", path)
	}
	return runGit(ctx, cfg, args...)
}

// GitLog returns up to count recent commits (capped by maxGitLogCount) from
// cfg.Root, newest first.
func GitLog(ctx context.Context, cfg Config, count int) ([]Commit, error) {
	if count <= 0 || count > maxGitLogCount {
		count = maxGitLogCount
	}
	out, err := runGit(ctx, cfg, "log", fmt.Sprintf("-n%d", count), "--pretty=format:"+gitLogFormat)
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for _, record := range strings.Split(out, "\x1e") {
		record = strings.Trim(record, "\n")
		if record == "" {
			continue
		}
		fields := strings.Split(record, "\x1f")
		if len(fields) != 4 {
			continue
		}
		commits = append(commits, Commit{
			SHA:     fields[0],
			Author:  fields[1],
			Date:    fields[2],
			Subject: fields[3],
		})
	}
	return commits, nil
}

// runGit executes a read-only git subcommand in cfg.Root under gitTimeout and
// returns its captured (capped) output. A non-zero git exit becomes an error
// that includes the output, so callers see the failure detail.
func runGit(ctx context.Context, cfg Config, args ...string) (string, error) {
	argv := append([]string{"git"}, args...)
	res, err := execArgv(ctx, cfg.Root, argv, gitTimeout)
	if err != nil {
		return "", err
	}
	if res.TimedOut {
		return "", fmt.Errorf("git %s timed out after %s", strings.Join(args, " "), gitTimeout)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("git %s failed (exit %d): %s", strings.Join(args, " "), res.ExitCode, res.Output)
	}
	return res.Output, nil
}
