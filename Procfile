db:  docker compose up postgres
api: cd api && PORT=8080 $HOME/go/bin/air
web: sh -c 'VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")-dev && cd frontend && PUBLIC_APP_VERSION=$VERSION npm run dev'
