# OGame Next-Gen: Deployment & Environment Architecture Guide

This document provides step-by-step instructions for spin-up operations in the Local Development Sandbox and migrating infrastructure blueprints into Bare-Metal Linux Production environments.

---

## 1. Local Development Setup (Docker Environment)

The local architecture utilizes lightweight Linux Alpine container runtimes wrapped in a unified isolated Docker network grid. Language runtimes (`Go`, `Rust`, `Node.js`) are decoupled from the host OS.

### Host Prerequisites
*   **Operating System:** Windows 10/11 with WSL2 (Ubuntu Backend recommended).
*   **Engine:** Docker Desktop v4.25+ (with Compose v2 enabled).

### Spin-Up Instructions
1. Open your host terminal (PowerShell/CMD) in the root directory `ogame-nextgen/`.
2. Clear any legacy or dangling volumes to ensure a zero-state database initialization:
   ```bash
   docker compose down --volumes --remove-orphans
   ```
3. Execute the global runtime engine build and aggregation launch:
   ```bash
   docker compose up --build
   ```
4. Verification Matrix:
   *   **User Interface Portal (React):** `http://localhost:5173`
   *   **Administration Dashboard (Vue 3):** `http://localhost:5174`
   *   **Public WebSocket Proxy Bridge (Go):** `ws://localhost:8080/ws`
   *   **Game Engine Kernel (Rust gRPC):** Internal routing via `core:50051`

---

## 2. Production Environment Setup (Live Linux Servers)

For real-world scaling, high-performance production operations avoid running heavy development servers (like Vite) or dynamic interpreters inside containers. Everything is optimized into bare-metal binary assets and multi-stage configurations.

### Infrastructure Blueprints
We divide the cluster topology into two major isolated tiers:
1.  **Application Tier (Public Web Layer):** Hosts Frontends and Go Gateway.
2.  **Stateful Database Tier (Private Layer):** Hosts Rust Core, PostgreSQL, and Redis.

[ Internet ] ---> [ Nginx Reverse Proxy (SSL) ]|+---------------+---------------+| (Port 80/443)                 | (Port 8080)v                               v[ Static Web Assets ]            [ Go Gateway ](Compiled React/Vue)                    | (Internal Secure VPC Network)v[ Rust Game Core ] ---> [ PostgreSQL & Redis ]


---

### Step-by-Step Production Deployment Guide

#### Phase A: Target Server Provisioning
Prepare a clean **Ubuntu Server 24.04 LTS** node. Update system architectures and install production runtimes:
```bash
sudo apt update && sudo apt upgrade -y
sudo apt install -y docker.io docker-compose-v2 ufw nginx
```

#### Phase B: Hardening the Network (UFW Firewall Configuration)
Strict security controls isolate backend communication. Only public-facing edge pipelines are exposed to the internet:
```bash
# Block all internal cluster chatter from outside eyes
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow standard encrypted web traffic
sudo ufw allow 22/tcp   # SSH Access
sudo ufw allow 80/tcp   # HTTP Validation
sudo ufw allow 443/tcp  # HTTPS Encrypted Traffic
sudo ufw allow 8080/tcp # Public Game WebSocket Port

sudo ufw enable
```

#### Phase C: Compiling Production Frontends
On the deployment agent or build node, run compilation scripts to turn source code into static minified assets (`.js`, `.css`, `.html`):
```bash
# Compile User Frontend
cd frontend && npm install && npm run build
# Compile Administration Dashboard
cd ../admin-portal && npm install && npm run build
```
*Resulting compilation files are generated inside `dist/` directories, completely ready to be served by high-performance Nginx web routers.*

#### Phase D: Nginx Edge Configuration
Configure the web server to act as a reverse proxy, serving frontends as pure static files and routing API pipelines. Create `/etc/nginx/sites-available/ogame`:
```nginx
server {
    listen 80;
    server_name yourgame.com;

    # Route 1: Serve main player client interface
    location / {
        root /var/www/ogame/frontend;
        index index.html;
        try_files uri uri/ /index.html;
    }

    # Route 2: Serve private Administration Dashboard
    location /admin {
        root /var/www/ogame/admin;
        index index.html;
        try_files uri uri/ /admin/index.html;
    }
}
```
Activate the system map routing links:
```bash
sudo ln -s /etc/nginx/sites-available/ogame /etc/nginx/sites-enabled/
sudo systemctl restart nginx
```

#### Phase E: Launching Backend Matrix via Production Compose
Create a dedicated production manifest file **`docker-compose.prod.yml`** on the deployment node. It uses stripped down configurations optimized for speed and reliability, discarding volumes meant for Windows syncing.

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:16-alpine
    restart: always
    environment:
      - POSTGRES_USER=ogame_prod_admin
      - POSTGRES_PASSWORD=set_extremely_secure_password_here
      - POSTGRES_DB=ogame_production
    volumes:
      - pg_prod_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    restart: always
    command: redis-server --appendonly yes
    volumes:
      - redis_prod_data:/data

  core:
    image: ogame-nextgen-core:latest
    restart: always
    depends_on:
      - postgres
      - redis

  gateway:
    image: ogame-nextgen-gateway:latest
    restart: always
    ports:
      - "8080:8080"
    depends_on:
      - core

volumes:
  pg_prod_data:
  redis_prod_data:
```

Execute compilation stack and run application processes persistently:
```bash
docker compose -f docker-compose.prod.yml up -d
```