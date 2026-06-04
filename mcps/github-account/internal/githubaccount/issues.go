package githubaccount

import (
	"context"

	"github.com/google/go-github/v66/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Issue is the trimmed projection of an issue in a list.
type Issue struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	State  string   `json:"state"`
	Labels []string `json:"labels"`
	Author string   `json:"author"`
	URL    string   `json:"url"`
}

func labelNames(labels []*github.Label) []string {
	var out []string
	for _, l := range labels {
		out = append(out, l.GetName())
	}
	return out
}

// ListIssuesInput is the argument schema for list_issues.
type ListIssuesInput struct {
	Owner    string   `json:"owner" jsonschema:"repository owner (user or org)"`
	Repo     string   `json:"repo" jsonschema:"repository name"`
	State    string   `json:"state,omitempty" jsonschema:"open|closed|all (default open)"`
	Labels   []string `json:"labels,omitempty" jsonschema:"filter by label names"`
	Assignee string   `json:"assignee,omitempty" jsonschema:"filter by assignee login, none, or *"`
	Page     int      `json:"page,omitempty" jsonschema:"1-based page number"`
	PerPage  int      `json:"per_page,omitempty" jsonschema:"results per page (max 100)"`
}

// ListIssuesOutput is the structured result of list_issues.
type ListIssuesOutput struct {
	Issues  []Issue `json:"issues"`
	HasMore bool    `json:"has_more"`
}

// ListIssues lists a repository's issues, excluding pull requests (GitHub's
// issues endpoint returns PRs too; they are filtered out here).
func ListIssues(ctx context.Context, gh *github.Client, in ListIssuesInput) (*mcp.CallToolResult, ListIssuesOutput, error) {
	opt := &github.IssueListByRepoOptions{
		State:       in.State,
		Labels:      in.Labels,
		Assignee:    in.Assignee,
		ListOptions: github.ListOptions{Page: in.Page, PerPage: clampPerPage(in.PerPage)},
	}
	issues, resp, err := gh.Issues.ListByRepo(ctx, in.Owner, in.Repo, opt)
	if err != nil {
		return nil, ListIssuesOutput{}, mapGitHubError(err)
	}
	out := ListIssuesOutput{HasMore: resp.NextPage != 0}
	for _, i := range issues {
		if i.IsPullRequest() {
			continue
		}
		out.Issues = append(out.Issues, Issue{
			Number: i.GetNumber(),
			Title:  i.GetTitle(),
			State:  i.GetState(),
			Labels: labelNames(i.Labels),
			Author: i.GetUser().GetLogin(),
			URL:    i.GetHTMLURL(),
		})
	}
	return textResult(out), out, nil
}

// GetIssueInput is the argument schema for get_issue.
type GetIssueInput struct {
	Owner  string `json:"owner" jsonschema:"repository owner (user or org)"`
	Repo   string `json:"repo" jsonschema:"repository name"`
	Number int    `json:"number" jsonschema:"issue number"`
}

// GetIssueOutput is the structured result of get_issue.
type GetIssueOutput struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	State  string   `json:"state"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
	Author string   `json:"author"`
	URL    string   `json:"url"`
}

// GetIssue fetches a single issue.
func GetIssue(ctx context.Context, gh *github.Client, in GetIssueInput) (*mcp.CallToolResult, GetIssueOutput, error) {
	i, _, err := gh.Issues.Get(ctx, in.Owner, in.Repo, in.Number)
	if err != nil {
		return nil, GetIssueOutput{}, mapGitHubError(err)
	}
	out := GetIssueOutput{
		Number: i.GetNumber(),
		Title:  i.GetTitle(),
		State:  i.GetState(),
		Body:   i.GetBody(),
		Labels: labelNames(i.Labels),
		Author: i.GetUser().GetLogin(),
		URL:    i.GetHTMLURL(),
	}
	return textResult(out), out, nil
}

// IssueComment is the trimmed projection of an issue comment.
type IssueComment struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	URL       string `json:"url"`
}

// ListIssueCommentsInput is the argument schema for list_issue_comments.
type ListIssueCommentsInput struct {
	Owner   string `json:"owner" jsonschema:"repository owner (user or org)"`
	Repo    string `json:"repo" jsonschema:"repository name"`
	Number  int    `json:"number" jsonschema:"issue number"`
	Page    int    `json:"page,omitempty" jsonschema:"1-based page number"`
	PerPage int    `json:"per_page,omitempty" jsonschema:"results per page (max 100)"`
}

// ListIssueCommentsOutput is the structured result of list_issue_comments.
type ListIssueCommentsOutput struct {
	Comments []IssueComment `json:"comments"`
	HasMore  bool           `json:"has_more"`
}

// ListIssueComments lists the comments on a single issue.
func ListIssueComments(ctx context.Context, gh *github.Client, in ListIssueCommentsInput) (*mcp.CallToolResult, ListIssueCommentsOutput, error) {
	opt := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{Page: in.Page, PerPage: clampPerPage(in.PerPage)},
	}
	comments, resp, err := gh.Issues.ListComments(ctx, in.Owner, in.Repo, in.Number, opt)
	if err != nil {
		return nil, ListIssueCommentsOutput{}, mapGitHubError(err)
	}
	out := ListIssueCommentsOutput{HasMore: resp.NextPage != 0}
	for _, c := range comments {
		var created string
		if t := c.GetCreatedAt(); !t.Time.IsZero() {
			created = t.Time.Format("2006-01-02T15:04:05Z07:00")
		}
		out.Comments = append(out.Comments, IssueComment{
			Author:    c.GetUser().GetLogin(),
			Body:      c.GetBody(),
			CreatedAt: created,
			URL:       c.GetHTMLURL(),
		})
	}
	return textResult(out), out, nil
}
