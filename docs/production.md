# Production Deployment Guide

This deploys to **Hetzner Cloud** with:
- Caddy as reverse proxy with a Cloudflare Origin CA certificate
- Cloudflare proxy (orange cloud) for DDoS protection and CDN
- Docker running api + postgres + Caddy

---

## Overview of what you need

| Service | What for | Cost |
|---|---|---|
| Hetzner Cloud | VM to run everything | ~€4/month (cax11) |
| Cloudflare | DNS + proxy + origin cert | Free |
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

## Step 2: Cloudflare — DNS and origin certificate

1. Add your domain to [Cloudflare](https://dash.cloudflare.com) (e.g. `karstenrabe.dev`)
2. Create a DNS **A record**: `chaosdraft` → your server IP (proxied/orange cloud)
3. Go to **SSL/TLS → Origin Server → Create Certificate**
   - Hostnames: `*.karstenrabe.dev, karstenrabe.dev` (or just `chaosdraft.karstenrabe.dev`)
   - Validity: 15 years
   - Save both the **Origin Certificate** (`cf_origin_cert.pem`) and **Private Key** (`cf_origin_key.pem`)
4. Set SSL/TLS encryption mode to **Full (Strict)**
5. Place both files in the repo root alongside `docker-compose.yml`

> **Important:** The private key is only shown once by Cloudflare. If lost, you must generate a new certificate.

After Terraform runs (Step 3), update the A record to point to the server IP.

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

Note the `server_ip` output — update your Cloudflare A record to point to it.

---

## Step 4: Google OAuth2 — production credentials

1. Go to [Google Cloud Console → APIs & Services → Credentials](https://console.cloud.google.com/apis/credentials)
2. Edit your OAuth client (or create a new one)
3. Under **Authorized redirect URIs**, add:
   ```
   https://chaosdraft.karstenrabe.dev/api/auth/callback
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
| `DOMAIN` | `chaosdraft.karstenrabe.dev` |
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
GOOGLE_REDIRECT_URL=https://chaosdraft.karstenrabe.dev/api/auth/callback
ADMIN_EMAIL=<your email>
VIEWER_EMAILS=<optional comma-separated emails>
DOMAIN=chaosdraft.karstenrabe.dev
EOF

# Copy origin cert files to repo root
# (scp them from your local machine, or paste directly)
# cf_origin_cert.pem and cf_origin_key.pem must be in ~/app/

# Start everything
docker compose up -d
```

> Docker is installed automatically by the Terraform cloud-init script — no manual install needed.

---

## Step 7: First deploy

Push to `main` — GitHub Actions will:
1. Run checks and tests
2. Build and push Docker images to GHCR
3. SSH in and pull + restart containers

Or trigger manually: GitHub → Actions → Deploy → Run workflow.

---

## Caddy + TLS

Caddy uses a **Cloudflare Origin CA certificate** for TLS termination. The cert and key are mounted into the Caddy container from `cf_origin_cert.pem` and `cf_origin_key.pem` in the repo root.

Since Cloudflare proxies all traffic (orange cloud / Full Strict mode), Caddy does not use Let's Encrypt — the origin cert is what Cloudflare verifies when connecting to the server.

The origin certificate is valid for up to 15 years. When it expires, generate a new one from the Cloudflare dashboard and replace the files.

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