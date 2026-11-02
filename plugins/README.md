# Writing a ServerDash plugin

A plugin is a folder with a `plugin.json` and the scripts it runs. The plugins in this folder ship with ServerDash. Put your own in `<data dir>/plugins/` (shown on the Plugins → Manage page), then click **Rescan folders**. A plugin of yours with the same `id` as a bundled one replaces it.

Every plugin starts turned off. An admin turns it on, fills in its settings, and chooses whether its widgets appear on the dashboard.

## plugin.json

```json
{
  "id": "my-plugin",
  "name": "My plugin",
  "version": "1.0.0",
  "description": "One line about what it does.",
  "icon": "Puzzle",
  "requires": ["curl"],
  "settings": [
    { "key": "url", "label": "URL to check", "type": "text", "default": "https://example.com" }
  ],
  "widgets": [
    { "id": "status", "title": "Status", "format": "stats", "command": "sh ./status.sh", "interval": 60 }
  ],
  "actions": [
    { "id": "reset", "label": "Reset", "command": "sh ./reset.sh", "confirm": "Really reset?", "role": "operator", "danger": true }
  ]
}
```

| Field | Meaning |
| --- | --- |
| `id` | 2 to 41 lowercase letters, digits and dashes. |
| `requires` | Commands that must be installed. Missing ones are shown as a warning. |
| `settings[].type` | `text`, `textarea`, `number`, `checkbox` (`"true"`/`"false"`), `password` or `select` (with `options`). |
| `widgets[].format` | `stats`, `table` or `text`. See below. |
| `widgets[].interval` | Seconds between refreshes (at least 10, default 60). Results are cached, so many viewers don't run the command many times. |
| `widgets[].wide` | Take the full width of the grid. |
| `actions[].role` | `admin` (default) or `operator`: who may run it. |
| `actions[].confirm` | Ask before running, with this text. |

## What commands get

Commands run with `sh -c` inside the plugin's folder, with ServerDash's own permissions. Widgets time out after 20 seconds and actions after 5 minutes. These environment variables are set:

- `SETTING_<KEY>`: each setting, with the key in upper case (for example `SETTING_URL`).
- `PLUGIN_DIR`: the plugin's folder.
- `PLUGIN_DATA`: a folder the plugin can keep files in. It survives updates.
- `SERVERDASH_HOST_ROOT`: where the host's filesystem is mounted (`/rootfs` in the container; unset on native installs). Read host paths as `$SERVERDASH_HOST_ROOT/etc/...`.
- `SERVERDASH_DB_PATH` and `SERVERDASH_DATA_DIR`.

In the container image, `sh` is BusyBox, and `curl`, `openssl`, `sqlite3`, `git` and `docker` are available. Stick to POSIX `sh` so a plugin works in both the container and native installs.

## Widget output

A command that exits with a non-zero status shows its error output in the widget.

**stats** prints JSON tiles. `status` (`ok`, `warn` or `crit`) and `hint` are optional:
```json
{ "items": [ { "label": "Users", "value": "42", "hint": "3 new today", "status": "ok" } ] }
```

**table** prints columns and rows. `status` is optional, one entry per row:
```json
{ "columns": ["Site", "Status"], "rows": [["example.com", "up"]], "status": ["ok"] }
```

**text** shows whatever the command prints.

## Using actions in workflows

The **Run plugin action** workflow block runs any action of an enabled plugin. For example, a nightly schedule plus ServerDash backup's "Back up now" gives you automatic backups.

## Bundled plugins

| Plugin | What it does |
| --- | --- |
| SSL certificates | Days until each host's TLS certificate expires. |
| Uptime monitor | Status code and response time of your URLs. |
| Docker disk usage | Space used by images, containers, volumes and build cache, with buttons to free it. |
| Folder sizes | Size of the host folders you list. |
| ServerDash backup | Backs up ServerDash's database and keeps the newest N copies. |
| Public IP | The server's public IP address and location (from ipinfo.io). |
