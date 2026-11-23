esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(awk '/^nameserver/{printf "%s ",$2}' /etc/resolv.conf 2>/dev/null)
emit_stats "DNS Servers" "$value" "Resolver addresses" "ok"
