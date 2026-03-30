# Architecture

## Scope

The panel manages only one runtime data-plane service:

- Hysteria 2 (`UDP`, configurable port, default `443`)

The panel itself is split into:

- `panel-api` (`127.0.0.1:18080`)
- `panel-web` (`127.0.0.1:13000`)
- `caddy` (public HTTPS entrypoint)

## Persistence model

SQLite storage:

- single DB file (default `/var/lib/h2v2/data/h2v2.db`)
- WAL mode enabled
- foreign keys enabled
- busy timeout enabled
- indexed read paths:
  - sessions by token/expiry
  - users by normalized username
  - snapshots by user/timestamp
  - audit logs by timestamp

## Hysteria configuration ownership

- Active config path: `/etc/h2v2/hysteria/server.yaml`
- Managed auth mode is `userpass`
- User credentials are sourced from panel-managed Hysteria users
- During validate/save/apply, panel-managed auth is injected to prevent drift

## Runtime and metrics

- Hysteria live stats endpoint is loopback-only (`127.0.0.1:${HY2_STATS_PORT}`)
- Scheduler polls:
  - Hysteria traffic/online snapshots
  - managed service statuses
- Host metrics are collected via local procfs readers

## Service control

Service actions are mediated through restricted sudo rules and limited to configured `MANAGED_SERVICES`.
Default:

- `h2v2-api`
- `h2v2-web`
- `hysteria-server`
