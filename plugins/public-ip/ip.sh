#!/bin/sh
js() { printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | tr -d '\000-\037'; }
info=$(curl -fsS --max-time 10 https://ipinfo.io/json) || { echo "couldn't reach ipinfo.io" >&2; exit 1; }
field() { printf '%s' "$info" | sed -n "s/.*\"$1\": *\"\([^\"]*\)\".*/\1/p" | head -n 1; }
info=$(printf '%s' "$info" | tr '\n' ' ' | sed 's/,/,\n/g')
ip=$(field ip) city=$(field city) region=$(field region) country=$(field country) org=$(field org)
printf '{"items":[{"label":"IP address","value":"%s"},{"label":"Location","value":"%s","hint":"%s"},{"label":"Network","value":"%s"}]}\n' \
	"$(js "$ip")" "$(js "$city")" "$(js "$region, $country")" "$(js "$org")"
