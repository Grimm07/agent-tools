# Templates

A single [copier](https://copier.readthedocs.io/) template that scaffolds new repo units. The `kind`
answer selects which project tree is rendered (via a Jinja-templated `_subdirectory`), so one shared
questionnaire produces a Go MCP server, a Python tool, or a project-scoped MCP server.

## Usage

From the repo root:

```bash
make new KIND=go-mcp      NAME="Vector Store"   # -> mcps/vector-store/
make new KIND=python-tool NAME="Log Parser"     # -> tools/log-parser/
make new KIND=project-mcp NAME="My Project"     # -> mcps/my-project/
```

`make new` runs copier ephemerally through `uvx` (nothing to install globally) with `--trust`
(required because the template has post-generation `_tasks`) and `--defaults` (non-interactive; uses
the defaults for `description`/`author`). Override any answer with `--data`, e.g.:

```bash
uvx --from 'copier>=9,<10' copier copy --trust \
  --data kind=go-mcp --data name="Vector Store" \
  --data description="Semantic search over docs." templates/ mcps/vector-store
```

## What the answers drive

| Answer | Derived from | Used for |
|--------|--------------|----------|
| `slug` | `name` kebab-cased | directory, binary/script name, module/dist name |
| `go_pkg` | `slug` without `-` | Go package identifier (`internal/<go_pkg>`) |
| `py_module` | `slug` with `-`→`_` | Python import package (`src/<py_module>`) |

Post-gen `_tasks` run `go mod tidy` (go-mcp, project-mcp) or `uv sync` (python-tool), so a freshly
scaffolded unit is immediately testable. `make new` therefore needs network access on first run.

## The `project-mcp` kind

`project-mcp` generates a self-contained **Go** MCP server (built on the same SDK as `go-mcp`) that
exposes a single software project's commands, code, docs, and read-only git state to an agent over
stdio. It renders into `mcps/<slug>/` and ships these tools: `run_tests` / `run_lint` / `run_build`
(each registered only when configured), `search_code`, `read_file`, `list_docs`, `read_doc`,
`git_status`, `git_diff`, and `git_log`. See the generated unit's README for the full tool list and
security model (allowlisted argv only — never `sh -c`; path confinement; timeouts; output caps;
read-only git).

All behavior is driven by a committed `project-mcp.toml`, seeded at scaffold time from three optional
questions (asked only when `kind == project-mcp`) and read at runtime so commands stay editable later:

| Question | Meaning |
|----------|---------|
| `test_command` | argv for `run_tests`, as a TOML array, e.g. `["go","test","./..."]` (empty = tool disabled) |
| `lint_command` | argv for `run_lint`, e.g. `["golangci-lint","run"]` (empty = disabled) |
| `build_command` | argv for `run_build`, e.g. `["go","build","./..."]` (empty = disabled) |

Each answer is a TOML **argv array**, not a shell string — the server execs the argv directly. The
template re-normalizes the answer into valid TOML, so it renders correctly whether the inner quotes
survive or are stripped by an intervening shell. To pass a command through `make new`, use the `DATA`
passthrough (which forwards extra `--data` flags to copier):

```bash
make new KIND=project-mcp NAME="My Project" \
  DATA='--data test_command=["go","test","./..."]'
```

Or call copier directly (quotes are preserved verbatim):

```bash
uvx --from 'copier>=9,<10' copier copy --trust --defaults \
  --data kind=project-mcp --data name="My Project" \
  --data 'test_command=["go","test","./..."]' \
  --data 'lint_command=["golangci-lint","run"]' \
  templates/ mcps/my-project
```

Edit `project-mcp.toml` afterward to enable/adjust commands or the exposed `docs.paths`; changes take
effect on the next run with no re-scaffold.

## Updating existing units

Each generated unit commits a `.copier-answers.yml`. To roll template improvements into it:

```bash
cd mcps/vector-store
copier update --trust
```

`copier update` diffs template versions using this repo's git history, so commit (and ideally tag)
template changes before running it. With no tag, copier compares against `HEAD`.

## Layout

```
templates/
├── copier.yml          # shared questionnaire, _subdirectory, _tasks
├── go-mcp/             # rendered when kind == go-mcp
├── python-tool/        # rendered when kind == python-tool
└── project-mcp/        # rendered when kind == project-mcp
```

## Adding a new kind

1. Create `templates/<new-kind>/` with the project tree; suffix templated files with `.jinja` and use
   `{{ slug }}` / `{{ go_pkg }}` / `{{ py_module }}` in file contents and path names.
2. Add a `{{ _copier_conf.answers_file }}.jinja` at its root (body: `{{ '{{' }} _copier_answers | to_nice_yaml {{ '}}' }}`).
3. Add the kind to the `kind` choices in `copier.yml` (and any new `_tasks`).
4. Map it to an output directory in the root `Makefile` `new` target.
```
