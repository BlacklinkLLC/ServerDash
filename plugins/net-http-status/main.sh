esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 "${SETTING_URL:-https://example.com}" 2>/dev/null || echo n/a)
emit_stats "HTTP Status" "$value" "Target response code" "$(v=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 "${SETTING_URL:-https://example.com}" 2>/dev/null); case "$v" in 2??|3??) echo ok ;; 4??) echo warn ;; 5??) echo crit ;; *) echo warn ;; esac)"
