# Operations

## Service status

```bash
systemctl status h2v2-api h2v2-web hysteria-server caddy
```

## Service restart / reload

```bash
systemctl restart h2v2-api
systemctl restart h2v2-web
systemctl restart hysteria-server
systemctl reload caddy
```

## Logs

```bash
journalctl -u h2v2-api -n 200 --no-pager
journalctl -u h2v2-web -n 200 --no-pager
journalctl -u hysteria-server -n 200 --no-pager
journalctl -u caddy -n 200 --no-pager
```

## Config paths

- Panel env: `/opt/h2v2/.env.generated`
- Hysteria config: `/etc/h2v2/hysteria/server.yaml`
- Hysteria TLS cert/key: `/etc/h2v2/hysteria/tls.crt`, `/etc/h2v2/hysteria/tls.key`
- Storage root: `/var/lib/h2v2`
- SQLite DB: `/var/lib/h2v2/data/h2v2.db` (default path for sqlite driver)
- Audit dir: `/var/log/h2v2/audit`

## Smoke check

```bash
sudo bash /opt/h2v2/current/scripts/smoke-check.sh /opt/h2v2/.env.generated
```

## Enable sqlite

1. Migrate existing file data:

```bash
runuser -u h2v2 -- /opt/h2v2/bin/panel-api migrate-to-sqlite --db /var/lib/h2v2/data/h2v2.db --storage-root /var/lib/h2v2 --audit-dir /var/log/h2v2/audit --runtime-dir /run/h2v2
```

2. Edit `/opt/h2v2/.env.generated`:

```bash
PANEL_STORAGE_DRIVER=sqlite
PANEL_SQLITE_PATH=/var/lib/h2v2/data/h2v2.db
```

3. Restart API:

```bash
systemctl restart h2v2-api
```

## Backup / export / restore

SQLite backup:

```bash
runuser -u h2v2 -- /opt/h2v2/bin/panel-api sqlite-backup --db /var/lib/h2v2/data/h2v2.db --out /var/lib/h2v2/backups/panel-$(date -u +%Y%m%d-%H%M).db
```

JSON export:

```bash
runuser -u h2v2 -- /opt/h2v2/bin/panel-api export --db /var/lib/h2v2/data/h2v2.db --out /var/lib/h2v2/backups/export-$(date -u +%Y%m%d-%H%M).json
```

Restore SQLite:

```bash
systemctl stop h2v2-api
runuser -u h2v2 -- /opt/h2v2/bin/panel-api sqlite-restore --db /var/lib/h2v2/data/h2v2.db --from /var/lib/h2v2/backups/panel-YYYYmmdd-HHMM.db
systemctl start h2v2-api
```

Daily backup + rotation:

```bash
bash /opt/h2v2/current/scripts/sqlite-backup-rotate.sh --env-file /opt/h2v2/.env.generated --keep-days 14
```

## Rollback to file mode

```bash
sed -i 's/^PANEL_STORAGE_DRIVER=.*/PANEL_STORAGE_DRIVER=file/' /opt/h2v2/.env.generated
systemctl restart h2v2-api
```
