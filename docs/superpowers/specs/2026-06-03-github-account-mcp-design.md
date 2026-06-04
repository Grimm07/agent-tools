# Design: `github-account` MCP server

**Date:** 2026-06-03
**Status:** Approved (design)
**Unit:** `mcps/github-account/`

## Purpose

A read-only Model Context Protocol server that lets an agent browse the owner's
GitHub account: repositories and code, issues, and pull requests. It is one unit
in the `agent-tools` monorepo, scaffolded from the existing `go-mcp` copier kind.

## Language choice

**Go.** Per the repo's rule "Go where it makes sense": the unit ships as a single
static binary, runs as a long-lived stdio process, and both the official MCP SDK
(`github.com/modelcontextprotocol/go-sdk`) and the GitHub REST client
(`github.com/google/go-github`) are first-class in Go. Rationale recorded in the
unit README.

## Scope

In scope (all **read-only**):

- **Repos & code:** list repos, get repo metadata, list branches, list commits,
  read file/dir contents.
- **Issues:** list/filter issues, get one issue, list issue comments.
- **Pull requests:** list/filter PRs, get one PR, get a PR's diff/changed files.

Explicitly out of scope: any mutation (no create/comment/merge/close), account &
activity feeds, notifications, organization administration.

## Authentication

- On startup the server reads `GITHUB_TOKEN` from the environment. The expected
  value is a **fine-grained PAT** scoped read-only.
- If `GITHUB_TOKEN` is unset or empty, the server **fails fast** with a clear
  error and exits — it never silently runs unauthenticated.
- The token is supplied to `go-github` via an OAuth2 transport and is never
  logged.
- README documents the required fine-grained permissions: Contents (read),
  Issues (read), Pull requests (read), Metadata (read).

## Approach: `google/go-github`

Use the `google/go-github` REST client rather than hand-rolled `net/http` or
GraphQL. It provides typed responses, auth, pagination, and rate-limit parsing,
and — critically for the repo's "tests run offline" bar — its `BaseURL` is
overridable so unit tests point the client at an `httptest.Server`.

## Tools (11)

The server registers 11 tools: five for repos/code, three for issues, three for
pull requests. This table is the authoritative tool list.

**Repos & code**

| Tool | Input (key fields) | Output (trimmed projection) |
|------|--------------------|-----------------------------|
| `list_repos` | `visibility`, `affiliation`, `page`, `per_page` | name, full_name, private, description, default_branch, url |
| `get_repo` | `owner`, `repo` | repo metadata projection |
| `list_branches` | `owner`, `repo`, `page`, `per_page` | name, commit sha, protected |
| `list_commits` | `owner`, `repo`, `ref`, `path`, `page`, `per_page` | sha, message, author, date, url |
| `get_file` | `owner`, `repo`, `path`, `ref` | path, type, encoding, content (decoded), size |

**Issues**

| Tool | Input (key fields) | Output (trimmed projection) |
|------|--------------------|-----------------------------|
| `list_issues` | `owner`, `repo`, `state`, `labels`, `assignee`, `page`, `per_page` | number, title, state, labels, author, url |
| `get_issue` | `owner`, `repo`, `number` | number, title, state, body, labels, author, url |
| `list_issue_comments` | `owner`, `repo`, `number`, `page`, `per_page` | author, body, created_at, url |

**Pull requests**

| Tool | Input (key fields) | Output (trimmed projection) |
|------|--------------------|-----------------------------|
| `list_pull_requests` | `owner`, `repo`, `state`, `base`, `head`, `page`, `per_page` | number, title, state, base, head, author, url |
| `get_pull_request` | `owner`, `repo`, `number` | number, title, state, body, base, head, merged, url |
| `get_pull_request_diff` | `owner`, `repo`, `number` | files: filename, status, additions, deletions, patch |

## Architecture & layout

```
mcps/github-account/
  cmd/githubaccount/main.go   # calls Run(ctx)
  internal/githubaccount/
    server.go                 # NewServer(): build client from GITHUB_TOKEN, register tools; Run()
    client.go                 # newClient(token, baseURL) — base-URL override for tests
    repos.go                  # handlers + In/Out structs for repo/code tools
    issues.go                 # handlers + In/Out structs for issue tools
    pulls.go                  # handlers + In/Out structs for PR tools
    repos_test.go / issues_test.go / pulls_test.go / server_test.go
  README.md  Makefile  go.mod  .copier-answers.yml
```

- Each handler is a **free function taking `*github.Client`**, e.g.
  `func ListRepos(ctx context.Context, gh *github.Client, in ListReposInput) (*mcp.CallToolResult, ListReposOutput, error)`,
  so it unit-tests directly without a transport.
- `NewServer` builds one shared `*github.Client` and registers each tool via a
  thin closure binding the client into the SDK's `AddTool` signature.
- Input/output structs carry `json` + `jsonschema` tags so the SDK
  auto-generates schemas.
- Outputs are **trimmed projections** of go-github's large structs — only the
  fields an agent needs — to keep tool responses compact.

## Data flow

1. Client calls a tool over stdio → SDK decodes args into the handler's input
   struct.
2. Handler calls the relevant `go-github` method with the shared client.
3. go-github performs the authenticated HTTPS request to api.github.com.
4. Handler maps the response into the trimmed output struct and returns a
   `CallToolResult` (text summary + structured output).

## Error handling

- Missing/empty `GITHUB_TOKEN` → startup error, process exits non-zero.
- GitHub `404` / `403` / rate-limit / other non-2xx → mapped to a clear tool
  error carrying the HTTP status and GitHub's message (so the agent distinguishes
  "not found" from "rate limited"); no stack traces leak to the client.
- Pagination: tools accept `page` / `per_page` (default `per_page=30`, capped at
  100) and report whether more pages exist rather than auto-fetching everything.

## Testing

- Per-file `httptest.Server` fakes returning canned GitHub JSON; client `BaseURL`
  pointed at the fake.
- Coverage: happy path per tool; `404`/`403` mapping; pagination parameter
  handling; missing-token startup failure.
- Fully offline and deterministic. Run via `make test` (`go test -race ./...`);
  `make lint` runs `gofmt -l` + `go vet` + `staticcheck`.

## Repo integration (done by coordinator, not the unit)

- README states purpose, language rationale, exact run/build/test commands, and
  the required `GITHUB_TOKEN` permissions.
- CLAUDE.md: once this is the first real MCP unit, update the "Mostly greenfield"
  note to point at it as a worked example if useful.
