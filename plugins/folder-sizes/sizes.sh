#!/bin/sh
# du for each path in SETTING_PATHS, read through the host root in the container.
js() { printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | tr -d '\000-\037'; }
root=${SERVERDASH_HOST_ROOT:-}
warn_kb=$(( ${SETTING_WARN_GB:-50} * 1024 * 1024 ))
rows="" status="" sep=""
for p in $(printf '%s\n' "$SETTING_PATHS"); do
	kb=$(timeout 60 du -sk "$root$p" 2>/dev/null | tail -n 1 | cut -f1)
	if [ -z "$kb" ]; then
		size="not found" st=warn
	else
		size=$(awk -v k="$kb" 'BEGIN { split("KB MB GB TB", u); i = 1; while (k >= 1024 && i < 4) { k /= 1024; i++ } printf "%.1f %s", k, u[i] }')
		st=ok
		[ "$kb" -gt "$warn_kb" ] && st=warn
	fi
	rows="$rows$sep[\"$(js "$p")\",\"$size\"]"
	status="$status$sep\"$st\""
	sep=","
done
printf '{"columns":["Folder","Size"],"rows":[%s],"status":[%s]}\n' "$rows" "$status"
