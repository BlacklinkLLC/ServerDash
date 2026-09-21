# ServerDash

A self-hosted dashboard for monitoring servers and controlling their Docker/Podman
containers and Kubernetes pods. Deployed as a single container, served at
**nova.blacklink.net**. Visual design matches Blacklink's NOVA design system
(dark, amber accent, Syne/DM Mono type) — see `web/src/app.css`. App name, logo,
accent color, and which dashboard sections show are all runtime-configurable from
Admin → Appearance, for white-labeling and per-deployment customization.

## Architecture

- **`server/`** — Go backend.
  - Host metrics (CPU, memory, load, disk) via gopsutil.
  - Docker/Podman container control over the Docker Engine API (Podman's socket is
    API-compatible, so the same client works for either), plus an interactive
    web terminal (`docker exec`-equivalent) over WebSocket.
  - Kubernetes pod listing/restart/logs by shelling out to `kubectl` — reuses
    whatever kubeconfig/contexts/auth plugins are already set up rather than
    reimplementing cluster auth.
  - Auth (session cookies, bcrypt), users/roles, nicknames, automation rules,
    scripts, workflows, and settings/branding, all persisted in an embedded
    SQLite database (`internal/store`) — see [Data persistence](#data-persistence)
    for what has to be true for this to survive an update.
  - Built-in automation + user script + workflow scheduling via a cron runner
    (`internal/automation`), including per-workflow timezones.
  - `internal/workflow`: the block-based deployment automation language — see
    [Workflows](#workflows-block-based-deployment-automation) below.
  - `internal/selfupdate`: checks ServerDash's own git checkout for upstream
    changes and can pull + rebuild + restart itself — see
    [Self-update](#self-update) below.
  - Exposes all of it through an authenticated HTTP + WebSocket API.
- **`web/`** — Svelte + Vite dashboard, served by a small Express server that also
  proxies `/api` (including WebSocket log/terminal streams) through to the Go
  backend, so the browser only ever talks to one origin.
- Both processes run in one container, managed by `supervisord` (see `Dockerfile`,
  `docker/supervisord.conf`).
- **`install.sh`** — installs Docker or Podman if missing, fetches ServerDash, and
  runs it as a systemd-managed service. See [Installing on a server](#installing-on-a-server).

## First run

The first time ServerDash starts (empty database), every page redirects to a setup
screen that **requires creating an administrator account** before anything else —
the dashboard, containers, Kubernetes view, and every API route (other than health,
setup, and login) are unreachable until that account exists. From then on, sign-in
is required for everyone; `admin` users manage other accounts, roles, automation,
and scripts from the Admin panel, `operator` can act on containers/pods but not
touch admin settings, and `viewer` is read-only.

## Data persistence

Users, sessions, nicknames, settings/branding, scripts, and workflows all live in
one SQLite file at `SERVERDASH_DB_PATH` (default `/data/serverdash.db` in the
container); an uploaded logo lives alongside it under `SERVERDASH_DATA_DIR`
(defaults to that same `/data`). `docker-compose.yml` keeps `/data` in a named
volume (`serverdash-data`), which — like any Docker volume — survives
`docker compose up -d --build`, `docker compose down` (without `-v`), and image
rebuilds; it's only lost if you explicitly remove that volume or run ServerDash
some other way (e.g. plain `docker run`) without mounting a persistent path there.
On startup, ServerDash logs exactly where it's storing data, including a loud
warning if it's about to use the non-persistent default path — check `docker logs`
if something seems to have reset.

## Running it

```sh
cp .env.example .env   # adjust SERVERDASH_PUBLIC_HOST, port, etc.
docker compose up -d --build
```

This builds the image and starts ServerDash on `http://localhost:9900` (or
`SERVERDASH_PORT` from `.env`); reverse-proxy it at **nova.blacklink.net** for the
real deployment. Open it and create the admin account to finish setup.

`docker-compose.yml`:
- Mounts the Docker socket (swap for Podman's socket + `SERVERDASH_DOCKER_HOST` to
  manage Podman instead — also works unmodified via `podman-compose` and Podman's
  rootless socket).
- Bind-mounts the host's `/proc`, `/sys`, and root filesystem (read-only) so host
  metrics reflect the real machine rather than the container's own view —
  `pid: host` and the `/rootfs` bind mount are both load-bearing, not incidental;
  see the comments in the file for why.
- Keeps users/sessions/nicknames/scripts in a named volume (`serverdash-data`), so
  they survive `docker compose down` / image rebuilds.
- Has a commented-out kubeconfig mount for Kubernetes support — uncomment it and
  set `SERVERDASH_KUBECONFIG=/kubeconfig` in `.env` to enable the Kubernetes tab.
  **On SELinux-enforcing hosts (Fedora/RHEL/CentOS)**, add `:Z` to that mount or
  the container won't be able to read the file even as root. Pointing it at a
  local minikube/kind cluster additionally needs `network_mode: host`, since their
  kubeconfigs reference the host's loopback address — a real remote cluster's
  kubeconfig (embedded certs, a real endpoint) doesn't have this problem.
- Has a commented-out mount + `security_opt` for the workflow feature's
  `git_pull`/`compose_up` blocks — see
  [Workflows](#workflows-block-based-deployment-automation).

### Local development

```sh
# terminal 1 — Go API on :8080
cd server && go run ./cmd/serverdash

# terminal 2 — Svelte dev server on :5173, proxying /api to :8080
cd web && npm install && npm run dev
```

## Installing on a server

```sh
curl -fsSL https://raw.githubusercontent.com/turkfork/ServerDash/main/install.sh | sudo bash
```

Installs Docker (or Podman with `--runtime podman`), clones ServerDash to
`/opt/serverdash`, builds the image, and runs it as a systemd service (`systemctl
status serverdash`). Safe to re-run to upgrade — see `install.sh --help` for
options (`--dir`, `--public-host`, `--port`, `--branch`, `--no-start`).

## Workflows: block-based deployment automation

Admin → Workflows builds scheduled automations out of a small set of fixed,
typed blocks — not a general scripting language, so there's nothing to sandbox
beyond what each block already does. A trigger ("at 00:00 America/Chicago")
plus an ordered list of blocks, e.g. the canonical case — nightly redeploy of a
git-based service:

1. **Git Pull** — `dir: /opt/myapp`
2. **Rebuild & Run (docker compose up)** — `dir: /opt/myapp`, `build: true`

Blocks run top to bottom and stop at the first failure; every run (scheduled or
"Run now") is logged block-by-block in the workflow's history. Available blocks:
Git Pull, Rebuild & Run (`docker compose up -d [--build]`), Restart Container,
Run Command (arbitrary shell, optional working directory), Wait.

**This needs real filesystem access that the default `docker-compose.yml`
doesn't grant**, because ServerDash itself runs containerized: a workflow's
`dir` has to exist inside *ServerDash's own container*, at the same path it
has on the host, for `git`/`docker compose` (both shelled out to, bundled in
the image) to find it. Uncomment the commented-out mount in
`docker-compose.yml` and point it at whatever parent directory holds your
deployed projects (e.g. `/opt:/opt`) — never `/tmp`, which under a
systemd-managed Docker daemon with `PrivateTmp` resolves to dockerd's own
private empty tmpfs, not the host `/tmp` your shell sees, so a mount from
there silently binds the wrong directory.

Two more things that come up specifically on SELinux-enforcing hosts
(Fedora/RHEL/CentOS — not an issue elsewhere):
- Bind-mounted directories need `:Z` to be readable at all (same as the
  kubeconfig mount above).
- `:Z` alone isn't enough for these two block types specifically: SELinux's
  container policy separately blocks a container's own nested `docker`/`git`
  CLI calls against the mounted socket, even though the dashboard's normal
  container controls (start/stop/restart, which go straight to the Engine
  API, not through the CLI) are unaffected. If you want `git_pull`/
  `compose_up`, uncomment `security_opt: label=disable` in
  `docker-compose.yml` — it runs ServerDash unconfined, which is a real
  reduction in isolation, so only do it if you're using those two blocks.
  (`git`'s separate "dubious ownership" check, since the container runs as
  root over a host-user-owned directory, is handled for you — no
  `safe.directory` config needed.)

## Self-update

Admin → Updates checks ServerDash's own git checkout against its remote (`git
fetch` + compare `HEAD` to `origin/<branch>`) and, if there's something new, pulls
and redeploys with one click. Needs `SERVERDASH_INSTALL_DIR` set to that
checkout's path on the host, mounted into the container at the same path — same
requirement as a workflow's `git_pull`/`compose_up` blocks, and the same `:Z` note
applies on SELinux-enforcing hosts. Unlike those workflow blocks, self-update
*doesn't* need `security_opt: label=disable` on ServerDash's own container: since
ServerDash is rebuilding and restarting itself, the actual `docker compose up -d
--build` runs in a separate, short-lived helper container (launched with `docker
run -d`, from the same image ServerDash itself is running, so nothing extra needs
pulling) that's unconfined on its own and exits once the rebuild's handed off —
the `git pull` itself, which doesn't need that, still runs directly in ServerDash's
own container. Your data isn't affected either way — see
[Data persistence](#data-persistence).

## Web terminal

Admin-only "Terminal" button on any running container (Services list) opens an
interactive shell — `docker exec`-equivalent, xterm.js in the browser talking to
the Engine API's exec/attach endpoints over WebSocket, full PTY resize support.
Tries `bash`, falls back to `sh` for minimal images. Works against Podman
containers unmodified, same as every other container control — it's the same
Engine API client throughout.

## Implemented so far

- Server overview: hostname, platform, uptime, load average
- CPU, memory, and per-disk usage (with usage bars)
- Docker/Podman: container list, start/stop/restart, live log streaming, an
  interactive web terminal (admin-only) — see above
- Kubernetes: pod list (across contexts/namespaces), restart (delete → controller
  recreates), live log streaming
- Nicknames: rename any container or pod to a friendlier display name
- Auth: session-cookie login, forced first-run admin setup, admin/operator/viewer
  roles, user management
- Automation: auto-restart unhealthy containers, scheduled prune, scheduled
  tar+gzip backups with retention
- Scripts: admin-authored shell scripts, run on-demand or on a cron schedule, with
  run history and captured output
- Workflows: block-based scheduled deployment automation (git pull, rebuild/run
  via compose, restart a container, run a command, wait) — see above
- Appearance: rename the app, swap in your own logo, pick an accent color, and
  choose which dashboard sections show, all from Admin → Appearance, shown
  everywhere including the sign-in screen
- Self-update: check ServerDash's own git checkout for updates and redeploy from
  the Admin panel — see above
- `install.sh` for a one-line install on a bare server

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