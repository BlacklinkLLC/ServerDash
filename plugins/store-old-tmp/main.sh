esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(find /tmp -type f -mtime +7 2>/dev/null | wc -l)
emit_stats "Old Temp Files" "$value" "Older than 7 days" "$(v=$(find /tmp -type f -mtime +7 2>/dev/null | wc -l); [ "$v" -gt 100 ] && echo warn || echo ok)"
