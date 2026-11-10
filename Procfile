db:  docker compose up postgres
api: cd api && PORT=8080 GOOGLE_REDIRECT_URL=${DEV_GOOGLE_REDIRECT_URL:-http://localhost:4321/api/auth/callback} $HOME/go/bin/air
web: sh -c 'VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")-dev && cd frontend && PUBLIC_APP_VERSION=$VERSION npm run dev'
