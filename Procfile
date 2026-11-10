db:  docker compose up postgres
api: cd api && PORT=8080 GOOGLE_REDIRECT_URL=${DEV_GOOGLE_REDIRECT_URL:-http://localhost:4321/api/auth/callback} $HOME/go/bin/air
web: sh -c 'git fetch --tags --force --quiet 2>/dev/null; LATEST=$(git tag --list "v*" --sort=-v:refname | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+$" | head -1); VERSION=${LATEST:-v0.0.0}-dev && cd frontend && PUBLIC_APP_VERSION=$VERSION npm run dev'
