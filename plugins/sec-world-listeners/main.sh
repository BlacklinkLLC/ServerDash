esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(ss -lnt 2>/dev/null | awk 'NR>1 && $4 !~ /127\.0\.0\.1|::1/' | wc -l)
emit_stats "Public Listeners" "$value" "Non-loopback listeners" "ok"
