#!/bin/sh
# Copies the SQLite database into PLUGIN_DATA and keeps the newest SETTING_KEEP.
set -e
db=${SERVERDASH_DB_PATH:?SERVERDASH_DB_PATH is not set}
dest="$PLUGIN_DATA/serverdash-$(date +%Y%m%d-%H%M%S).db"
if command -v sqlite3 >/dev/null 2>&1; then
	# .backup takes a consistent snapshot even while ServerDash is writing.
	sqlite3 "$db" ".backup '$dest'"
else
	cp "$db" "$dest"
	[ -f "$db-wal" ] && cp "$db-wal" "$dest-wal"
fi
gzip -f "$dest"
[ -f "$dest-wal" ] && gzip -f "$dest-wal"
echo "Saved $(basename "$dest").gz"
keep=${SETTING_KEEP:-7}
ls -1t "$PLUGIN_DATA"/serverdash-*.db.gz 2>/dev/null | tail -n +"$((keep + 1))" | while read -r old; do
	rm -f "$old" "${old%.db.gz}.db-wal.gz"
	echo "Removed $(basename "$old")"
done
