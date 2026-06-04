# agent-tools — monorepo task runner.
#
# `make new` scaffolds a unit from the copier template in templates/.
.PHONY: help new

help: ## List targets
	@grep -hE '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*##"}{printf "  \033[36m%-6s\033[0m %s\n", $$1, $$2}'

new: ## Scaffold a unit: make new KIND=<go-mcp|python-tool|project-mcp|claude-agent> NAME="<name>"
	@test -n "$(KIND)" || { echo 'KIND required (go-mcp|python-tool|project-mcp|claude-agent)'; exit 1; }
	@test -n "$(NAME)" || { echo 'NAME required, e.g. NAME="vector store"'; exit 1; }
	@slug=$$(printf '%s' "$(NAME)" | tr '[:upper:]' '[:lower:]' | tr ' _' '--' | sed 's/[^a-z0-9-]//g'); \
	if [ "$(KIND)" = "claude-agent" ]; then \
	  dest=".claude/agents/$$slug.md"; \
	  if [ -e "$$dest" ]; then echo "$$dest already exists"; exit 1; fi; \
	  tmp=$$(mktemp -d); \
	  uvx --from 'copier>=9,<10' copier copy --trust --defaults \
	    --data kind="$(KIND)" --data name="$(NAME)" $(DATA) \
	    templates/ "$$tmp" >/dev/null; \
	  mkdir -p .claude/agents; \
	  mv "$$tmp/$$slug.md" "$$dest"; \
	  rm -rf "$$tmp"; \
	  echo "Created claude-agent -> $$dest"; \
	  exit 0; \
	fi; \
	case "$(KIND)" in \
	  go-mcp)      dest="mcps/$$slug" ;; \
	  python-tool) dest="tools/$$slug" ;; \
	  project-mcp) dest="mcps/$$slug" ;; \
	  *) echo "unknown KIND: $(KIND) (want go-mcp|python-tool|project-mcp|claude-agent)"; exit 1 ;; \
	esac; \
	if [ -e "$$dest" ]; then echo "$$dest already exists"; exit 1; fi; \
	echo "Scaffolding $(KIND) -> $$dest"; \
	uvx --from 'copier>=9,<10' copier copy --trust --defaults \
	  --data kind="$(KIND)" --data name="$(NAME)" $(DATA) \
	  templates/ "$$dest"
