esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(docker inspect --format '{{.RestartCount}}' $(docker ps -q) 2>/dev/null | awk '{s+=$1}END{print s+0}')
emit_stats "Container Restarts" "$value" "Total restart count" "ok"
