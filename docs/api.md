# API

Protocol-agnostic endpoints operate on HY2 and VLESS in one model.

## Unified users

- `GET /api/users?limit=&offset=&protocol=hy2|vless`
- `POST /api/users`
- `GET /api/users/{id}`
- `PATCH /api/users/{id}`
- `DELETE /api/users/{id}`
- `POST /api/users/state` (`ids[]`, `enabled`, optional `protocol`)
- `POST /api/users/delete` (`ids[]`)
- `POST /api/users/{id}/kick`
- `POST /api/users/kick` (`ids[]`)

User create/update payload:

```json
{
  "name": "alice",
  "enabled": true,
  "traffic_limit_bytes": 1073741824,
  "expire_at": "2026-12-31T23:59:59Z",
  "note": "team-a",
  "credentials": [
    { "protocol": "hy2", "identity": "f57486cd-c4f3-44f0-accc-9664449d95ba", "secret": "secret-value" },
    { "protocol": "vless", "identity": "2b7ee3cd-20f0-4bd3-b9cc-10aeeb6a46ad" }
  ]
}
```

## Unified inbounds

- `GET /api/inbounds?protocol=hy2|vless`
- `POST /api/inbounds`
- `GET /api/inbounds/{id}`
- `PATCH /api/inbounds/{id}`
- `DELETE /api/inbounds/{id}`

Inbound payload:

```json
{
  "id": "vless-default",
  "node_id": "local",
  "name": "VLESS Reality",
  "protocol": "vless",
  "transport": "tcp",
  "security": "reality",
  "host": "example.com",
  "port": 443,
  "enabled": true,
  "params_json": "{\"flow\":\"xtls-rprx-vision\",\"sni\":\"cdn.example.com\",\"pbk\":\"...\",\"sid\":\"...\"}"
}
```

## Subscription tokens

- `GET /api/users/{id}/subscription-token`
- `POST /api/users/{id}/subscription-token/rotate`
- `POST /api/users/{id}/subscription-token/revoke`
- `POST /api/users/{id}/subscription-token/restore`

Token model:

- HMAC-based signature
- stable `subject`
- monotonic `version`
- revoke/restore support

## Unified subscription endpoint

- `GET /api/subscriptions/{token}?format=uri|clash|singbox`
- `GET /subscriptions/{token}?format=uri|clash|singbox`

Response headers:

- `Subscription-Userinfo`
- `Profile-Update-Interval`
- `Profile-Title`

Formats:

- `uri`: plain URI list
- `clash`: Clash YAML with proxy list/group/rules
- `singbox`: Sing-box JSON with `outbounds`

## Errors

Unified endpoints use consistent JSON error envelope:

```json
{
  "error": "message",
  "error_type": "validation|not_found|runtime|sync|service"
}
```

## Core v1 (new backend)

`/api/v1` endpoints are backed by new SQLite `core_*` tables and sing-box render/apply flow:

- `GET|POST /api/v1/servers`
- `GET|PATCH|DELETE /api/v1/servers/{id}`
- `POST /api/v1/servers/{id}/config/render`
- `POST /api/v1/servers/{id}/config/validate`
- `POST /api/v1/servers/{id}/config/apply`
- `GET /api/v1/servers/{id}/config/revisions`
- `POST /api/v1/servers/{id}/config/rollback/{revisionID}`
- `GET|POST /api/v1/inbounds`
- `GET|PATCH|DELETE /api/v1/inbounds/{id}`
- `GET|POST /api/v1/users`
- `GET|PATCH|DELETE /api/v1/users/{id}`
- `GET /api/v1/users/{id}/access`
- `POST /api/v1/access`
- `DELETE /api/v1/access/{id}`
- `GET /api/v1/users/{id}/artifacts`
- `GET /api/v1/users/{id}/artifacts/profile.json`
- `GET /api/v1/users/{id}/artifacts/uris.txt`
- `GET /api/v1/users/{id}/artifacts/qr.png`
- `GET /api/v1/users/{id}/subscription/tokens`
- `POST /api/v1/users/{id}/subscription/tokens`
- `POST /api/v1/users/{id}/subscription/tokens/rotate`
- `POST /api/v1/users/{id}/subscription/tokens/revoke`

Public subscription endpoints:

- `GET /sub/{token}/profile.singbox.json`
- `GET /sub/{token}/uris.txt`
- `GET /sub/{token}/qr.png`
