package githubaccount

import (
	"context"

	"github.com/google/go-github/v66/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultPerPage = 30
const maxPerPage = 100

// clampPerPage applies the per_page default (30) and cap (100) so a tool never
// asks GitHub for an unbounded page.
func clampPerPage(n int) int {
	if n <= 0 {
		return defaultPerPage
	}
	if n > maxPerPage {
		return maxPerPage
	}
	return n
}

// Repo is the trimmed projection of a repository returned by repo tools.
type Repo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	Description   string `json:"description"`
	DefaultBranch string `json:"default_branch"`
	URL           string `json:"url"`
}

func toRepo(r *github.Repository) Repo {
	return Repo{
		Name:          r.GetName(),
		FullName:      r.GetFullName(),
		Private:       r.GetPrivate(),
		Description:   r.GetDescription(),
		DefaultBranch: r.GetDefaultBranch(),
		URL:           r.GetHTMLURL(),
	}
}

// ListReposInput is the argument schema for list_repos.
type ListReposInput struct {
	Visibility  string `json:"visibility,omitempty" jsonschema:"all|public|private"`
	Affiliation string `json:"affiliation,omitempty" jsonschema:"owner,collaborator,organization_member"`
	Page        int    `json:"page,omitempty" jsonschema:"1-based page number"`
	PerPage     int    `json:"per_page,omitempty" jsonschema:"results per page (max 100)"`
}

// ListReposOutput is the structured result of list_repos.
type ListReposOutput struct {
	Repos   []Repo `json:"repos"`
	HasMore bool   `json:"has_more"`
}

// ListRepos lists the authenticated user's repositories.
func ListRepos(ctx context.Context, gh *github.Client, in ListReposInput) (*mcp.CallToolResult, ListReposOutput, error) {
	opt := &github.RepositoryListByAuthenticatedUserOptions{
		Visibility:  in.Visibility,
		Affiliation: in.Affiliation,
		ListOptions: github.ListOptions{Page: in.Page, PerPage: clampPerPage(in.PerPage)},
	}
	repos, resp, err := gh.Repositories.ListByAuthenticatedUser(ctx, opt)
	if err != nil {
		return nil, ListReposOutput{}, mapGitHubError(err)
	}
	out := ListReposOutput{HasMore: resp.NextPage != 0}
	for _, r := range repos {
		out.Repos = append(out.Repos, toRepo(r))
	}
	return textResult(out), out, nil
}

// GetRepoInput is the argument schema for get_repo.
type GetRepoInput struct {
	Owner string `json:"owner" jsonschema:"repository owner (user or org)"`
	Repo  string `json:"repo" jsonschema:"repository name"`
}

// GetRepoOutput is the structured result of get_repo.
type GetRepoOutput struct {
	Repo Repo `json:"repo"`
}

// GetRepo fetches metadata for a single repository.
func GetRepo(ctx context.Context, gh *github.Client, in GetRepoInput) (*mcp.CallToolResult, GetRepoOutput, error) {
	r, _, err := gh.Repositories.Get(ctx, in.Owner, in.Repo)
	if err != nil {
		return nil, GetRepoOutput{}, mapGitHubError(err)
	}
	out := GetRepoOutput{Repo: toRepo(r)}
	return textResult(out), out, nil
}

// Branch is the trimmed projection of a branch.
type Branch struct {
	Name      string `json:"name"`
	CommitSHA string `json:"commit_sha"`
	Protected bool   `json:"protected"`
}

// ListBranchesInput is the argument schema for list_branches.
type ListBranchesInput struct {
	Owner   string `json:"owner" jsonschema:"repository owner (user or org)"`
	Repo    string `json:"repo" jsonschema:"repository name"`
	Page    int    `json:"page,omitempty" jsonschema:"1-based page number"`
	PerPage int    `json:"per_page,omitempty" jsonschema:"results per page (max 100)"`
}

// ListBranchesOutput is the structured result of list_branches.
type ListBranchesOutput struct {
	Branches []Branch `json:"branches"`
	HasMore  bool     `json:"has_more"`
}

// ListBranches lists the branches of a repository.
func ListBranches(ctx context.Context, gh *github.Client, in ListBranchesInput) (*mcp.CallToolResult, ListBranchesOutput, error) {
	opt := &github.BranchListOptions{
		ListOptions: github.ListOptions{Page: in.Page, PerPage: clampPerPage(in.PerPage)},
	}
	branches, resp, err := gh.Repositories.ListBranches(ctx, in.Owner, in.Repo, opt)
	if err != nil {
		return nil, ListBranchesOutput{}, mapGitHubError(err)
	}
	out := ListBranchesOutput{HasMore: resp.NextPage != 0}
	for _, b := range branches {
		out.Branches = append(out.Branches, Branch{
			Name:      b.GetName(),
			CommitSHA: b.GetCommit().GetSHA(),
			Protected: b.GetProtected(),
		})
	}
	return textResult(out), out, nil
}

// Commit is the trimmed projection of a commit.
type Commit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	URL     string `json:"url"`
}

// ListCommitsInput is the argument schema for list_commits.
type ListCommitsInput struct {
	Owner   string `json:"owner" jsonschema:"repository owner (user or org)"`
	Repo    string `json:"repo" jsonschema:"repository name"`
	Ref     string `json:"ref,omitempty" jsonschema:"SHA or branch to start listing from"`
	Path    string `json:"path,omitempty" jsonschema:"only commits touching this path"`
	Page    int    `json:"page,omitempty" jsonschema:"1-based page number"`
	PerPage int    `json:"per_page,omitempty" jsonschema:"results per page (max 100)"`
}

// ListCommitsOutput is the structured result of list_commits.
type ListCommitsOutput struct {
	Commits []Commit `json:"commits"`
	HasMore bool     `json:"has_more"`
}

// ListCommits lists the commits of a repository.
func ListCommits(ctx context.Context, gh *github.Client, in ListCommitsInput) (*mcp.CallToolResult, ListCommitsOutput, error) {
	opt := &github.CommitsListOptions{
		SHA:         in.Ref,
		Path:        in.Path,
		ListOptions: github.ListOptions{Page: in.Page, PerPage: clampPerPage(in.PerPage)},
	}
	commits, resp, err := gh.Repositories.ListCommits(ctx, in.Owner, in.Repo, opt)
	if err != nil {
		return nil, ListCommitsOutput{}, mapGitHubError(err)
	}
	out := ListCommitsOutput{HasMore: resp.NextPage != 0}
	for _, c := range commits {
		var date string
		if d := c.GetCommit().GetAuthor().GetDate(); !d.Time.IsZero() {
			date = d.Time.Format("2006-01-02T15:04:05Z07:00")
		}
		out.Commits = append(out.Commits, Commit{
			SHA:     c.GetSHA(),
			Message: c.GetCommit().GetMessage(),
			Author:  c.GetCommit().GetAuthor().GetName(),
			Date:    date,
			URL:     c.GetHTMLURL(),
		})
	}
	return textResult(out), out, nil
}

// GetFileInput is the argument schema for get_file.
type GetFileInput struct {
	Owner string `json:"owner" jsonschema:"repository owner (user or org)"`
	Repo  string `json:"repo" jsonschema:"repository name"`
	Path  string `json:"path" jsonschema:"path to the file or directory"`
	Ref   string `json:"ref,omitempty" jsonschema:"branch, tag, or commit SHA (default: default branch)"`
}

// FileEntry is one item returned for a directory listing.
type FileEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int    `json:"size"`
}

// GetFileOutput is the structured result of get_file. For a file, Content is the
// decoded text and Entries is nil; for a directory, Entries lists the children
// and Content/Encoding are empty.
type GetFileOutput struct {
	Path     string      `json:"path"`
	Type     string      `json:"type"`
	Encoding string      `json:"encoding,omitempty"`
	Content  string      `json:"content,omitempty"`
	Size     int         `json:"size,omitempty"`
	Entries  []FileEntry `json:"entries,omitempty"`
}

// GetFile reads a file's decoded contents, or lists a directory's entries.
func GetFile(ctx context.Context, gh *github.Client, in GetFileInput) (*mcp.CallToolResult, GetFileOutput, error) {
	fileContent, dirContent, _, err := gh.Repositories.GetContents(ctx, in.Owner, in.Repo, in.Path,
		&github.RepositoryContentGetOptions{Ref: in.Ref})
	if err != nil {
		return nil, GetFileOutput{}, mapGitHubError(err)
	}
	if fileContent != nil {
		decoded, derr := fileContent.GetContent()
		if derr != nil {
			return nil, GetFileOutput{}, derr
		}
		out := GetFileOutput{
			Path:     fileContent.GetPath(),
			Type:     fileContent.GetType(),
			Encoding: fileContent.GetEncoding(),
			Content:  decoded,
			Size:     fileContent.GetSize(),
		}
		return textResult(out), out, nil
	}
	out := GetFileOutput{Path: in.Path, Type: "dir"}
	for _, e := range dirContent {
		out.Entries = append(out.Entries, FileEntry{
			Path: e.GetPath(),
			Type: e.GetType(),
			Size: e.GetSize(),
		})
	}
	return textResult(out), out, nil
}
