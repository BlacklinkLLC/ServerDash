esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(find / -xdev -type d -perm -0002 2>/dev/null | wc -l)
emit_stats "World Writable Dirs" "$value" "Risky directory modes" "ok"
