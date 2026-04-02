# Operations

## Service status

```bash
systemctl status h2v2-api h2v2-web hysteria-server xray caddy
```

## Restart / reload

```bash
systemctl restart h2v2-api
systemctl restart h2v2-web
systemctl restart hysteria-server
systemctl restart xray
systemctl reload caddy
```

## Logs

```bash
journalctl -u h2v2-api -n 200 --no-pager
journalctl -u h2v2-web -n 200 --no-pager
journalctl -u hysteria-server -n 200 --no-pager
journalctl -u xray -n 200 --no-pager
journalctl -u caddy -n 200 --no-pager
```

## Paths

- Panel env: `/opt/h2v2/.env.generated`
- HY2 config: `/etc/h2v2/hysteria/server.yaml`
- Xray config: `/etc/h2v2/xray/config.json`
- SQLite DB: `/var/lib/h2v2/data/h2v2.db`
- Storage root: `/var/lib/h2v2`
- Audit dir: `/var/log/h2v2/audit`

## API checks

```bash
curl -fsS http://127.0.0.1:18080/healthz
curl -fsS http://127.0.0.1:18080/readyz
```

## Smoke check

```bash
sudo bash /opt/h2v2/current/scripts/smoke-check.sh /opt/h2v2/.env.generated
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

SQLite restore:

```bash
systemctl stop h2v2-api
runuser -u h2v2 -- /opt/h2v2/bin/panel-api sqlite-restore --db /var/lib/h2v2/data/h2v2.db --from /var/lib/h2v2/backups/panel-YYYYmmdd-HHMM.db
systemctl start h2v2-api
```

Daily backup + rotation:

```bash
bash /opt/h2v2/current/scripts/sqlite-backup-rotate.sh --env-file /opt/h2v2/.env.generated --keep-days 14
```
