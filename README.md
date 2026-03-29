# Hysteria 2 Panel

Production-oriented control plane for managing a native Hysteria 2 service on Ubuntu 24.04 LTS or Debian 12+.

Control plane stack:

- `panel-api`: Go
- `panel-web`: React + Vite + Tailwind CSS + Radix UI + React Router + TanStack Query + react-hook-form + Recharts + Framer Motion
- Local filesystem storage under `/var/lib/h2v2`
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

## What deploy does

Installer phases:

1. Validates host OS support (Ubuntu 24.04 or Debian 12+) + root access
2. Installs host dependencies (Go, Node.js/npm, Caddy)
3. Installs Hysteria binary
4. Creates system users (`h2v2`, `hysteria`)
5. Generates runtime env files and admin credentials
6. Builds backend and frontend
7. Renders Caddy + Hysteria runtime configuration
8. Bootstraps file storage and admin account
9. Installs systemd units + restricted sudoers policy
10. Starts panel services, Hysteria, and Caddy
11. Syncs Caddy-issued cert into `/etc/h2v2/hysteria/`
12. Runs smoke checks

## Generated files and directories

- Main generated env: `/opt/h2v2/.env.generated`
- Initial admin credentials file: `/root/h2v2-initial-admin.txt`
- File-backed control-plane state: `/var/lib/h2v2/state/`
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

## Documentation

- [Architecture](./docs/architecture.md)
- [Deploy details](./docs/deploy.md)
- [Operations](./docs/operations.md)
