esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(awk -F: '/model name/{print $2;exit}' /proc/cpuinfo 2>/dev/null || echo n/a)
emit_stats "CPU Model" "$value" "Processor" "ok"
