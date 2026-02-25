-include .env
export

DATABASE_URL      ?= postgres://mtg:mtg@localhost:5432/mtg_chaos_draft
DATABASE_TEST_URL ?= postgres://mtg:mtg@localhost:5432/mtg_chaos_draft_test
HIVEMIND := $(HOME)/go/bin/hivemind
AIR      := $(HOME)/go/bin/air

COVERAGE_THRESHOLD := 60

.PHONY: dev db db-test api frontend check install test test-api test-frontend

dev: ## Start everything for local development (requires hivemind)
	$(HIVEMIND)

db: ## Start postgres in Docker (standalone)
	docker compose up postgres -d

db-test: db ## Create the test database (run once after `make db`)
	until docker compose exec postgres pg_isready -U mtg -q; do true; done
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

check: ## Run pre-commit checks (fmt, vet, astro check) and tests
	pre-commit run --all-files
	$(MAKE) test

test-api: ## Run Go tests with coverage (40% threshold)
	@cd api && DATABASE_URL=$(DATABASE_TEST_URL) go test -p 1 ./... -coverprofile=coverage.out -covermode=atomic \
		-coverpkg=mtg-chaos-draft,mtg-chaos-draft/db,mtg-chaos-draft/handlers,mtg-chaos-draft/middleware \
		| grep -E "^(ok|FAIL)\b|^--- (FAIL|PASS):|panic:" | sed 's/ of statements in.*//' \
		| sed -E 's/^(ok|FAIL)(\s+)mtg-chaos-draft(\s)/\1\2mtg-chaos-draft\/main.go\3/' \
		| column -t
	@cd api && go tool cover -func=coverage.out | \
		awk '/^total:/ { pct=$$3; sub(/%/,"",pct); printf "Coverage: %s%%\n", pct; \
		if (pct+0 < $(COVERAGE_THRESHOLD)) { printf "FAIL: %s%% < %d%%\n", pct, $(COVERAGE_THRESHOLD); exit 1 } }'

coverage: ## Open HTML coverage report in browser (run make test-api first)
	cd api && go tool cover -html=coverage.out -o coverage.html && explorer.exe coverage.html

test-frontend: ## Run frontend tests with coverage
	cd frontend && npm test

test: test-api test-frontend ## Run all tests with coverage thresholds
