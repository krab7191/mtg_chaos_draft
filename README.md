# App Template

Go + Astro on Oracle Cloud Free Tier. Static-first, SSR optional.

## Quick Start

```bash
# Clone and rename
git clone <this-repo> my-app && cd my-app

# Initialize Go API
cd api && go mod init my-app && cd ..

# Initialize Astro frontend (static mode)
cd frontend && npm create astro@latest . && cd ..

# Provision infrastructure
cd terraform
cp terraform.tfvars.example terraform.tfvars
# edit terraform.tfvars with your OCI credentials
terraform init && terraform apply
```

## Infrastructure (Terraform)

Provisions on Oracle Cloud Always Free tier:
- 1 ARM VM (1 OCPU, 2GB RAM)
- VCN with security rules (22, 80, 443)
- Docker + fail2ban pre-installed

```bash
cd terraform
terraform init
terraform apply
```

Outputs the server IP for GitHub secrets.

## GitHub Secrets

| Secret | Description |
|--------|-------------|
| `SERVER_HOST` | Server IP (from terraform output) |
| `SERVER_USER` | `ubuntu` |
| `SERVER_SSH_KEY` | Private SSH key |
| `DATABASE_URL` | `postgres://user:pass@db:5432/myapp` |

## Server Setup (One-Time)

After `terraform apply`:

```bash
ssh ubuntu@<SERVER_IP>

# Run PostgreSQL
docker run -d --name db --network app \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=changeme \
  -e POSTGRES_DB=myapp \
  -v postgres_data:/var/lib/postgresql/data \
  --restart unless-stopped \
  postgres:16-alpine

# Run Caddy
docker run -d --name caddy --network app \
  -p 80:80 -p 443:443 \
  -e DOMAIN=yourapp.duckdns.org \
  -v /home/ubuntu/Caddyfile:/etc/caddy/Caddyfile \
  -v caddy_data:/data \
  -v /srv:/srv:ro \
  --restart unless-stopped \
  caddy:2-alpine

# Login to GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin
```

## DuckDNS

Add to crontab (`crontab -e`):

```
*/5 * * * * curl -s "https://www.duckdns.org/update?domains=YOURDOMAIN&token=YOURTOKEN&ip=" >/dev/null 2>&1
```

## Deployment Modes

### Static (default)
- Astro builds to HTML/CSS/JS
- Caddy serves static files
- Smallest footprint

### SSR
- Astro runs as Node server
- See comments in `Caddyfile` and `.github/workflows/deploy.yml`
- Uncomment SSR blocks, comment static blocks

## Project Structure

```
├── api/
│   └── Dockerfile
├── frontend/
│   └── Dockerfile        # SSR mode only
├── terraform/
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   └── terraform.tfvars.example
├── .github/workflows/
│   └── deploy.yml
├── Caddyfile
├── .env.example
├── .gitignore
└── .pre-commit-config.yaml
```

## License

MIT
