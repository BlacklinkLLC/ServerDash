esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(curl -fsS --max-time 5 -6 https://api64.ipify.org 2>/dev/null || echo n/a)
emit_stats "Public IPv6" "$value" "External address" "ok"
