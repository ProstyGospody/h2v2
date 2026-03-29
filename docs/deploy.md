# Deploy Guide

## Host requirements

- Ubuntu 24.04 LTS or Debian 12+
- Root access (`sudo`)
- Public DNS records:
  - panel host (`PANEL_PUBLIC_HOST`)
  - subscription host (`SUBSCRIPTION_PUBLIC_HOST`)
  - Hysteria host (`HY2_DOMAIN`)

## Open ports

- Panel HTTPS: `${PANEL_PUBLIC_PORT}` (default `8443`, TCP)
- Hysteria transport: `${HY2_PORT}` (default `443`, UDP)

## One-command install

Remote bootstrap:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/ProstyGospody/h2v2/main/install.sh)
```

From repository root:

```bash
sudo bash ./deploy/install.sh
```

Legacy wrapper (same behavior):

```bash
sudo bash ./deploy/ubuntu24-host-install.sh
```

## Non-interactive mode

You can preseed values with environment variables and run:

```bash
H2V2_NONINTERACTIVE=1 \
PANEL_PUBLIC_HOST=panel.example.com \
SUBSCRIPTION_PUBLIC_HOST=sub.example.com \
HY2_DOMAIN=hy2.example.com \
INITIAL_ADMIN_EMAIL=admin@example.com \
sudo -E bash ./deploy/install.sh --non-interactive
```

## Reconfigure existing host

```bash
sudo bash ./deploy/install.sh --reconfigure
```

## What gets generated

- `/opt/h2v2/.env.generated`
- `/root/h2v2-initial-admin.txt`
- `/etc/h2v2/hysteria/server.yaml`
- `/etc/h2v2/hysteria/tls.crt`
- `/etc/h2v2/hysteria/tls.key`

## Post-install verification

```bash
sudo bash ./deploy/verify.sh
```

This validates:

- core systemd services
- API health/readiness
- Hysteria listener check
- admin login flow
