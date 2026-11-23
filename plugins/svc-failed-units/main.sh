esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(systemctl --failed --no-legend 2>/dev/null | wc -l)
emit_stats "Failed Units" "$value" "Failed systemd units" "$(v=$(systemctl --failed --no-legend 2>/dev/null | wc -l); [ "$v" -gt 0 ] && echo crit || echo ok)"
