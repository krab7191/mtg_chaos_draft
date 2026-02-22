-include .env
export

DATABASE_URL ?= postgres://mtg:mtg@localhost:5432/mtg_chaos_draft
HIVEMIND := $(HOME)/go/bin/hivemind
AIR      := $(HOME)/go/bin/air

.PHONY: dev db api frontend check install

dev: ## Start everything for local development (requires hivemind)
	$(HIVEMIND)

db: ## Start postgres in Docker (standalone)
	docker compose up postgres -d

api: ## Start Go API with hot reload
	cd api && $(AIR)

frontend: ## Start Astro dev server
	cd frontend && npm run dev

install: ## Install all dev dependencies and git hooks
	cd frontend && npm install
	go install github.com/air-verse/air@latest
	go install github.com/DarthSim/hivemind@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	pre-commit install

check: ## Run pre-commit checks (fmt, vet, astro check)
	pre-commit run --all-files
