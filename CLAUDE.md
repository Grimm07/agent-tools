# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

`agent-tools` is a polyglot **monorepo of reusable building blocks for AI agents** used across SDLC
workflows: standalone tools, MCP servers, and Claude Code skills. Each unit is self-contained and
independently buildable/testable — there is intentionally no repo-wide build that must run before
working on a single tool.

> The **go-mcp**, **python-tool**, **project-mcp**, **claude-agent**, and **skill** unit kinds have working,
> verified copier templates (see Scaffolding below) — prefer `make new` over hand-rolling them. Real MCP units live in
> `mcps/` (`github-account`, `aws-resources`) and are good worked examples of the conventions below.
> Conventions for things without a template yet (skills, libs, infra, CI) are still **prescriptive**;
> when you add the first real instance, update this file to point at the runnable command.

## Scaffolding new units

New units are generated from a single [copier](https://copier.readthedocs.io/) template in
`templates/`, switched by a `kind` answer. **Always scaffold via the Makefile** rather than copying by
hand — it keeps every unit at the quality bar and re-templatable:

```bash
make new KIND=go-mcp      NAME="Vector Store"   # -> mcps/vector-store/ (Go MCP server)
make new KIND=python-tool NAME="Log Parser"      # -> tools/log-parser/  (Python CLI tool)
make new KIND=project-mcp NAME="My Project"      # -> mcps/my-project/  (Go MCP server scoped to a project)
make new KIND=claude-agent NAME="Repo Auditor"   # -> .claude/agents/repo-auditor.md (Claude Code subagent)
make new KIND=skill        NAME="Flaky Finder"   # -> skills/flaky-finder/SKILL.md (Claude Code skill)
```

The **project-mcp** kind scaffolds a Go MCP server that exposes a project's own commands
(`run_tests`/`run_lint`/`run_build`), code search/read, docs, and read-only git state. It prompts for
the project's test/lint/build commands and writes them into a committed `project-mcp.toml` the server
reads at startup (editable without re-scaffolding). Commands are allowlisted argv (no arbitrary exec),
and file access is confined to the project root. Pass non-default commands through `make new` with
`DATA='--data test_command=["go","test","./..."]'`.

`make new` runs copier through `uvx` (no global install) and then runs the unit's setup task
(`go mod tidy` or `uv sync`), so the new unit is immediately testable. It needs network on first run.

Each unit commits a `.copier-answers.yml`; to pull later template improvements into an existing unit:
`cd <unit> && copier update --trust`. To add a new kind or change what gets generated, edit
`templates/` — see `templates/README.md`.

- Adding a `kind` means editing **both** `templates/copier.yml` (choices + gated questions) **and** the
  `Makefile` `new` target's `case "$(KIND)"` dest mapping — they are not auto-synced.
- Pass non-default copier answers via `make new ... DATA='--data key=value'`; `make` strips inner quotes
  from array answers (`["a","b"]`→`[a,b]`), so templates consuming argv arrays must re-normalize them.
- Scaffolding generates `cmd/<slug>/` with the hyphenated slug (e.g. `cmd/aws-resources/`), not de-hyphenated.

## Layout

| Path | Holds |
|------|-------|
| `tools/<name>/` | Standalone CLIs / utilities (one self-contained unit per dir) |
| `mcps/<name>/` | MCP servers (each its own module/package) |
| `skills/<name>/` | Claude Code skills (`SKILL.md` + supporting files) |
| `libs/go/`, `libs/python/` | Shared code reused by 2+ units. Don't put shared logic in a tool dir |
| `infra/` | OpenTofu (`.tf`) for AWS resources |
| `.github/workflows/` | CI per language/unit |

A unit graduates a helper into `libs/` only when a **second** unit needs it — avoid premature shared abstractions.

## Language choice (the core decision rule)

Default split: **Go where it makes sense, Python otherwise.**

- **Go** — CLIs and MCP servers meant to ship as a single static binary, anything
  latency/concurrency-sensitive, long-lived daemons, tools distributed to other machines.
- **Python** — ML/LLM glue, data wrangling, quick orchestration, anything leaning on a library that
  only exists (or is far better) in Python.
- If a different tool is clearly the right call for a unit (e.g. Bash for a thin wrapper), use it and
  note why in that unit's README.

State the language and the reason in each new unit's README so the choice is auditable.

## Per-unit conventions

Every unit (`tools/*`, `mcps/*`, `libs/*`) is independently runnable and carries its own per-unit
`Makefile` (`make test`, `make lint`, etc. — run from the unit dir):

- **Go MCP server** (`make new KIND=go-mcp`): own `go.mod` (module path
  `github.com/Grimm07/agent-tools/mcps/<name>`), built on the official
  `github.com/modelcontextprotocol/go-sdk` (requires Go ≥ 1.25 — `GOTOOLCHAIN=auto` fetches it).
  `*_test.go` alongside code. `make test` → `go test -race ./...`; `make lint` → `gofmt -l` check +
  `go vet` + `staticcheck` (skipped with a hint if not installed). Tool handlers are kept as plain
  functions so they unit-test directly; full dispatch is tested via `mcp.NewInMemoryTransports()`.
  For servers wrapping an external API, inject the client so handlers test offline: REST clients
  (e.g. `go-github`) via an `httptest.Server` with `client.BaseURL` overridden; `aws-sdk-go-v2` via the
  SDK's own `*APIClient` paginator interfaces with fake implementations (never live calls). For
  read-only servers, import only `Get/List/Describe` methods and guard with
  `grep -E '(Create|Delete|Put|Update|Modify|Terminate)[A-Z]' internal/` (filter the benign `CreateDate` field).
- **Python tool** (`make new KIND=python-tool`): `pyproject.toml` per package, **uv**-managed, src
  layout (`src/<module>/`). Logic lives in `core.py`, free of argparse, so it tests offline; `cli.py`
  is a thin shell. `make test` → `uv run pytest`; `make lint` → `ruff check` + `mypy` (strict). Prefer
  the stdlib and a small, justified dependency set.
- **Skills** (`make new KIND=skill`): a `skills/<slug>/SKILL.md` with YAML frontmatter (`name`,
  `description`) plus any scripts/references it needs. The `description` is the only text Claude reads
  to decide whether to load the skill, so it must list *triggering conditions* ("Use when…") and must
  **not** summarize the workflow — a process-summary description makes Claude act on the summary and
  skip the body. The scaffold renders the recommended section skeleton with fill-in guidance comments;
  it has no `Makefile`/`make test` (a skill is Markdown), so it's the one unit kind exempt from the
  per-unit `Makefile` rule above.

CI is not wired yet: GitHub only runs workflows from the repo-root `.github/workflows/`, so per-unit CI
needs a path-filtered root workflow — design that when the first unit needs it, mirroring the unit's
`make test` / `make lint` targets.

## Quality bar (non-negotiable, per owner preference)

- **Lush docs.** Every unit has a README covering purpose, the language-choice rationale, and exact
  run/build/test commands. Public Go identifiers carry doc comments; Python public APIs carry docstrings.
- **Full test suites.** New behavior ships with tests in the same change. Cover the edge cases, not just
  the happy path. No unit is "done" without them.
- **Cheap, fast runs.** Tests run offline and deterministically — no real AWS/network calls in unit
  tests (fake/mock the boundary; reserve live calls for explicitly-tagged integration tests that don't
  run by default). Keep the dependency surface and cold-start cost low. CI should be runnable per-unit,
  not all-or-nothing.

## Infrastructure & platform

- **IaC is OpenTofu** (`tofu`, not `terraform`): `tofu fmt`, `tofu validate`, `tofu plan` from `infra/`.
  State/backend lives in AWS — never commit state or `.tfvars` containing secrets.
- **Cloud is AWS.** Prefer least-privilege IAM; pull credentials from the environment/SSO, never hardcode.
- **VCS/CI is GitHub** + GitHub Actions. Keep workflows matrixed/path-filtered so changing one unit
  doesn't rebuild the whole monorepo.

## Editor note

Code is read in JetBrains IDEs, Claude Code, Cursor, and occasionally Copilot — keep formatting tool-driven
(`gofmt`, `ruff format`, `tofu fmt`) so it's stable across all of them rather than hand-aligned.
