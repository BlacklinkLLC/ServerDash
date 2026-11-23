esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(nc -z -w 2 "${SETTING_HOST:-127.0.0.1}" "${SETTING_PORT:-80}" 2>/dev/null && echo open || echo closed)
emit_stats "Port Open" "$value" "TCP probe result" "$(test "$(nc -z -w 2 "${SETTING_HOST:-127.0.0.1}" "${SETTING_PORT:-80}" 2>/dev/null && echo open || echo closed)" = open && echo ok || echo warn)"
