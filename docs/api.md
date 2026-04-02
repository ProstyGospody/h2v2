# API

## Compatibility

Legacy HY2 endpoints stay available and keep response/error behavior:

- `/api/hysteria/users/*`
- `/api/hysteria/settings/*`
- `/api/hysteria/stats/*`
- `/api/hysteria/subscription/{token}`

New endpoints are protocol-agnostic and operate on HY2 and VLESS in one model.

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
    { "protocol": "hy2", "identity": "alice", "secret": "secret-value" },
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
