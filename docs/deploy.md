# Deploy Guide

## Host requirements

- Ubuntu 24.04 LTS or Debian 12+
- Root access (`sudo`)
- Public DNS records:
  - panel host (`PANEL_PUBLIC_HOST`)
  - subscription host (`SUBSCRIPTION_PUBLIC_HOST`)

## Open ports

- Panel HTTPS: `${PANEL_PUBLIC_PORT}` (default `8443`, TCP)
- Runtime ports: configured in `/api/inbounds` and rendered to `${SINGBOX_CONFIG_PATH}`

## Install

Remote bootstrap:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/ProstyGospody/h2v2/main/install.sh)
```

From repository root:

```bash
sudo bash ./deploy/install.sh
```

Modes:

- `--install`
- `--reconfigure`
- `--upgrade`
- `--non-interactive`
- `--dry-run`

## Non-interactive example

```bash
H2V2_NONINTERACTIVE=1 \
PANEL_PUBLIC_HOST=panel.example.com \
SUBSCRIPTION_PUBLIC_HOST=sub.example.com \
INITIAL_ADMIN_EMAIL=admin@example.com \
sudo -E bash ./deploy/install.sh --non-interactive
```

## Generated files

- `/opt/h2v2/.env.generated`
- `/root/h2v2-initial-admin.txt`
- `/etc/h2v2/sing-box/config.json`
- `/var/lib/h2v2/backups/install-YYYYmmdd-HHMMSS`

## Runtime environment

- `RUNTIME_POLL_INTERVAL`
- `SINGBOX_BINARY_PATH`
- `SINGBOX_CONFIG_PATH`
- `SINGBOX_SERVICE_NAME`

## Verification

```bash
sudo bash ./deploy/verify.sh
```

Checks include:

- systemd services (`h2v2-api`, `h2v2-web`, `sing-box`, `caddy`)
- API health/readiness
- runtime listeners from configured inbounds
- admin login flow
