# GitHub Account

A **read-only** [MCP](https://modelcontextprotocol.io) server that lets an agent browse a GitHub
account — repositories and code, issues, and pull requests — over stdio. Built on the official
[`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk) and the
[`google/go-github`](https://github.com/google/go-github) REST client.

It only ever calls go-github `Get`/`List` methods: there is no tool that creates, comments, merges,
closes, or otherwise mutates anything.

## Language: Go

Per the repo's rule "Go where it makes sense": this unit ships as a single static binary, runs as a
long-lived stdio process, and both the MCP SDK and the GitHub REST client are first-class in Go.
go-github also gives typed responses, auth, pagination, and rate-limit parsing for free, and its
overridable `BaseURL` lets the whole test suite run offline against `httptest`.

## Authentication

The server reads `GITHUB_TOKEN` from the environment on startup. If it is unset or empty the server
**fails fast** with `GITHUB_TOKEN is not set` and exits non-zero — it never runs unauthenticated. The
token is handed to go-github via an OAuth2 transport and is never logged.

The expected value is a **fine-grained personal access token** scoped read-only. Required repository
permissions (all **read**):

| Permission     | Why |
|----------------|-----|
| Metadata       | Required base scope; repo listing and metadata. |
| Contents       | `get_file`, `list_branches`, `list_commits`. |
| Issues         | `list_issues`, `get_issue`, `list_issue_comments`. |
| Pull requests  | `list_pull_requests`, `get_pull_request`, `get_pull_request_diff`. |

## Requirements

- Go ≥ 1.25 (the SDK requires it). With `GOTOOLCHAIN=auto`, the right toolchain downloads on first build.

## Commands

```bash
make test    # go test -race ./...   (fully offline; httptest-backed)
make build   # -> bin/github-account
make run     # serve over stdio (needs GITHUB_TOKEN)
make lint    # gofmt check + go vet + staticcheck (if installed)
make tidy    # go mod tidy
```

Run the server directly:

```bash
GITHUB_TOKEN=ghp_xxx go run ./cmd/github-account
```

## Tools (11)

**Repos & code**

| Tool | Input (key fields) | Output (trimmed) |
|------|--------------------|------------------|
| `list_repos` | `visibility`, `affiliation`, `page`, `per_page` | name, full_name, private, description, default_branch, url |
| `get_repo` | `owner`, `repo` | repo metadata projection |
| `list_branches` | `owner`, `repo`, `page`, `per_page` | name, commit_sha, protected |
| `list_commits` | `owner`, `repo`, `ref`, `path`, `page`, `per_page` | sha, message, author, date, url |
| `get_file` | `owner`, `repo`, `path`, `ref` | file: path, type, encoding, content (decoded), size; dir: entries |

**Issues**

| Tool | Input (key fields) | Output (trimmed) |
|------|--------------------|------------------|
| `list_issues` | `owner`, `repo`, `state`, `labels`, `assignee`, `page`, `per_page` | number, title, state, labels, author, url (pull requests excluded) |
| `get_issue` | `owner`, `repo`, `number` | number, title, state, body, labels, author, url |
| `list_issue_comments` | `owner`, `repo`, `number`, `page`, `per_page` | author, body, created_at, url |

**Pull requests**

| Tool | Input (key fields) | Output (trimmed) |
|------|--------------------|------------------|
| `list_pull_requests` | `owner`, `repo`, `state`, `base`, `head`, `page`, `per_page` | number, title, state, base, head, author, url |
| `get_pull_request` | `owner`, `repo`, `number` | number, title, state, body, base, head, merged, url |
| `get_pull_request_diff` | `owner`, `repo`, `number`, `page`, `per_page` | files: filename, status, additions, deletions, patch |

### Pagination

List tools accept `page` (1-based) and `per_page` (default 30, capped at 100) and report `has_more`
rather than auto-fetching every page.

### Errors

GitHub `404` / `403` / rate-limit / other non-2xx responses are mapped to a clear tool error carrying
the HTTP status and GitHub's message (so the agent can tell "not found" from "rate limited"); no stack
traces leak to the client.

## Layout

```
cmd/github-account/main.go         # entrypoint: wires stdio transport, calls Run(ctx)
internal/githubaccount/
  server.go                        # NewServer() (token-gated) + tool registration; Run()
  client.go                        # newClient(token, baseURL) — base-URL override for tests
  result.go                        # textResult + mapGitHubError helpers
  repos.go                         # repo/code tool handlers + In/Out structs
  issues.go                        # issue tool handlers + In/Out structs
  pulls.go                         # pull request tool handlers + In/Out structs
  *_test.go                        # httptest-backed unit tests (offline)
```

## Updating from the template

```bash
copier update --trust   # pulls template improvements; answers are in .copier-answers.yml
```
