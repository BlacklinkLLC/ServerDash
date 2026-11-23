esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(ip link 2>/dev/null | grep -E 'tun|tap|wg' | wc -l)
emit_stats "VPN Interfaces" "$value" "Tunnel interfaces" "ok"
