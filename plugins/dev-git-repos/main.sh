esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(find /srv /opt /home -maxdepth 5 -type d -name .git 2>/dev/null | wc -l)
emit_stats "Git Repositories" "$value" "Repos discovered" "ok"
