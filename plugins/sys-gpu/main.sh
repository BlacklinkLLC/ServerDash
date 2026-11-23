esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(lspci 2>/dev/null | grep -i -m1 vga | cut -d: -f3 | cut -c2-70 || echo n/a)
emit_stats "GPU" "$value" "Graphics controller" "ok"
