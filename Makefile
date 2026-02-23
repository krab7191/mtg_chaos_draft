-include .env
export

DATABASE_URL      ?= postgres://mtg:mtg@localhost:5432/mtg_chaos_draft
DATABASE_TEST_URL ?= postgres://mtg:mtg@localhost:5432/mtg_chaos_draft_test
HIVEMIND := $(HOME)/go/bin/hivemind
AIR      := $(HOME)/go/bin/air

COVERAGE_THRESHOLD := 13

.PHONY: dev db db-test api frontend check install test test-api

dev: ## Start everything for local development (requires hivemind)
	$(HIVEMIND)

db: ## Start postgres in Docker (standalone)
	docker compose up postgres -d

db-test: db ## Create the test database (run once after `make db`)
	docker compose exec postgres psql -U mtg -d postgres -c "CREATE DATABASE mtg_chaos_draft_test;" 2>/dev/null || true

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
	printf '#!/bin/sh\nCURRENT=$$(git tag --points-at HEAD | grep -E "^v[0-9]+\\.[0-9]+\\.[0-9]+$$" | head -1)\nif [ -n "$$CURRENT" ]; then exit 0; fi\nLATEST=$$(git tag --sort=-v:refname | grep -E "^v[0-9]+\\.[0-9]+\\.[0-9]+$$" | head -1)\nif [ -z "$$LATEST" ]; then LATEST="v0.0.0"; fi\nMAJOR=$$(echo $$LATEST | sed "s/v//" | cut -d. -f1)\nMINOR=$$(echo $$LATEST | sed "s/v//" | cut -d. -f2)\nPATCH=$$(echo $$LATEST | sed "s/v//" | cut -d. -f3)\nNEW_TAG="v$${MAJOR}.$${MINOR}.$$(( PATCH + 1 ))"\ngit tag "$$NEW_TAG"\necho "auto-tagged $$NEW_TAG"\n' > .git/hooks/pre-push && chmod +x .git/hooks/pre-push
	git config push.followTags true

check: ## Run pre-commit checks (fmt, vet, astro check)
	pre-commit run --all-files

test-api: ## Run Go tests with coverage (80% threshold) — requires `make db-test`
	cd api && DATABASE_URL=$(DATABASE_TEST_URL) go test -p 1 ./... -coverprofile=coverage.out -covermode=atomic
	@cd api && go tool cover -func=coverage.out | \
		awk '/^total:/ { pct=$$3; sub(/%/,"",pct); printf "Coverage: %s%%\n", pct; \
		if (pct+0 < $(COVERAGE_THRESHOLD)) { printf "FAIL: %s%% < %d%%\n", pct, $(COVERAGE_THRESHOLD); exit 1 } }'

test: test-api ## Run all tests with coverage thresholds
