// Package githubaccount implements a read-only GitHub Account MCP server.
//
// It exposes tools over the authenticated user's repositories and code, issues,
// and pull requests, built on the official MCP Go SDK and google/go-github. The
// server is read-only: it only calls go-github Get/List methods.
package githubaccount

import (
	"context"
	"errors"
	"os"

	"github.com/google/go-github/v66/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is reported to MCP clients during initialization.
const version = "0.1.0"

// NewServer builds the MCP server from the GITHUB_TOKEN environment variable.
// It fails fast with a clear error if the token is unset or empty so the server
// never silently runs unauthenticated.
func NewServer() (*mcp.Server, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, errors.New("GITHUB_TOKEN is not set")
	}
	gh := newClient(token, "")
	return newServerWithClient(gh), nil
}

// newServerWithClient registers all tools against gh. It is split out from
// NewServer so tests can inject an httptest-backed client.
func newServerWithClient(gh *github.Client) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "github-account",
		Version: version,
	}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_repos",
		Description: "List the authenticated user's repositories.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ListReposInput) (*mcp.CallToolResult, ListReposOutput, error) {
		return ListRepos(ctx, gh, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_repo",
		Description: "Get metadata for a single repository.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in GetRepoInput) (*mcp.CallToolResult, GetRepoOutput, error) {
		return GetRepo(ctx, gh, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_branches",
		Description: "List the branches of a repository.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ListBranchesInput) (*mcp.CallToolResult, ListBranchesOutput, error) {
		return ListBranches(ctx, gh, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_commits",
		Description: "List the commits of a repository, optionally filtered by ref and path.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ListCommitsInput) (*mcp.CallToolResult, ListCommitsOutput, error) {
		return ListCommits(ctx, gh, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_file",
		Description: "Read a file's decoded contents, or list a directory's entries.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in GetFileInput) (*mcp.CallToolResult, GetFileOutput, error) {
		return GetFile(ctx, gh, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_issues",
		Description: "List a repository's issues (pull requests excluded).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ListIssuesInput) (*mcp.CallToolResult, ListIssuesOutput, error) {
		return ListIssues(ctx, gh, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_issue",
		Description: "Get a single issue.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in GetIssueInput) (*mcp.CallToolResult, GetIssueOutput, error) {
		return GetIssue(ctx, gh, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_issue_comments",
		Description: "List the comments on a single issue.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ListIssueCommentsInput) (*mcp.CallToolResult, ListIssueCommentsOutput, error) {
		return ListIssueComments(ctx, gh, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_pull_requests",
		Description: "List a repository's pull requests.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ListPullRequestsInput) (*mcp.CallToolResult, ListPullRequestsOutput, error) {
		return ListPullRequests(ctx, gh, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_pull_request",
		Description: "Get a single pull request.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in GetPullRequestInput) (*mcp.CallToolResult, GetPullRequestOutput, error) {
		return GetPullRequest(ctx, gh, in)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_pull_request_diff",
		Description: "List the changed files (with patches) of a pull request.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in GetPullRequestDiffInput) (*mcp.CallToolResult, GetPullRequestDiffOutput, error) {
		return GetPullRequestDiff(ctx, gh, in)
	})

	return s
}

// Run serves the MCP server over stdio until ctx is cancelled or the client
// disconnects. It fails fast if GITHUB_TOKEN is missing.
func Run(ctx context.Context) error {
	s, err := NewServer()
	if err != nil {
		return err
	}
	return s.Run(ctx, &mcp.StdioTransport{})
}
