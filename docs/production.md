# Production Deployment Guide

This deploys to **Oracle Cloud Always Free** (ARM VM) with:
- Caddy as reverse proxy (auto HTTPS via Let's Encrypt)
- DuckDNS for a free domain
- Docker Compose running api + frontend + postgres

---

## Overview of what you need

| Service | What for | Cost |
|---|---|---|
| Oracle Cloud | ARM VM to run everything | Free (Always Free tier) |
| DuckDNS | Free subdomain (e.g. `yourname.duckdns.org`) | Free |
| Google Cloud | OAuth2 for login | Free |
| GitHub | Source + Actions CI/CD | Free |

---

## Step 1: DuckDNS — get a free domain

1. Go to [duckdns.org](https://www.duckdns.org) and sign in (GitHub/Google)
2. Create a subdomain, e.g. `mtg-chaos.duckdns.org`
3. Copy your **DuckDNS token** (shown at the top of the page after login)
4. After your Oracle VM is provisioned (Step 2), come back and point the DuckDNS subdomain to your VM's public IP

---

## Step 2: Oracle Cloud — provision the VM

### One-time account setup
1. Create an account at [cloud.oracle.com](https://cloud.oracle.com)
   - Use a real credit card — it's required but you won't be charged for Always Free resources
   - Choose a home region close to you (can't be changed later)

### Provision with Terraform
The `terraform/` directory contains all the infra config.

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
```

Fill in `terraform/terraform.tfvars`:
```hcl
# Your OCI tenancy OCID — Oracle Cloud Console → Profile → Tenancy
tenancy_ocid = "ocid1.tenancy.oc1..xxxx"

# Your user OCID — Oracle Cloud Console → Profile → User Settings
user_ocid = "ocid1.user.oc1..xxxx"

# Fingerprint of your API key (generated below)
fingerprint = "xx:xx:xx:xx:..."

# Path to your OCI API private key (generated below)
private_key_path = "~/.oci/oci_api_key.pem"

# Availability domain — find in Console → Identity → Availability Domains
availability_domain = "xxxx:AP-SYDNEY-1-AD-1"

# Your DuckDNS subdomain (just the subdomain, not the full URL)
duckdns_domain = "mtg-chaos"

# Your DuckDNS token from duckdns.org
duckdns_token = "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
```

### Generate an OCI API key
```bash
mkdir -p ~/.oci
openssl genrsa -out ~/.oci/oci_api_key.pem 2048
chmod 600 ~/.oci/oci_api_key.pem
openssl rsa -pubout -in ~/.oci/oci_api_key.pem -out ~/.oci/oci_api_key_public.pem
cat ~/.oci/oci_api_key_public.pem
```

Upload the public key:
Oracle Cloud Console → Profile → User Settings → API Keys → Add API Key → paste public key content.
Copy the **fingerprint** shown — paste it into `terraform.tfvars`.

### Apply Terraform
```bash
cd terraform
terraform init
terraform plan
terraform apply
```

This creates the VM, opens firewall ports 80/443, and outputs the public IP.
Update your DuckDNS subdomain to point to that IP.

---

## Step 3: Google OAuth2 — production credentials

1. Go to [Google Cloud Console → APIs & Services → Credentials](https://console.cloud.google.com/apis/credentials)
2. Either create a new OAuth client or edit your existing one
3. Under **Authorized redirect URIs**, add your production URL:
   ```
   https://mtg-chaos.duckdns.org/api/auth/callback
   ```
   (Keep `http://localhost:4321/api/auth/callback` for dev too)
4. Copy the **Client ID** and **Client Secret**

---

## Step 4: GitHub — set secrets for CI/CD

Go to your repo → **Settings → Secrets and variables → Actions → New repository secret**.

Add each of these:

| Secret name | Where to get it |
|---|---|
| `GOOGLE_CLIENT_ID` | Google Cloud Console → Credentials → your OAuth client |
| `GOOGLE_CLIENT_SECRET` | Same place |
| `ADMIN_EMAIL` | Your Google account email address |
| `DOMAIN` | Your full DuckDNS domain, e.g. `mtg-chaos.duckdns.org` |
| `SSH_PRIVATE_KEY` | Private key to SSH into your Oracle VM (see below) |
| `SSH_HOST` | Your VM's public IP from Terraform output |
| `SSH_USER` | `ubuntu` (or `opc` depending on image) |

### Generate SSH key for CI
```bash
ssh-keygen -t ed25519 -C "github-actions" -f ~/.ssh/mtg_chaos_deploy -N ""
cat ~/.ssh/mtg_chaos_deploy      # → paste as SSH_PRIVATE_KEY secret
cat ~/.ssh/mtg_chaos_deploy.pub  # → add to VM's ~/.ssh/authorized_keys
```

To add the public key to the VM:
```bash
ssh ubuntu@<vm-ip> "echo '$(cat ~/.ssh/mtg_chaos_deploy.pub)' >> ~/.ssh/authorized_keys"
```

---

## Step 5: First deploy

Push to `main` — GitHub Actions will:
1. Build the Docker images
2. SSH into the VM
3. Pull the latest images and run `docker compose up -d`

Or trigger it manually: GitHub → Actions → Deploy → Run workflow.

### First-time VM setup

SSH into the VM and run once:
```bash
ssh ubuntu@<vm-ip>

# Install Docker
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker ubuntu
# Log out and back in

# Clone the repo
git clone https://github.com/krab7191/mtg_chaos_draft ~/app
cd ~/app

# Create .env on the server
cat > .env << 'EOF'
GOOGLE_CLIENT_ID=<your prod client id>
GOOGLE_CLIENT_SECRET=<your prod client secret>
GOOGLE_REDIRECT_URL=https://mtg-chaos.duckdns.org/api/auth/callback
ADMIN_EMAIL=<your email>
DOMAIN=mtg-chaos.duckdns.org
EOF

# Start everything
docker compose up -d
```

---

## Caddy + HTTPS

Caddy handles TLS automatically via Let's Encrypt as long as:
- Port 80 and 443 are open on the Oracle firewall (Terraform does this)
- Your DuckDNS domain points to the VM's IP
- `DOMAIN` env var is set correctly in `.env`

No certificate config needed — Caddy handles renewal too.

---

## Useful production commands

```bash
# View all logs
docker compose logs -f

# View just API logs
docker compose logs api -f

# Restart a single service
docker compose restart api

# Update after a git pull
docker compose pull && docker compose up -d

# Connect to the database
docker compose exec postgres psql -U mtg -d mtg_chaos_draft
```
