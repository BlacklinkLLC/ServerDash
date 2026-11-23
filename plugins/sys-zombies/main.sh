esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(ps -eo stat= 2>/dev/null | grep -c Z || true)
emit_stats "Zombie Processes" "$value" "Defunct processes" "$(v=$(ps -eo stat= 2>/dev/null | grep -c Z || true); [ "$v" -gt 0 ] && echo warn || echo ok)"
