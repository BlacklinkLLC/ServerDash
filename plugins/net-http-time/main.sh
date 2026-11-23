esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(curl -sS -o /dev/null -w '%{time_total}s' --max-time 5 "${SETTING_URL:-https://example.com}" 2>/dev/null || echo n/a)
emit_stats "HTTP Latency" "$value" "Time to first byte" "ok"
