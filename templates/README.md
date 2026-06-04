# Templates

A single [copier](https://copier.readthedocs.io/) template that scaffolds new repo units. The `kind`
answer selects which project tree is rendered (via a Jinja-templated `_subdirectory`), so one shared
questionnaire produces either a Go MCP server or a Python tool.

## Usage

From the repo root:

```bash
make new KIND=go-mcp      NAME="Vector Store"   # -> mcps/vector-store/
make new KIND=python-tool NAME="Log Parser"     # -> tools/log-parser/
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

Post-gen `_tasks` run `go mod tidy` (go-mcp) or `uv sync` (python-tool), so a freshly scaffolded unit is
immediately testable. `make new` therefore needs network access on first run.

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
└── python-tool/        # rendered when kind == python-tool
```

## Adding a new kind

1. Create `templates/<new-kind>/` with the project tree; suffix templated files with `.jinja` and use
   `{{ slug }}` / `{{ go_pkg }}` / `{{ py_module }}` in file contents and path names.
2. Add a `{{ _copier_conf.answers_file }}.jinja` at its root (body: `{{ '{{' }} _copier_answers | to_nice_yaml {{ '}}' }}`).
3. Add the kind to the `kind` choices in `copier.yml` (and any new `_tasks`).
4. Map it to an output directory in the root `Makefile` `new` target.
```
