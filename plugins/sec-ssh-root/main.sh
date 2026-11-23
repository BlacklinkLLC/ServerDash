esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(grep -h -c 'Accepted password for root' /var/log/secure /var/log/auth.log 2>/dev/null | awk -F: '{s+=$1}END{print s+0}')
emit_stats "Root SSH Logins" "$value" "Successful root logins" "$(v=$(grep -h -c 'Accepted password for root' /var/log/secure /var/log/auth.log 2>/dev/null | awk -F: '{s+=$1}END{print s+0}'); [ "$v" -gt 0 ] && echo warn || echo ok)"
