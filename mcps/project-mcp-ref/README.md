# Project MCP Ref

A go-mcp unit.

A project-scoped [MCP](https://modelcontextprotocol.io) server built on the official
[`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk). It exposes a single
software project's commands, code, docs, and read-only git state to an agent over stdio.

**Language: Go.** Like the `go-mcp` kind, this unit ships as a single static binary, execs project
commands and `git` cheaply, and reads the filesystem directly — exactly the case the repo reserves for
Go (see the root `CLAUDE.md`). Its tools wrap the **local environment** instead of a remote API.

## Requirements

- Go ≥ 1.25 (the SDK requires it). With `GOTOOLCHAIN=auto`, the right toolchain downloads on first build.

## Configuration

All behavior is driven by `project-mcp.toml` (read at startup, so edits take effect on the next run —
no re-scaffold needed):

```toml
# Project root the server operates within (relative to this config file).
root = "."

[commands]
# Each entry is an argv array (NOT a shell string). Empty/absent = tool disabled.
test  = ["go", "test", "./..."]
lint  = ["golangci-lint", "run"]
build = ["go", "build", "./..."]

[docs]
# Files or directories (under root) exposed via the docs tools.
paths = ["README.md", "CLAUDE.md", "docs"]
```

The server finds its config via `PROJECT_MCP_CONFIG` (a path) or, if unset, `project-mcp.toml` in the
working directory.

## Tools

| Tool | Behavior |
|------|----------|
| `run_tests` / `run_lint` / `run_build` | Execute the matching `commands.*` argv; return exit code + combined output (capped). Only registered when configured. |
| `search_code` | Regex search of files under `root`; returns `path:line:match` hits (count-capped). |
| `read_file` | Read one file under `root` (size-capped). |
| `list_docs` | List the files resolved from `docs.paths`. |
| `read_doc` | Read one doc file (must resolve under a configured docs path). |
| `git_status` | `git status --porcelain` parsed to entries. |
| `git_diff` | Read-only `git diff` (optional `staged`, `path`). |
| `git_log` | Recent commits (`count`, capped): sha, author, date, subject. |

## Security model

This server execs commands and reads files, so it is security-sensitive:

- **No arbitrary-exec tool.** Command tools run only the fixed argv arrays from `project-mcp.toml`, via
  `exec.CommandContext` with separate arguments — never `sh -c`. No tool accepts a raw command string.
- **Path confinement.** `read_file`, `read_doc`, and `search_code` resolve the requested path against
  `root`, follow symlinks, and reject anything resolving outside `root` (defends against `..` traversal
  and symlink escape).
- **Timeouts.** Every exec (commands and git) runs under a context timeout.
- **Output caps.** Command output, file contents, and search results are truncated to documented caps.
- **Read-only git.** Only `status` / `diff` / `log` are run; the server never mutates the repository.

## Commands

```bash
make test    # go test -race ./...
make build   # -> bin/project-mcp-ref
make run     # serve over stdio
make lint    # gofmt check + go vet + staticcheck (if installed)
make tidy    # go mod tidy
```

## Layout

```
cmd/project-mcp-ref/main.go            # entrypoint: calls Run(ctx)
internal/projectmcpref/
  server.go                            # config discovery, NewServer(), tool registration, Run()
  config.go                            # parse/validate project-mcp.toml; path confinement
  commands.go                          # allowlisted command exec (test/lint/build)
  fsutil.go                            # search_code, read_file
  docs.go                              # list_docs, read_doc
  git.go                               # read-only git_status / git_diff / git_log
  *_test.go                            # one per file (offline, deterministic)
project-mcp.toml                       # the project configuration
```

## Updating from the template

```bash
copier update --trust   # pulls template improvements; answers are in .copier-answers.yml
```
