esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(stat -c '%a' /etc/ssh/sshd_config 2>/dev/null || echo n/a)
emit_stats "SSH Config Mode" "$value" "sshd_config mode" "ok"
