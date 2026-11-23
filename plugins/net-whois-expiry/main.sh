esc() { printf '%s' "$1" | tr '\n\r\t' '   ' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
emit_stats() { printf '{"items":[{"label":"%s","value":"%s","hint":"%s","status":"%s"}]}' "$(esc "$1")" "$(esc "$2")" "$(esc "$3")" "$(esc "$4")"; }
value=$(whois "${SETTING_DOMAIN:-example.com}" 2>/dev/null | awk -F: '/Registry Expiry Date|Expiry Date/{print $2;exit}' | cut -c1-32 || echo n/a)
emit_stats "Domain Expiry" "$value" "Registration expiry" "ok"
