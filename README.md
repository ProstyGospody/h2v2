# H2V2 Panel

Production-oriented control plane for HY2 + VLESS with SQLite state and sing-box runtime sync.

Control plane stack:

- `panel-api`: Go
- `panel-web`: React + Vite + Tailwind CSS + Radix UI + React Router + TanStack Query
- Storage: SQLite (default `/var/lib/h2v2/data/h2v2.db`)
- Data-plane runtime: `sing-box` (VLESS + HY2)
- Reverse proxy: Caddy
- Host metrics: native procfs readers
- Service manager: systemd

## Core capabilities

- Core v1 server/inbound/user/access lifecycle
- Tokenized subscription profile generation
- Sing-box config revision flow (`render`, `validate`, `apply`, `rollback`)
- Public subscription delivery via `/sub/{token}/...`

## One-command deploy (Ubuntu 24.04 / Debian 12+)

Remote bootstrap:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/ProstyGospody/h2v2/main/install.sh)
```

From cloned repository:

```bash
sudo bash ./deploy/install.sh
```

Installer modes:

- `sudo bash ./deploy/install.sh --install` (default)
- `sudo bash ./deploy/install.sh --reconfigure`
- `sudo bash ./deploy/install.sh --upgrade`
- add `--non-interactive` or `--dry-run` when needed

## Generated paths

- Main env: `/opt/h2v2/.env.generated`
- Initial admin credentials: `/root/h2v2-initial-admin.txt`
- SQLite DB: `/var/lib/h2v2/data/h2v2.db`
- Backups: `/var/lib/h2v2/backups/`
- Runtime locks/temp: `/run/h2v2/`
- Sing-box config: `/etc/h2v2/sing-box/config.json`

## Service names

- `h2v2-api.service`
- `sing-box.service` (or `${SINGBOX_SERVICE_NAME}` from env)
- `caddy.service` (also serves the static web panel from `/opt/h2v2/current/web/dist`)

Check status:

```bash
systemctl status h2v2-api sing-box caddy
```

## API routes

Core routes:

- `/api/v1/servers`
- `/api/v1/inbounds`
- `/api/v1/users`
- `/api/v1/access`
- `/api/v1/users/{id}/artifacts`
- `/sub/{token}/profile.singbox.json`
- `/sub/{token}/uris.txt`
- `/sub/{token}/qr.png`

## Runtime environment

- `RUNTIME_POLL_INTERVAL`
- `SINGBOX_BINARY_PATH`
- `SINGBOX_CONFIG_PATH`
- `SINGBOX_SERVICE_NAME`

## Smoke check

```bash
sudo bash ./deploy/verify.sh
```

Or directly:

```bash
sudo bash /opt/h2v2/current/scripts/smoke-check.sh /opt/h2v2/.env.generated
```

## Docs

- [Architecture](./docs/architecture.md)
- [API](./docs/api.md)
- [Deploy](./docs/deploy.md)
- [Operations](./docs/operations.md)
