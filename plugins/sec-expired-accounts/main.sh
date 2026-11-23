esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(passwd -Sa 2>/dev/null | awk '$2=="L"||$2=="E"{n++}END{print n+0}')
emit_stats "Expired Accounts" "$value" "Locked or expired" "ok"
