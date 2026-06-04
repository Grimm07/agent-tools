# agent-tools

A polyglot **monorepo of reusable building blocks for AI agents** — standalone tools, MCP servers, and
Claude Code skills used across SDLC workflows. Every unit is self-contained: independently buildable,
testable, and runnable, with its own README and per-unit `Makefile`. There is intentionally **no
repo-wide build** you must run before working on a single unit.

> Building or working *inside* a unit? The authoritative conventions for this repo live in
> [`CLAUDE.md`](CLAUDE.md). This README is the orientation; `CLAUDE.md` is the rulebook.

## What's here

| Path | Holds |
|------|-------|
| [`mcps/`](mcps/) | MCP servers — each its own Go module/package |
| [`tools/`](tools/) | Standalone CLIs / utilities (one self-contained unit per dir) |
| [`skills/`](skills/) | Claude Code skills (`SKILL.md` + supporting files) |
| [`libs/go/`](libs/go), [`libs/python/`](libs/python) | Shared code, added only once a **second** unit needs it |
| [`infra/`](infra/) | OpenTofu (`.tf`) for AWS resources |
| [`templates/`](templates/) | The copier scaffolding template (see below) |
| [`docs/`](docs/) | Design specs and plans |

### MCP servers currently in `mcps/`

| Unit | What it does |
|------|--------------|
| [`aws-resources`](mcps/aws-resources/) | Read-only MCP server to view live AWS resources |
| [`github-account`](mcps/github-account/) | Read-only MCP server to browse a GitHub account |
| [`agent-tools-mcp`](mcps/agent-tools-mcp/) | `project-mcp` scoped to **this** repo — code search/read, docs, read-only git |
| [`portfolio-mcp`](mcps/portfolio-mcp/) | `project-mcp` scoped to the sibling `portfolio` repo |
| [`infrastructure-mcp`](mcps/infrastructure-mcp/) | `project-mcp` scoped to the sibling `infrastructure` repo |

## Scaffolding a new unit

New units are generated from a single [copier](https://copier.readthedocs.io/) template, switched by a
`kind` answer. **Always scaffold via the Makefile** — it keeps every unit at the quality bar and
re-templatable. It runs copier through `uvx` (nothing to install globally) and runs the unit's setup
task, so the result is immediately testable (needs network on first run).

```bash
make new KIND=go-mcp       NAME="Vector Store"   # -> mcps/vector-store/   (Go MCP server)
make new KIND=python-tool  NAME="Log Parser"     # -> tools/log-parser/    (Python CLI tool)
make new KIND=project-mcp  NAME="My Project"     # -> mcps/my-project/     (MCP scoped to a project)
make new KIND=claude-agent NAME="Repo Auditor"   # -> .claude/agents/repo-auditor.md
make new KIND=skill        NAME="Flaky Finder"   # -> skills/flaky-finder/SKILL.md
```

Each generated unit commits a `.copier-answers.yml`; pull later template improvements into it with
`cd <unit> && copier update --trust`. To add or change a `kind`, edit [`templates/`](templates/) — see
[`templates/README.md`](templates/README.md).

## Working in a unit

Every unit carries its own `Makefile`; run targets from the unit directory:

```bash
cd mcps/github-account
make test    # Go: go test -race ./...   |  Python: uv run pytest
make lint    # Go: gofmt + go vet + staticcheck  |  Python: ruff + mypy
```

(Skills are Markdown and have no `Makefile`.)

## Language choice

Default split: **Go where it makes sense, Python otherwise.**

- **Go** — CLIs and MCP servers that ship as a single static binary, anything latency/concurrency-
  sensitive, long-lived daemons, tools distributed to other machines.
- **Python** — ML/LLM glue, data wrangling, quick orchestration, anything leaning on a library that is
  better (or only exists) in Python.

Each unit states its language **and the reason** in its README, so the choice is auditable.

## Quality bar

Non-negotiable for every unit (details in [`CLAUDE.md`](CLAUDE.md)):

- **Lush docs** — a README with purpose, language rationale, and exact run/build/test commands; doc
  comments on public Go identifiers and docstrings on public Python APIs.
- **Full test suites** — new behavior ships with tests in the same change, covering edge cases.
- **Cheap, fast runs** — tests run offline and deterministically (no real AWS/network calls; mock the
  boundary). CI is per-unit, not all-or-nothing.

## Platform

- **IaC** is OpenTofu (`tofu`), state in AWS — never commit state or secret `.tfvars`.
- **Cloud** is AWS — least-privilege IAM, credentials from the environment/SSO, never hardcoded.
- **VCS/CI** is GitHub + GitHub Actions, kept matrixed/path-filtered so changing one unit doesn't
  rebuild the whole monorepo.
