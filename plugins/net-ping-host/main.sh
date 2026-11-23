esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(ping -c 4 -W 2 "${SETTING_HOST:-1.1.1.1}" 2>/dev/null | tail -n2 | head -n1 || echo n/a)
emit_stats "Host Ping" "$value" "ICMP to configured host" "ok"
