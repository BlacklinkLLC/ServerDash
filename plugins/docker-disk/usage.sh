#!/bin/sh
# Turns `docker system df` into stat tiles.
js() { printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | tr -d '\000-\037'; }
items="" sep=""
docker system df --format '{{.Type}}|{{.TotalCount}}|{{.Size}}|{{.Reclaimable}}' | while IFS='|' read -r type count size reclaim; do
	printf '%s{"label":"%s","value":"%s","hint":"%s total, %s reclaimable"}' "$sep" "$(js "$type")" "$(js "$size")" "$(js "$count")" "$(js "$reclaim")"
	sep=","
done > "${TMPDIR:-/tmp}/docker-disk.$$"
items=$(cat "${TMPDIR:-/tmp}/docker-disk.$$")
rm -f "${TMPDIR:-/tmp}/docker-disk.$$"
printf '{"items":[%s]}\n' "$items"
