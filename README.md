# Hysteria 2 Panel

Production-oriented control plane for managing a native Hysteria 2 service on Ubuntu 24.04 LTS or Debian 12+.

Control plane stack:

- `panel-api`: Go
- `panel-web`: React + Vite + Tailwind CSS + Radix UI + React Router + TanStack Query + react-hook-form + Recharts + Framer Motion
- Storage driver: `file` (default) or `sqlite`
- Caddy (TLS reverse proxy and certificate issuer)
- Native procfs-based host metrics (live CPU/RAM/network)
- systemd

## One-command deploy (Ubuntu 24.04 / Debian 12+ host)

Remote bootstrap (same style as other panels):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/ProstyGospody/h2v2/main/install.sh)
```

From cloned repository:

```bash
sudo bash ./deploy/install.sh
```

Legacy compatibility wrapper:

```bash
sudo bash ./deploy/ubuntu24-host-install.sh
```

Installer modes:

- `sudo bash ./deploy/install.sh --install` (default)
- `sudo bash ./deploy/install.sh --reconfigure`
- `sudo bash ./deploy/install.sh --upgrade`
- `sudo bash ./deploy/install.sh --migrate-to-sqlite`
- add `--non-interactive` or `--dry-run` when needed

Storage default behavior:

- fresh install: `PANEL_STORAGE_DRIVER=sqlite`
- existing install (reconfigure/upgrade): current driver is preserved

## What deploy does

Installer phases:

1. Preflight checks + installation detection
2. Backup of env/config/storage/systemd before changes
3. Build/install phase (packages, binaries, source sync)
4. Config render phase (`.env`, Caddy, Hysteria config, systemd)
5. Optional `file -> sqlite` migration
6. Health checks (service state + smoke script)
7. Auto-rollback from backup on failed health checks

## Generated files and directories

- Main generated env: `/opt/h2v2/.env.generated`
- Initial admin credentials file: `/root/h2v2-initial-admin.txt`
- File-backed state: `/var/lib/h2v2/state/`
- SQLite data dir: `/var/lib/h2v2/data/`
- Historical snapshots: `/var/lib/h2v2/snapshots/`
- Backups: `/var/lib/h2v2/backups/`
- Audit records: `/var/log/h2v2/audit/`
- Runtime locks/temp: `/run/h2v2/`
- Hysteria config: `/etc/h2v2/hysteria/server.yaml`
- Hysteria synced TLS cert/key: `/etc/h2v2/hysteria/tls.crt`, `/etc/h2v2/hysteria/tls.key`

## Service names

- `h2v2-api.service`
- `h2v2-web.service`
- `hysteria-server.service`
- `caddy.service`

Check status:

```bash
systemctl status h2v2-api h2v2-web hysteria-server caddy
```

## Smoke check

```bash
sudo bash ./deploy/verify.sh
```

Or directly:

```bash
sudo bash /opt/h2v2/current/scripts/smoke-check.sh /opt/h2v2/.env.generated
```

SQLite operations:

```bash
runuser -u h2v2 -- /opt/h2v2/bin/panel-api migrate-to-sqlite --db /var/lib/h2v2/data/h2v2.db --storage-root /var/lib/h2v2 --audit-dir /var/log/h2v2/audit --runtime-dir /run/h2v2
runuser -u h2v2 -- /opt/h2v2/bin/panel-api sqlite-backup --db /var/lib/h2v2/data/h2v2.db --out /var/lib/h2v2/backups/panel-$(date -u +%Y%m%d-%H%M).db
runuser -u h2v2 -- /opt/h2v2/bin/panel-api export --db /var/lib/h2v2/data/h2v2.db --out /var/lib/h2v2/backups/export-$(date -u +%Y%m%d-%H%M).json
runuser -u h2v2 -- /opt/h2v2/bin/panel-api sqlite-restore --db /var/lib/h2v2/data/h2v2.db --from /var/lib/h2v2/backups/panel-YYYYmmdd-HHMM.db
```

Daily backup with rotation:

```bash
bash /opt/h2v2/current/scripts/sqlite-backup-rotate.sh --env-file /opt/h2v2/.env.generated --keep-days 14
```

## Documentation

- [Architecture](./docs/architecture.md)
- [Deploy details](./docs/deploy.md)
- [Operations](./docs/operations.md)
