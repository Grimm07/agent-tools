# Design: `project-mcp` copier template kind

**Date:** 2026-06-03
**Status:** Approved (design)
**Unit:** `templates/project-mcp/` (new copier kind) + `templates/copier.yml` changes

## Purpose

Add a third copier `kind` to `templates/` that scaffolds a self-contained **Go**
MCP server tailored to a single software project. Running it in `agent-tools`
(`make new KIND=project-mcp NAME="..."`) or via `copier copy` directly inside any
other repository produces a project-scoped server exposing that project's
commands, code, docs, and git state to an agent. "Templatable" means: it is a
reusable generator, and each generated server is independently editable and can
pull later template improvements via `copier update`.

## Language choice

**Go**, matching the existing `go-mcp` kind: single static binary, fast process
exec for running project commands, trivial `git` and filesystem access. The
generated unit reuses the same SDK, stdio transport, and handler-as-free-function
pattern. Rationale recorded in the generated README.

## Relationship to `go-mcp`

This kind is `go-mcp` specialized: identical MCP plumbing, but the tools wrap the
**local environment** (exec, filesystem, git) instead of a remote API. Where
`go-mcp` has a sample `greet` tool, `project-mcp` ships the project tool set
below plus a config file.

## Configuration model

Config-seeded at scaffold time, read at runtime:

- Copier prompts (only when `kind == project-mcp`) for the project's `test`,
  `lint`, and `build` commands, each defaulting to empty (empty = that command
  tool is disabled). Copier writes them into a committed `project-mcp.toml`.
- The generated server reads `project-mcp.toml` at startup, so commands stay
  editable later without re-scaffolding.

`project-mcp.toml` shape:

```toml
# Project root the server operates within (relative to the config file).
root = "."

[commands]
# Each entry is an argv array (NOT a shell string). Empty/absent = tool disabled.
test  = ["go", "test", "./..."]
lint  = ["golangci-lint", "run"]
build = ["go", "build", "./..."]

[docs]
# Paths (files or dirs, under root) exposed via the docs tools.
paths = ["README.md", "CLAUDE.md", "docs"]
```

## Tools

**Commands** (run only allowlisted argv from config)

| Tool | Behavior |
|------|----------|
| `run_tests` | Execute `commands.test`; return exit code + combined stdout/stderr (truncated). Disabled if unset. |
| `run_lint` | Execute `commands.lint`; same contract. |
| `run_build` | Execute `commands.build`; same contract. |

**Code**

| Tool | Behavior |
|------|----------|
| `search_code` | Regex search of files under `root`; returns path:line:match hits (capped count). |
| `read_file` | Read one file under `root`; returns content (size-capped). |

**Docs**

| Tool | Behavior |
|------|----------|
| `list_docs` | List the files resolved from `docs.paths`. |
| `read_doc` | Read one doc file (must resolve under a configured docs path). |

**State** (read-only git)

| Tool | Behavior |
|------|----------|
| `git_status` | `git status --porcelain` parsed to entries. |
| `git_diff` | `git diff` (optional `staged`, `path` args). |
| `git_log` | Recent commits (`count` arg, capped): sha, author, date, subject. |

## Security

This server executes commands and reads files, so it is the security-sensitive
unit:

- **No arbitrary-exec tool.** Command tools run only the fixed argv arrays from
  `project-mcp.toml`, via `exec.CommandContext` with separate args — never
  `sh -c` with interpolation. No tool accepts a raw command string.
- **Path confinement.** `read_file`, `read_doc`, and `search_code` resolve the
  requested path against `root`, follow symlinks, and reject anything that
  resolves outside `root` (defends against `..` traversal and symlink escape).
- **Timeouts.** Every exec (commands and git) runs under a context timeout.
- **Output caps.** Command output, file contents, and search results are
  truncated to documented caps to keep responses bounded.

## Architecture & layout (generated unit)

```
<dst>/
  cmd/{{ slug }}/main.go        # calls Run(ctx)
  internal/{{ go_pkg }}/
    server.go                   # load config, NewServer(), register enabled tools, Run()
    config.go                   # parse + validate project-mcp.toml; resolve root
    commands.go                 # RunCommand(ctx, cfg, name) for test/lint/build
    fsutil.go                   # path confinement; search_code, read_file, docs
    git.go                      # git_status / git_diff / git_log
    *_test.go                   # one per file
  project-mcp.toml              # seeded from copier answers
  README.md  Makefile  go.mod  .copier-answers.yml
```

- Handlers are **free functions** taking the parsed config (and project root) so
  they unit-test without an MCP transport, e.g.
  `func RunCommand(ctx, cfg Config, name string) (CommandResult, error)`.
- `NewServer` registers a command tool only when its config entry is non-empty,
  so disabled commands do not appear to clients.

## Data flow

1. Startup: load `project-mcp.toml`, resolve and validate `root`, build the tool
   set (enabled commands + code/docs/git tools).
2. Tool call over stdio → SDK decodes args → free-function handler runs.
3. Handler execs an allowlisted argv / reads a confined path / runs read-only
   git, applies caps, returns a `CallToolResult`.

## Error handling

- Missing or unparseable `project-mcp.toml` → startup error, process exits.
- Calling a disabled command tool → not registered, so not callable.
- Path outside `root` → tool error "path escapes project root", no read.
- Command non-zero exit → returned as a result with the exit code and output
  (not a transport error); the agent sees the failure detail.
- Exec timeout → tool error naming the timeout.

## Testing

- Temp-dir fixtures: a throwaway `git init` repo for state tools; stub argv such
  as `["printf", "..."]` / `["false"]` for command tools (offline, deterministic,
  cross-platform-friendly); sample files for search/read; a `..` and symlink case
  for path-confinement.
- Coverage: each tool happy path; disabled-command registration; path-escape
  rejection; non-zero exit surfaced; output truncation; config parse failure.
- Run via the generated unit's `make test` (`go test -race ./...`) and
  `make lint`.

## Template integration (`templates/` changes)

- `templates/copier.yml`: add `project-mcp` to the `kind` choices; add
  `test_command` / `lint_command` / `build_command` questions gated
  `when: "{{ kind == 'project-mcp' }}"`; add a `_tasks` entry running `go mod
  tidy` for this kind (mirrors `go-mcp`).
- `templates/project-mcp/`: the Jinja template tree above.
- `templates/README.md`: document the new kind.
- Root `CLAUDE.md`: add `project-mcp` to the scaffolding section and the layout
  guidance once it exists.

> These shared-file edits (`copier.yml`, `templates/README.md`, `CLAUDE.md`) are
> made by the coordinator, not by a parallel implementation agent, to avoid
> write conflicts with the concurrently-built `github-account` unit.
