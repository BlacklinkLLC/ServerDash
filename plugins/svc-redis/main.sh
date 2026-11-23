esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(systemctl is-active redis 2>/dev/null || systemctl is-active redis-server 2>/dev/null || echo unknown)
emit_stats "Redis Service" "$value" "cache unit" "ok"
