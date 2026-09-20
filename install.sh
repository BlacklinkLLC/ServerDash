#!/usr/bin/env bash
# ServerDash installer: installs Docker or Podman if missing, fetches
# ServerDash, and runs it as a systemd-managed `docker compose` (or
# `podman compose`) service.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/turkfork/ServerDash/main/install.sh | sudo bash
#   sudo ./install.sh [options]
#
# Options:
#   --runtime docker|podman   Container runtime to install/use (default: docker)
#   --dir PATH                Install directory (default: /opt/serverdash)
#   --public-host HOST        Value for SERVERDASH_PUBLIC_HOST (default: nova.blacklink.net)
#   --port PORT                Host port to publish the dashboard on (default: 9900)
#   --branch NAME              Git branch/tag to install (default: main)
#   --no-start                 Install and build, but don't enable/start the systemd service
#   -h, --help                 Show this help
#
# Safe to re-run: pulls the latest code, rebuilds the image, and restarts
# the service. Never touches an existing .env or the SQLite data volume.

set -euo pipefail

REPO_URL="https://github.com/turkfork/ServerDash.git"
RUNTIME="docker"
INSTALL_DIR="/opt/serverdash"
PUBLIC_HOST="nova.blacklink.net"
PORT="9900"
BRANCH="main"
START_SERVICE=1

log()  { printf '\033[1;34m==>\033[0m %s\n' "$1"; }
warn() { printf '\033[1;33m!!\033[0m %s\n' "$1" >&2; }
die()  { printf '\033[1;31merror:\033[0m %s\n' "$1" >&2; exit 1; }

usage() { sed -n '2,20p' "$0"; }

while [ $# -gt 0 ]; do
	case "$1" in
		--runtime) RUNTIME="$2"; shift 2 ;;
		--dir) INSTALL_DIR="$2"; shift 2 ;;
		--public-host) PUBLIC_HOST="$2"; shift 2 ;;
		--port) PORT="$2"; shift 2 ;;
		--branch) BRANCH="$2"; shift 2 ;;
		--no-start) START_SERVICE=0; shift ;;
		-h|--help) usage; exit 0 ;;
		*) die "unknown option: $1 (see --help)" ;;
	esac
done

case "$RUNTIME" in
	docker|podman) ;;
	*) die "--runtime must be 'docker' or 'podman', got '$RUNTIME'" ;;
esac

[ "$(id -u)" -eq 0 ] || die "must be run as root (installs packages and a systemd unit) — try: sudo $0 $*"
command -v systemctl >/dev/null 2>&1 || die "systemd (systemctl) is required"

# --- 1. Install the chosen container runtime, if it's missing ---

install_docker() {
	if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
		log "Docker + Compose plugin already installed ($(docker --version))"
		return
	fi
	log "Installing Docker via the official convenience script…"
	curl -fsSL https://get.docker.com | sh
	systemctl enable --now docker
}

install_podman() {
	if command -v podman >/dev/null 2>&1; then
		log "Podman already installed ($(podman --version))"
	else
		log "Installing Podman via the system package manager…"
		if command -v dnf >/dev/null 2>&1; then
			dnf install -y podman
		elif command -v apt-get >/dev/null 2>&1; then
			apt-get update && apt-get install -y podman
		elif command -v pacman >/dev/null 2>&1; then
			pacman -Sy --noconfirm podman
		elif command -v zypper >/dev/null 2>&1; then
			zypper install -y podman
		elif command -v apk >/dev/null 2>&1; then
			apk add --no-cache podman
		else
			die "no supported package manager found; install podman manually and re-run"
		fi
	fi

	if ! command -v podman-compose >/dev/null 2>&1 && ! podman compose version >/dev/null 2>&1; then
		log "Installing podman-compose…"
		if command -v pip3 >/dev/null 2>&1; then
			pip3 install --quiet podman-compose
		else
			die "podman-compose isn't installed and pip3 isn't available to install it; install podman-compose manually"
		fi
	fi

	systemctl enable --now podman.socket 2>/dev/null || true
}

if [ "$RUNTIME" = docker ]; then
	install_docker
	COMPOSE_CMD="docker compose"
else
	install_podman
	if command -v podman-compose >/dev/null 2>&1; then
		COMPOSE_CMD="podman-compose"
	else
		COMPOSE_CMD="podman compose"
	fi
fi

# --- 2. Fetch ServerDash ---

if [ -f "$(dirname "$0")/docker-compose.yml" ] && [ -f "$(dirname "$0")/Dockerfile" ]; then
	# Running from inside an already-checked-out copy of the repo — use it
	# in place instead of cloning a second copy.
	SOURCE_DIR="$(cd "$(dirname "$0")" && pwd)"
	if [ "$SOURCE_DIR" != "$INSTALL_DIR" ]; then
		log "Using existing checkout at $SOURCE_DIR (ignoring --dir)"
		INSTALL_DIR="$SOURCE_DIR"
	fi
elif [ -d "$INSTALL_DIR/.git" ]; then
	log "Updating existing install at $INSTALL_DIR…"
	git -C "$INSTALL_DIR" fetch --depth 1 origin "$BRANCH"
	git -C "$INSTALL_DIR" checkout "$BRANCH"
	git -C "$INSTALL_DIR" reset --hard "origin/$BRANCH"
else
	command -v git >/dev/null 2>&1 || die "git is required to fetch ServerDash"
	log "Cloning ServerDash into $INSTALL_DIR…"
	mkdir -p "$(dirname "$INSTALL_DIR")"
	git clone --branch "$BRANCH" --depth 1 "$REPO_URL" "$INSTALL_DIR"
fi

cd "$INSTALL_DIR"

# --- 3. Configure ---

if [ ! -f .env ]; then
	log "Writing .env (public host: $PUBLIC_HOST, port: $PORT)"
	cp .env.example .env
	sed -i "s#^SERVERDASH_PUBLIC_HOST=.*#SERVERDASH_PUBLIC_HOST=$PUBLIC_HOST#" .env
	sed -i "s#^SERVERDASH_PORT=.*#SERVERDASH_PORT=$PORT#" .env
else
	log ".env already exists, leaving it as-is"
fi

if [ "$RUNTIME" = podman ]; then
	# The Docker Engine API socket path in docker-compose.yml assumes
	# Docker; point it at Podman's socket instead. Root Podman's socket is
	# used here since the systemd service below runs as root.
	if ! grep -q '^SERVERDASH_DOCKER_HOST=' .env; then
		echo 'SERVERDASH_DOCKER_HOST=unix:///run/podman/podman.sock' >> .env
	fi
fi

# --- 4. Build and start ---

log "Building the ServerDash image (this can take a few minutes the first time)…"
$COMPOSE_CMD build

SERVICE_FILE=/etc/systemd/system/serverdash.service
log "Writing systemd unit at $SERVICE_FILE"
cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=ServerDash
After=network-online.target ${RUNTIME}.service
Wants=network-online.target
Requires=${RUNTIME}.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=$INSTALL_DIR
ExecStart=$COMPOSE_CMD up -d
ExecStop=$COMPOSE_CMD down
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

if [ "$START_SERVICE" -eq 1 ]; then
	log "Enabling and starting the serverdash service…"
	systemctl enable --now serverdash
	log "ServerDash is starting. Once it's up, open http://<this-host>:$PORT to create the admin account,"
	log "then put a reverse proxy in front of it for https://$PUBLIC_HOST."
else
	log "Skipping service start (--no-start). Run 'systemctl start serverdash' when ready."
fi

log "Install directory: $INSTALL_DIR"
log "Data (users, sessions, nicknames, scripts) persists in the 'serverdash-data' volume across upgrades."
log "To upgrade later: re-run this script, or 'cd $INSTALL_DIR && git pull && $COMPOSE_CMD up -d --build'."
