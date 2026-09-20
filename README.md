# ServerDash

A self-hosted dashboard for monitoring servers and controlling their Docker/Podman
containers. Deployed as a single container, served at **nova.blacklink.net**.

## Architecture

- **`server/`** — Go backend. Collects host metrics (CPU, memory, load, disk) and
  controls containers over the Docker Engine API (Podman's socket is API-compatible,
  so the same client works for either). Exposes an HTTP + WebSocket API.
- **`web/`** — Svelte + Vite dashboard, served by a small Express server that also
  proxies `/api` (including the WebSocket log stream) through to the Go backend, so
  the browser only ever talks to one origin.
- Both processes run in one container, managed by `supervisord` (see `Dockerfile`,
  `docker/supervisord.conf`).

## Running it

```sh
cp .env.example .env   # adjust SERVERDASH_PUBLIC_HOST, port, etc.
docker compose up -d --build
```

This builds the image and starts ServerDash on `http://localhost:3000` (or
`SERVERDASH_PORT` from `.env`), reverse-proxy it at **nova.blacklink.net** for the
real deployment. `docker-compose.yml` mounts the Docker socket (swap for Podman's
socket + `SERVERDASH_DOCKER_HOST` to manage Podman instead), and bind-mounts the
host's `/proc`, `/sys`, and root filesystem (read-only) so host metrics reflect the
real machine rather than the container's own view — see the comments in
`docker-compose.yml` for why (`pid: host` and the `/rootfs` bind mount are both
load-bearing, not incidental).

Works the same with `podman-compose` and Podman's rootless socket.

### Local development

```sh
# terminal 1 — Go API on :8080
cd server && go run ./cmd/serverdash

# terminal 2 — Svelte dev server on :5173, proxying /api to :8080
cd web && npm install && npm run dev
```

## Implemented so far

- Server overview: hostname, platform, uptime, load average
- CPU, memory, and per-disk usage (with usage bars)
- Container list: name, status, image, ports
- Start / stop / restart containers
- Live container log streaming over WebSocket

Everything below is the product roadmap — features to build toward, not yet
implemented.

## Roadmap

Core dashboard

* Server overview
* CPU, RAM, disk, and network usage
* Server uptime
* Load average
* Temperature monitoring where available
* Storage health
* Online/offline server status
* Last heartbeat
* Quick health indicator for every server
* Server groups/tags
* Multi-server overview

Service management

* View all running services
* Start/stop/restart services
* Restart individual containers
* View container status
* View container resource usage
* Container uptime
* View exposed ports
* View environment variables with secrets hidden
* View mounted volumes
* View container image/version
* Pull/update images
* Recreate
* Enable/disable automatic restart
* Service dependencies
* Service health checks

Logs

* Live container logs
* Search logs
* Filter by severity
* Filter by time
* Download logs
* Clear/rotate logs
* Automatically detect common errors
* Log streaming through WebSockets

Deployments

* Deploy a new service
* Update an existing service
* Roll back an update
* Deployment history
* Deployment status
* Git-based deployments
* Image-based deployments
* Deployment logs
* Scheduled deployments
* Automatic health verification after deployment
* Automatic rollback if a deployment fails

Server administration

* SSH terminal through the dashboard
* File browser
* File upload/download
* Service configuration editor
* Environment variable management
* Systemd service management
* Firewall status
* Network interfaces
* DNS configuration
* Reboot/shutdown controls
* Package/update status

Networking

* Port/service map
* Open ports
* Listening processes
* Internal service addresses
* Reverse proxy status
* Cloudflare Tunnel status
* DNS status
* Network traffic graphs
* Connection monitoring

Storage

* Disk usage
* Volume management
* Container volumes
* Backup volumes
* Storage cleanup
* Large-file detection
* Disk health
* Snapshot/backup status

Backups

* Create backup
* Restore backup
* Scheduled backups
* Backup history
* Backup verification
* Remote backup destinations
* Backup encryption
* Retention policies

Security

* Blacklink Auth integration
* Role-based access control
* Admin/developer/operator roles
* 2FA
* API keys
* Session management
* Audit logs
* Login history
* IP allow/deny lists
* Secrets management
* Automatic session expiration
* Destructive-action confirmation

Monitoring & alerts

* CPU alerts
* Memory alerts
* Disk-space alerts
* Service-down alerts
* Container crash alerts
* High-temperature alerts
* SSL certificate expiration alerts
* Failed deployment alerts
* Custom alerts
* Email/webhook notifications
* Alert history
* Maintenance mode

Blacklink-specific stuff

* Blacklink service registry
* Blacklink product/service logos
* Service ownership
* Production/staging/development environments
* Service version tracking
* Internal domains
* Deployment channels
* Blacklink Auth status
* Cloudflare status
* R2 status
* Database status
* API health checks
* Dependency visualization

A really nice feature would be a “Service Details” page. For example:

BellRinger
Production • Online

Version: 2.4.1
Server: BL-SRV-01
Container: blacklink-bellringer
Uptime: 14d 6h
CPU: 4.2%
Memory: 312 MB

[Restart] [Update] [Terminal] [Logs]

Health
✓ Application
✓ Database
✓ API
✓ Cloudflare Tunnel

Recent deployments
2.4.1 — Aug 26 — Successful
2.4.0 — Aug 20 — Successful
2.3.9 — Aug 12 — Successful