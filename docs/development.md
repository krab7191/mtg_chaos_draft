# Local Development Guide

## Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (for Postgres)
- [Go 1.26+](https://go.dev/dl/) — `go version` to verify
- [Node 20+](https://nodejs.org/) — `node --version` to verify
- [air](https://github.com/air-verse/air) for Go hot reload:
  ```bash
  go install github.com/air-verse/air@latest
  ```

---

## 1. Google OAuth2 Credentials

You need a Google Cloud project with an OAuth2 client.

1. Go to [Google Cloud Console → APIs & Services → Credentials](https://console.cloud.google.com/apis/credentials)
2. Click **Create Credentials → OAuth 2.0 Client ID**
3. Application type: **Web application**
4. Name it anything (e.g. `MTG Chaos Draft Dev`)
5. Under **Authorized redirect URIs**, add:
   ```
   http://localhost:4321/api/auth/callback
   ```
6. Click **Create** — copy the **Client ID** and **Client Secret**

> You'll also need to configure the OAuth consent screen if you haven't already:
> APIs & Services → OAuth consent screen → External → fill in app name + your email.

---

## 2. Create your .env file

```bash
cp .env.example .env
```

Fill in `.env` at the project root:

```env
GOOGLE_CLIENT_ID=<paste Client ID>
GOOGLE_CLIENT_SECRET=<paste Client Secret>
GOOGLE_REDIRECT_URL=http://localhost:4321/api/auth/callback
ADMIN_EMAIL=<your Google account email>
DOMAIN=localhost
```

---

## 3. Start Postgres

Run just the database in Docker (no need for the full stack):

```bash
docker compose up postgres -d
```

Postgres will be available at `localhost:5432` with:
- DB: `mtg_chaos_draft`
- User: `mtg`
- Password: `mtg`

The Go API runs migrations automatically on startup — no manual SQL needed.

---

## 4. Run the Go API

Open a terminal in `api/`:

```bash
cd api
set -a && source ../.env && set +a
export DATABASE_URL=postgres://mtg:mtg@localhost:5432/mtg_chaos_draft
air
```

API will be at `http://localhost:8080`. Air watches all `.go` and `.sql` files and rebuilds on save.

---

## 5. Run the Astro Frontend

Open a second terminal in `frontend/`:

```bash
cd frontend

# Tell SSR pages where the API is
API_URL=http://localhost:8080 npm run dev
```

Frontend will be at **http://localhost:4321**

The Vite dev proxy is already configured — browser-side `/api/*` calls are automatically forwarded to `localhost:8080`, so Caddy is not needed locally.

---

## Shortcut: run everything with `make dev`

Instead of three terminals, install [hivemind](https://github.com/DarthSim/hivemind):

```bash
# macOS
brew install hivemind

# Linux (download from GitHub releases)
curl -Lo hivemind.gz https://github.com/DarthSim/hivemind/releases/latest/download/hivemind-Linux-x86_64.gz
gunzip hivemind.gz && chmod +x hivemind && sudo mv hivemind /usr/local/bin/
```

Then from the project root:

```bash
make dev
```

This starts all three processes (db, api, web) defined in `Procfile` in one terminal.

---

## Dev flow summary

| Terminal | Command |
|---|---|
| 1 | `docker compose up postgres -d` |
| 2 | `cd api && set -a && source ../.env && set +a && export DATABASE_URL=postgres://mtg:mtg@localhost:5432/mtg_chaos_draft && air` |
| 3 | `cd frontend && API_URL=http://localhost:8080 npm run dev` |

Then open **http://localhost:4321** — sign in with Google, get redirected based on your role.

---

## Pre-commit hooks

The repo ships with a `.pre-commit-config.yaml` that runs on every commit:
- `trailing-whitespace`, `end-of-file-fixer`, `check-yaml`, `check-added-large-files`
- `go fmt` and `go vet` on all Go files
- `astro check` (TypeScript check) on frontend files

To install the hooks:

```bash
pip install pre-commit   # or: brew install pre-commit
pre-commit install
```

After that, hooks run automatically on `git commit`. To run manually:

```bash
pre-commit run --all-files
```

---

## Useful commands

```bash
# View API logs
docker compose logs api -f

# Wipe and reset the database
docker compose down -v && docker compose up postgres -d

# Check the DB directly
docker compose exec postgres psql -U mtg -d mtg_chaos_draft

# Build everything (what CI does)
docker compose build
```
