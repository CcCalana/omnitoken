# ADR 0001: Locked Phase 0 Technology Stack

## Context

OmniToken needs a high-concurrency gateway foundation for streaming AI API
traffic, key management, quotas, billing, and audit trails. The planning
document locks the initial stack to Go, PostgreSQL, Redis, NATS JetStream,
OpenTelemetry, and Docker-based local development.

## Decision

Phase 0 uses Go 1.23+ and the standard library for the first runnable gateway
and admin services. Third-party Go dependencies are intentionally deferred until
their owning implementation tasks document and approve them. Local infrastructure
is represented with Docker Compose using PostgreSQL 16, Redis 7, and NATS with
JetStream enabled.

## Consequences

The scaffold can run tests without network access or dependency downloads. Later
tasks will introduce `chi`, `viper`, `sqlc`, `pgx`, `golang-migrate`, and OTel
packages in smaller reviewed changes.
