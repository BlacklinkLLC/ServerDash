esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(sensors 2>/dev/null | awk -F'[+°]' '/Tctl|Package id 0|CPU/{print $2;exit}' || cat /sys/class/thermal/thermal_zone0/temp 2>/dev/null || echo n/a)
emit_stats "CPU Temperature" "$value" "Thermal sensor" "$(v=$(sensors 2>/dev/null | awk -F'[+°]' '/Tctl|Package id 0|CPU/{gsub(/[^0-9.]/,"",$2);print $2;exit}'); [ -n "$v" ] && [ "${v%.*}" -ge 90 ] 2>/dev/null && echo crit || echo ok)"
