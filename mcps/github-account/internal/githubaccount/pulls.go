package githubaccount

import (
	"context"

	"github.com/google/go-github/v66/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// PullRequest is the trimmed projection of a pull request in a list.
type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Base   string `json:"base"`
	Head   string `json:"head"`
	Author string `json:"author"`
	URL    string `json:"url"`
}

// ListPullRequestsInput is the argument schema for list_pull_requests.
type ListPullRequestsInput struct {
	Owner   string `json:"owner" jsonschema:"repository owner (user or org)"`
	Repo    string `json:"repo" jsonschema:"repository name"`
	State   string `json:"state,omitempty" jsonschema:"open|closed|all (default open)"`
	Base    string `json:"base,omitempty" jsonschema:"filter by base branch name"`
	Head    string `json:"head,omitempty" jsonschema:"filter by head in user:ref-name form"`
	Page    int    `json:"page,omitempty" jsonschema:"1-based page number"`
	PerPage int    `json:"per_page,omitempty" jsonschema:"results per page (max 100)"`
}

// ListPullRequestsOutput is the structured result of list_pull_requests.
type ListPullRequestsOutput struct {
	PullRequests []PullRequest `json:"pull_requests"`
	HasMore      bool          `json:"has_more"`
}

// ListPullRequests lists a repository's pull requests.
func ListPullRequests(ctx context.Context, gh *github.Client, in ListPullRequestsInput) (*mcp.CallToolResult, ListPullRequestsOutput, error) {
	opt := &github.PullRequestListOptions{
		State:       in.State,
		Base:        in.Base,
		Head:        in.Head,
		ListOptions: github.ListOptions{Page: in.Page, PerPage: clampPerPage(in.PerPage)},
	}
	prs, resp, err := gh.PullRequests.List(ctx, in.Owner, in.Repo, opt)
	if err != nil {
		return nil, ListPullRequestsOutput{}, mapGitHubError(err)
	}
	out := ListPullRequestsOutput{HasMore: resp.NextPage != 0}
	for _, p := range prs {
		out.PullRequests = append(out.PullRequests, PullRequest{
			Number: p.GetNumber(),
			Title:  p.GetTitle(),
			State:  p.GetState(),
			Base:   p.GetBase().GetRef(),
			Head:   p.GetHead().GetRef(),
			Author: p.GetUser().GetLogin(),
			URL:    p.GetHTMLURL(),
		})
	}
	return textResult(out), out, nil
}

// GetPullRequestInput is the argument schema for get_pull_request.
type GetPullRequestInput struct {
	Owner  string `json:"owner" jsonschema:"repository owner (user or org)"`
	Repo   string `json:"repo" jsonschema:"repository name"`
	Number int    `json:"number" jsonschema:"pull request number"`
}

// GetPullRequestOutput is the structured result of get_pull_request.
type GetPullRequestOutput struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Body   string `json:"body"`
	Base   string `json:"base"`
	Head   string `json:"head"`
	Merged bool   `json:"merged"`
	URL    string `json:"url"`
}

// GetPullRequest fetches a single pull request.
func GetPullRequest(ctx context.Context, gh *github.Client, in GetPullRequestInput) (*mcp.CallToolResult, GetPullRequestOutput, error) {
	p, _, err := gh.PullRequests.Get(ctx, in.Owner, in.Repo, in.Number)
	if err != nil {
		return nil, GetPullRequestOutput{}, mapGitHubError(err)
	}
	out := GetPullRequestOutput{
		Number: p.GetNumber(),
		Title:  p.GetTitle(),
		State:  p.GetState(),
		Body:   p.GetBody(),
		Base:   p.GetBase().GetRef(),
		Head:   p.GetHead().GetRef(),
		Merged: p.GetMerged(),
		URL:    p.GetHTMLURL(),
	}
	return textResult(out), out, nil
}

// PullRequestFile is the trimmed projection of one changed file in a PR.
type PullRequestFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch,omitempty"`
}

// GetPullRequestDiffInput is the argument schema for get_pull_request_diff.
type GetPullRequestDiffInput struct {
	Owner   string `json:"owner" jsonschema:"repository owner (user or org)"`
	Repo    string `json:"repo" jsonschema:"repository name"`
	Number  int    `json:"number" jsonschema:"pull request number"`
	Page    int    `json:"page,omitempty" jsonschema:"1-based page number"`
	PerPage int    `json:"per_page,omitempty" jsonschema:"results per page (max 100)"`
}

// GetPullRequestDiffOutput is the structured result of get_pull_request_diff.
type GetPullRequestDiffOutput struct {
	Files   []PullRequestFile `json:"files"`
	HasMore bool              `json:"has_more"`
}

// GetPullRequestDiff lists the changed files (with patches) of a pull request.
func GetPullRequestDiff(ctx context.Context, gh *github.Client, in GetPullRequestDiffInput) (*mcp.CallToolResult, GetPullRequestDiffOutput, error) {
	opt := &github.ListOptions{Page: in.Page, PerPage: clampPerPage(in.PerPage)}
	files, resp, err := gh.PullRequests.ListFiles(ctx, in.Owner, in.Repo, in.Number, opt)
	if err != nil {
		return nil, GetPullRequestDiffOutput{}, mapGitHubError(err)
	}
	out := GetPullRequestDiffOutput{HasMore: resp.NextPage != 0}
	for _, f := range files {
		out.Files = append(out.Files, PullRequestFile{
			Filename:  f.GetFilename(),
			Status:    f.GetStatus(),
			Additions: f.GetAdditions(),
			Deletions: f.GetDeletions(),
			Patch:     f.GetPatch(),
		})
	}
	return textResult(out), out, nil
}
