#!/usr/bin/env bash
# ServerDash installer
#
# Installs Docker or Podman if missing, fetches ServerDash, and runs it as a
# systemd-managed `docker compose` (or `podman compose`) service.
#
# Run it interactively and it walks you through the setup:
#   curl -fsSL https://raw.githubusercontent.com/turkfork/ServerDash/main/install.sh | sudo bash
#   sudo ./install.sh
#
# Or script it. Any flag you pass is used as the default, and --yes skips every
# prompt. Without a terminal (CI, cron, ssh -T) it never prompts.
#
# Safe to re-run: pulls the latest code, rebuilds the image, and restarts the
# service. Never touches an existing .env or the SQLite data volume.

set -euo pipefail

REPO_URL="${SERVERDASH_REPO_URL:-https://github.com/turkfork/ServerDash.git}"
UNIT_DIR="${SERVERDASH_UNIT_DIR:-/etc/systemd/system}"

RUNTIME=""
RUNTIME_FLAG=0
INSTALL_DIR="/opt/serverdash"
PUBLIC_HOST="nova.blacklink.net"
PORT="9900"
BRANCH="main"
START_SERVICE=1
ASSUME_YES=0

SCRIPT_PATH=""
SCRIPT_DIR=""
if [ -n "${BASH_SOURCE[0]:-}" ] && [ -f "${BASH_SOURCE[0]}" ]; then
	SCRIPT_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
	SCRIPT_DIR="$(dirname "$SCRIPT_PATH")"
fi

RST="" BOLD="" DIM="" RED="" GREEN="" YELLOW="" GRAY="" AMBER="" AMBER_HI="" AMBER_LO=""
G_OK="+" G_FAIL="x" G_WARN="!" G_SEL=">" G_DOT="*"
UNICODE=0

MODE=clone          # clone | update | local
ENV_EXISTS=0
COMPOSE_CMD=""
LOG=""
BG_PID=""
STEP_SOFT=0
MENU_RESULT=0
PANEL_LINES=0
ANIM=0
PROMPTS=0
W=72

# ───────────────────────────── usage ─────────────────────────────

usage() {
	cat <<'EOF'
ServerDash installer

Usage:
  curl -fsSL https://raw.githubusercontent.com/turkfork/ServerDash/main/install.sh | sudo bash
  sudo ./install.sh [options]

Options:
  --runtime docker|podman   Container runtime to install/use (default: docker)
  --dir PATH                Install directory (default: /opt/serverdash)
  --public-host HOST        Value for SERVERDASH_PUBLIC_HOST (default: nova.blacklink.net)
  --port PORT               Host port to publish the dashboard on (default: 9900)
  --branch NAME             Git branch/tag to install (default: main)
  --no-start                Install and build, but don't enable/start the service
  -y, --yes                 Don't prompt; use the flags and defaults as given
  -h, --help                Show this help

With a terminal attached, the installer is interactive and the flags above
become the pre-filled defaults. NO_COLOR=1 turns off colors.
EOF
}

# ───────────────────────────── ui setup ─────────────────────────────

init_ui() {
	local cols

	if [ -t 1 ]; then ANIM=1; fi
	if [ "$ANIM" -eq 1 ] && [ "$ASSUME_YES" -eq 0 ] && { : </dev/tty; } 2>/dev/null; then
		PROMPTS=1
	fi

	RST="" BOLD="" DIM="" RED="" GREEN="" YELLOW="" GRAY="" AMBER="" AMBER_HI="" AMBER_LO=""
	if [ "$ANIM" -eq 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != dumb ]; then
		RST=$'\033[0m' BOLD=$'\033[1m' DIM=$'\033[2m'
		RED=$'\033[38;5;203m' GREEN=$'\033[38;5;114m' YELLOW=$'\033[38;5;221m' GRAY=$'\033[38;5;245m'
		case "${COLORTERM:-}" in
			*truecolor*|*24bit*) AMBER=$'\033[38;2;255;176;32m' ;;
			*) AMBER=$'\033[38;5;214m' ;;
		esac
		AMBER_HI=$'\033[38;5;222m'
		AMBER_LO=$'\033[38;5;208m'
	fi

	UNICODE=0
	case "${LC_ALL:-${LC_CTYPE:-${LANG:-}}}" in
		*[Uu][Tt][Ff]-8*|*[Uu][Tt][Ff]8*) UNICODE=1 ;;
	esac
	if [ "${TERM:-}" = linux ]; then UNICODE=0; fi

	if [ "$UNICODE" -eq 1 ]; then
		G_OK="✔" G_FAIL="✖" G_WARN="▲" G_SEL="❯" G_DOT="●"
		B_TL="╭" B_TR="╮" B_BL="╰" B_BR="╯" B_H="─" B_V="│"
		SPIN=("⠋" "⠙" "⠹" "⠸" "⠼" "⠴" "⠦" "⠧" "⠇" "⠏")
	else
		G_OK="+" G_FAIL="x" G_WARN="!" G_SEL=">" G_DOT="*"
		B_TL="+" B_TR="+" B_BL="+" B_BR="+" B_H="-" B_V="|"
		SPIN=("|" "/" "-" "\\")
	fi

	PROMPT=$'\001'"${AMBER}"$'\002'"${G_SEL} "$'\001'"${RST}"$'\002'

	cols="$(tput cols 2>/dev/null || true)"
	[[ "$cols" =~ ^[0-9]+$ ]] || cols=80
	W=$cols
	if [ "$W" -gt 76 ]; then W=76; fi
	if [ "$W" -lt 60 ]; then W=60; fi
}

cursor_hide() { if [ "$ANIM" -eq 1 ]; then printf '\033[?25l'; fi; return 0; }
cursor_show() { if [ "$ANIM" -eq 1 ]; then printf '\033[?25h'; fi; return 0; }

cleanup() {
	cursor_show
	if [ -n "$BG_PID" ]; then kill "$BG_PID" 2>/dev/null || true; fi
	if [ "$PROMPTS" -eq 1 ]; then stty sane </dev/tty 2>/dev/null || true; fi
	return 0
}

on_signal() {
	cursor_show
	printf '\n\n  %s%s%s Cancelled.\n\n' "$YELLOW" "$G_WARN" "$RST"
	exit 130
}

trap cleanup EXIT
trap on_signal INT TERM

# ───────────────────────────── print helpers ─────────────────────────────

rep() { # rep CHAR COUNT
	local s="" i
	for ((i = 0; i < $2; i++)); do s+="$1"; done
	printf '%s' "$s"
}

vlen() { # visible length, ignoring ANSI escapes
	local s
	s="$(printf '%s' "$1" | sed $'s/\033\\[[0-9;?]*[a-zA-Z]//g')"
	printf '%s' "${#s}"
}

strip_ansi() { sed $'s/\033\\[[0-9;?]*[a-zA-Z]//g'; }

ok()   { printf '  %s%s%s %s\n' "$GREEN" "$G_OK" "$RST" "$1"; }
warn() { printf '  %s%s%s %s\n' "$YELLOW" "$G_WARN" "$RST" "$1"; }
note() { printf '    %s%s%s\n' "$DIM" "$1" "$RST"; }
die()  { printf '\n  %s%s error:%s %s\n\n' "$RED" "$G_FAIL" "$RST" "$1" >&2; exit 1; }

section() {
	printf '\n  %s%s%s %s%s%s\n\n' "$AMBER" "$B_H$B_H" "$RST" "$BOLD" "$1" "$RST"
}

fmt_time() {
	local s=$1
	if [ "$s" -ge 60 ]; then
		printf '%dm %02ds' $((s / 60)) $((s % 60))
	else
		printf '%ds' "$s"
	fi
}

banner() {
	printf '\n'
	if [ "$UNICODE" -eq 1 ]; then
		printf '  %s┏━┓┏━╸┏━┓╻ ╻┏━╸┏━┓╺┳┓┏━┓┏━┓╻ ╻%s\n' "$AMBER_HI" "$RST"
		printf '  %s┗━┓┣╸ ┣┳┛┃┏┛┣╸ ┣┳┛ ┃┃┣━┫┗━┓┣━┫%s\n' "$AMBER" "$RST"
		printf '  %s┗━┛┗━╸╹┗╸┗┛ ┗━╸╹┗╸╺┻┛╹ ╹┗━┛╹ ╹%s\n' "$AMBER_LO" "$RST"
	else
		printf '  %sServerDash%s\n' "$AMBER$BOLD" "$RST"
	fi
	printf '  %sself-hosted server dashboard  %s  installer%s\n' "$GRAY" "$G_DOT" "$RST"
}

panel() { # panel TITLE ROW...   (rows may contain color codes)
	local title=$1 inner=$((W - 4)) rest row pad
	shift
	PANEL_LINES=2
	rest=$((inner - 3 - $(vlen "$title")))
	if [ "$rest" -lt 1 ]; then rest=1; fi
	printf '  %s%s%s%s %s%s%s %s%s%s%s\n' \
		"$AMBER" "$B_TL" "$B_H" "$RST" "$BOLD" "$title" "$RST" "$AMBER" "$(rep "$B_H" "$rest")" "$B_TR" "$RST"
	for row in "$@"; do
		pad=$((inner - 2 - $(vlen "$row")))
		if [ "$pad" -lt 0 ]; then pad=0; fi
		printf '  %s%s%s %s%*s %s%s%s\n' "$AMBER" "$B_V" "$RST" "$row" "$pad" "" "$AMBER" "$B_V" "$RST"
		PANEL_LINES=$((PANEL_LINES + 1))
	done
	printf '  %s%s%s%s%s\n' "$AMBER" "$B_BL" "$(rep "$B_H" "$inner")" "$B_BR" "$RST"
}

kv() { # kv LABEL VALUE [TAG]
	printf '%s%-13s%s %s%s' "$GRAY" "$1" "$RST" "$2" "${3:+  $DIM$3$RST}"
}

erase_up() { # erase_up LINES: move up and clear everything below
	printf '\033[%dA\033[J' "$1"
}

# ───────────────────────────── input widgets ─────────────────────────────

# menu TITLE DEFAULT_INDEX LABEL DESC [LABEL DESC ...]  ->  MENU_RESULT
menu() {
	local title=$1 sel=$2
	shift 2
	local -a labels=() descs=()
	local n i key rest
	while [ $# -gt 0 ]; do
		labels+=("$1")
		descs+=("${2:-}")
		shift 2
	done
	n=${#labels[@]}

	_menu_draw() {
		for ((i = 0; i < n; i++)); do
			if [ "$i" -eq "$sel" ]; then
				printf '\r\033[K  %s%s%s %s%-12s%s %s%s%s\n' "$AMBER" "$G_SEL" "$RST" "$BOLD" "${labels[$i]}" "$RST" "$GRAY" "${descs[$i]}" "$RST"
			else
				printf '\r\033[K    %s%-12s %s%s\n' "$RST" "${labels[$i]}" "$DIM" "${descs[$i]}$RST"
			fi
		done
	}

	printf '  %s%s%s  %s↑/↓ move · enter select%s\n' "$BOLD" "$title" "$RST" "$DIM" "$RST"
	cursor_hide
	_menu_draw
	while :; do
		key=""
		IFS= read -rsn1 key </dev/tty || true
		case "$key" in
			$'\033')
				rest=""
				IFS= read -rsn2 -t 0.05 rest </dev/tty || true
				case "$rest" in
					'[A') sel=$(((sel + n - 1) % n)) ;;
					'[B') sel=$(((sel + 1) % n)) ;;
				esac
				;;
			k) sel=$(((sel + n - 1) % n)) ;;
			j) sel=$(((sel + 1) % n)) ;;
			[1-9])
				if [ "$((key - 1))" -lt "$n" ]; then sel=$((key - 1)); fi
				;;
			q) cursor_show; on_signal ;;
			'') break ;;
		esac
		printf '\033[%dA' "$n"
		_menu_draw
	done
	erase_up $((n + 1))
	cursor_show
	MENU_RESULT=$sel
}

# ask LABEL DEFAULT VALIDATOR [HINT]  ->  REPLY   (prefilled, editable)
ask() {
	local label=$1 def=$2 validator=$3 hint=${4:-} val err extra=0
	while :; do
		printf '  %s%s%s%s\n' "$BOLD" "$label" "$RST" "${hint:+  $DIM$hint$RST}"
		val=""
		IFS= read -r -e -i "$def" -p "$PROMPT" val </dev/tty || true
		val="${val#"${val%%[![:space:]]*}"}"
		val="${val%"${val##*[![:space:]]}"}"
		if err="$("$validator" "$val")"; then
			erase_up $((2 + extra))
			REPLY=$val
			return 0
		fi
		erase_up $((2 + extra))
		printf '  %s%s%s %s\n' "$RED" "$G_FAIL" "$RST" "$err"
		extra=$((extra + 1))
		def=$val
	done
}

# ───────────────────────────── validators ─────────────────────────────

v_dir() {
	if [[ "$1" =~ ^/[A-Za-z0-9._/@+-]+$ ]]; then return 0; fi
	echo "Use an absolute path (letters, numbers and . _ / @ + - only)"
	return 1
}
v_host() {
	if [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9._:-]*$ ]]; then return 0; fi
	echo "Enter a hostname like nova.example.com"
	return 1
}
v_port() {
	if [[ "$1" =~ ^[0-9]+$ ]] && [ "$1" -ge 1 ] && [ "$1" -le 65535 ]; then return 0; fi
	echo "Port must be a number from 1 to 65535"
	return 1
}
v_branch() {
	if [[ "$1" =~ ^[A-Za-z0-9._/-]+$ ]]; then return 0; fi
	echo "Branch or tag names can't contain spaces or special characters"
	return 1
}

# ───────────────────────────── step runner ─────────────────────────────

# run_step LABEL COMMAND...  - runs with output going to $LOG, shows a spinner
run_step() {
	local label=$1 start=$SECONDS rc=0 i=0 frame last room soft=$STEP_SOFT elapsed
	shift
	STEP_SOFT=0
	printf '\n### %s\n' "$label" >>"$LOG"

	"$@" >>"$LOG" 2>&1 </dev/null &
	BG_PID=$!

	if [ "$ANIM" -eq 1 ]; then
		cursor_hide
		while kill -0 "$BG_PID" 2>/dev/null; do
			frame="${SPIN[$((i % ${#SPIN[@]}))]}"
			room=$((W - 14 - ${#label}))
			last=""
			if [ "$room" -gt 12 ]; then
				last="$(tail -c 600 "$LOG" | tr '\r' '\n' | strip_ansi | grep -v '^[[:space:]]*$' | tail -n 1 | cut -c1-"$room" || true)"
			fi
			printf '\r\033[K  %s%s%s %s  %s%s%s' "$AMBER" "$frame" "$RST" "$label" "$DIM" "$(fmt_time $((SECONDS - start)))" "$RST"
			if [ -n "$last" ]; then printf '  %s%s%s' "$GRAY" "$last" "$RST"; fi
			i=$((i + 1))
			sleep 0.1
		done
		printf '\r\033[K'
		cursor_show
	fi
	wait "$BG_PID" || rc=$?
	BG_PID=""

	elapsed=$((SECONDS - start))
	if [ "$rc" -eq 0 ]; then
		ok "$label$([ "$elapsed" -ge 2 ] && printf '  %s%s%s' "$DIM" "$(fmt_time "$elapsed")" "$RST")"
		return 0
	fi
	if [ "$soft" -eq 1 ]; then
		warn "$label  ${DIM}(skipped)${RST}"
		return 0
	fi

	printf '  %s%s%s %s\n\n' "$RED" "$G_FAIL" "$RST" "$label"
	printf '  %sLast output:%s\n' "$GRAY" "$RST"
	tail -n 15 "$LOG" | tr '\r' '\n' | strip_ansi | cut -c1-$((W - 6)) | sed "s/^/    ${DIM}/; s/\$/${RST}/"
	printf '\n  %sFull log:%s %s\n\n' "$GRAY" "$RST" "$LOG"
	exit 1
}

# ───────────────────────────── install steps ─────────────────────────────

docker_ready()    { command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; }
podman_ready()    { command -v podman >/dev/null 2>&1; }
podman_compose_ready() { command -v podman-compose >/dev/null 2>&1 || podman compose version >/dev/null 2>&1; }

install_docker() {
	command -v curl >/dev/null 2>&1 || die "curl is required to install Docker"
	curl -fsSL https://get.docker.com | sh
	systemctl enable --now docker
}

install_podman_pkg() {
	export DEBIAN_FRONTEND=noninteractive
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
}

install_podman_compose() {
	if command -v pip3 >/dev/null 2>&1; then
		pip3 install --quiet podman-compose
	else
		die "podman-compose isn't installed and pip3 isn't available to install it; install podman-compose manually"
	fi
}

enable_podman_socket() { systemctl enable --now podman.socket; }

fetch_source() {
	case "$MODE" in
		local)
			echo "Using existing checkout at $INSTALL_DIR"
			;;
		update)
			git -C "$INSTALL_DIR" fetch --depth 1 origin "$BRANCH"
			git -C "$INSTALL_DIR" checkout "$BRANCH"
			git -C "$INSTALL_DIR" reset --hard "origin/$BRANCH"
			;;
		clone)
			command -v git >/dev/null 2>&1 || die "git is required to fetch ServerDash"
			mkdir -p "$(dirname "$INSTALL_DIR")"
			git clone --branch "$BRANCH" --depth 1 "$REPO_URL" "$INSTALL_DIR"
			;;
	esac
}

configure_env() {
	cd "$INSTALL_DIR"
	if [ ! -f .env ]; then
		cp .env.example .env
		sed -i "s#^SERVERDASH_PUBLIC_HOST=.*#SERVERDASH_PUBLIC_HOST=$PUBLIC_HOST#" .env
		sed -i "s#^SERVERDASH_PORT=.*#SERVERDASH_PORT=$PORT#" .env
		echo "Wrote .env (public host: $PUBLIC_HOST, port: $PORT)"
	else
		echo ".env already exists, leaving it as-is"
	fi
	if [ "$RUNTIME" = podman ]; then
		# docker-compose.yml assumes Docker's socket path; point it at root
		# Podman's socket, since the systemd service below runs as root.
		if ! grep -q '^SERVERDASH_DOCKER_HOST=' .env; then
			echo 'SERVERDASH_DOCKER_HOST=unix:///run/podman/podman.sock' >>.env
		fi
	fi
}

build_image() {
	cd "$INSTALL_DIR"
	export BUILDKIT_PROGRESS=plain COMPOSE_PROGRESS=plain
	$COMPOSE_CMD build
}

write_unit() {
	cat >"$UNIT_DIR/serverdash.service" <<EOF
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
}

start_service() {
	systemctl enable serverdash
	systemctl restart serverdash
}

wait_healthy() {
	local i
	command -v curl >/dev/null 2>&1 || return 0
	for ((i = 0; i < 90; i++)); do
		if curl -s -o /dev/null --max-time 2 "http://127.0.0.1:$PORT/"; then return 0; fi
		sleep 1
	done
	echo "no response from http://127.0.0.1:$PORT after 90s"
	return 1
}

# ───────────────────────────── wizard ─────────────────────────────

env_val() { sed -n "s/^$1=//p" "$INSTALL_DIR/.env" 2>/dev/null | tail -n 1 | tr -d '"\r' || true; }

resolve_source() {
	local v
	if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/docker-compose.yml" ] && [ -f "$SCRIPT_DIR/Dockerfile" ]; then
		MODE=local
		INSTALL_DIR=$SCRIPT_DIR
	elif [ -d "$INSTALL_DIR/.git" ]; then
		MODE=update
	else
		MODE=clone
	fi
	ENV_EXISTS=0
	if [ -f "$INSTALL_DIR/.env" ]; then
		ENV_EXISTS=1
		v="$(env_val SERVERDASH_PUBLIC_HOST)"; if [ -n "$v" ]; then PUBLIC_HOST=$v; fi
		v="$(env_val SERVERDASH_PORT)"; if [ -n "$v" ]; then PORT=$v; fi
	fi
}

default_runtime() {
	if [ "$RUNTIME_FLAG" -eq 1 ]; then return 0; fi
	if docker_ready; then
		RUNTIME=docker
	elif podman_ready; then
		RUNTIME=podman
	else
		RUNTIME=docker
	fi
}

choose_runtime() {
	local d_tag="" p_tag="" idx=0
	if docker_ready; then d_tag="$G_DOT installed"; fi
	if podman_ready; then p_tag="$G_DOT installed"; fi
	if [ "$RUNTIME" = podman ]; then idx=1; fi
	menu "Container runtime" "$idx" \
		"Docker" "Official install script ${d_tag:+· $d_tag}" \
		"Podman" "Daemonless, from your package manager ${p_tag:+· $p_tag}"
	if [ "$MENU_RESULT" -eq 0 ]; then RUNTIME=docker; else RUNTIME=podman; fi
}

print_settings() {
	local mode_label dir_tag="" env_tag=""
	case "$MODE" in
		local) mode_label="Use this checkout"; dir_tag="(this checkout)" ;;
		update) mode_label="Upgrade existing install" ;;
		clone) mode_label="Fresh install" ;;
	esac
	if [ "$ENV_EXISTS" -eq 1 ]; then env_tag="(from existing .env)"; fi
	panel "Install settings" \
		"$(kv "Mode" "$mode_label")" \
		"$(kv "Runtime" "$RUNTIME")" \
		"$(kv "Directory" "$INSTALL_DIR" "$dir_tag")" \
		"$(kv "Public host" "$PUBLIC_HOST" "$env_tag")" \
		"$(kv "Port" "$PORT" "$env_tag")" \
		"$(kv "Branch" "$BRANCH")" \
		"$(kv "Service" "$([ "$START_SERVICE" -eq 1 ] && echo 'enable and start on boot' || echo 'install only, do not start')")"
}

edit_settings() {
	if [ "$MODE" != local ]; then
		ask "Install directory" "$INSTALL_DIR" v_dir "where ServerDash will live"
		INSTALL_DIR=$REPLY
		resolve_source
	fi
	if [ "$ENV_EXISTS" -eq 0 ]; then
		ask "Public host" "$PUBLIC_HOST" v_host "the domain you'll serve it from"
		PUBLIC_HOST=$REPLY
		ask "Port" "$PORT" v_port "host port for the dashboard"
		PORT=$REPLY
	fi
	ask "Branch or tag" "$BRANCH" v_branch "what to install"
	BRANCH=$REPLY
	choose_runtime
	menu "Service" "$([ "$START_SERVICE" -eq 1 ] && echo 0 || echo 1)" \
		"Start" "Enable on boot and start now" \
		"Install only" "Build everything, start it yourself later"
	if [ "$MENU_RESULT" -eq 0 ]; then START_SERVICE=1; else START_SERVICE=0; fi
}

review_settings() {
	if [ "$PROMPTS" -eq 0 ]; then
		print_settings
		return 0
	fi
	section "Setup"
	if [ "$RUNTIME_FLAG" -eq 0 ]; then choose_runtime; fi
	while :; do
		print_settings
		printf '\n'
		menu "Ready to install?" 0 \
			"Install" "Go ahead with these settings" \
			"Change" "Edit directory, host, port, runtime…" \
			"Cancel" "Exit without changing anything"
		case "$MENU_RESULT" in
			0) return 0 ;;
			1) erase_up $((PANEL_LINES + 1)); edit_settings ;;
			2) on_signal ;;
		esac
	done
}

# ───────────────────────────── main flow ─────────────────────────────

preflight() {
	local os arch
	if [ "$(id -u)" -ne 0 ]; then
		if [ -n "$SCRIPT_PATH" ] && command -v sudo >/dev/null 2>&1; then
			warn "Root is required, re-running with sudo…"
			exec sudo -E bash "$SCRIPT_PATH" ${ORIG_ARGS[@]+"${ORIG_ARGS[@]}"}
		fi
		die "must be run as root (installs packages and a systemd unit). Try: curl -fsSL <url> | sudo bash"
	fi
	command -v systemctl >/dev/null 2>&1 || die "systemd (systemctl) is required"

	os="$(. /etc/os-release 2>/dev/null && printf '%s' "${PRETTY_NAME:-Linux}" || echo Linux)"
	arch="$(uname -m)"
	section "System"
	ok "$os  ${DIM}$arch${RST}"
	ok "Running as root, systemd available"
	if command -v getenforce >/dev/null 2>&1 && [ "$(getenforce 2>/dev/null)" = Enforcing ]; then
		warn "SELinux is enforcing"
		note "workflows and the Kubernetes mount need extra options, see the README"
	fi
}

install_all() {
	section "Installing"

	if [ "$RUNTIME" = docker ]; then
		if docker_ready; then
			ok "Docker ready  ${DIM}$(docker --version | cut -d, -f1)${RST}"
		else
			run_step "Installing Docker" install_docker
		fi
		COMPOSE_CMD="docker compose"
	else
		if podman_ready; then
			ok "Podman ready  ${DIM}$(podman --version)${RST}"
		else
			run_step "Installing Podman" install_podman_pkg
		fi
		if podman_compose_ready; then
			ok "podman-compose ready"
		else
			run_step "Installing podman-compose" install_podman_compose
		fi
		STEP_SOFT=1 run_step "Enabling the Podman socket" enable_podman_socket
		if command -v podman-compose >/dev/null 2>&1; then
			COMPOSE_CMD="podman-compose"
		else
			COMPOSE_CMD="podman compose"
		fi
	fi

	case "$MODE" in
		local)  run_step "Using checkout at $INSTALL_DIR" fetch_source ;;
		update) run_step "Updating $INSTALL_DIR ($BRANCH)" fetch_source ;;
		clone)  run_step "Cloning ServerDash ($BRANCH)" fetch_source ;;
	esac

	run_step "Writing configuration" configure_env
	PORT_FROM_ENV="$(sed -n 's/^SERVERDASH_PORT=//p' "$INSTALL_DIR/.env" | tail -n 1 | tr -d '"\r' || true)"
	if [ -n "$PORT_FROM_ENV" ]; then PORT=$PORT_FROM_ENV; fi
	PUBLIC_FROM_ENV="$(sed -n 's/^SERVERDASH_PUBLIC_HOST=//p' "$INSTALL_DIR/.env" | tail -n 1 | tr -d '"\r' || true)"
	if [ -n "$PUBLIC_FROM_ENV" ]; then PUBLIC_HOST=$PUBLIC_FROM_ENV; fi

	run_step "Building the image (a few minutes the first time)" build_image
	run_step "Installing the systemd service" write_unit

	if [ "$START_SERVICE" -eq 1 ]; then
		run_step "Starting ServerDash" start_service
		STEP_SOFT=1 run_step "Waiting for the dashboard to respond" wait_healthy
	else
		warn "Not starting the service ${DIM}(install only)${RST}"
	fi
}

summary() {
	local ip title took
	ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"
	if [ -z "$ip" ]; then ip="<this-host>"; fi
	took="$(fmt_time $((SECONDS - T0)))"

	if [ "$START_SERVICE" -eq 1 ]; then
		case "$MODE" in update) title="ServerDash upgraded" ;; *) title="ServerDash is running" ;; esac
	else
		title="ServerDash is installed"
	fi

	printf '\n'
	if [ "$START_SERVICE" -eq 1 ]; then
		panel "$title" \
			"$(kv "Open" "${AMBER}http://$ip:$PORT${RST}")" \
			"$(kv "Public URL" "https://$PUBLIC_HOST" "(via reverse proxy)")" \
			"$(kv "Directory" "$INSTALL_DIR")" \
			"$(kv "Runtime" "$COMPOSE_CMD")" \
			"$(kv "Data" "serverdash-data volume, kept on upgrade")"
	else
		panel "$title" \
			"$(kv "Start it" "${AMBER}systemctl start serverdash${RST}")" \
			"$(kv "Directory" "$INSTALL_DIR")" \
			"$(kv "Runtime" "$COMPOSE_CMD")" \
			"$(kv "Data" "serverdash-data volume, kept on upgrade")"
	fi

	if [ "$START_SERVICE" -eq 1 ] && [ "$MODE" != update ]; then
		printf '\n  %sNext%s\n' "$BOLD" "$RST"
		printf '    %s1.%s Open the URL above and create the admin account\n' "$AMBER" "$RST"
		printf '    %s2.%s Put a reverse proxy in front of it for https://%s\n' "$AMBER" "$RST" "$PUBLIC_HOST"
	fi

	printf '\n  %sUseful commands%s\n' "$BOLD" "$RST"
	printf '    %ssystemctl status serverdash%s\n' "$GRAY" "$RST"
	printf '    %scd %s && %s logs -f%s\n' "$GRAY" "$INSTALL_DIR" "$COMPOSE_CMD" "$RST"
	printf '    %sre-run this installer to upgrade%s\n' "$GRAY" "$RST"
	printf '\n  %s%s%s Done in %s\n\n' "$GREEN" "$G_OK" "$RST" "$took"
	rm -f "$LOG"
}

parse_args() {
	while [ $# -gt 0 ]; do
		case "$1" in
			--runtime)     [ $# -ge 2 ] || die "--runtime needs a value"; RUNTIME=$2; RUNTIME_FLAG=1; shift 2 ;;
			--dir)         [ $# -ge 2 ] || die "--dir needs a value"; INSTALL_DIR=$2; shift 2 ;;
			--public-host) [ $# -ge 2 ] || die "--public-host needs a value"; PUBLIC_HOST=$2; shift 2 ;;
			--port)        [ $# -ge 2 ] || die "--port needs a value"; PORT=$2; shift 2 ;;
			--branch)      [ $# -ge 2 ] || die "--branch needs a value"; BRANCH=$2; shift 2 ;;
			--no-start)    START_SERVICE=0; shift ;;
			-y|--yes)      ASSUME_YES=1; shift ;;
			-h|--help)     usage; exit 0 ;;
			*)             die "unknown option: $1 (see --help)" ;;
		esac
	done

	case "$RUNTIME" in
		""|docker|podman) ;;
		*) die "--runtime must be 'docker' or 'podman', got '$RUNTIME'" ;;
	esac
	local err
	err="$(v_dir "$INSTALL_DIR")" || die "--dir: $err"
	err="$(v_host "$PUBLIC_HOST")" || die "--public-host: $err"
	err="$(v_port "$PORT")" || die "--port: $err"
	err="$(v_branch "$BRANCH")" || die "--branch: $err"
}

main() {
	ORIG_ARGS=("$@")
	T0=$SECONDS
	parse_args "$@"
	init_ui
	banner
	preflight
	LOG="$(mktemp /tmp/serverdash-install.XXXXXX)"
	default_runtime
	resolve_source
	review_settings
	install_all
	summary
}

main "$@"
