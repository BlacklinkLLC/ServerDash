esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(timedatectl 2>/dev/null | awk -F: '/System clock synchronized/{gsub(/^ +/,"",$2);print $2;exit}' || echo n/a)
emit_stats "Clock Synced" "$value" "Synchronized state" "ok"
