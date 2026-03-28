# Hysteria 2 Panel

Production-oriented control plane for managing a native Hysteria 2 service on Ubuntu 24.04 LTS or Debian 12+.

Control plane stack:

- `panel-api`: Go
- `panel-web`: React + Vite + MUI + React Router + TanStack Query + react-hook-form
- Local filesystem storage under `/var/lib/proxy-panel`
- Caddy (TLS reverse proxy and certificate issuer)
- Native procfs-based host metrics (live CPU/RAM/network)
- systemd

## One-command deploy (Ubuntu 24.04 / Debian 12+ host)

Remote bootstrap (same style as other panels):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/ProstyGospody/webpanel/main/install.sh)
```

From cloned repository:

```bash
sudo bash ./deploy/install.sh
```

Legacy compatibility wrapper:

```bash
sudo bash ./deploy/ubuntu24-host-install.sh
```

## What deploy does

Installer phases:

1. Validates host OS support (Ubuntu 24.04 or Debian 12+) + root access
2. Installs host dependencies (Go, Node.js/npm, Caddy)
3. Installs Hysteria binary
4. Creates system users (`proxy-panel`, `hysteria`)
5. Generates runtime env files and admin credentials
6. Builds backend and frontend
7. Renders Caddy + Hysteria runtime configuration
8. Bootstraps file storage and admin account
9. Installs systemd units + restricted sudoers policy
10. Starts panel services, Hysteria, and Caddy
11. Syncs Caddy-issued cert into `/etc/proxy-panel/hysteria/`
12. Runs smoke checks

## Generated files and directories

- Main generated env: `/opt/proxy-panel/.env.generated`
- Initial admin credentials file: `/root/proxy-panel-initial-admin.txt`
- File-backed control-plane state: `/var/lib/proxy-panel/state/`
- Historical snapshots: `/var/lib/proxy-panel/snapshots/`
- Backups: `/var/lib/proxy-panel/backups/`
- Audit records: `/var/log/proxy-panel/audit/`
- Runtime locks/temp: `/run/proxy-panel/`
- Hysteria config: `/etc/proxy-panel/hysteria/server.yaml`
- Hysteria synced TLS cert/key: `/etc/proxy-panel/hysteria/tls.crt`, `/etc/proxy-panel/hysteria/tls.key`

## Service names

- `proxy-panel-api.service`
- `proxy-panel-web.service`
- `hysteria-server.service`
- `caddy.service`

Check status:

```bash
systemctl status proxy-panel-api proxy-panel-web hysteria-server caddy
```

## Smoke check

```bash
sudo bash ./deploy/verify.sh
```

Or directly:

```bash
sudo bash /opt/proxy-panel/current/scripts/smoke-check.sh /opt/proxy-panel/.env.generated
```

## Documentation

- [Architecture](./docs/architecture.md)
- [Deploy details](./docs/deploy.md)
- [Operations](./docs/operations.md)
