# Control Plane Workflows

## Users

- Users table keeps the existing list flow and adds row selection, bulk toolbar, access-aware details, and operator filters.
- Bulk user actions use preview first, then create or update a draft revision when runtime state changes.
- User details now separate `User`, `Access`, and `Artifacts`.
- User delete and access delete are separate actions.

## Settings

- Settings keeps the existing `Server`, `VLESS`, `Hysteria2`, and `Config` areas.
- New `Profiles` and `Policies` tabs manage client, DNS, reality, transport, multiplex, HY2 masquerade, log, TLS, route rules, and outbounds.
- Each managed entity shows usage counters for users, access, inbounds, and runtime/artifact impact.
- Unsafe delete is blocked when the entity is still in use.
- Client profile changes refresh dependent artifacts without forcing a runtime draft.

## Revision Flow

- Runtime-affecting changes now move to a draft revision instead of silent apply.
- The main operator flow is `change -> preview -> draft -> validate -> apply`.
- Settings shows current revision, draft revision, validate status, and apply status.
- Users bulk actions surface pending drafts and allow validate/apply from the UI.

## Artifacts

- Subscription token stays stable unless rotated.
- Artifacts and subscription outputs are marked dirty when related user, access, inbound, or profile state changes.
- Runtime profile and policy changes also invalidate dependent subscription outputs before apply.
- User details show artifact refresh state, token prefix, and last render time.
- Manual artifact refresh rebuilds outputs from current state.

## Changelog

### Reused

- Existing `Users` page list, search, paging, create/edit dialog, and details entry point.
- Existing `Settings` page structure, save bar pattern, and config preview section.
- Existing UI primitives for dialogs, drawers, toggles, selects, and badges.

### Extended

- Users list with operational filters, selection-driven bulk flows, draft visibility, and clearer user/access separation.
- Settings with profile references in inbound forms, revision controls, and embedded profile/policy management.
- Backend user listing with subscription and artifact visibility.
- Existing mutation handlers to render draft revisions without immediate apply.

### Added

- Bulk impact/apply endpoints for users and access.
- Policy usage queries and TLS profile CRUD.
- Artifact dirty-state tracking, stable primary token handling, and manual artifact refresh endpoint.
- Generic Settings entity manager for profile and policy CRUD with usage-aware delete flow.
