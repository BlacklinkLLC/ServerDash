esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(find /srv /opt /home -maxdepth 4 \( -name docker-compose.yml -o -name compose.yml -o -name compose.yaml \) -type f 2>/dev/null | wc -l)
emit_stats "Compose Projects" "$value" "Compose files found" "ok"
