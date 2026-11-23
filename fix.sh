#!/usr/bin/env bash
set -euo pipefail

CONF="${SERVERDASH_CONF:-/etc/serverdash/install.conf}"
SERVICE=serverdash
RST="" BOLD="" DIM="" RED="" GREEN="" YELLOW="" GRAY="" AMBER=""
G_OK="+" G_FAIL="x" G_WARN="!" G_DOT="*" G_TO="->"
ANIM=0
INTERACTIVE=0
W=76
MENU_RESULT=0
NEEDS_RESTART=0
LOG="$(mktemp /tmp/serverdash-fix.XXXXXX)"
DIR=""
BIN=""
RUNTIME="docker"
COMPOSE_CMD="docker compose"
DATA_DIR="/var/lib/serverdash"
PORT=""
HOST=""
SELINUX_ACTIVE=0
SELINUX_STATE="Disabled"
declare -a ISSUE_IDS=() ISSUE_LABELS=() ISSUE_DETAILS=() ISSUE_FIXES=()

need_root() {
	if [ "$(id -u)" -eq 0 ]; then return 0; fi
	if command -v sudo >/dev/null 2>&1; then
		exec sudo -E "$(readlink -f "$0")" "$@"
	fi
	echo "fix.sh: run this as root" >&2
	exit 1
}

init_ui() {
	if [ -t 1 ]; then ANIM=1; fi
	if [ -t 0 ] && [ -t 1 ]; then INTERACTIVE=1; fi
	if [ "$ANIM" -eq 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != dumb ]; then
		RST=$'\033[0m' BOLD=$'\033[1m' DIM=$'\033[2m'
		RED=$'\033[38;5;203m' GREEN=$'\033[38;5;114m' YELLOW=$'\033[38;5;221m' GRAY=$'\033[38;5;245m'
		AMBER=$'\033[38;5;214m'
	fi
	UNICODE=0
	case "${LC_ALL:-${LC_CTYPE:-${LANG:-}}}" in
		*[Uu][Tt][Ff]-8*|*[Uu][Tt][Ff]8*) UNICODE=1 ;;
	esac
	if [ "${TERM:-}" = linux ]; then UNICODE=0; fi
	if [ "$UNICODE" -eq 1 ]; then
		G_OK="✔" G_FAIL="✖" G_WARN="▲" G_DOT="●" G_TO="→"
		B_TL="╭" B_TR="╮" B_BL="╰" B_BR="╯" B_H="─" B_V="│"
	else
		G_OK="+" G_FAIL="x" G_WARN="!" G_DOT="*" G_TO="->"
		B_TL="+" B_TR="+" B_BL="+" B_BR="+" B_H="-" B_V="|"
	fi
	local cols
	cols="$(tput cols 2>/dev/null || true)"
	[[ "$cols" =~ ^[0-9]+$ ]] || cols=76
	[ "$cols" -gt 76 ] && cols=76
	[ "$cols" -lt 60 ] && cols=60
	W=$cols
}

cursor_hide() { if [ "$ANIM" -eq 1 ]; then printf '\033[?25l'; fi; return 0; }
cursor_show() { if [ "$ANIM" -eq 1 ]; then printf '\033[?25h'; fi; return 0; }
cleanup() { cursor_show; rm -f "$LOG"; return 0; }
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
header() { printf '\n  %s%s%s %s%s%s  %s%s%s\n\n' "$AMBER" "$G_DOT" "$RST" "$BOLD" "ServerDash Doctor" "$RST" "$GRAY" "$1" "$RST"; }
section() { printf '\n  %s%s%s\n' "$BOLD" "$1" "$RST"; }

panel() {
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

menu() {
	local title=$1 sel=$2 lw=${MENU_LW:-16}
	shift 2
	local -a labels=() descs=()
	local n i key rest
	while [ $# -gt 0 ]; do
		labels+=("$1")
		descs+=("${2:-}")
		shift 2
	done
	n=${#labels[@]}
	[ "$n" -gt 0 ] || return 0
	_draw_menu() {
		for ((i = 0; i < n; i++)); do
			if [ "$i" -eq "$sel" ]; then
				printf '\r\033[K  %s%s%s %s%-*s%s %s%s%s\n' "$AMBER" "$G_DOT" "$RST" "$BOLD" "$lw" "${labels[$i]}" "$RST" "$GRAY" "${descs[$i]}" "$RST"
			else
				printf '\r\033[K    %-*s %s%s%s\n' "$lw" "${labels[$i]}" "$DIM" "${descs[$i]}" "$RST"
			fi
		done
	}
	printf '  %s%s%s  %s↑/↓ move · enter select · q back%s\n' "$BOLD" "$title" "$RST" "$DIM" "$RST"
	cursor_hide
	_draw_menu
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
		_draw_menu
	done
	erase_up $((n + 1))
	cursor_show
	MENU_RESULT=$sel
}

step() {
	local label=$1 fn=$2 rc=0
	if [ "$ANIM" -eq 1 ]; then
		printf '  %s%s%s %s... ' "$AMBER" "$G_DOT" "$RST" "$label"
	else
		printf '  %s... ' "$label"
	fi
	: >"$LOG"
	if "$fn" >>"$LOG" 2>&1; then
		printf '%s%s%s\n' "$GREEN" "$G_OK" "$RST"
		return 0
	fi
	rc=$?
	printf '%s%s%s\n' "$RED" "$G_FAIL" "$RST"
	tail -n 8 "$LOG" | strip_ansi | cut -c1-$((W - 6)) | sed "s/^/    /"
	return "$rc"
}

env_get() {
	sed -n "s/^$1=//p" "$DIR/.env" 2>/dev/null | tail -n 1 | sed -e 's/^"\(.*\)"$/\1/' -e 's/\r$//'
}

load_config() {
	local script_dir
	script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
	if [ -r "$CONF" ]; then
		. "$CONF"
	fi
	DIR="${SERVERDASH_DIR:-$script_dir}"
	BIN="$DIR/bin/serverdash"
	RUNTIME="${RUNTIME:-docker}"
	COMPOSE_CMD="${COMPOSE_CMD:-docker compose}"
	DATA_DIR="${NATIVE_DATA_DIR:-/var/lib/serverdash}"
	PORT="$(env_get SERVERDASH_PORT)"
	HOST="$(env_get SERVERDASH_PUBLIC_HOST)"
	SELINUX_STATE="$(getenforce 2>/dev/null || echo Disabled)"
	case "$SELINUX_STATE" in
		Enforcing|Permissive) SELINUX_ACTIVE=1 ;;
		*) SELINUX_ACTIVE=0 ;;
	esac
}

reset_issues() {
	ISSUE_IDS=()
	ISSUE_LABELS=()
	ISSUE_DETAILS=()
	ISSUE_FIXES=()
}

has_issue() {
	local id
	for id in "${ISSUE_IDS[@]}"; do
		[ "$id" = "$1" ] && return 0
	done
	return 1
}

add_issue() {
	has_issue "$1" && return 0
	ISSUE_IDS+=("$1")
	ISSUE_LABELS+=("$2")
	ISSUE_DETAILS+=("$3")
	ISSUE_FIXES+=("$4")
}

issue_rows() {
	local i
	for ((i = 0; i < ${#ISSUE_IDS[@]}; i++)); do
		printf '%2d  %-22s %s\n' "$((i + 1))" "${ISSUE_LABELS[$i]}" "${ISSUE_DETAILS[$i]}"
	done
}

fix_selinux() {
	command -v restorecon >/dev/null 2>&1 || { echo "restorecon is required" >&2; return 1; }
	local parent ctx rc=0 all bin exe
	parent="$(dirname "$DIR")"
	if [ "$parent" != / ] && [ -d "$parent" ]; then
		restorecon -F "$parent"
	fi
	restorecon -RF "$DIR"
	case "$DIR" in
		/home/*|/root/*)
			all="$DIR(/.*)?"
			bin="$DIR/bin(/.*)?"
			exe="$DIR/bin/serverdash"
			if command -v semanage >/dev/null 2>&1; then
				semanage fcontext -a -t usr_t "$all" 2>/dev/null || semanage fcontext -m -t usr_t "$all" 2>/dev/null || rc=1
				semanage fcontext -a -t bin_t "$bin" 2>/dev/null || semanage fcontext -m -t bin_t "$bin" 2>/dev/null || rc=1
				semanage fcontext -a -t bin_t "$exe" 2>/dev/null || semanage fcontext -m -t bin_t "$exe" 2>/dev/null || rc=1
				if [ "$rc" -eq 0 ]; then
					restorecon -RF "$DIR"
					ctx="$(stat -c '%C' "$DIR/.env" 2>/dev/null || true)"
					case "$ctx" in *user_home_t*|*container_file_t*) rc=1 ;; esac
				fi
			fi
			;;
		*) rc=1 ;;
	esac
	if [ "$rc" -ne 0 ]; then
		echo "semanage rules unavailable; using chcon labels"
		chcon -R -t usr_t "$DIR"
		if [ -d "$DIR/bin" ]; then chcon -R -t bin_t "$DIR/bin"; fi
	fi
	if [ -d "$DATA_DIR" ]; then restorecon -F "$DATA_DIR" || true; fi
	NEEDS_RESTART=1
}

fix_perm() {
	if [ -f "$BIN" ]; then chmod 755 "$BIN"; fi
	if [ ! -d "$DATA_DIR" ]; then mkdir -p "$DATA_DIR"; fi
	chmod 700 "$DATA_DIR" 2>/dev/null || true
	NEEDS_RESTART=1
}

fix_restart() {
	systemctl reset-failed "$SERVICE" 2>/dev/null || true
	systemctl restart "$SERVICE"
	NEEDS_RESTART=0
}

fix_compose() {
	cd "$DIR"
	$COMPOSE_CMD up -d
}

fix_rebuild() {
	[ -f "$DIR/scripts/build-native.sh" ] || { echo "build script not found" >&2; return 1; }
	bash "$DIR/scripts/build-native.sh" "$DIR"
	NEEDS_RESTART=1
}

fix_config() {
	mkdir -p "$(dirname "$CONF")"
	{
		printf 'SERVERDASH_DIR=%q\n' "$DIR"
		printf 'COMPOSE_CMD=%q\n' "$COMPOSE_CMD"
		printf 'RUNTIME=%q\n' "$RUNTIME"
		printf 'UNIT_DIR=%q\n' /etc/systemd/system
		printf 'NATIVE_DATA_DIR=%q\n' "$DATA_DIR"
		printf 'BIN_DIR=%q\n' /usr/local/bin
		printf 'CONF_DIR=%q\n' "$(dirname "$CONF")"
	} >"$CONF"
}

check_config() {
	if [ ! -r "$CONF" ]; then
		add_issue config "Install config missing" "$CONF" fix_config
		warn "install config is missing"
	else
		ok "install config"
	fi
}

check_selinux() {
	if [ "$SELINUX_ACTIVE" -ne 1 ]; then
		ok "SELinux not active"
		return 0
	fi
	local f ctx bad="" parent
	parent="$(dirname "$DIR")"
	for f in "$parent" "$DIR" "$DIR/.env" "$BIN" "$DIR/web/dist/index.html" "$DIR/bin" "$DIR/plugins" "$DIR/plugins/docker-disk/plugin.json"; do
		[ -e "$f" ] || continue
		ctx="$(stat -c '%C' "$f" 2>/dev/null || true)"
		case "$ctx" in
			*container_file_t*) bad="container_file_t" ;;
			*user_home_t*) bad="user_home_t" ;;
		esac
	done
	if [ -n "$bad" ]; then
		add_issue selinux "SELinux labels" "files labeled $bad" fix_selinux
		warn "SELinux labels are wrong ($bad)"
	else
		ok "SELinux labels"
	fi
}

check_files() {
	if [ "$RUNTIME" != native ]; then
		ok "install files"
		return 0
	fi
	if [ ! -f "$BIN" ] || [ ! -f "$DIR/web/dist/index.html" ]; then
		add_issue build "Build artifacts missing" "run scripts/build-native.sh" fix_rebuild
		warn "build artifacts are missing"
	else
		ok "build artifacts"
	fi
	if [ ! -x "$BIN" ] || [ ! -d "$DATA_DIR" ]; then
		add_issue perm "Permissions" "binary or data dir needs fixing" fix_perm
		warn "permissions need attention"
	else
		ok "permissions"
	fi
}

check_service() {
	local state restarts
	state="$(systemctl is-active "$SERVICE" 2>/dev/null || true)"
	if [ "$state" != active ]; then
		add_issue service "Service is $state" "systemctl restart $SERVICE" fix_restart
		warn "service is $state"
		return 0
	fi
	restarts="$(systemctl show "$SERVICE" -p NRestarts --value 2>/dev/null || echo 0)"
	if [[ "$restarts" =~ ^[0-9]+$ ]] && [ "$restarts" -gt 0 ]; then
		add_issue service "Service restart loop" "$restarts restarts recorded" fix_restart
		warn "service has restarted $restarts times"
		return 0
	fi
	if [ "$RUNTIME" != native ]; then
		if ! (cd "$DIR" && $COMPOSE_CMD ps 2>/dev/null | grep -qi up); then
			add_issue container "Container not running" "$COMPOSE_CMD up -d" fix_compose
			warn "container is not running"
			return 0
		fi
	fi
	ok "service"
}

check_env() {
	if [ "$RUNTIME" != native ] || has_issue service; then return 0; fi
	local pid run_port run_host
	pid="$(systemctl show "$SERVICE" -p MainPID --value 2>/dev/null || echo 0)"
	if [[ ! "$pid" =~ ^[0-9]+$ ]] || [ "$pid" -lt 1 ] || [ ! -r "/proc/$pid/environ" ]; then
		return 0
	fi
	run_port="$(tr '\0' '\n' <"/proc/$pid/environ" 2>/dev/null | sed -n 's/^SERVERDASH_PORT=//p' | tail -n 1)"
	run_host="$(tr '\0' '\n' <"/proc/$pid/environ" 2>/dev/null | sed -n 's/^SERVERDASH_PUBLIC_HOST=//p' | tail -n 1)"
	if { [ -n "$PORT" ] && [ "$run_port" != "$PORT" ]; } || { [ -n "$HOST" ] && [ "$run_host" != "$HOST" ]; }; then
		add_issue env "Service ignoring .env" "running ${run_port:-?}/${run_host:-?}, expected ${PORT:-?}/${HOST:-?}" fix_restart
		warn "service is not using .env"
	else
		ok "service environment"
	fi
}

run_checks() {
	section "Checking"
	check_config
	check_selinux
	check_files
	check_service
	check_env
}

apply_one() {
	local i=$1
	if step "Fix: ${ISSUE_LABELS[$i]}" "${ISSUE_FIXES[$i]}"; then
		return 0
	fi
	warn "could not apply ${ISSUE_LABELS[$i]}"
	return 1
}

finish_restart() {
	if [ "$NEEDS_RESTART" -eq 1 ] && [ "$RUNTIME" = native ]; then
		step "Restarting ServerDash" fix_restart || warn "restart failed; check: journalctl -u $SERVICE -n 50 --no-pager"
	fi
}

fix_all() {
	local i
	for ((i = 0; i < ${#ISSUE_IDS[@]}; i++)); do
		apply_one "$i" || true
	done
	finish_restart
}

choose_fixes() {
	local -a chosen=()
	local -a margs=()
	local i mark
	while :; do
		margs=()
		for ((i = 0; i < ${#ISSUE_IDS[@]}; i++)); do
			mark=" "
			if [ "${chosen[$i]:-0}" = 1 ]; then mark="${GREEN}${G_DOT}${RST}"; fi
			margs+=("${mark} ${ISSUE_LABELS[$i]}" "${ISSUE_DETAILS[$i]}")
		done
		margs+=("Apply" "Fix the selected issues")
		MENU_LW=26 menu "Select fixes" 0 "${margs[@]}"
		if [ "$MENU_RESULT" -lt "${#ISSUE_IDS[@]}" ]; then
			if [ "${chosen[$MENU_RESULT]:-0}" = 1 ]; then
				chosen[$MENU_RESULT]=0
			else
				chosen[$MENU_RESULT]=1
			fi
			continue
		fi
		break
	done
	for ((i = 0; i < ${#chosen[@]}; i++)); do
		if [ "${chosen[$i]:-0}" = 1 ]; then apply_one "$i" || true; fi
	done
	finish_restart
}

main() {
	need_root "$@"
	load_config
	[ -d "$DIR" ] || die "install directory $DIR not found"
	init_ui
	header "diagnose and repair"
	run_checks
	if [ "${#ISSUE_IDS[@]}" -eq 0 ]; then
		panel "All clear" "No problems found"
		printf '\n'
		exit 0
	fi
	panel "Found ${#ISSUE_IDS[@]} issue(s)" "$(issue_rows)"
	if [ "$INTERACTIVE" -eq 1 ]; then
		MENU_LW=14 menu "What now?" 0 \
			"Fix all" "Apply every fix" \
			"Choose" "Pick which ones to apply" \
			"Recheck" "Run the checks again" \
			"Quit" "Change nothing"
		case "$MENU_RESULT" in
			0) section "Applying"; fix_all ;;
			1) section "Applying"; choose_fixes ;;
			2)
				printf '\n'
				reset_issues
				run_checks
				if [ "${#ISSUE_IDS[@]}" -eq 0 ]; then panel "All clear" "No problems found"; else panel "Still needs attention" "$(issue_rows)"; fi
				printf '\n'
				exit 0
				;;
			*) exit 0 ;;
		esac
	else
		section "Applying"
		fix_all
	fi
	printf '\n'
	section "Result"
	reset_issues
	run_checks
	if [ "${#ISSUE_IDS[@]}" -eq 0 ]; then
		ok "everything looks healthy"
	else
		panel "Still needs attention" "$(issue_rows)"
	fi
	printf '\n'
}

main "$@"
