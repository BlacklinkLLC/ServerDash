esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(ssh -V 2>&1 | cut -d_ -f2 | cut -d' ' -f1 || echo n/a)
emit_stats "OpenSSH Version" "$value" "SSH client" "ok"
