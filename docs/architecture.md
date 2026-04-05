# Architecture

## Scope

The panel controls sing-box runtime for:

- Hysteria 2 (`hy2`)
- VLESS (`vless`)

Control plane services:

- `panel-api` (`127.0.0.1:18080`)
- `panel-web` (`127.0.0.1:13000`)
- `caddy` (public HTTPS entrypoint)

## Core backend

`core v1` is the source of truth for user and runtime configuration flow:

- server/inbound/user/access CRUD
- hashed subscription tokens with revoke/rotate
- URI/profile/QR generation
- sing-box config revisioning: `render -> validate -> apply`

UI actions use `/api/v1/*`. Runtime updates are applied through server config render/apply.

## Persistence model

SQLite storage uses `core_*` tables:

- `core_servers`
- `core_inbounds`
- `core_inbound_vless_settings`
- `core_inbound_hysteria2_settings`
- `core_users`
- `core_user_access`
- `core_subscriptions`
- `core_subscription_tokens`
- `core_config_revisions`

## Public subscription delivery

Public endpoints:

- `/sub/{token}/profile.singbox.json`
- `/sub/{token}/uris.txt`
- `/sub/{token}/qr.png`

## Service control

Service actions are restricted to `MANAGED_SERVICES`.

Default list:

- `h2v2-api`
- `sing-box`
