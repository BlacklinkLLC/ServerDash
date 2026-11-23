esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(traceroute -m 10 -w 1 "${SETTING_HOST:-example.com}" 2>/dev/null | tail -n1 | awk '{print $1}' || echo n/a)
emit_stats "Traceroute Hops" "$value" "Hops to target" "ok"
