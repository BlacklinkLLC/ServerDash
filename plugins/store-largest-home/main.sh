esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(du -sh /home/* 2>/dev/null | sort -hr | head -n1 | cut -c1-90)
emit_stats "Largest Home" "$value" "Biggest home folder" "ok"
