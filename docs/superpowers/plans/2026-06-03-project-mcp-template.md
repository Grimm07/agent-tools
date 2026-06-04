# project-mcp copier template Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `project-mcp` copier kind (`templates/project-mcp/`) that scaffolds a self-contained Go MCP server exposing a project's commands, code search/read, docs, and read-only git state, configured via a committed `project-mcp.toml`.

**Architecture:** Build a **working reference server first** (plain Go, real values), drive it to green with TDD, then **templatize** it into Jinja (`.jinja` files, `{{ slug }}`/`{{ go_pkg }}` substitutions) and wire it into `templates/copier.yml`. This guarantees the template renders to code that actually compiles and tests pass.

**Tech Stack:** Go ≥1.25, `github.com/modelcontextprotocol/go-sdk`, `github.com/BurntSushi/toml`, stdlib `os/exec`, `path/filepath`, `regexp`. Copier (Jinja2) for templating.

**Authoritative detail source:** `docs/superpowers/specs/2026-06-03-project-mcp-template-design.md` (config shape, tools, security rules). This plan defines build order and TDD steps.

**Build location:** Develop the reference implementation in a scratch unit `mcps/_project-mcp-ref/` (not committed to `mcps/` long-term — it becomes the template source). Final committed artifacts are `templates/project-mcp/**` and `templates/copier.yml` edits.

---

### Phase A — Reference implementation (real Go, TDD)

### Task 1: Scaffold reference unit

- [ ] **Step 1:** `make new KIND=go-mcp NAME="Project MCP Ref"` → `mcps/project-mcp-ref/`.
- [ ] **Step 2:** `cd mcps/project-mcp-ref && make test` → PASS.
- [ ] **Step 3:** `go get github.com/BurntSushi/toml && go mod tidy`. Remove sample greet tool/test. Commit `chore(project-mcp): scaffold reference unit`.

### Task 2: Config parsing + root resolution

**Files:** `internal/projectmcpref/config.go`, `config_test.go`

- [ ] **Step 1: Failing test**

```go
func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "project-mcp.toml"), []byte(`
root = "."
[commands]
test = ["go", "test", "./..."]
[docs]
paths = ["README.md"]
`), 0o600)
	cfg, err := LoadConfig(filepath.Join(dir, "project-mcp.toml"))
	if err != nil { t.Fatal(err) }
	if cfg.Commands["test"][0] != "go" { t.Fatalf("got %+v", cfg.Commands) }
	if !filepath.IsAbs(cfg.Root) { t.Fatalf("root must resolve absolute, got %q", cfg.Root) }
}
```

- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** `Config` struct (`Root string`, `Commands map[string][]string`, `Docs struct{ Paths []string }`), `LoadConfig(path)` that decodes TOML via `toml.DecodeFile`, resolves `Root` to an absolute path relative to the config file's dir (default "."), and validates it exists. Add `confinePath(root, rel string) (string, error)` that `filepath.Join`s, resolves symlinks via `filepath.EvalSymlinks`, and returns an error if the result is outside `root`.
- [ ] **Step 4:** Add a **path-confinement test**: `confinePath(root, "../etc/passwd")` returns an error. Run → PASS.
- [ ] **Step 5:** Commit `feat(project-mcp): config parsing + path confinement`.

### Task 3: Command tools (`run_tests`/`run_lint`/`run_build`)

**Files:** `internal/projectmcpref/commands.go`, `commands_test.go`

- [ ] **Step 1: Failing test** (use a portable stub command)

```go
func TestRunCommand(t *testing.T) {
	cfg := Config{Root: t.TempDir(), Commands: map[string][]string{
		"test": {"printf", "ok"},
	}}
	res, err := RunCommand(context.Background(), cfg, "test", 5*time.Second)
	if err != nil { t.Fatal(err) }
	if res.ExitCode != 0 || !strings.Contains(res.Output, "ok") {
		t.Fatalf("got %+v", res)
	}
}

func TestRunCommandDisabled(t *testing.T) {
	cfg := Config{Root: t.TempDir(), Commands: map[string][]string{}}
	if _, err := RunCommand(context.Background(), cfg, "test", time.Second); err == nil {
		t.Fatal("expected error for unconfigured command")
	}
}
```

- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** `CommandResult{ ExitCode int; Output string; TimedOut bool }` and `RunCommand(ctx, cfg, name, timeout)`:
  - Look up `cfg.Commands[name]`; if empty → error "command %q not configured".
  - `exec.CommandContext` with `argv[0]` + `argv[1:]` (NEVER `sh -c`); set `cmd.Dir = cfg.Root`.
  - Capture combined output; truncate to a cap const (e.g. 64 KiB); set `TimedOut` if `ctx.Err()==DeadlineExceeded`.
  - Return non-zero exit as a populated result (not a Go error).
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5:** Add MCP handlers `run_tests`/`run_lint`/`run_build` (each calls `RunCommand` with its name) — but **only register them in `NewServer` when the command is configured** (non-empty). Commit `feat(project-mcp): allowlisted command tools`.

### Task 4: Code tools (`search_code`, `read_file`)

**Files:** `internal/projectmcpref/fsutil.go`, `fsutil_test.go`

- [ ] **Step 1: Failing tests** — create temp files; assert `SearchCode(root, "needle", cap)` returns `path:line:match` hits; assert `ReadFile(cfg, "sub/a.txt")` returns content; assert `ReadFile(cfg, "../escape")` errors (uses `confinePath`).
- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** `SearchCode` (walk `root` with `filepath.WalkDir`, skip `.git`, compile the pattern with `regexp.Compile`, collect capped hits) and `ReadFile` (via `confinePath`, size-capped read). Register `search_code`/`read_file` handlers.
- [ ] **Step 4: Run** → PASS. Commit `feat(project-mcp): code search + read with confinement`.

### Task 5: Docs tools (`list_docs`, `read_doc`)

**Files:** `internal/projectmcpref/docs.go`, `docs_test.go`

- [ ] **Step 1: Failing tests** — `ListDocs(cfg)` expands `cfg.Docs.Paths` (files + dir contents) under root; `ReadDoc(cfg, path)` reads only files resolving under a configured docs path, else errors.
- [ ] **Step 2–4:** Implement, register handlers, run → PASS. Commit `feat(project-mcp): docs tools`.

### Task 6: Git tools (`git_status`, `git_diff`, `git_log`)

**Files:** `internal/projectmcpref/git.go`, `git_test.go`

- [ ] **Step 1: Failing test** (build a throwaway repo)

```go
func TestGitStatus(t *testing.T) {
	root := t.TempDir()
	run(t, root, "git", "init")
	os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o600)
	entries, err := GitStatus(context.Background(), Config{Root: root})
	if err != nil { t.Fatal(err) }
	if len(entries) == 0 { t.Fatal("expected untracked entry") }
}
```
(`run` is a test helper running a command in `root`.)

- [ ] **Step 2: Run** → FAIL.
- [ ] **Step 3: Implement** `GitStatus` (`git status --porcelain`, parse to `{status, path}`), `GitDiff(ctx, cfg, staged bool, path string)` (`git diff [--staged] [-- path]`, capped), `GitLog(ctx, cfg, count int)` (`git log -n <cap> --pretty=...` → sha, author, date, subject). All via `exec.CommandContext` with `cmd.Dir = cfg.Root`, read-only flags only, under timeout.
- [ ] **Step 4: Run** → PASS. Register handlers. Commit `feat(project-mcp): read-only git tools`.

### Task 7: server.go wiring + startup

**Files:** `internal/projectmcpref/server.go`

- [ ] **Step 1:** Implement `NewServer()` to: find `project-mcp.toml` (cwd, or `PROJECT_MCP_CONFIG` env), `LoadConfig`, build the server, register code/docs/git tools always and command tools conditionally, return `(*mcp.Server, error)`. `Run(ctx)` loads and serves; on config error, exits with a clear message.
- [ ] **Step 2:** `server_test.go` — `NewServer` errors when no config present; succeeds against a temp config (set `PROJECT_MCP_CONFIG`).
- [ ] **Step 3:** `make test && make lint` → PASS. Commit `feat(project-mcp): server wiring + config-driven registration`.

---

### Phase B — Templatize

### Task 8: Convert reference unit into `templates/project-mcp/`

**Files:**
- Create: `templates/project-mcp/**` (mirror of go-mcp's structure)

- [ ] **Step 1:** Copy go-mcp's template skeleton names: `go.mod.jinja`, `Makefile.jinja`, `README.md.jinja`, `.gitignore`, `{{ _copier_conf.answers_file }}.jinja`, `cmd/{{ slug }}/main.go.jinja`, `internal/{{ go_pkg }}/*.go.jinja`.
- [ ] **Step 2:** Port each reference `.go` file to `internal/{{ go_pkg }}/<name>.go.jinja`, replacing `package projectmcpref` → `package {{ go_pkg }}`, and the module import path → `github.com/Grimm07/agent-tools/mcps/{{ slug }}/internal/{{ go_pkg }}`. Keep test files as `*_test.go.jinja`.
- [ ] **Step 3:** Add `templates/project-mcp/project-mcp.toml.jinja` seeded from copier answers:
```toml
root = "."

[commands]
{% if test_command %}test = {{ test_command }}{% endif %}
{% if lint_command %}lint = {{ lint_command }}{% endif %}
{% if build_command %}build = {{ build_command }}{% endif %}

[docs]
paths = ["README.md", "CLAUDE.md", "docs"]
```
(Answers are entered as TOML arrays, e.g. `["go","test","./..."]`; validate in copier — see Task 9.)
- [ ] **Step 4:** README.md.jinja documents purpose, Go rationale, config editing, the tool list, and the security model.
- [ ] **Step 5:** Commit `feat(templates): add project-mcp template tree`.

### Task 9: Wire the kind into copier.yml

**Files:**
- Modify: `templates/copier.yml`

- [ ] **Step 1:** Add `project-mcp` to the `kind` `choices` list.
- [ ] **Step 2:** Add gated questions:
```yaml
test_command:
  type: str
  help: 'Test command as a TOML array, e.g. ["go","test","./..."] (empty to disable)'
  default: ""
  when: "{{ kind == 'project-mcp' }}"
lint_command:
  type: str
  help: 'Lint command as a TOML array (empty to disable)'
  default: ""
  when: "{{ kind == 'project-mcp' }}"
build_command:
  type: str
  help: 'Build command as a TOML array (empty to disable)'
  default: ""
  when: "{{ kind == 'project-mcp' }}"
```
- [ ] **Step 3:** Add a `_tasks` entry running `go mod tidy` when `kind == 'project-mcp'` (mirror go-mcp).
- [ ] **Step 4:** Commit `feat(templates): register project-mcp kind in copier.yml`.

### Task 10: Render-and-test the template (acceptance)

- [ ] **Step 1:** From repo root: `make new KIND=project-mcp NAME="Demo Project"` with `test_command=["go","test","./..."]`. Expected: generates `mcps/demo-project/`, runs `go mod tidy`.
- [ ] **Step 2:** `cd mcps/demo-project && make test` → Expected: PASS (the ported tests run against the generated code).
- [ ] **Step 3:** `make lint` → no findings.
- [ ] **Step 4:** Remove the scratch reference unit `mcps/project-mcp-ref/` and the `mcps/demo-project/` acceptance render (keep the repo clean; the template is the deliverable). Commit `test(templates): verify project-mcp renders and passes; remove scratch`.

### Task 11: Docs

**Files:** `templates/README.md`

- [ ] **Step 1:** Document the `project-mcp` kind in `templates/README.md` (what it generates, the copier questions, config editing). Commit `docs(templates): document project-mcp kind`.

> **Coordinator note:** the root `CLAUDE.md` scaffolding/layout edits are made by the
> coordinator (not this plan's agent) to avoid conflicting with the parallel
> github-account / aws-resources work, which also touches CLAUDE.md.

---

## Self-Review

- **Spec coverage:** config-seeded/runtime-read TOML (Tasks 2, 8) ✓; command tools allowlist-only + conditional registration (Task 3, 7) ✓; code search/read with confinement (Tasks 2, 4) ✓; docs tools (Task 5) ✓; read-only git tools (Task 6) ✓; no-arbitrary-exec + no `sh -c` (Task 3) ✓; path confinement + symlink eval (Task 2) ✓; timeouts + output caps (Tasks 3, 6) ✓; new copier kind + gated questions + `_tasks` (Task 9) ✓; render acceptance test (Task 10) ✓; templates/README (Task 11) ✓.
- **Placeholders:** none — each tool has concrete signatures, test code, and exec contracts. Build-first-then-templatize removes the usual "does the template compile" risk.
- **Type consistency:** `Config`, `LoadConfig`, `confinePath`, `CommandResult`, `RunCommand`, `SearchCode`, `ReadFile`, `ListDocs`/`ReadDoc`, `GitStatus`/`GitDiff`/`GitLog` defined in Phase A and referenced unchanged when templatized in Phase B.
- **Risk noted:** copier answers as TOML-array strings need validation; Task 9 questions document the format. If brittle, fallback is three scalar string answers joined into argv in the template — acceptable alternative.
