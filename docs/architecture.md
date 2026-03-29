# Architecture

## Scope

The panel manages only one runtime data-plane service:

- Hysteria 2 (`UDP`, configurable port, default `443`)

The panel itself is split into:

- `panel-api` (`127.0.0.1:18080`)
- `panel-web` (`127.0.0.1:13000`)
- `caddy` (public HTTPS entrypoint)

## Persistence model

The application is file-backed and does not use a database.

Primary storage roots:

- `/var/lib/h2v2/state`
  - admins
  - sessions
  - hysteria users
  - service-state cache
  - metadata counters
- `/var/lib/h2v2/snapshots/hy2`
  - per-user traffic/online snapshots
- `/var/lib/h2v2/snapshots/system`
  - host CPU/RAM/network trend snapshots for dashboard history
- `/var/lib/h2v2/backups`
  - saved config backups
- `/var/log/h2v2/audit`
  - audit trail entries

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
