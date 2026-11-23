esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(nmcli -t -f active,ssid dev wifi 2>/dev/null | awk -F: '$1=="yes"{print $2;exit}' || echo n/a)
emit_stats "WiFi Network" "$value" "Active SSID" "ok"
