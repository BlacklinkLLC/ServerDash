#!/bin/sh
# Requests each URL in SETTING_URLS and reports status code and latency.
js() { printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | tr -d '\000-\037'; }
slow=${SETTING_SLOW_MS:-1500}
rows="" status="" sep=""
for url in $(printf '%s\n' "$SETTING_URLS"); do
	out=$(curl -o /dev/null -sS -L --max-time 10 -w '%{http_code} %{time_total}' "$url" 2>/dev/null)
	code=${out%% *}
	secs=${out##* }
	ms=$(awk -v s="${secs:-0}" 'BEGIN { printf "%d", s * 1000 }')
	if [ -z "$code" ] || [ "$code" = 000 ]; then
		st=crit text="down" timing=""
	elif [ "$code" -ge 400 ]; then
		st=crit text="HTTP $code" timing="$ms ms"
	elif [ "$ms" -gt "$slow" ]; then
		st=warn text="HTTP $code (slow)" timing="$ms ms"
	else
		st=ok text="HTTP $code" timing="$ms ms"
	fi
	rows="$rows$sep[\"$(js "$url")\",\"$text\",\"$timing\"]"
	status="$status$sep\"$st\""
	sep=","
done
printf '{"columns":["URL","Status","Response time"],"rows":[%s],"status":[%s]}\n' "$rows" "$status"
