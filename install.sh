#!/usr/bin/env bash
# ServerDash installer
#
# Installs Docker or Podman if missing, fetches ServerDash, and runs it as a
# systemd-managed `docker compose` (or `podman compose`) service. Or, with
# `--runtime native`, builds it from source and runs it directly on the host
# as a single binary, with no container runtime needed at all.
#
# Run it interactively and it walks you through the setup:
#   curl -fsSL https://raw.githubusercontent.com/BlacklinkLLC/ServerDash/main/install.sh | sudo bash
#   sudo ./install.sh
#
# Or script it. Any flag you pass is used as the default, and --yes skips every
# prompt. Without a terminal (CI, cron, ssh -T) it never prompts.
#
# It creates the first admin login for you (a generated password, shown once at the
# end, unless you choose your own or pass --no-admin to use the setup screen).
#
# It also installs a `serverdash` command, so you can change settings later:
#   serverdash config          edit settings (port, public host, ...) interactively
#   serverdash port 8080       change the port and restart
#   serverdash status | restart | logs | update
#
# Safe to re-run: pulls the latest code, rebuilds the image, and restarts the
# service. Never touches the SQLite data volume. An existing .env is left alone
# unless you pass --port / --public-host or change them in the wizard.

set -euo pipefail

REPO_URL="${SERVERDASH_REPO_URL:-https://github.com/BlacklinkLLC/ServerDash.git}"
UNIT_DIR="${SERVERDASH_UNIT_DIR:-/etc/systemd/system}"

RUNTIME=""
RUNTIME_FLAG=0
INSTALL_DIR="/opt/serverdash"
PUBLIC_HOST="nova.blacklink.net"
PORT="9900"
BRANCH="main"
START_SERVICE=1
ASSUME_YES=0
PORT_SET=0
HOST_SET=0
ADMIN_MODE=generate              # generate | custom | skip
ADMIN_USER="admin"
ADMIN_PASS="${SERVERDASH_ADMIN_PASSWORD:-}"
ADMIN_RESULT=""                  # created | exists | failed | "" (not attempted)
SETUP_MODE=terminal              # terminal | web
SETUP_CODE=""
if [ -n "$ADMIN_PASS" ]; then ADMIN_MODE=custom; fi

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
STEP_RC=0
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
  curl -fsSL https://raw.githubusercontent.com/BlacklinkLLC/ServerDash/main/install.sh | sudo bash
  sudo ./install.sh [options]

Options:
  --runtime docker|podman|native
                            How to run ServerDash (default: docker). native
                            builds it from source and runs it directly under
                            systemd, without any container runtime; it also
                            enables the Services and System logs tools.
  --dir PATH                Install directory (default: /opt/serverdash)
  --public-host HOST        Value for SERVERDASH_PUBLIC_HOST (default: nova.blacklink.net)
  --port PORT               Host port to publish the dashboard on (default: 9900).
                            Also updates an existing install's .env.
  --branch NAME             Git branch/tag to install (default: main)
  --admin-user NAME         Username for the first admin login (default: admin)
  --admin-password PASS     Password for it (8+ chars). Visible in `ps`, so prefer
                            the SERVERDASH_ADMIN_PASSWORD environment variable.
                            Without one, a random password is generated and shown
                            once at the end.
  --no-admin                Don't create a login; use the browser setup screen
  --web-setup               Install with defaults, then finish setup in your browser
                            (admin account, branding, and cluster role). A one-time
                            setup code is shown so nobody else can claim the server.
  --no-start                Install and build, but don't enable/start the service
  -y, --yes                 Don't prompt; use the flags and defaults as given
  -h, --help                Show this help

With a terminal attached, the installer is interactive and the flags above
become the pre-filled defaults. NO_COLOR=1 turns off colors.

After installing, use the `serverdash` command to change settings:
  serverdash config         interactive settings editor
  serverdash port 8080      change the port and restart
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

# ask_secret LABEL -> REPLY   (hidden input, asked twice, validated)
ask_secret() {
	local label=$1 p1 p2 err extra=0
	while :; do
		printf '  %s%s%s  %sinput is hidden%s\n' "$BOLD" "$label" "$RST" "$DIM" "$RST"
		printf '  %s%s%s ' "$AMBER" "$G_SEL" "$RST"
		p1=""
		IFS= read -rs p1 </dev/tty || true
		printf '\n  %sRepeat it%s\n  %s%s%s ' "$BOLD" "$RST" "$AMBER" "$G_SEL" "$RST"
		p2=""
		IFS= read -rs p2 </dev/tty || true
		printf '\n'
		err=""
		if ! err="$(v_password "$p1")"; then :
		elif [ "$p1" != "$p2" ]; then err="Those didn't match"
		fi
		erase_up $((4 + extra))
		if [ -z "$err" ]; then
			REPLY=$p1
			return 0
		fi
		printf '  %s%s%s %s\n' "$RED" "$G_FAIL" "$RST" "$err"
		extra=$((extra + 1))
	done
}

gen_password() { # 16 unambiguous characters, grouped for easy copying
	local raw
	raw="$({ LC_ALL=C tr -dc 'A-HJ-NP-Za-km-z2-9' </dev/urandom 2>/dev/null | head -c 16; } || true)"
	[ "${#raw}" -eq 16 ] || die "could not generate a password (is /dev/urandom readable?)"
	printf '%s-%s-%s-%s' "${raw:0:4}" "${raw:4:4}" "${raw:8:4}" "${raw:12:4}"
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
v_user() {
	if [[ "$1" =~ ^[A-Za-z0-9._-]{3,32}$ ]]; then return 0; fi
	echo "Username: 3 to 32 letters, numbers and . _ -"
	return 1
}
v_password() {
	if [ "${#1}" -lt 8 ]; then
		echo "Password must be at least 8 characters"
		return 1
	fi
	if [[ "$1" =~ [[:cntrl:]] ]]; then
		echo "Password can't contain control characters"
		return 1
	fi
	return 0
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

	STEP_RC=$rc
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

NATIVE_DATA_DIR="${SERVERDASH_NATIVE_DATA_DIR:-/var/lib/serverdash}"

install_git() {
	export DEBIAN_FRONTEND=noninteractive
	if command -v dnf >/dev/null 2>&1; then dnf install -y git curl tar xz
	elif command -v apt-get >/dev/null 2>&1; then apt-get update && apt-get install -y git curl tar xz-utils
	elif command -v pacman >/dev/null 2>&1; then pacman -Sy --noconfirm git curl tar xz
	elif command -v zypper >/dev/null 2>&1; then zypper install -y git curl tar xz
	elif command -v apk >/dev/null 2>&1; then apk add --no-cache git curl tar xz bash
	else die "no supported package manager found; install git, curl, tar and xz manually and re-run"
	fi
}

stop_container_install() { # stop_container_install COMPOSE_CMD
	systemctl stop serverdash 2>/dev/null || true
	cd "$INSTALL_DIR" && $1 down
}

build_native() {
	bash "$INSTALL_DIR/scripts/build-native.sh" "$INSTALL_DIR"
}

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

set_env_key() { # set_env_key KEY VALUE (values are validated: no sed-special characters)
	if grep -q "^$1=" .env; then
		sed -i "s#^$1=.*#$1=$2#" .env
	else
		if [ -s .env ] && [ -n "$(tail -c1 .env)" ]; then echo >>.env; fi
		echo "$1=$2" >>.env
	fi
}

configure_env() {
	cd "$INSTALL_DIR"
	if [ ! -f .env ]; then
		cp .env.example .env
		set_env_key SERVERDASH_PUBLIC_HOST "$PUBLIC_HOST"
		set_env_key SERVERDASH_PORT "$PORT"
		echo "Wrote .env (public host: $PUBLIC_HOST, port: $PORT)"
	else
		echo ".env already exists, leaving it as-is"
		if [ "$HOST_SET" -eq 1 ]; then
			set_env_key SERVERDASH_PUBLIC_HOST "$PUBLIC_HOST"
			echo "Updated public host to $PUBLIC_HOST"
		fi
		if [ "$PORT_SET" -eq 1 ]; then
			set_env_key SERVERDASH_PORT "$PORT"
			echo "Updated port to $PORT"
		fi
	fi
	if [ "$SETUP_MODE" = web ]; then
		set_env_key SERVERDASH_SETUP_CODE "$SETUP_CODE"
		echo "Wrote a one-time setup code for the browser setup screen"
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
	if [ "$RUNTIME" = native ]; then build_native; return; fi
	cd "$INSTALL_DIR"
	export BUILDKIT_PROGRESS=plain COMPOSE_PROGRESS=plain
	$COMPOSE_CMD build
}

write_native_unit() {
	mkdir -p "$NATIVE_DATA_DIR"
	chmod 700 "$NATIVE_DATA_DIR"
	cat >"$UNIT_DIR/serverdash.service" <<EOF
[Unit]
Description=ServerDash
Documentation=https://github.com/BlacklinkLLC/ServerDash
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
# .env holds SERVERDASH_PORT / SERVERDASH_PUBLIC_HOST and any overrides,
# the same file the container install uses.
EnvironmentFile=-$INSTALL_DIR/.env
Environment=SERVERDASH_WEB_DIR=$INSTALL_DIR/web/dist
Environment=SERVERDASH_DB_PATH=$NATIVE_DATA_DIR/serverdash.db
Environment=SERVERDASH_INSTALL_DIR=$INSTALL_DIR
Environment="SERVERDASH_UPDATE_CMD=/usr/local/bin/serverdash update"
ExecStart=$INSTALL_DIR/bin/serverdash
Restart=on-failure
RestartSec=3
# Runs as root: it manages host services, processes and (if present) the
# Docker/Podman socket, with the same reach the container install has.

[Install]
WantedBy=multi-user.target
EOF
	systemctl daemon-reload
}

write_unit() {
	if [ "$RUNTIME" = native ]; then write_native_unit; return; fi
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

install_cli() {
	local bin_dir="${SERVERDASH_BIN_DIR:-/usr/local/bin}" conf_dir="${SERVERDASH_CONF_DIR:-/etc/serverdash}"
	mkdir -p "$bin_dir" "$conf_dir"
	{
		printf 'SERVERDASH_DIR=%q\n' "$INSTALL_DIR"
		printf 'COMPOSE_CMD=%q\n' "$COMPOSE_CMD"
		printf 'RUNTIME=%q\n' "$RUNTIME"
	} >"$conf_dir/install.conf"

	cat >"$bin_dir/serverdash" <<'SERVERDASH_CLI_EOF'
#!/usr/bin/env bash
# serverdash: manage a ServerDash install.
#
#   serverdash config            interactive settings editor
#   serverdash config set port 8080
#   serverdash status | start | stop | restart | logs | update
#
# Installed by the ServerDash installer. Run `serverdash help` for everything.

set -euo pipefail

CONF="${SERVERDASH_CONF:-/etc/serverdash/install.conf}"
if [ ! -r "$CONF" ]; then
	echo "serverdash: $CONF not found. Re-run the ServerDash installer." >&2
	exit 1
fi
# shellcheck disable=SC1090
. "$CONF"
ENV_FILE="$SERVERDASH_DIR/.env"
SERVICE=serverdash
ARGS=("$@")

# ───────────────────────────── ui ─────────────────────────────

ANIM=0
INTERACTIVE=0
if [ -t 1 ]; then ANIM=1; fi
if [ -t 0 ] && [ -t 1 ]; then INTERACTIVE=1; fi

RST="" BOLD="" DIM="" RED="" GREEN="" YELLOW="" GRAY="" AMBER=""
if [ "$ANIM" -eq 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != dumb ]; then
	RST=$'\033[0m' BOLD=$'\033[1m' DIM=$'\033[2m'
	RED=$'\033[38;5;203m' GREEN=$'\033[38;5;114m' YELLOW=$'\033[38;5;221m' GRAY=$'\033[38;5;245m'
	case "${COLORTERM:-}" in
		*truecolor*|*24bit*) AMBER=$'\033[38;2;255;176;32m' ;;
		*) AMBER=$'\033[38;5;214m' ;;
	esac
fi

UNICODE=0
case "${LC_ALL:-${LC_CTYPE:-${LANG:-}}}" in
	*[Uu][Tt][Ff]-8*|*[Uu][Tt][Ff]8*) UNICODE=1 ;;
esac
if [ "${TERM:-}" = linux ]; then UNICODE=0; fi

if [ "$UNICODE" -eq 1 ]; then
	G_OK="✔" G_FAIL="✖" G_WARN="▲" G_SEL="❯" G_DOT="●" G_TO="→"
	B_TL="╭" B_TR="╮" B_BL="╰" B_BR="╯" B_H="─" B_V="│"
	SPIN=("⠋" "⠙" "⠹" "⠸" "⠼" "⠴" "⠦" "⠧" "⠇" "⠏")
else
	G_OK="+" G_FAIL="x" G_WARN="!" G_SEL=">" G_DOT="*" G_TO="->"
	B_TL="+" B_TR="+" B_BL="+" B_BR="+" B_H="-" B_V="|"
	SPIN=("|" "/" "-" "\\")
fi

PROMPT=$'\001'"${AMBER}"$'\002'"${G_SEL} "$'\001'"${RST}"$'\002'

W="$(tput cols 2>/dev/null || true)"
[[ "$W" =~ ^[0-9]+$ ]] || W=80
if [ "$W" -gt 76 ]; then W=76; fi
if [ "$W" -lt 60 ]; then W=60; fi

BG_PID=""
MENU_RESULT=0
DIRTY=0

cursor_hide() { if [ "$ANIM" -eq 1 ]; then printf '\033[?25l'; fi; return 0; }
cursor_show() { if [ "$ANIM" -eq 1 ]; then printf '\033[?25h'; fi; return 0; }
cleanup() {
	cursor_show
	if [ -n "$BG_PID" ]; then kill "$BG_PID" 2>/dev/null || true; fi
	if [ "$INTERACTIVE" -eq 1 ]; then stty sane 2>/dev/null || true; fi
	return 0
}
on_signal() { cursor_show; printf '\n'; exit 130; }
trap cleanup EXIT
trap on_signal INT TERM

ok()   { printf '  %s%s%s %s\n' "$GREEN" "$G_OK" "$RST" "$1"; }
warn() { printf '  %s%s%s %s\n' "$YELLOW" "$G_WARN" "$RST" "$1"; }
note() { printf '    %s%s%s\n' "$DIM" "$1" "$RST"; }
die()  { printf '\n  %s%s%s %s\n\n' "$RED" "$G_FAIL" "$RST" "$1" >&2; exit 1; }

rep() { local s="" i; for ((i = 0; i < $2; i++)); do s+="$1"; done; printf '%s' "$s"; }
vlen() { local s; s="$(printf '%s' "$1" | sed $'s/\033\\[[0-9;?]*[a-zA-Z]//g')"; printf '%s' "${#s}"; }
strip_ansi() { sed $'s/\033\\[[0-9;?]*[a-zA-Z]//g'; }
erase_up() { printf '\033[%dA\033[J' "$1"; }
fmt_time() { if [ "$1" -ge 60 ]; then printf '%dm %02ds' $(($1 / 60)) $(($1 % 60)); else printf '%ds' "$1"; fi; }

header() {
	printf '\n  %s%s%s %s%s%s  %s%s%s\n\n' "$AMBER" "$G_DOT" "$RST" "$BOLD" "ServerDash" "$RST" "$GRAY" "$1" "$RST"
}

panel() { # panel TITLE ROW...
	local title=$1 inner=$((W - 4)) rest row pad
	shift
	rest=$((inner - 3 - $(vlen "$title")))
	if [ "$rest" -lt 1 ]; then rest=1; fi
	printf '  %s%s%s%s %s%s%s %s%s%s%s\n' "$AMBER" "$B_TL" "$B_H" "$RST" "$BOLD" "$title" "$RST" "$AMBER" "$(rep "$B_H" "$rest")" "$B_TR" "$RST"
	for row in "$@"; do
		pad=$((inner - 2 - $(vlen "$row")))
		if [ "$pad" -lt 0 ]; then pad=0; fi
		printf '  %s%s%s %s%*s %s%s%s\n' "$AMBER" "$B_V" "$RST" "$row" "$pad" "" "$AMBER" "$B_V" "$RST"
	done
	printf '  %s%s%s%s%s\n' "$AMBER" "$B_BL" "$(rep "$B_H" "$inner")" "$B_BR" "$RST"
}

kv() { printf '%s%-13s%s %s%s' "$GRAY" "$1" "$RST" "$2" "${3:+  $DIM$3$RST}"; }

# menu TITLE DEFAULT LABEL DESC ...   -> MENU_RESULT  (q picks the last item)
menu() {
	local title=$1 sel=$2 lw=${MENU_LW:-14}
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
				printf '\r\033[K  %s%s%s %s%-*s%s %s%s%s\n' "$AMBER" "$G_SEL" "$RST" "$BOLD" "$lw" "${labels[$i]}" "$RST" "$GRAY" "${descs[$i]}" "$RST"
			else
				printf '\r\033[K    %-*s %s%s%s\n' "$lw" "${labels[$i]}" "$DIM" "${descs[$i]}" "$RST"
			fi
		done
	}

	printf '  %s%s%s  %s↑/↓ move · enter select · q back%s\n' "$BOLD" "$title" "$RST" "$DIM" "$RST"
	cursor_hide
	_menu_draw
	while :; do
		key=""
		IFS= read -rsn1 key || true
		case "$key" in
			$'\033')
				rest=""
				IFS= read -rsn2 -t 0.05 rest || true
				case "$rest" in
					'[A') sel=$(((sel + n - 1) % n)) ;;
					'[B') sel=$(((sel + 1) % n)) ;;
				esac
				;;
			k) sel=$(((sel + n - 1) % n)) ;;
			j) sel=$(((sel + 1) % n)) ;;
			[1-9]) if [ "$((key - 1))" -lt "$n" ]; then sel=$((key - 1)); fi ;;
			q) sel=$((n - 1)); break ;;
			'') break ;;
		esac
		printf '\033[%dA' "$n"
		_menu_draw
	done
	erase_up $((n + 1))
	cursor_show
	MENU_RESULT=$sel
}

# ask KEY LABEL DEFAULT [HINT] -> REPLY   (validated against KEY, prefilled)
ask() {
	local key=$1 label=$2 def=$3 hint=${4:-} val err extra=0
	while :; do
		printf '  %s%s%s%s\n' "$BOLD" "$label" "$RST" "${hint:+  $DIM$hint$RST}"
		val=""
		IFS= read -r -e -i "$def" -p "$PROMPT" val || true
		val="${val#"${val%%[![:space:]]*}"}"
		val="${val%"${val##*[![:space:]]}"}"
		if err="$(validate "$key" "$val")"; then
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

# step LABEL COMMAND...   spinner around a command; shows the tail of its output on failure
step() {
	local label=$1 start=$SECONDS rc=0 i=0 log
	shift
	log="$(mktemp /tmp/serverdash-cli.XXXXXX)"
	"$@" >"$log" 2>&1 </dev/null &
	BG_PID=$!
	if [ "$ANIM" -eq 1 ]; then
		cursor_hide
		while kill -0 "$BG_PID" 2>/dev/null; do
			printf '\r\033[K  %s%s%s %s  %s%s%s' "$AMBER" "${SPIN[$((i % ${#SPIN[@]}))]}" "$RST" "$label" "$DIM" "$(fmt_time $((SECONDS - start)))" "$RST"
			i=$((i + 1))
			sleep 0.1
		done
		printf '\r\033[K'
		cursor_show
	fi
	wait "$BG_PID" || rc=$?
	BG_PID=""
	if [ "$rc" -eq 0 ]; then
		ok "$label"
		rm -f "$log"
		return 0
	fi
	printf '  %s%s%s %s\n\n' "$RED" "$G_FAIL" "$RST" "$label"
	tail -n 15 "$log" | strip_ansi | cut -c1-$((W - 6)) | sed "s/^/    ${DIM}/; s/\$/${RST}/"
	printf '\n'
	rm -f "$log"
	exit 1
}

# ───────────────────────────── settings ─────────────────────────────

v_port() {
	if [[ "$1" =~ ^[0-9]+$ ]] && [ "$1" -ge 1 ] && [ "$1" -le 65535 ]; then return 0; fi
	echo "Port must be a number from 1 to 65535"
	return 1
}
v_host() {
	if [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9._:-]*$ ]]; then return 0; fi
	echo "Enter a hostname like nova.example.com"
	return 1
}

validate() { # validate KEY VALUE -> prints an error and returns 1 if invalid
	case "$2" in *$'\n'*) echo "Value can't contain a newline"; return 1 ;; esac
	case "$1" in
		SERVERDASH_PORT) v_port "$2" ;;
		SERVERDASH_PUBLIC_HOST) v_host "$2" ;;
		*) return 0 ;;
	esac
}

norm_key() { # port -> SERVERDASH_PORT, public-host -> SERVERDASH_PUBLIC_HOST
	local k
	k="$(printf '%s' "${1//-/_}" | tr 'a-z' 'A-Z')"
	case "$k" in SERVERDASH_*) ;; *) k="SERVERDASH_$k" ;; esac
	printf '%s' "$k"
}

label_for() {
	case "$1" in
		SERVERDASH_PORT) echo "Port" ;;
		SERVERDASH_PUBLIC_HOST) echo "Public host" ;;
		*) echo "$1" ;;
	esac
}

env_get() {
	{ sed -n "s/^$1=//p" "$ENV_FILE" 2>/dev/null || true; } | tail -n 1 | sed -e 's/^"\(.*\)"$/\1/' -e 's/\r$//'
}

env_set() {
	local key=$1 val=$2 esc
	case "$val" in *[[:space:]]*) val="\"$val\"" ;; esac
	esc="$(printf '%s' "$val" | sed -e 's/[\\&|]/\\&/g')"
	touch "$ENV_FILE"
	if grep -q "^$key=" "$ENV_FILE"; then
		sed -i "s|^$key=.*|$key=$esc|" "$ENV_FILE"
	else
		if [ -s "$ENV_FILE" ] && [ -n "$(tail -c1 "$ENV_FILE")" ]; then echo >>"$ENV_FILE"; fi
		printf '%s=%s\n' "$key" "$val" >>"$ENV_FILE"
	fi
}

env_keys() { grep -E '^[A-Za-z_][A-Za-z0-9_]*=' "$ENV_FILE" 2>/dev/null | cut -d= -f1 || true; }

need_root() {
	if [ "$(id -u)" -ne 0 ]; then
		if command -v sudo >/dev/null 2>&1; then
			exec sudo -E "$(readlink -f "$0")" ${ARGS[@]+"${ARGS[@]}"}
		fi
		die "this needs root. Re-run with sudo."
	fi
}

service_active() { systemctl is-active --quiet "$SERVICE" 2>/dev/null; }

port_in_use() {
	command -v ss >/dev/null 2>&1 && ss -ltnH "sport = :$1" 2>/dev/null | grep -q .
}

host_ip() {
	local ip
	ip="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "src") { print $(i + 1); exit }}' || true)"
	if [ -z "$ip" ]; then ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"; fi
	printf '%s' "${ip:-<this-host>}"
}

show_url() {
	local port
	port="$(env_get SERVERDASH_PORT)"
	printf '    %sOpen%s  %shttp://%s:%s%s\n' "$GRAY" "$RST" "$AMBER" "$(host_ip)" "${port:-9900}" "$RST"
}

apply_changes() {
	if service_active; then
		step "Restarting ServerDash to apply changes" systemctl restart "$SERVICE"
		show_url
	else
		warn "ServerDash isn't running, so nothing was restarted"
		note "changes apply the next time it starts: serverdash start"
	fi
}

# ───────────────────────────── config ─────────────────────────────

config_list() {
	local key val
	printf '\n'
	while IFS= read -r key; do
		[ -n "$key" ] || continue
		val="$(env_get "$key")"
		printf '  %s%-28s%s %s\n' "$GRAY" "$key" "$RST" "$val"
	done < <(env_keys)
	printf '\n'
}

config_set() {
	local restart=1 key="" val="" have_val=0 a old err
	for a in "$@"; do
		case "$a" in
			--no-restart) restart=0 ;;
			*)
				if [ -z "$key" ]; then key=$a
				elif [ "$have_val" -eq 0 ]; then val=$a; have_val=1
				else die "too many arguments (quote values that contain spaces)"; fi
				;;
		esac
	done
	if [ -z "$key" ] || [ "$have_val" -eq 0 ]; then die "usage: serverdash config set KEY VALUE [--no-restart]"; fi
	need_root
	key="$(norm_key "$key")"
	err="$(validate "$key" "$val")" || die "$key: $err"
	old="$(env_get "$key")"
	if [ "$old" = "$val" ]; then
		ok "$key is already ${val:-empty}"
		return 0
	fi
	env_set "$key" "$val"
	printf '\n'
	ok "$key  ${DIM}${old:-empty} $G_TO ${val:-empty}${RST}"
	if [ "$restart" -eq 1 ]; then
		apply_changes
	else
		note "not restarted. Apply with: serverdash restart"
	fi
	printf '\n'
}

config_edit() {
	need_root
	local ed="${VISUAL:-${EDITOR:-}}"
	if [ -z "$ed" ]; then
		for ed in nano vim vi; do command -v "$ed" >/dev/null 2>&1 && break; done
	fi
	command -v "$ed" >/dev/null 2>&1 || die "no editor found. Set \$EDITOR."
	"$ed" "$ENV_FILE"
	DIRTY=1
}

edit_value() { # edit_value KEY -> sets DIRTY when the value changed
	local key=$1 cur new hint=""
	cur="$(env_get "$key")"
	if [ "$key" = SERVERDASH_PORT ]; then hint="host port for the dashboard"; fi
	if [ "$key" = SERVERDASH_PUBLIC_HOST ]; then hint="the domain you serve it from"; fi
	while :; do
		ask "$key" "$(label_for "$key")" "$cur" "$hint"
		new=$REPLY
		if [ "$key" = SERVERDASH_PORT ] && [ "$new" != "$cur" ] && port_in_use "$new"; then
			menu "Port $new is already in use" 1 "Use it anyway" "ServerDash may fail to start" "Pick another" "Choose a different port"
			if [ "$MENU_RESULT" -eq 1 ]; then cur=$new; continue; fi
		fi
		break
	done
	if [ "$new" != "$(env_get "$key")" ]; then
		env_set "$key" "$new"
		DIRTY=1
	fi
}

config_interactive() {
	if [ "$INTERACTIVE" -eq 0 ]; then die "not a terminal. Use: serverdash config set KEY VALUE"; fi
	need_root
	header "settings"
	local -a keys margs
	local k v n
	while :; do
		keys=(SERVERDASH_PORT SERVERDASH_PUBLIC_HOST)
		while IFS= read -r k; do
			[ -n "$k" ] || continue
			case "$k" in SERVERDASH_PORT|SERVERDASH_PUBLIC_HOST) continue ;; esac
			keys+=("$k")
		done < <(env_keys)

		margs=()
		for k in "${keys[@]}"; do
			v="$(env_get "$k")"
			margs+=("$(label_for "$k")" "${v:0:$((W - 36))}")
		done
		n=${#keys[@]}
		margs+=("Edit .env" "Open the file in your editor")
		margs+=("Apply" "$([ "$DIRTY" -eq 1 ] && echo 'Restart ServerDash to use your changes' || echo 'Restart ServerDash')")
		margs+=("Done" "Leave settings")

		MENU_LW=26 menu "Settings" 0 "${margs[@]}"
		if [ "$MENU_RESULT" -lt "$n" ]; then
			edit_value "${keys[$MENU_RESULT]}"
		elif [ "$MENU_RESULT" -eq "$n" ]; then
			config_edit
		elif [ "$MENU_RESULT" -eq $((n + 1)) ]; then
			apply_changes
			DIRTY=0
		else
			break
		fi
	done
	if [ "$DIRTY" -eq 1 ]; then
		menu "Restart now to apply your changes?" 0 "Restart" "Apply the new settings" "Later" "Keep running with the old settings"
		if [ "$MENU_RESULT" -eq 0 ]; then apply_changes; else note "apply later with: serverdash restart"; fi
	fi
	printf '\n'
}

cmd_config() {
	local sub=""
	if [ $# -gt 0 ]; then sub=$1; shift; fi
	case "$sub" in
		"") config_interactive ;;
		list|ls) config_list ;;
		get)
			if [ $# -lt 1 ]; then die "usage: serverdash config get KEY"; fi
			env_get "$(norm_key "$1")"
			;;
		set) config_set "$@" ;;
		edit) config_edit; apply_changes ;;
		*) die "unknown config command '$sub' (try: list, get, set, edit)" ;;
	esac
}

cmd_port() {
	if [ $# -eq 0 ]; then
		env_get SERVERDASH_PORT
	else
		config_set SERVERDASH_PORT "$@"
	fi
}

# ───────────────────────────── service control ─────────────────────────────

cmd_status() {
	local state version port host
	port="$(env_get SERVERDASH_PORT)"
	host="$(env_get SERVERDASH_PUBLIC_HOST)"
	if service_active; then
		state="${GREEN}${G_DOT} running${RST}"
	else
		state="${YELLOW}${G_DOT} stopped${RST}"
	fi
	version="$(git -C "$SERVERDASH_DIR" log -1 --format='%h  %cs' 2>/dev/null || echo unknown)"
	printf '\n'
	panel "ServerDash" \
		"$(kv "Status" "$state")" \
		"$(kv "Local URL" "http://$(host_ip):${port:-9900}")" \
		"$(kv "Directory" "$SERVERDASH_DIR")" \
		"$(kv "Runtime" "$(if [ "${RUNTIME:-}" = native ]; then echo "native (systemd)"; else echo "$COMPOSE_CMD"; fi)")" \
		"$(kv "Version" "$version")"
	printf '\n'
}

cmd_start()   { need_root; step "Starting ServerDash" systemctl start "$SERVICE"; show_url; }
cmd_stop()    { need_root; step "Stopping ServerDash" systemctl stop "$SERVICE"; }
cmd_restart() { need_root; step "Restarting ServerDash" systemctl restart "$SERVICE"; show_url; }

cmd_logs() {
	need_root
	if [ "${RUNTIME:-}" = native ]; then
		if [ $# -eq 0 ]; then set -- -f -n 100; fi
		exec journalctl -u "$SERVICE" "$@"
	fi
	cd "$SERVERDASH_DIR"
	if [ $# -eq 0 ]; then set -- -f --tail 100; fi
	# shellcheck disable=SC2086
	exec $COMPOSE_CMD logs "$@"
}

do_pull()  { git -C "$SERVERDASH_DIR" pull --ff-only; }
do_build() {
	if [ "${RUNTIME:-}" = native ]; then
		bash "$SERVERDASH_DIR/scripts/build-native.sh" "$SERVERDASH_DIR" && systemctl restart "$SERVICE"
		return
	fi
	cd "$SERVERDASH_DIR" && BUILDKIT_PROGRESS=plain $COMPOSE_CMD up -d --build
}

cmd_update() {
	need_root
	[ -d "$SERVERDASH_DIR/.git" ] || die "$SERVERDASH_DIR is not a git checkout, so it can't update itself"
	printf '\n'
	step "Fetching the latest ServerDash" do_pull
	step "Rebuilding and restarting" do_build
	show_url
	printf '\n'
}

cmd_setup_code() {
	need_root
	local code
	code="$(env_get SERVERDASH_SETUP_CODE)"
	if [ -z "$code" ]; then
		echo "No setup code is set: this install doesn't use browser setup, or it was removed."
		return 0
	fi
	printf '\n  Setup code: %s%s%s\n' "$BOLD" "$code" "$RST"
	note "enter it on the setup screen at http://$(host_ip):$(env_get SERVERDASH_PORT)"
	printf '\n'
}

cmd_version() {
	git -C "$SERVERDASH_DIR" log -1 --format='serverdash %h (%cs)' 2>/dev/null || echo "serverdash (unknown version)"
}

cmd_help() {
	cat <<EOF

  ${BOLD}serverdash${RST}  manage your ServerDash install

  ${AMBER}Settings${RST}
    serverdash config               edit settings interactively
    serverdash config list          show every setting
    serverdash config get KEY       print one setting
    serverdash config set KEY VAL   change a setting and restart  (--no-restart to skip)
    serverdash config edit          open .env in \$EDITOR
    serverdash port [NUMBER]        show or change the port

  ${AMBER}Service${RST}
    serverdash status               state, URLs and version
    serverdash start | stop | restart
    serverdash logs [ARGS]          follow logs (defaults to -f --tail 100)
    serverdash update               pull the latest code and rebuild
    serverdash version
    serverdash setup-code           show the browser setup code

  KEY can be short: ${DIM}port${RST}, ${DIM}public-host${RST}, or a full name like ${DIM}SERVERDASH_KUBECONFIG${RST}.

EOF
}

main() {
	local cmd=""
	if [ $# -gt 0 ]; then cmd=$1; shift; fi
	case "$cmd" in
		config|cfg|settings) cmd_config "$@" ;;
		port) cmd_port "$@" ;;
		status|st) cmd_status ;;
		start) cmd_start ;;
		stop) cmd_stop ;;
		restart) cmd_restart ;;
		logs|log) cmd_logs "$@" ;;
		update|upgrade) cmd_update ;;
		version|--version|-v) cmd_version ;;
		setup-code) cmd_setup_code ;;
		""|help|-h|--help) cmd_help ;;
		*) die "unknown command '$cmd'. Try: serverdash help" ;;
	esac
}

main "$@"
SERVERDASH_CLI_EOF
	chmod 755 "$bin_dir/serverdash"

	if [ -d /etc/bash_completion.d ]; then
		cat >/etc/bash_completion.d/serverdash <<'COMPLETION_EOF'
_serverdash() {
	local cur=${COMP_WORDS[COMP_CWORD]}
	if [ "$COMP_CWORD" -eq 1 ]; then
		COMPREPLY=($(compgen -W "config port status start stop restart logs update version help" -- "$cur"))
	elif [ "${COMP_WORDS[1]}" = config ] && [ "$COMP_CWORD" -eq 2 ]; then
		COMPREPLY=($(compgen -W "list get set edit" -- "$cur"))
	fi
}
complete -F _serverdash serverdash
COMPLETION_EOF
	fi
}

json_escape() {
	local v=$1
	v=${v//\\/\\\\}
	v=${v//\"/\\\"}
	printf '%s' "$v"
}

# Creates the first admin through the same one-time endpoint the setup screen uses.
# Writes the outcome (created | exists) to "$LOG.admin".
create_admin() {
	local url="http://127.0.0.1:$PORT" status="" code i
	command -v curl >/dev/null 2>&1 || { echo "curl is required to create the admin login"; return 1; }
	for ((i = 0; i < 30; i++)); do
		status="$(curl -s --max-time 3 "$url/api/setup/status" || true)"
		if grep -Eq '"needsSetup"[[:space:]]*:' <<<"$status"; then break; fi
		sleep 1
	done
	if grep -Eq '"needsSetup"[[:space:]]*:[[:space:]]*false' <<<"$status"; then
		echo "an admin account already exists; leaving it unchanged"
		echo exists >"$LOG.admin"
		return 0
	fi
	if ! grep -Eq '"needsSetup"[[:space:]]*:[[:space:]]*true' <<<"$status"; then
		echo "the dashboard did not answer at $url (got: ${status:-nothing})"
		return 1
	fi
	# The password goes in over stdin, never on the command line.
	code="$(printf '{"username":"%s","password":"%s"}' "$(json_escape "$ADMIN_USER")" "$(json_escape "$ADMIN_PASS")" |
		curl -s -o /dev/null -w '%{http_code}' --max-time 10 -X POST \
			-H 'Content-Type: application/json' --data-binary @- "$url/api/setup" || true)"
	if [ "$code" = 201 ]; then
		echo created >"$LOG.admin"
		return 0
	fi
	echo "setup endpoint answered HTTP ${code:-000}"
	return 1
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
		v="$(env_val SERVERDASH_PUBLIC_HOST)"; if [ -n "$v" ] && [ "$HOST_SET" -eq 0 ]; then PUBLIC_HOST=$v; fi
		v="$(env_val SERVERDASH_PORT)"; if [ -n "$v" ] && [ "$PORT_SET" -eq 0 ]; then PORT=$v; fi
	fi
}

# previous_install VAR: prints a value from an existing install's
# install.conf (written by install_cli), or nothing on a fresh host.
previous_install() {
	local conf="${SERVERDASH_CONF_DIR:-/etc/serverdash}/install.conf"
	[ -r "$conf" ] || return 0
	sed -n "s/^$1=//p" "$conf" | tail -n 1 | tr -d "'\""
}

default_runtime() {
	if [ "$RUNTIME_FLAG" -eq 1 ]; then return 0; fi
	local prev
	prev="$(previous_install RUNTIME)"
	# An upgrade keeps whatever the existing install runs on.
	if [ -n "$prev" ]; then
		RUNTIME=$prev
	elif docker_ready; then
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
	if [ "$RUNTIME" = native ]; then idx=2; fi
	menu "How should ServerDash run?" "$idx" \
		"Docker" "In a container, official install script ${d_tag:+· $d_tag}" \
		"Podman" "In a container, daemonless ${p_tag:+· $p_tag}" \
		"Native" "Directly on this host, no containers needed"
	case "$MENU_RESULT" in
		0) RUNTIME=docker ;;
		1) RUNTIME=podman ;;
		*) RUNTIME=native ;;
	esac
}

print_settings() {
	local mode_label dir_tag="" env_tag="" host_tag="" port_tag="" admin_text="" admin_tag=""
	case "$MODE" in
		local) mode_label="Use this checkout"; dir_tag="(this checkout)" ;;
		update) mode_label="Upgrade existing install" ;;
		clone) mode_label="Fresh install" ;;
	esac
	if [ "$ENV_EXISTS" -eq 1 ]; then env_tag="(from existing .env)"; fi
	host_tag=$env_tag port_tag=$env_tag
	case "$ADMIN_MODE" in
		generate) admin_text="$ADMIN_USER  ${DIM}+ generated password${RST}" ;;
		custom)   admin_text="$ADMIN_USER  ${DIM}+ the password you set${RST}" ;;
		skip)
			if [ "$SETUP_MODE" = web ]; then
				admin_text="${DIM}created in the browser setup${RST}"
			else
				admin_text="${DIM}none, use the browser setup screen${RST}"
			fi
			;;
	esac
	if [ "$ADMIN_MODE" != skip ] && [ "$START_SERVICE" -eq 0 ]; then
		admin_tag="(needs the service started)"
	elif [ "$ADMIN_MODE" != skip ] && [ "$ENV_EXISTS" -eq 1 ]; then
		admin_tag="(skipped if one exists)"
	fi
	if [ "$ENV_EXISTS" -eq 1 ] && [ "$HOST_SET" -eq 1 ]; then host_tag="(will update .env)"; fi
	if [ "$ENV_EXISTS" -eq 1 ] && [ "$PORT_SET" -eq 1 ]; then port_tag="(will update .env)"; fi
	panel "Install settings" \
		"$(kv "Mode" "$mode_label")" \
		"$(kv "Runtime" "$RUNTIME")" \
		"$(kv "Directory" "$INSTALL_DIR" "$dir_tag")" \
		"$(kv "Public host" "$PUBLIC_HOST" "$host_tag")" \
		"$(kv "Port" "$PORT" "$port_tag")" \
		"$(kv "Branch" "$BRANCH")" \
		"$(kv "Admin login" "$admin_text" "$admin_tag")" \
		"$(kv "Service" "$([ "$START_SERVICE" -eq 1 ] && echo 'enable and start on boot' || echo 'install only, do not start')")"
}

edit_admin() {
	local idx=0
	case "$ADMIN_MODE" in custom) idx=1 ;; skip) idx=2 ;; esac
	menu "Admin login" "$idx" \
		"Generate" "Random password, shown once when the install finishes" \
		"Choose" "Pick your own username and password" \
		"Skip" "Create it in the browser instead"
	case "$MENU_RESULT" in
		0)
			ADMIN_MODE=generate
			ask "Admin username" "$ADMIN_USER" v_user "3 to 32 characters"
			ADMIN_USER=$REPLY
			ADMIN_PASS=""
			;;
		1)
			ask "Admin username" "$ADMIN_USER" v_user "3 to 32 characters"
			ADMIN_USER=$REPLY
			ask_secret "Admin password (8+ characters)"
			ADMIN_PASS=$REPLY
			ADMIN_MODE=custom
			;;
		2) ADMIN_MODE=skip; ADMIN_PASS="" ;;
	esac
}

# pick_web_port: ask for the dashboard port, warning if something already listens on it.
pick_web_port() {
	while :; do
		edit_port
		if command -v ss >/dev/null 2>&1 && ss -Hltn "sport = :$PORT" 2>/dev/null | grep -q .; then
			printf '  %s%s%s Port %s is already in use on this machine. Pick another.\n' "$YELLOW" "$G_WARN" "$RST" "$PORT"
			continue
		fi
		break
	done
}

edit_port() {
	ask "Port" "$PORT" v_port "host port for the dashboard"
	if [ "$REPLY" != "$PORT" ]; then PORT=$REPLY; PORT_SET=1; fi
}

edit_settings() {
	if [ "$MODE" != local ]; then
		ask "Install directory" "$INSTALL_DIR" v_dir "where ServerDash will live"
		INSTALL_DIR=$REPLY
		resolve_source
	fi
	ask "Public host" "$PUBLIC_HOST" v_host "the domain you'll serve it from"
	if [ "$REPLY" != "$PUBLIC_HOST" ]; then PUBLIC_HOST=$REPLY; HOST_SET=1; fi
	edit_port
	ask "Branch or tag" "$BRANCH" v_branch "what to install"
	BRANCH=$REPLY
	choose_runtime
	edit_admin
	menu "Service" "$([ "$START_SERVICE" -eq 1 ] && echo 0 || echo 1)" \
		"Start" "Enable on boot and start now" \
		"Install only" "Build everything, start it yourself later"
	if [ "$MENU_RESULT" -eq 0 ]; then START_SERVICE=1; else START_SERVICE=0; fi
}

# enable_web_setup: the browser handles the admin account (protected by a
# one-time setup code), branding, and cluster role.
enable_web_setup() {
	SETUP_MODE=web
	ADMIN_MODE=skip
	START_SERVICE=1
	if [ -z "$SETUP_CODE" ]; then
		SETUP_CODE="$(gen_password | cut -d- -f1-2)"
	fi
}

review_settings() {
	if [ "$PROMPTS" -eq 0 ]; then
		print_settings
		return 0
	fi
	section "Setup"
	if [ "$SETUP_MODE" != web ] && [ "$MODE" != update ]; then
		menu "How would you like to set up ServerDash?" 0 \
			"In the terminal" "Answer a few questions here" \
			"In your browser" "Install with defaults, then finish on a web page"
		if [ "$MENU_RESULT" -eq 1 ]; then enable_web_setup; fi
	fi
	if [ "$RUNTIME_FLAG" -eq 0 ]; then choose_runtime; fi
	if [ "$SETUP_MODE" = web ]; then
		# The port has to be known before the browser setup screen can be opened.
		if [ "$PORT_SET" -eq 0 ]; then pick_web_port; fi
		print_settings
		return 0
	fi
	while :; do
		print_settings
		printf '\n'
		menu "Ready to install?" 0 \
			"Install" "Go ahead with these settings" \
			"Admin login" "Choose the username and password" \
			"Port" "Change just the port (now $PORT)" \
			"Customize" "Directory, host, branch, runtime…" \
			"Cancel" "Exit without changing anything"
		case "$MENU_RESULT" in
			0) return 0 ;;
			1) erase_up $((PANEL_LINES + 1)); edit_admin ;;
			2) erase_up $((PANEL_LINES + 1)); edit_port ;;
			3) erase_up $((PANEL_LINES + 1)); edit_settings ;;
			4) on_signal ;;
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

	if [ "$RUNTIME" = native ]; then
		if command -v git >/dev/null 2>&1 && command -v curl >/dev/null 2>&1 && command -v xz >/dev/null 2>&1; then
			ok "git, curl and xz ready"
		else
			run_step "Installing git, curl and xz" install_git
		fi
		COMPOSE_CMD="native"
		local prev_compose
		prev_compose="$(previous_install COMPOSE_CMD)"
		if [ -n "$prev_compose" ] && [ "$prev_compose" != native ] && [ -f "$INSTALL_DIR/docker-compose.yml" ]; then
			# Switching from the container install: free the port first.
			# The serverdash-data volume is left alone.
			STEP_SOFT=1 run_step "Stopping the old ServerDash container" stop_container_install "$prev_compose"
		fi
		if docker_ready || podman_ready; then
			note "a container runtime is installed too; ServerDash will still manage its containers"
		fi
	elif [ "$RUNTIME" = docker ]; then
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

	if [ "$RUNTIME" = native ]; then
		run_step "Building ServerDash (a few minutes the first time)" build_image
	else
		run_step "Building the image (a few minutes the first time)" build_image
	fi
	run_step "Installing the systemd service" write_unit
	run_step "Installing the serverdash command" install_cli

	if [ "$START_SERVICE" -eq 1 ]; then
		run_step "Starting ServerDash" start_service
		STEP_SOFT=1 run_step "Waiting for the dashboard to respond" wait_healthy
		if [ "$ADMIN_MODE" != skip ]; then
			if [ "$ADMIN_MODE" = generate ]; then ADMIN_PASS="$(gen_password)"; fi
			rm -f "$LOG.admin"
			STEP_SOFT=1 run_step "Creating the admin login" create_admin
			if [ -f "$LOG.admin" ]; then ADMIN_RESULT="$(cat "$LOG.admin")"; else ADMIN_RESULT=failed; fi
			rm -f "$LOG.admin"
			if [ "$ADMIN_RESULT" = failed ]; then
				note "couldn't create it. Open the dashboard and use the setup screen instead"
			fi
		fi
	else
		warn "Not starting the service ${DIM}(install only)${RST}"
	fi
}

summary() {
	local ip title took
	ip="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "src") { print $(i + 1); exit }}' || true)"
	if [ -z "$ip" ]; then ip="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"; fi
	if [ -z "$ip" ]; then ip="<this-host>"; fi
	took="$(fmt_time $((SECONDS - T0)))"
	if [ "$RUNTIME" = native ]; then
		RUNTIME_LABEL="native (systemd)"
		DATA_LABEL="$NATIVE_DATA_DIR, kept on upgrade"
	else
		RUNTIME_LABEL="$COMPOSE_CMD"
		DATA_LABEL="serverdash-data volume, kept on upgrade"
	fi

	if [ "$START_SERVICE" -eq 1 ]; then
		case "$MODE" in update) title="ServerDash upgraded" ;; *) title="ServerDash is running" ;; esac
	else
		title="ServerDash is installed"
	fi

	printf '\n'
	if [ "$SETUP_MODE" = web ] && [ "$START_SERVICE" -eq 1 ]; then
		panel "Finish setup in your browser" \
			"$(kv "Open" "${AMBER}http://$ip:$PORT${RST}")" \
			"$(kv "Setup code" "${BOLD}${SETUP_CODE}${RST}")" \
			"$(kv "Directory" "$INSTALL_DIR")" \
			"$(kv "Runtime" "$RUNTIME_LABEL")" \
			"$(kv "Data" "$DATA_LABEL")"
		printf '\n  %s%s%s Open the address above and enter the setup code to create your admin account,\n' "$YELLOW" "$G_WARN" "$RST"
		note "pick your branding, and choose whether this server joins a cluster."
		note "lost the code? run: sudo serverdash setup-code"
	elif [ "$START_SERVICE" -eq 1 ] && [ "$ADMIN_RESULT" = created ]; then
		local pw_row
		if [ "$ADMIN_MODE" = generate ]; then
			pw_row="$(kv "Password" "${BOLD}${ADMIN_PASS}${RST}")"
		else
			pw_row="$(kv "Password" "${DIM}the one you chose${RST}")"
		fi
		panel "$title" \
			"$(kv "Open" "${AMBER}http://$ip:$PORT${RST}")" \
			"$(kv "Username" "${BOLD}${ADMIN_USER}${RST}")" \
			"$pw_row" \
			"$(kv "Directory" "$INSTALL_DIR")" \
			"$(kv "Runtime" "$RUNTIME_LABEL")" \
			"$(kv "Manage" "${AMBER}serverdash config${RST}" "(port, host, …)")" \
			"$(kv "Data" "$DATA_LABEL")"
	elif [ "$START_SERVICE" -eq 1 ]; then
		panel "$title" \
			"$(kv "Open" "${AMBER}http://$ip:$PORT${RST}")" \
			"$(kv "Directory" "$INSTALL_DIR")" \
			"$(kv "Runtime" "$RUNTIME_LABEL")" \
			"$(kv "Manage" "${AMBER}serverdash config${RST}" "(port, host, …)")" \
			"$(kv "Data" "$DATA_LABEL")"
	else
		panel "$title" \
			"$(kv "Start it" "${AMBER}systemctl start serverdash${RST}")" \
			"$(kv "Directory" "$INSTALL_DIR")" \
			"$(kv "Runtime" "$RUNTIME_LABEL")" \
			"$(kv "Data" "$DATA_LABEL")"
	fi

	if [ "$ADMIN_RESULT" = created ] && [ "$ADMIN_MODE" = generate ]; then
		printf '\n  %s%s%s Save that password now. It is not stored anywhere else.\n' "$YELLOW" "$G_WARN" "$RST"
		note "change it later under Admin → Users in the dashboard"
	fi
	if [ "$START_SERVICE" -eq 1 ] && [ "$MODE" != update ] && [ "$SETUP_MODE" != web ]; then
		printf '\n  %sNext%s\n' "$BOLD" "$RST"
		if [ "$ADMIN_RESULT" = created ]; then
			printf '    %s1.%s Open the URL above and sign in as %s\n' "$AMBER" "$RST" "$ADMIN_USER"
		elif [ "$ADMIN_RESULT" = exists ]; then
			printf '    %s1.%s Open the URL above and sign in with your existing admin login\n' "$AMBER" "$RST"
		else
			printf '    %s1.%s Open the URL above and create the admin account\n' "$AMBER" "$RST"
		fi
		printf '    %s2.%s Put a reverse proxy in front of it for https://%s\n' "$AMBER" "$RST" "$PUBLIC_HOST"
	fi

	printf '\n  %sUseful commands%s\n' "$BOLD" "$RST"
	printf '    %sserverdash config%s        change the port, public host and more\n' "$AMBER" "$RST"
	printf '    %sserverdash port 8080%s     change just the port\n' "$AMBER" "$RST"
	printf '    %sserverdash status%s        state, URLs and version\n' "$AMBER" "$RST"
	printf '    %sserverdash logs%s          follow the logs\n' "$AMBER" "$RST"
	printf '    %sserverdash update%s        pull the latest code and rebuild\n' "$AMBER" "$RST"
	printf '\n  %s%s%s Done in %s\n\n' "$GREEN" "$G_OK" "$RST" "$took"
	rm -f "$LOG"
}

parse_args() {
	while [ $# -gt 0 ]; do
		case "$1" in
			--runtime)     [ $# -ge 2 ] || die "--runtime needs a value"; RUNTIME=$2; RUNTIME_FLAG=1; shift 2 ;;
			--dir)         [ $# -ge 2 ] || die "--dir needs a value"; INSTALL_DIR=$2; shift 2 ;;
			--public-host) [ $# -ge 2 ] || die "--public-host needs a value"; PUBLIC_HOST=$2; HOST_SET=1; shift 2 ;;
			--port)        [ $# -ge 2 ] || die "--port needs a value"; PORT=$2; PORT_SET=1; shift 2 ;;
			--branch)      [ $# -ge 2 ] || die "--branch needs a value"; BRANCH=$2; shift 2 ;;
			--admin-user)  [ $# -ge 2 ] || die "--admin-user needs a value"; ADMIN_USER=$2; shift 2 ;;
			--admin-password) [ $# -ge 2 ] || die "--admin-password needs a value"; ADMIN_PASS=$2; ADMIN_MODE=custom; shift 2 ;;
			--no-admin)    ADMIN_MODE=skip; shift ;;
			--web-setup)   WEB_SETUP_FLAG=1; shift ;;
			--no-start)    START_SERVICE=0; shift ;;
			-y|--yes)      ASSUME_YES=1; shift ;;
			-h|--help)     usage; exit 0 ;;
			*)             die "unknown option: $1 (see --help)" ;;
		esac
	done

	case "$RUNTIME" in
		""|docker|podman|native) ;;
		*) die "--runtime must be 'docker', 'podman' or 'native', got '$RUNTIME'" ;;
	esac
	local err
	err="$(v_dir "$INSTALL_DIR")" || die "--dir: $err"
	err="$(v_host "$PUBLIC_HOST")" || die "--public-host: $err"
	err="$(v_port "$PORT")" || die "--port: $err"
	err="$(v_branch "$BRANCH")" || die "--branch: $err"
	err="$(v_user "$ADMIN_USER")" || die "--admin-user: $err"
	if [ "$ADMIN_MODE" = custom ]; then err="$(v_password "$ADMIN_PASS")" || die "--admin-password: $err"; fi
}

main() {
	ORIG_ARGS=("$@")
	T0=$SECONDS
	parse_args "$@"
	if [ "${WEB_SETUP_FLAG:-0}" -eq 1 ]; then enable_web_setup; fi
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