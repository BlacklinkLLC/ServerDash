#!/bin/sh
js() { printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | tr -d '\000-\037'; }
rows="" sep=""
for f in $(ls -1t "$PLUGIN_DATA"/serverdash-*.db.gz 2>/dev/null); do
	kb=$(du -k "$f" | cut -f1)
	rows="$rows$sep[\"$(js "$(basename "$f")")\",\"$kb KB\"]"
	sep=","
done
printf '{"columns":["Backup (in %s)","Size"],"rows":[%s]}\n' "$(js "$PLUGIN_DATA")" "$rows"
