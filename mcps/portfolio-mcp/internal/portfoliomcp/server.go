// Package portfoliomcp implements the Portfolio MCP MCP server.
//
// It exposes a single software project's commands (test/lint/build), code
// search/read, docs, and read-only git state to an agent over stdio. All
// behavior is driven by a committed project-mcp.toml; see config.go.
package portfoliomcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is reported to MCP clients during initialization.
const version = "0.1.0"

// configFileName is the default config file name searched for in the working
// directory when PROJECT_MCP_CONFIG is unset.
const configFileName = "project-mcp.toml"

// runCommandTimeout bounds the test/lint/build command tools.
const runCommandTimeout = 5 * time.Minute

// ---- Tool input/output schemas ----

// RunOutput is the structured result of a command tool.
type RunOutput struct {
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
	TimedOut bool   `json:"timed_out"`
}

// SearchInput is the argument schema for "search_code".
type SearchInput struct {
	Pattern string `json:"pattern" jsonschema:"regular expression to search for"`
	Limit   int    `json:"limit,omitempty" jsonschema:"max number of hits (default 100)"`
}

// SearchOutput is the structured result of "search_code".
type SearchOutput struct {
	Hits []SearchHit `json:"hits"`
}

// ReadFileInput is the argument schema for "read_file".
type ReadFileInput struct {
	Path string `json:"path" jsonschema:"file path relative to the project root"`
}

// ReadOutput carries file/doc contents.
type ReadOutput struct {
	Content string `json:"content"`
}

// ListDocsOutput lists the available doc paths.
type ListDocsOutput struct {
	Docs []string `json:"docs"`
}

// ReadDocInput is the argument schema for "read_doc".
type ReadDocInput struct {
	Path string `json:"path" jsonschema:"doc path relative to the project root"`
}

// GitStatusOutput is the structured result of "git_status".
type GitStatusOutput struct {
	Entries []StatusEntry `json:"entries"`
}

// GitDiffInput is the argument schema for "git_diff".
type GitDiffInput struct {
	Staged bool   `json:"staged,omitempty" jsonschema:"diff the staged index instead of the working tree"`
	Path   string `json:"path,omitempty" jsonschema:"restrict the diff to this path (relative to root)"`
}

// GitDiffOutput carries the diff text.
type GitDiffOutput struct {
	Diff string `json:"diff"`
}

// GitLogInput is the argument schema for "git_log".
type GitLogInput struct {
	Count int `json:"count,omitempty" jsonschema:"number of commits to return (default/cap applies)"`
}

// GitLogOutput is the structured result of "git_log".
type GitLogOutput struct {
	Commits []Commit `json:"commits"`
}

// textResult wraps a string as a CallToolResult with a single text block.
func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

// NewServer loads cfg and builds the MCP server, registering the code, docs, and
// git tools always, and each command tool only when its config entry is set.
func NewServer(cfg Config) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "portfolio-mcp",
		Version: version,
	}, nil)

	registerCommandTools(s, cfg)
	registerCodeTools(s, cfg)
	registerDocsTools(s, cfg)
	registerGitTools(s, cfg)
	return s
}

// registerCommandTools registers run_tests/run_lint/run_build, but only for the
// commands actually configured (non-empty argv), so disabled commands are not
// advertised to clients.
func registerCommandTools(s *mcp.Server, cfg Config) {
	type cmdTool struct {
		tool, name, desc string
	}
	for _, ct := range []cmdTool{
		{"run_tests", "test", "Run the project's configured test command."},
		{"run_lint", "lint", "Run the project's configured lint command."},
		{"run_build", "build", "Run the project's configured build command."},
	} {
		if len(cfg.Commands[ct.name]) == 0 {
			continue
		}
		name := ct.name // capture
		mcp.AddTool(s, &mcp.Tool{Name: ct.tool, Description: ct.desc},
			func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, RunOutput, error) {
				res, err := RunCommand(ctx, cfg, name, runCommandTimeout)
				if err != nil {
					return nil, RunOutput{}, err
				}
				out := RunOutput{ExitCode: res.ExitCode, Output: res.Output, TimedOut: res.TimedOut}
				return textResult(res.Output), out, nil
			})
	}
}

// registerCodeTools registers search_code and read_file.
func registerCodeTools(s *mcp.Server, cfg Config) {
	mcp.AddTool(s, &mcp.Tool{Name: "search_code", Description: "Regex-search files under the project root."},
		func(_ context.Context, _ *mcp.CallToolRequest, in SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
			hits, err := SearchCode(cfg, in.Pattern, in.Limit)
			if err != nil {
				return nil, SearchOutput{}, err
			}
			return textResult(fmt.Sprintf("%d hit(s)", len(hits))), SearchOutput{Hits: hits}, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "read_file", Description: "Read one file under the project root."},
		func(_ context.Context, _ *mcp.CallToolRequest, in ReadFileInput) (*mcp.CallToolResult, ReadOutput, error) {
			content, err := ReadFile(cfg, in.Path)
			if err != nil {
				return nil, ReadOutput{}, err
			}
			return textResult(content), ReadOutput{Content: content}, nil
		})
}

// registerDocsTools registers list_docs and read_doc.
func registerDocsTools(s *mcp.Server, cfg Config) {
	mcp.AddTool(s, &mcp.Tool{Name: "list_docs", Description: "List the project's configured doc files."},
		func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ListDocsOutput, error) {
			docs, err := ListDocs(cfg)
			if err != nil {
				return nil, ListDocsOutput{}, err
			}
			return textResult(fmt.Sprintf("%d doc(s)", len(docs))), ListDocsOutput{Docs: docs}, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "read_doc", Description: "Read one configured doc file."},
		func(_ context.Context, _ *mcp.CallToolRequest, in ReadDocInput) (*mcp.CallToolResult, ReadOutput, error) {
			content, err := ReadDoc(cfg, in.Path)
			if err != nil {
				return nil, ReadOutput{}, err
			}
			return textResult(content), ReadOutput{Content: content}, nil
		})
}

// registerGitTools registers git_status, git_diff, and git_log (read-only).
func registerGitTools(s *mcp.Server, cfg Config) {
	mcp.AddTool(s, &mcp.Tool{Name: "git_status", Description: "Show `git status --porcelain` entries."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, GitStatusOutput, error) {
			entries, err := GitStatus(ctx, cfg)
			if err != nil {
				return nil, GitStatusOutput{}, err
			}
			return textResult(fmt.Sprintf("%d change(s)", len(entries))), GitStatusOutput{Entries: entries}, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "git_diff", Description: "Show a read-only git diff."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in GitDiffInput) (*mcp.CallToolResult, GitDiffOutput, error) {
			diff, err := GitDiff(ctx, cfg, in.Staged, in.Path)
			if err != nil {
				return nil, GitDiffOutput{}, err
			}
			return textResult(diff), GitDiffOutput{Diff: diff}, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "git_log", Description: "Show recent commits (sha, author, date, subject)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in GitLogInput) (*mcp.CallToolResult, GitLogOutput, error) {
			commits, err := GitLog(ctx, cfg, in.Count)
			if err != nil {
				return nil, GitLogOutput{}, err
			}
			return textResult(fmt.Sprintf("%d commit(s)", len(commits))), GitLogOutput{Commits: commits}, nil
		})
}

// findConfig returns the path to project-mcp.toml: PROJECT_MCP_CONFIG if set,
// otherwise configFileName in the current working directory.
func findConfig() (string, error) {
	if p := os.Getenv("PROJECT_MCP_CONFIG"); p != "" {
		return p, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	path := filepath.Join(wd, configFileName)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("no %s found (set PROJECT_MCP_CONFIG): %w", configFileName, err)
	}
	return path, nil
}

// Run loads the config and serves the MCP server over stdio until ctx is
// cancelled or the client disconnects. A config error aborts startup.
func Run(ctx context.Context) error {
	path, err := findConfig()
	if err != nil {
		return err
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		return err
	}
	return NewServer(cfg).Run(ctx, &mcp.StdioTransport{})
}
