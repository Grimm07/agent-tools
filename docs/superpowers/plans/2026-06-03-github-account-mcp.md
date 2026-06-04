# github-account MCP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a read-only GitHub MCP server (`mcps/github-account/`) exposing 11 tools over repos/code, issues, and pull requests.

**Architecture:** Go MCP server on `modelcontextprotocol/go-sdk` over stdio. One shared `*github.Client` (from `google/go-github`) built from `GITHUB_TOKEN`. Each tool is a free function `func(ctx, gh, in) (*mcp.CallToolResult, Out, error)`, registered in `server.go` via a thin closure binding the client. Tests use `httptest.Server` with the client's `BaseURL` overridden.

**Tech Stack:** Go ≥1.25, `github.com/modelcontextprotocol/go-sdk`, `github.com/google/go-github/v66`, `golang.org/x/oauth2`, stdlib `net/http/httptest`.

**Authoritative detail source:** `docs/superpowers/specs/2026-06-03-github-account-mcp-design.md` (tool table, projections, error handling). This plan defines structure, build order, the canonical pattern, and TDD steps.

---

### Task 1: Scaffold the unit

**Files:**
- Create: `mcps/github-account/` (entire tree via copier)

- [ ] **Step 1: Scaffold from the go-mcp kind**

Run from repo root:
```bash
make new KIND=go-mcp NAME="GitHub Account"
```
Expected: creates `mcps/github-account/` with `cmd/githubaccount/main.go`, `internal/githubaccount/server.go`+`server_test.go`, `go.mod`, `Makefile`, `README.md`, `.copier-answers.yml`; runs `go mod tidy`.

- [ ] **Step 2: Verify the scaffold builds and tests pass**

Run: `cd mcps/github-account && make test`
Expected: PASS (the template's sample `greet` tool test).

- [ ] **Step 3: Add dependencies**

Run: `cd mcps/github-account && go get github.com/google/go-github/v66/github golang.org/x/oauth2 && go mod tidy`
Expected: both modules added to `go.mod`.

- [ ] **Step 4: Commit**

```bash
git add mcps/github-account
git commit -m "feat(github-account): scaffold go-mcp unit with go-github dep"
```

---

### Task 2: Testable client constructor

**Files:**
- Create: `mcps/github-account/internal/githubaccount/client.go`
- Test: `mcps/github-account/internal/githubaccount/client_test.go`

- [ ] **Step 1: Write the failing test**

```go
package githubaccount

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClientUsesBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"login":"octocat"}`))
	}))
	defer srv.Close()

	gh := newClient("test-token", srv.URL+"/")
	if gh.BaseURL.String() != srv.URL+"/" {
		t.Fatalf("BaseURL = %q, want %q", gh.BaseURL.String(), srv.URL+"/")
	}
}

func TestNewServerRequiresToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	if _, err := NewServer(); err == nil {
		t.Fatal("expected error when GITHUB_TOKEN is unset")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/githubaccount/ -run TestNewClient -v`
Expected: FAIL — `newClient` / `NewServer` signature mismatch (NewServer currently returns `*mcp.Server` with no error).

- [ ] **Step 3: Implement client.go**

```go
package githubaccount

import (
	"context"
	"net/url"

	"github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

// newClient builds a go-github client authenticated with token. If baseURL is
// non-empty it overrides the API endpoint (used by tests to target httptest).
func newClient(token, baseURL string) *github.Client {
	httpClient := oauth2.NewClient(context.Background(),
		oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
	gh := github.NewClient(httpClient)
	if baseURL != "" {
		u, _ := url.Parse(baseURL)
		gh.BaseURL = u
	}
	return gh
}
```

- [ ] **Step 4: Update server.go NewServer to require the token**

Change `NewServer()` to read the token and return an error:
```go
func NewServer() (*mcp.Server, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, errors.New("GITHUB_TOKEN is not set")
	}
	gh := newClient(token, "")
	return newServerWithClient(gh), nil
}

// newServerWithClient registers all tools against gh. Split out so tests can
// inject an httptest-backed client.
func newServerWithClient(gh *github.Client) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "github-account", Version: version}, nil)
	// tools registered in later tasks
	return s
}
```
Update `Run` to handle the error, and `cmd/githubaccount/main.go` accordingly. Remove the sample `greet` tool/structs/test.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/githubaccount/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add mcps/github-account
git commit -m "feat(github-account): token-gated server + testable client"
```

---

### Task 3: Canonical tool — `list_repos` (the pattern every other tool follows)

**Files:**
- Create: `mcps/github-account/internal/githubaccount/repos.go`
- Test: `mcps/github-account/internal/githubaccount/repos_test.go`

- [ ] **Step 1: Write the failing test**

```go
package githubaccount

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListRepos(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/repos" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`[{"name":"app","full_name":"me/app","private":true,
			"description":"d","default_branch":"main","html_url":"u"}]`))
	}))
	defer srv.Close()

	gh := newClient("t", srv.URL+"/")
	_, out, err := ListRepos(context.Background(), gh, ListReposInput{PerPage: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Repos) != 1 || out.Repos[0].FullName != "me/app" {
		t.Fatalf("got %+v", out.Repos)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/githubaccount/ -run TestListRepos -v`
Expected: FAIL — undefined `ListRepos`/`ListReposInput`.

- [ ] **Step 3: Implement repos.go (`list_repos`)**

```go
package githubaccount

import (
	"context"

	"github.com/google/go-github/v66/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultPerPage = 30
const maxPerPage = 100

func clampPerPage(n int) int {
	if n <= 0 {
		return defaultPerPage
	}
	if n > maxPerPage {
		return maxPerPage
	}
	return n
}

type Repo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	Description   string `json:"description"`
	DefaultBranch string `json:"default_branch"`
	URL           string `json:"url"`
}

type ListReposInput struct {
	Visibility  string `json:"visibility,omitempty" jsonschema:"all|public|private"`
	Affiliation string `json:"affiliation,omitempty" jsonschema:"owner,collaborator,organization_member"`
	Page        int    `json:"page,omitempty" jsonschema:"1-based page number"`
	PerPage     int    `json:"per_page,omitempty" jsonschema:"results per page (max 100)"`
}

type ListReposOutput struct {
	Repos   []Repo `json:"repos"`
	HasMore bool   `json:"has_more"`
}

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
		out.Repos = append(out.Repos, Repo{
			Name: r.GetName(), FullName: r.GetFullName(), Private: r.GetPrivate(),
			Description: r.GetDescription(), DefaultBranch: r.GetDefaultBranch(), URL: r.GetHTMLURL(),
		})
	}
	return textResult(out), out, nil
}
```

- [ ] **Step 4: Add shared helpers `textResult` and `mapGitHubError`**

Create `mcps/github-account/internal/githubaccount/result.go`:
```go
package githubaccount

import (
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v66/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func textResult(v any) *mcp.CallToolResult {
	b, _ := json.MarshalIndent(v, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
}

// mapGitHubError converts a go-github error into a clear, non-panic error that
// distinguishes not-found / forbidden / rate-limit from generic failures.
func mapGitHubError(err error) error {
	var rl *github.RateLimitError
	if errorsAs(err, &rl) {
		return fmt.Errorf("github rate limit exceeded; resets at %s", rl.Rate.Reset.Time)
	}
	var er *github.ErrorResponse
	if errorsAs(err, &er) {
		return fmt.Errorf("github %d: %s", er.Response.StatusCode, er.Message)
	}
	return err
}
```
Add `import "errors"` and `func errorsAs(err error, target any) bool { return errors.As(err, target) }` (or inline `errors.As`). Keep it simple — inline `errors.As` is fine; the wrapper exists only to keep imports tidy.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/githubaccount/ -run TestListRepos -v`
Expected: PASS.

- [ ] **Step 6: Register the tool in server.go**

In `newServerWithClient`, add:
```go
mcp.AddTool(s, &mcp.Tool{Name: "list_repos", Description: "List the authenticated user's repositories."},
	func(ctx context.Context, req *mcp.CallToolRequest, in ListReposInput) (*mcp.CallToolResult, ListReposOutput, error) {
		return ListRepos(ctx, gh, in)
	})
```

- [ ] **Step 7: Run all tests + commit**

Run: `make test`
```bash
git add mcps/github-account
git commit -m "feat(github-account): list_repos + shared result/error helpers"
```

---

### Tasks 4–13: Remaining tools (apply the Task 3 pattern)

Each task = one tool. For each: write the `httptest` test (canned JSON for the endpoint), define `XxxInput`/`XxxOutput` structs + `Xxx` free function calling the go-github method, map to the projection, register in `server.go`, run tests, commit. Follow the projections in the spec's tool table exactly.

- [ ] **Task 4 — `get_repo`** · file `repos.go` · `gh.Repositories.Get(ctx, owner, repo)` · in: `owner,repo` · out: repo projection. Endpoint `/repos/{owner}/{repo}`.
- [ ] **Task 5 — `list_branches`** · `repos.go` · `gh.Repositories.ListBranches(ctx, owner, repo, opt)` · in: `owner,repo,page,per_page` · out: `name, commit_sha, protected`. Endpoint `/repos/{o}/{r}/branches`.
- [ ] **Task 6 — `list_commits`** · `repos.go` · `gh.Repositories.ListCommits(ctx, owner, repo, &github.CommitsListOptions{SHA:ref, Path:path, ListOptions:...})` · out: `sha, message, author, date, url`.
- [ ] **Task 7 — `get_file`** · `repos.go` · `gh.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref:ref})` · out: `path, type, encoding, content (call .GetContent() to decode), size`. Handle dir (returns slice) vs file.
- [ ] **Task 8 — `list_issues`** · new file `issues.go` · `gh.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{State,Labels,Assignee,ListOptions})` · out: `number,title,state,labels,author,url`. NOTE: filter out PRs (items where `IsPullRequest()`).
- [ ] **Task 9 — `get_issue`** · `issues.go` · `gh.Issues.Get(ctx, owner, repo, number)` · out: `number,title,state,body,labels,author,url`.
- [ ] **Task 10 — `list_issue_comments`** · `issues.go` · `gh.Issues.ListComments(ctx, owner, repo, number, opt)` · out: `author,body,created_at,url`.
- [ ] **Task 11 — `list_pull_requests`** · new file `pulls.go` · `gh.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{State,Base,Head,ListOptions})` · out: `number,title,state,base,head,author,url`.
- [ ] **Task 12 — `get_pull_request`** · `pulls.go` · `gh.PullRequests.Get(ctx, owner, repo, number)` · out: `number,title,state,body,base,head,merged,url`.
- [ ] **Task 13 — `get_pull_request_diff`** · `pulls.go` · `gh.PullRequests.ListFiles(ctx, owner, repo, number, opt)` · out: `files:[{filename,status,additions,deletions,patch}]`.

Each task ends with `make test` (expect PASS) and a commit `feat(github-account): <tool>`.

---

### Task 14: README + final verification

**Files:**
- Modify: `mcps/github-account/README.md`

- [ ] **Step 1: Write README** — purpose; language rationale (Go: single binary, official SDKs); exact `make test`/`make lint`/run commands; required `GITHUB_TOKEN` fine-grained permissions (Contents/Issues/Pull requests/Metadata = read); full tool list.
- [ ] **Step 2: Run full suite** — `make test && make lint` → Expected: PASS / no lint findings.
- [ ] **Step 3: Smoke-test startup without token** — `GITHUB_TOKEN= go run ./cmd/githubaccount` → Expected: exits with "GITHUB_TOKEN is not set".
- [ ] **Step 4: Commit** — `git commit -am "docs(github-account): README + tool reference"`.

---

## Self-Review

- **Spec coverage:** 11 tools (Tasks 3–13) ✓; token gating (Task 2) ✓; trimmed projections per tool ✓; error mapping 403/404/rate-limit (Task 3 `mapGitHubError`) ✓; pagination defaults/cap (Task 3 `clampPerPage` + `has_more`) ✓; offline httptest tests ✓; README + perms (Task 14) ✓.
- **Placeholders:** none — every tool task names exact method, inputs, and output fields; canonical code shown in Task 3.
- **Type consistency:** `newClient`, `newServerWithClient`, `textResult`, `mapGitHubError`, `clampPerPage` defined in Tasks 2–3 and reused by name throughout.
