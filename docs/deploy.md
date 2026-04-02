# Deploy Guide

## Host requirements

- Ubuntu 24.04 LTS or Debian 12+
- Root access (`sudo`)
- Public DNS records:
  - panel host (`PANEL_PUBLIC_HOST`)
  - subscription host (`SUBSCRIPTION_PUBLIC_HOST`)
  - HY2 host (`HY2_DOMAIN`)

## Open ports

- Panel HTTPS: `${PANEL_PUBLIC_PORT}` (default `8443`, TCP)
- HY2 transport: `${HY2_PORT}` (default `443`, UDP)
- VLESS inbound port(s): configured in `/api/inbounds` and rendered to `${XRAY_CONFIG_PATH}`

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
HY2_DOMAIN=hy2.example.com \
INITIAL_ADMIN_EMAIL=admin@example.com \
sudo -E bash ./deploy/install.sh --non-interactive
```

## Generated files

- `/opt/h2v2/.env.generated`
- `/root/h2v2-initial-admin.txt`
- `/etc/h2v2/hysteria/server.yaml`
- `/etc/h2v2/xray/config.json`
- `/var/lib/h2v2/backups/install-YYYYmmdd-HHMMSS`

## Added runtime env for VLESS/Xray

- `XRAY_BINARY_PATH`
- `XRAY_CONFIG_PATH`
- `XRAY_RUNTIME_URL`
- `XRAY_RUNTIME_TOKEN`
- `XRAY_SERVICE_NAME`
- `XRAY_POLL_INTERVAL`

## Verification

```bash
sudo bash ./deploy/verify.sh
```

Checks include:

- systemd services (`h2v2-api`, `h2v2-web`, `hysteria-server`, `xray`, `caddy`)
- API health/readiness
- HY2 listener
- admin login flow
