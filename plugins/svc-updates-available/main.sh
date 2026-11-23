esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(dnf check-update -q 2>/dev/null | wc -l)
emit_stats "Updates Available" "$value" "Package updates" "$(v=$(dnf check-update -q 2>/dev/null | wc -l); [ "$v" -gt 20 ] && echo warn || echo ok)"
