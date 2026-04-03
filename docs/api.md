# API

## Auth

- `POST /api/auth/login`
- `GET /api/auth/me`
- `POST /api/auth/logout`

## Services and system

- `GET /api/services`
- `GET /api/services/{name}`
- `POST /api/services/{name}/restart`
- `POST /api/services/{name}/reload`
- `GET /api/system/live`
- `GET /api/system/history`

## Storage

- `GET /api/storage/sqlite/backup`
- `POST /api/storage/sqlite/restore`

## Core v1

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
- `GET /api/v1/users/{id}/artifacts/profile.raw`
- `GET /api/v1/users/{id}/artifacts/uris.txt`
- `GET /api/v1/users/{id}/artifacts/qr.png?kind=subscription|access`

- `GET /api/v1/users/{id}/subscription/tokens`
- `POST /api/v1/users/{id}/subscription/tokens`
- `POST /api/v1/users/{id}/subscription/tokens/rotate`
- `POST /api/v1/users/{id}/subscription/tokens/revoke`

`GET /api/v1/users` returns `items[]` where each item has:

- `user`
- `access[]`

Each `access` item includes `protocol` (`hy2|vless`) and credential fields.

## Public subscription endpoints

- `GET /sub/{token}/profile.singbox.json`
- `GET /sub/{token}/uris.txt`
- `GET /sub/{token}/qr.png`

## Errors

```json
{
  "error": "message",
  "error_type": "validation|not_found|runtime|service|rate_limit"
}
```
