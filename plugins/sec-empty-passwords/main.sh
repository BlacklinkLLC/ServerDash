esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(awk -F: '($2==""){n++}END{print n+0}' /etc/shadow 2>/dev/null || echo n/a)
emit_stats "Empty Passwords" "$value" "Accounts without password" "$(v=$(awk -F: '($2=="" ){n++}END{print n+0}' /etc/shadow 2>/dev/null || echo 0); [ "$v" -gt 0 ] && echo crit || echo ok)"
