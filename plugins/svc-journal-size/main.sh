esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(journalctl --disk-usage 2>/dev/null | awk '{print $NF " " $4}' || echo n/a)
emit_stats "Journal Size" "$value" "Persisted logs" "ok"
