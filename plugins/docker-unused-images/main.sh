esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(docker images -f dangling=true -q 2>/dev/null | wc -l)
emit_stats "Dangling Images" "$value" "Unused layers" "$(v=$(docker images -f dangling=true -q 2>/dev/null | wc -l); [ "$v" -gt 10 ] && echo warn || echo ok)"
