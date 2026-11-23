esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(fail2ban-client status sshd 2>/dev/null | awk -F'\t' '/Banned IP list/{print $2}' || echo n/a)
emit_stats "Banned IPs" "$value" "fail2ban sshd bans" "ok"
