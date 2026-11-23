esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(free 2>/dev/null | awk '/Mem:/{if($2==0)print "0%";else printf "%.0f%%",$3/$2*100}')
emit_stats "Memory Used" "$value" "Physical memory" "$(v=$(free 2>/dev/null | awk '/Mem:/{print int($3/$2*100)}'); [ "$v" -ge 90 ] && echo crit || { [ "$v" -ge 75 ] && echo warn || echo ok; })"
