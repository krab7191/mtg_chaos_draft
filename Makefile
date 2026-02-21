-include .env
export

DATABASE_URL ?= postgres://mtg:mtg@localhost:5432/mtg_chaos_draft
HIVEMIND := $(HOME)/go/bin/hivemind
AIR      := $(HOME)/go/bin/air

.PHONY: dev db api frontend

dev: ## Start everything for local development (requires hivemind)
	$(HIVEMIND)

db: ## Start postgres in Docker (standalone)
	docker compose up postgres -d

api: ## Start Go API with hot reload
	cd api && $(AIR)

frontend: ## Start Astro dev server
	cd frontend && npm run dev
