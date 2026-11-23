esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(awk '{print $1}' /proc/loadavg 2>/dev/null)
emit_stats "Load Average" "$value" "1 minute" "$(awk -v v="$(cat /proc/loadavg 2>/dev/null | awk '{print $1}')" 'BEGIN{print (v>4)?"crit":(v>2)?"warn":"ok"}')"
