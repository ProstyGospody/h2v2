# Architecture

## Scope

The panel controls sing-box runtime for both protocols:

- Hysteria 2 (`hy2`)
- VLESS (`vless`)
- Sing-box core (`core v1`) for VLESS + Hysteria2 unified generation

Control plane services:

- `panel-api` (`127.0.0.1:18080`)
- `panel-web` (`127.0.0.1:13000`)
- `caddy` (public HTTPS entrypoint)

## New core v1

`core v1` is implemented on top of separate SQLite `core_*` tables and includes:

- server/inbound/user/access CRUD
- hashed subscription tokens with revoke/rotate
- URI/profile/QR generation
- sing-box config revisioning with `render -> check -> apply`

## Domain model

Protocol-agnostic entities:

- `User`: lifecycle, traffic limit, expiry, enabled state
- `Credential`: per-protocol secret/identity (`hy2 password + uuid identity`, `vless uuid`)
- `Node` + `Inbound`: runtime protocol/transport/security parameters
- `SubscriptionToken`: subject, version, revoked/rotated state

## Persistence model

SQLite storage (single DB file, WAL mode) now includes unified tables:

- `users`
- `credentials`
- `nodes`
- `inbounds`
- `subscription_tokens`
- `traffic_counters`
- `runtime_user_state`

## Data migration

On startup, SQLite schema is migrated to `user_version=2` if needed and default `hy2`/`vless` inbounds are created if absent.

## Runtime adapter layer

Runtime operations are mediated by protocol adapters:

- `SingBoxAdapter` (primary sing-box runtime/config source)
- `SingBoxHY2Adapter` (HY2 stats/artifacts via the same sing-box runtime state)

Adapter contract:

- `SyncConfig`
- `AddUser`
- `UpdateUser`
- `RemoveUser`
- `SetUsersStateBatch`
- `KickUser`
- `CollectTraffic`
- `CollectOnline`
- `BuildArtifacts`

`UserManager` orchestrates repository + adapters with rollback-first behavior when runtime sync fails.

## Subscription engine

Unified subscription endpoint:

- `/api/subscriptions/{token}`
- `/subscriptions/{token}`

Renderer outputs:

- URI list
- Clash YAML
- Sing-box JSON

Token format is HMAC-signed with stable subject and versioning; rotate/revoke are supported.

In-memory cache is invalidated on user/inbound/token mutation.

## Service control

Service actions are restricted to `MANAGED_SERVICES`.

Default list:

- `h2v2-api`
- `h2v2-web`
- `sing-box`

Runtime startup and periodic poll collection run through scheduler + adapters.
