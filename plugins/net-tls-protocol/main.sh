esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(echo | openssl s_client -connect "${SETTING_HOST:-example.com}:${SETTING_PORT:-443}" -servername "${SETTING_HOST:-example.com}" 2>/dev/null | awk '/Protocol/{print $3;exit}' || echo n/a)
emit_stats "TLS Protocol" "$value" "Negotiated protocol" "ok"
