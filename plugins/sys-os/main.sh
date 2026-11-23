esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(. /etc/os-release 2>/dev/null; printf '%s %s' "${PRETTY_NAME:-unknown}" "${VERSION_ID:-}")
emit_stats "Operating System" "$value" "Distribution" "ok"
