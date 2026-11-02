# ServerDash

A self-hosted dashboard for monitoring your servers and managing the Docker, Podman, and Kubernetes workloads running on them. It ships as a single container, and Blacklink runs it at **nova.blacklink.net**.

- **Monitor** CPU, memory, load, and disk usage in real time.
- **Control** containers and pods: start, stop, restart, stream logs, or open a shell.
- **Automate** with scheduled tasks, shell scripts, and block-based deployment workflows.
- **Make it yours**: name, logo, accent color, and visible sections are all configurable at runtime.

The UI follows Blacklink's NOVA design system (dark, amber accent). The styles live in `web/src/app.css`.

## Contents

- [Quick start](#quick-start)
- [Installing on a server](#installing-on-a-server)
- [Features](#features)
- [Roles and first run](#roles-and-first-run)
- [Configuration notes](#configuration-notes)
- [Data persistence](#data-persistence)
- [Workflows](#workflows)
- [Self-update](#self-update)
- [Web terminal](#web-terminal)
- [Architecture](#architecture)
- [Local development](#local-development)
- [Roadmap](#roadmap)

## Quick start

```sh
cp .env.example .env   # set SERVERDASH_PUBLIC_HOST, SERVERDASH_PORT, etc.
docker compose up -d --build
```

ServerDash will be available at `http://localhost:9900` (or whatever `SERVERDASH_PORT` is set to). Open it and create your administrator account to finish setup. For a real deployment, put it behind a reverse proxy (Blacklink uses **nova.blacklink.net**).

## Installing on a server

On a bare server, one command does everything:

```sh
curl -fsSL https://raw.githubusercontent.com/BlacklinkLLC/ServerDash/main/install.sh | sudo bash
```

The script installs Docker if it's missing (or Podman, with `--runtime podman`), clones ServerDash to `/opt/serverdash`, builds the image, and runs it as a systemd service. Check on it with `systemctl status serverdash`.

It's safe to re-run, which also upgrades an existing install. Run `install.sh --help` for the full list of options: `--dir`, `--public-host`, `--port`, `--branch`, and `--no-start`.

### Without Docker (native install)

ServerDash can also run directly on the host, with no container runtime:

```sh
curl -fsSL https://raw.githubusercontent.com/BlacklinkLLC/ServerDash/main/install.sh | sudo bash -s -- --runtime native
```

The interactive installer offers the same choice under "How should ServerDash run?". A native install:

- builds ServerDash from source with `scripts/build-native.sh`. It uses the system's Go and Node if they're new enough (Go 1.21+, Node 20.19+/22.12+), and otherwise downloads official toolchains into `/usr/local/lib/serverdash-toolchain`, used only for building
- runs as one process: the Go binary (`bin/serverdash`) serves both the API and the dashboard, so Node isn't needed at runtime
- runs as the `serverdash` systemd service, reading settings from the same `.env` file and storing data in `/var/lib/serverdash`
- turns on **Tools → Services** (start, stop, restart, enable, or disable systemd services and read their logs) and **Tools → System logs** (search and filter the system journal)

Containers still work if Docker or Podman is installed. Without either, the container sections are simply hidden. `serverdash logs` follows the journal and `serverdash update` rebuilds and restarts. Re-running the installer on an existing native install keeps it native, and switching an existing container install to `--runtime native` stops the old container first; the `serverdash-data` volume isn't migrated automatically.

To run it by hand without the installer:

```sh
scripts/build-native.sh
SERVERDASH_WEB_DIR=$PWD/web/dist SERVERDASH_PORT=9900 SERVERDASH_DB_PATH=/var/lib/serverdash/serverdash.db sudo -E bin/serverdash
```

## Features

**Monitoring**
- Server overview: health banner, resource gauges, and an hour of CPU, memory, and network history
- Per-core CPU, swap, storage volumes, container summary, and top processes

**Server tools**
- Process manager: search, sort, and stop or kill processes (admin)
- Listening ports and network interfaces
- systemd services and the system journal (native installs)

**Containers and pods**
- Docker and Podman: list containers, start/stop/restart, live log streaming, and an admin-only [web terminal](#web-terminal)
- Kubernetes: list pods across contexts and namespaces, restart them (the pod is deleted and its controller recreates it), and stream logs
- Nicknames: give any container or pod a friendlier display name

**Automation**
- Built-in rules: auto-restart unhealthy containers, scheduled prune, and scheduled tar+gzip backups with retention
- Scripts: admin-written shell scripts that run on demand or on a cron schedule, with run history and captured output
- [Workflows](#workflows): scheduled, block-based deployment automation (git pull, compose up, restart, run a command, wait)

**Administration**
- Session-cookie login, forced first-run admin setup, and admin/operator/viewer roles
- Appearance: rename the app, upload your own logo, pick an accent color, and choose which dashboard sections appear, all under Admin → Appearance and applied everywhere, including the sign-in screen
- [Self-update](#self-update): check for upstream changes and redeploy from the Admin panel

## Roles and first run

On first start, with an empty database, every page redirects to a setup screen that requires you to create an administrator account. Until that account exists, nothing else works: the dashboard, the container and Kubernetes views, and every API route except health, setup, and login are blocked. After setup, everyone must sign in.

| Role | Can do |
| --- | --- |
| `admin` | Everything, including managing users, roles, automation, scripts, workflows, and settings |
| `operator` | Act on containers and pods, but not change admin settings |
| `viewer` | Read-only access |

## Configuration notes

`docker-compose.yml` is set up for the common case. Here is what it does and what you may need to change.

**Container runtime.** It mounts the Docker socket. To manage Podman instead, swap in Podman's socket and set `SERVERDASH_DOCKER_HOST`. It also works unmodified with `podman-compose` and Podman's rootless socket.

**Host metrics.** The host's `/proc`, `/sys`, and root filesystem are bind-mounted read-only so the numbers reflect the real machine, not the container's own view. `pid: host` and the `/rootfs` mount are both required. The comments in the file explain why.

**Kubernetes.** The kubeconfig mount is commented out. To enable the Kubernetes tab, uncomment it and set `SERVERDASH_KUBECONFIG=/kubeconfig` in `.env`.
- On SELinux-enforcing hosts (Fedora, RHEL, CentOS), add `:Z` to the mount, or the container can't read the file even as root.
- For a local minikube or kind cluster, also set `network_mode: host`, because their kubeconfigs point at the host's loopback address. A remote cluster with embedded certs and a real endpoint doesn't need this.

**Workflows and self-update.** These need extra filesystem access, covered in [Workflows](#workflows) and [Self-update](#self-update).

## Data persistence

Everything ServerDash stores (users, sessions, nicknames, settings and branding, scripts, and workflows) lives in one SQLite file at `SERVERDASH_DB_PATH`, which defaults to `/data/serverdash.db` inside the container. An uploaded logo is stored next to it under `SERVERDASH_DATA_DIR`, which defaults to the same `/data`.

`docker-compose.yml` keeps `/data` in a named volume, `serverdash-data`. Like any Docker volume, it survives `docker compose up -d --build`, image rebuilds, and `docker compose down` (without `-v`). Your data is lost only if you delete the volume or run ServerDash another way, such as plain `docker run`, without mounting a persistent path at `/data`.

At startup, ServerDash logs exactly where it is storing data, with a loud warning if it is about to use the non-persistent default. If something seems to have reset, check `docker logs`.

## Workflows

Under Admin → Workflows you can build scheduled automations from a small set of fixed, typed blocks. This is deliberately not a general scripting language, so there is nothing to sandbox beyond what each block already does.

A workflow is a trigger (for example, "at 00:00 America/Chicago") plus an ordered list of blocks. The classic example is a nightly redeploy of a git-based service:

1. **Git Pull**: `dir: /opt/myapp`
2. **Rebuild & Run**: `dir: /opt/myapp`, `build: true` (runs `docker compose up -d --build`)

Blocks run top to bottom and stop at the first failure. Every run, scheduled or started with "Run now", is logged block by block in the workflow's history.

**Available blocks:** Git Pull, Rebuild & Run (`docker compose up -d [--build]`), Restart Container, Run Command (arbitrary shell, with an optional working directory), and Wait.

### Filesystem access

Because ServerDash itself runs in a container, the default `docker-compose.yml` can't reach your projects. A workflow's `dir` must exist inside ServerDash's container at the same path as on the host, so that the bundled `git` and `docker compose` can find it.

Uncomment the mount in `docker-compose.yml` and point it at the parent directory that holds your deployed projects (for example, `/opt:/opt`). Don't use `/tmp`. Under a systemd-managed Docker daemon with `PrivateTmp`, `/tmp` resolves to dockerd's own empty tmpfs rather than the host's, so the mount would silently bind the wrong directory.

### SELinux hosts

On SELinux-enforcing hosts (Fedora, RHEL, CentOS), two extra steps apply:

- Bind-mounted directories need `:Z` to be readable, the same as the kubeconfig mount.
- `:Z` is not enough for the `git_pull` and `compose_up` blocks. SELinux's container policy separately blocks a container's own nested `docker` and `git` CLI calls against the mounted socket. The dashboard's normal container controls (start, stop, restart) are unaffected because they call the Engine API directly instead of going through the CLI.

To use those two blocks, uncomment `security_opt: label=disable` in `docker-compose.yml`. This runs ServerDash unconfined, which is a real reduction in isolation, so only do it if you need them.

Git's "dubious ownership" check, which would otherwise trigger because the container runs as root over a host-user-owned directory, is handled for you. No `safe.directory` setup is needed.

## Self-update

Admin → Updates compares ServerDash's own git checkout against its remote (`git fetch`, then `HEAD` against `origin/<branch>`). If something new is available, one click pulls and redeploys it.

**Requirements:** set `SERVERDASH_INSTALL_DIR` to the checkout's path on the host, and mount it into the container at the same path. This is the same requirement as the workflow git blocks, and the same `:Z` note applies on SELinux hosts.

Unlike those workflow blocks, self-update does not need `security_opt: label=disable`. The `git pull` runs directly in ServerDash's container, but the `docker compose up -d --build` runs in a separate, short-lived helper container. That helper is started with `docker run -d` from the image ServerDash is already running, so nothing extra needs pulling. It is unconfined on its own and exits once the rebuild has been handed off.

Your data is unaffected either way. See [Data persistence](#data-persistence).

## Web terminal

Admins see a **Terminal** button on every running container in the Services list. It opens an interactive shell in the browser: xterm.js talking to the Engine API's exec/attach endpoints over WebSocket, with full PTY resize support. It tries `bash` first and falls back to `sh` for minimal images. It works with Podman containers unmodified, because it uses the same Engine API client as every other container control.

## Architecture

| Component | Stack | Role |
| --- | --- | --- |
| `server/` | Go | Backend and API |
| `web/` | Svelte + Vite, served by Express | Dashboard UI; proxies `/api` (including WebSocket log and terminal streams) to the backend, so the browser talks to a single origin |
| `install.sh` | Shell | Installs the runtime if needed, fetches ServerDash, and runs it as a systemd service |

Both processes run in one container under `supervisord` (see `Dockerfile` and `docker/supervisord.conf`).

**What the backend does:**
- **Host metrics** (CPU, memory, load, disk) via gopsutil.
- **Container control** through the Docker Engine API. Podman's socket is API-compatible, so the same client handles both. This also powers the web terminal.
- **Kubernetes** pod listing, restart, and logs by shelling out to `kubectl`. This reuses whatever kubeconfig, contexts, and auth plugins you already have instead of reimplementing cluster auth.
- **Storage** of auth (session cookies, bcrypt), users and roles, nicknames, automation rules, scripts, workflows, and settings in an embedded SQLite database (`internal/store`).
- **Scheduling** of automation rules, scripts, and workflows through a cron runner (`internal/automation`), including per-workflow time zones.
- **Workflows** via `internal/workflow`, the block-based deployment language described [above](#workflows).
- **Self-update** via `internal/selfupdate`, described [above](#self-update).
- An authenticated HTTP and WebSocket API that exposes all of it.

## Local development

```sh
# Terminal 1: Go API on :8080
cd server && go run ./cmd/serverdash

# Terminal 2: Svelte dev server on :5173, proxying /api to :8080
cd web && npm install && npm run dev
```

## Roadmap

Everything above is implemented. What follows is planned and **not built yet**.

### Core dashboard
- Network usage, temperature monitoring (where available), and storage health
- Online/offline status, last heartbeat, and a quick health indicator for every server
- Server groups and tags, and a multi-server overview

### Service management
- Container resource usage, uptime, exposed ports, mounted volumes, and image/version
- Environment variables with secrets hidden
- Pull/update images, recreate containers, and toggle automatic restart
- Service dependencies and health checks

### Logs
- Search, plus filtering by severity and time
- Download, clear, and rotate logs
- Automatic detection of common errors

### Deployments
- Deploy new services and update existing ones, from git or from images
- Deployment history, status, and logs, plus scheduled deployments
- Health verification after each deploy, with automatic rollback on failure, and manual rollback

### Server administration
- SSH terminal, file browser, and file upload/download
- Service configuration editor and environment variable management
- systemd service management, firewall status, network interfaces, and DNS configuration
- Reboot/shutdown controls and package update status

### Networking
- Port and service map, open ports, listening processes, and internal service addresses
- Reverse proxy, Cloudflare Tunnel, and DNS status
- Network traffic graphs and connection monitoring

### Storage and backups
- Volume management, storage cleanup, large-file detection, disk health, and snapshot status
- Create, restore, and verify backups, with history and retention policies
- Remote backup destinations and backup encryption

### Security
- Blacklink Auth integration and 2FA
- An admin/developer/operator role model
- API keys, session management, automatic session expiration, and login history
- Audit logs, IP allow/deny lists, and secrets management
- Confirmation prompts for destructive actions

### Monitoring and alerts
- Alerts for CPU, memory, disk space, service down, container crashes, high temperature, SSL certificate expiry, and failed deployments
- Custom alerts, email and webhook notifications, and alert history
- Maintenance mode

### Blacklink-specific
- A Blacklink service registry with product logos and service ownership
- Production, staging, and development environments, with version tracking and deployment channels
- Internal domains
- Status for Blacklink Auth, Cloudflare, R2, and databases, plus API health checks
- Dependency visualization

### Planned: Service Details page

A single page per service that pulls it all together. A rough sketch:

```
BellRinger
Production • Online

Version:    2.4.1
Server:     BL-SRV-01
Container:  blacklink-bellringer
Uptime:     14d 6h
CPU:        4.2%
Memory:     312 MB

[Restart]  [Update]  [Terminal]  [Logs]

Health
  ✓ Application
  ✓ Database
  ✓ API
  ✓ Cloudflare Tunnel

Recent deployments
  2.4.1 — Aug 26 — Successful
  2.4.0 — Aug 20 — Successful
  2.3.9 — Aug 12 — Successful
```
