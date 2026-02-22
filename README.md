# MTG Chaos Draft

[![Deploy](https://github.com/krab7191/mtg_chaos_draft/actions/workflows/deploy.yml/badge.svg)](https://github.com/krab7191/mtg_chaos_draft/actions/workflows/deploy.yml)

A personal app for running Magic: The Gathering chaos drafts. Instead of everyone drafting from the same set, players draw randomly from a mixed collection of sealed packs — with configurable odds.

## How it works

An admin builds a collection of sealed products (Draft Boosters, Set Boosters, Collector Boosters, etc.) from any sets they own. Before a draft, players choose which packs to include in the pool and hit a button to draw one at random.

Odds aren't purely uniform — cheaper packs get picked more often and packs with fewer copies are less likely to be drawn, so rare or expensive product doesn't dominate the pool.

## Features

- Google SSO — sign in with your Google account
- Admin: search sets and add packs by product type (Draft, Set, Collector, Play, Jumpstart)
- Admin: track quantities and link MTGStocks IDs to pull live market prices
- Viewer role: view and use all admin pages read-only (set via `VIEWER_EMAILS` env var)
- Weighted draws based on market price (cheaper packs picked more often) and scarcity (lower-qty packs picked less often)
- Player: choose which packs to include, see live odds per pack, draw a random result
- Draft history tracked in-browser (last 12 picks)

## Project Structure

```
├── api/
│   ├── main.go
│   ├── db/
│   ├── handlers/
│   ├── middleware/
│   └── Dockerfile
├── docs/
│   ├── development.md
│   └── production.md
├── frontend/
│   ├── src/
│   └── Dockerfile
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
