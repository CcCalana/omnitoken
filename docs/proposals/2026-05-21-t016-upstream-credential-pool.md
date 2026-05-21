# PROPOSAL: T-016 Upstream Credential Pool

refs T-016.

## Decision 1: master key source

Choose **file first, env fallback**:

- `OMNITOKEN_MASTER_KEY_FILE` points to a file containing one 32-byte master key encoded as 64 lowercase/uppercase hex chars.
- `OMNITOKEN_MASTER_KEY` remains supported as a local-dev and test fallback.
- If both are set, the file wins. The process reads the key once at startup, validates length, then keeps only the decoded bytes in memory.

Rationale: `规划.md` §零A requires encrypted upstream credentials, and ADR 0003 allows env injection for v1. A mounted secret file gives the same v1 simplicity while avoiding a literal master key in `deploy/docker-compose.yml`. The env fallback keeps unit tests and one-off local smoke commands simple without adding KMS scope.

Operational guardrail: logs and error messages may say `master key file missing`, `invalid master key length`, or `invalid master key encoding`, but must never print the path contents, key value, or any derived prefix.

## Decision 2: credential loading and invalidation

Choose **startup load into memory**, with **restart-based reload for v1**.

At gateway startup, load active Ark credentials from Postgres, decrypt once, sort by priority/weight, and keep decrypted secrets in the selector. Per-request DB reads and per-request decrypt are intentionally rejected because they put crypto and database latency on the hot path and increase master-key exposure frequency.

For v1, runtime credential changes are seed/migration/ops-only, so reload is `restart gateway`. Admin CRUD UI and live mutation are explicitly v1.1 scope; that task should add either periodic `updated_at` polling or PG NOTIFY. Do not add a partial SIGHUP-only mechanism in T-016 because it is Linux-centric, hard to test on the current Windows dev host, and not needed while v1 has no credential write API.

## Decision 3: 429 backoff

Choose **fixed 30s in-memory degradation** for 429.

When a credential returns 429, mark it `degraded_until = now + 30s` in the selector and retry with the next eligible credential, up to the task limit of 2 retries. This matches the R-CONC-CHECK preflight signal that a single Ark coding-plan key is rate-limited around a low single-digit RPS range; immediately reusing that key is wasteful.

Do not use exponential backoff in v1. Exponential state is harder to reason about with only 2-3 keys and can over-quarantine the pool after short bursts. A fixed TTL is easier to test deterministically and can be promoted later if real traffic shows repeated 429 waves.

5xx responses trigger per-request fallback, but v1 should not persistently quarantine on a single 5xx. Persisted `status` / `health_state` remain operator-controlled DB fields; hot-path degradation is in memory and WARN-logged with credential id only.

## Decision 4: SSE retry boundary

Choose **retry before the first client-visible byte only**.

For streaming requests, the proxy should:

1. Pick a credential and send the upstream request.
2. If upstream status is 429/5xx before a valid event-stream response, close that response body and retry another credential.
3. If status is 2xx event-stream, read and buffer the first chunk before writing the downstream header.
4. If the first read fails before any downstream write, retry within the same 2-retry budget.
5. Once headers or any chunk are written to the client, never switch credentials for that request.

The current proxy already delays downstream header write until after the first SSE read succeeds, so T-016 should preserve and make that behavior explicit in retry tests. A mid-stream retry would duplicate partial completions and corrupt the client contract.

## Decision 5: metadata jsonb usage

Choose **schema reserved, optional seed metadata only**.

`upstream_credentials.metadata` should default to `{}` and must not participate in v1 routing. Seed tooling may accept non-secret operational labels such as `alias`, `ark_user_id`, or `purchase_owner` for future admin display, but no field is required and no metadata value may be used as a secret, token, prompt, or billing source of truth.

The admin list endpoint may return metadata only if it is already stored as non-secret operator metadata. Logs should prefer `credential_id`, `provider`, `priority`, and `health_state`, not arbitrary metadata fields.

## Implementation notes

- No new third-party dependencies are needed for the proposal decisions.
- T-016 and T-CONC-COST-ATTR should share the next usage migration: add `usage_records.upstream_credential_id` / `usage_events.upstream_credential_id` and `model_routed` in one up/down pair if the current schema still uses `usage_events` as the physical table.
- The selector should expose a deterministic clock in tests so 30s degradation can be asserted without sleeps.
- Secret leakage tests should grep stdout/stderr/log capture for known fake key material, but never inspect real `.env` or real user credential files.
