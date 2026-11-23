esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(find /var/log -type f -printf '%s %p\n' 2>/dev/null | sort -nr | head -n1 | cut -c1-90)
emit_stats "Largest Log" "$value" "Biggest log file" "ok"
