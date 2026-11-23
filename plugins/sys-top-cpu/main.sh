esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(ps -eo comm=,%cpu= --sort=-%cpu 2>/dev/null | head -n1 | awk '{print $1" "$2"%"}')
emit_stats "Top CPU Process" "$value" "Highest CPU consumer" "ok"
