# Production Deployment Guide

This deploys to **Hetzner Cloud** with:
- Caddy as reverse proxy (auto HTTPS via Let's Encrypt)
- A domain pointing to your server IP (e.g. DuckDNS subdomain or your own)
- Docker running api + postgres + Caddy

---

## Overview of what you need

| Service | What for | Cost |
|---|---|---|
| Hetzner Cloud | VM to run everything | ~€4/month (cx22) |
| Domain / DuckDNS | HTTPS requires a real domain | Free (DuckDNS) |
| Google Cloud | OAuth2 for login | Free |
| GitHub | Source + Actions CI/CD | Free |

---

## Step 1: Hetzner — get an API token

1. Create an account at [hetzner.com/cloud](https://www.hetzner.com/cloud)
2. Create a new project (e.g. `mtg-chaos-draft`)
3. Go to **Security → API Tokens → Generate API Token**
   - Give it Read & Write permissions
   - Copy the token — you won't see it again

---

## Step 2: Domain — point a name at your server IP

You can use [DuckDNS](https://www.duckdns.org) for a free subdomain:

1. Sign in at duckdns.org
2. Create a subdomain, e.g. `mtg-chaos.duckdns.org`
3. After Terraform runs (Step 3), copy the server IP from the output and paste it into DuckDNS

Or use any domain registrar and create an A record pointing to the server IP.

---

## Step 3: Provision the server with Terraform

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
```

Fill in `terraform/terraform.tfvars`:
```hcl
hcloud_token = "your-hetzner-api-token"

app_name     = "mtg-chaos-draft"
server_type  = "cax11"   # ARM64 Ampere, 2 vCPU, 4 GB RAM, 40 GB NVMe
location     = "nbg1"    # Nuremberg

# Name of an existing SSH key in your Hetzner project
# Cloud Console → Security → SSH Keys
ssh_key_name = "your-key-name"
```

If you haven't added your SSH key to the Hetzner project yet:
Cloud Console → Security → SSH Keys → Add SSH Key → paste your public key and give it a name.

### Apply

```bash
terraform init
terraform plan
terraform apply
```

Note the `server_ip` output — update your DuckDNS subdomain (or DNS A record) to point to it.

---

## Step 4: Google OAuth2 — production credentials

1. Go to [Google Cloud Console → APIs & Services → Credentials](https://console.cloud.google.com/apis/credentials)
2. Edit your OAuth client (or create a new one)
3. Under **Authorized redirect URIs**, add:
   ```
   https://mtg-chaos.duckdns.org/api/auth/callback
   ```
4. Copy the **Client ID** and **Client Secret**

---

## Step 5: GitHub — set secrets for CI/CD

Go to your repo → **Settings → Secrets and variables → Actions → New repository secret**.

| Secret name | Value |
|---|---|
| `GOOGLE_CLIENT_ID` | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | Google OAuth client secret |
| `ADMIN_EMAIL` | Your Google account email |
| `VIEWER_EMAILS` | Optional comma-separated viewer emails |
| `DOMAIN` | Your domain, e.g. `mtg-chaos.duckdns.org` |
| `DATABASE_URL` | `postgres://mtg:mtg@postgres:5432/mtg_chaos_draft` |
| `SERVER_HOST` | Server IP from Terraform output |
| `SERVER_USER` | `root` |
| `SERVER_SSH_KEY` | Your SSH private key (the one whose public key is in Hetzner) |

---

## Step 6: First-time server setup

SSH in and bootstrap once:

```bash
ssh root@<server-ip>

# Clone the repo
git clone https://github.com/krab7191/mtg_chaos_draft ~/app
cd ~/app

# Create .env
cat > .env << 'EOF'
GOOGLE_CLIENT_ID=<your prod client id>
GOOGLE_CLIENT_SECRET=<your prod client secret>
GOOGLE_REDIRECT_URL=https://mtg-chaos.duckdns.org/api/auth/callback
ADMIN_EMAIL=<your email>
VIEWER_EMAILS=<optional comma-separated emails>
DOMAIN=mtg-chaos.duckdns.org
EOF

# Start everything
docker compose up -d
```

> Docker is installed automatically by the Terraform cloud-init script — no manual install needed.

---

## Step 7: First deploy

Push to `main` — GitHub Actions will:
1. Build the frontend static files
2. Build and push the API Docker image to GHCR
3. SCP static files to `/srv` on the server
4. SSH in and pull + restart the API container

Or trigger manually: GitHub → Actions → Deploy → Run workflow.

---

## Caddy + HTTPS

Caddy handles TLS automatically via Let's Encrypt as long as:
- Ports 80 and 443 are open (Terraform firewall does this)
- Your domain's A record points to the server IP
- `DOMAIN` in `.env` matches exactly

No cert config needed — Caddy handles renewal too.

---

## Useful production commands

```bash
# View all logs
docker compose logs -f

# View just API logs
docker compose logs api -f

# Restart a single service
docker compose restart api

# Connect to the database
docker compose exec postgres psql -U mtg -d mtg_chaos_draft

# Wipe and reset the database
docker compose down -v && docker compose up -d
```
