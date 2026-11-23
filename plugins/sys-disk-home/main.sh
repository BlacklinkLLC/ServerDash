esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(df -h /home 2>/dev/null | awk 'NR==2{print $5}')
emit_stats "Home Disk" "$value" "Filesystem /home" "$(v=$(df -P /home 2>/dev/null | awk 'NR==2{gsub("%","",$5);print $5}'); [ "$v" -ge 90 ] && echo crit || { [ "$v" -ge 75 ] && echo warn || echo ok; })"
