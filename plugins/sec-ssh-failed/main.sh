esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(grep -h -c 'Failed password' /var/log/secure /var/log/auth.log 2>/dev/null | awk -F: '{s+=$1}END{print s+0}')
emit_stats "Failed SSH Logins" "$value" "Auth failures" "$(v=$(grep -h -c 'Failed password' /var/log/secure /var/log/auth.log 2>/dev/null | awk -F: '{s+=$1}END{print s+0}'); [ "$v" -gt 50 ] && echo warn || echo ok)"
