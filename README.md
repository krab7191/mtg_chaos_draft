# MTG Chaos Draft

[![Deploy](https://github.com/krab7191/mtg_chaos_draft/actions/workflows/deploy.yml/badge.svg)](https://github.com/krab7191/mtg_chaos_draft/actions/workflows/deploy.yml)

A personal app for facilitating Magic: The Gathering chaos drafts. Admins manage a sealed product collection; players get a weighted-random pack drawn from it.

## Stack

- **API**: Go + chi, PostgreSQL
- **Frontend**: Astro (SSR, Node adapter)
- **Auth**: Google OAuth2 with server-side sessions
- **Reverse proxy**: Caddy
- **Infra**: Oracle Cloud Always Free (ARM VM), provisioned with Terraform

## Features

- Google SSO — admin role granted to `ADMIN_EMAIL`
- Admin: search sets (via Scryfall), add packs by product type, manage quantities
- Admin: link MTGStocks IDs to pull live market prices
- Admin: configure price and scarcity sensitivity for weighted draws
- Player: select which packs to include, see live odds, draw a random pack

## GitHub Secrets

| Secret | Description |
|--------|-------------|
| `SERVER_HOST` | Server IP |
| `SERVER_USER` | `ubuntu` |
| `SERVER_SSH_KEY` | Private SSH key |
| `DATABASE_URL` | `postgres://user:pass@db:5432/myapp` |
| `GOOGLE_CLIENT_ID` | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | Google OAuth client secret |
| `GOOGLE_REDIRECT_URL` | `https://yourdomain/api/auth/callback` |
| `ADMIN_EMAIL` | Email address granted admin role |

## Project Structure

```
├── api/
│   ├── main.go
│   ├── db/
│   ├── handlers/
│   ├── middleware/
│   └── Dockerfile
├── frontend/
│   ├── src/
│   └── Dockerfile
├── docs/
│   ├── development.md
│   └── production.md
├── terraform/
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   └── terraform.tfvars.example
├── .github/workflows/deploy.yml
├── Caddyfile
└── .env.example
```

## Documentation

- [Development setup](docs/development.md)
- [Production deployment](docs/production.md)

## License

MIT
