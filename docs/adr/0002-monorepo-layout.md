# ADR 0002: Monorepo Layout

## Context

The project needs separate data-plane, control-plane, worker, and scheduler
entry points while keeping shared domain packages private until they are stable.

## Decision

Use a Go monorepo with `cmd/` for binaries, `internal/` for private application
packages, `pkg/` reserved for future stable APIs, `deploy/` for local and
container assets, `migrations/` for SQL migrations, and `docs/` for ADRs,
OpenAPI contracts, and runbooks.

## Consequences

This keeps future service extraction possible without prematurely publishing
shared APIs. New internal packages must include a short README explaining their
responsibility before production code grows around them.
