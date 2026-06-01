# T-100 Proposal: L2 Multi-Tenant Correctness Runner

Task: T-100
Date: 2026-06-01
Owner: Codex
Status: Proposed

## Summary

Build `cmd/e2e-runner` plus `test/e2e/l2_test.go` as a true deployed-environment L2 suite. The runner drives the public gateway/admin HTTP surfaces for behavior checks, and uses a direct Postgres connection only for test fixture gaps and post-run ledger/attribution assertions.

No production gateway/admin/internal code changes are included in T-100. If Claude rejects direct Postgres verification or seed-user fixture mode, T-100 needs a separate prerequisite API task.

## Decision 1: User Data Preparation

Recommendation: use the existing seeded demo users, then have the runner create virtual keys and budgets.

Reasoning:
- Current admin API has `GET /api/admin/users`, `PATCH /api/admin/users/{id}/quota`, and `POST /api/admin/dev/virtual-keys`, but no create-user endpoint.
- T-100 explicitly says not to modify `internal/`, `cmd/gateway`, or `cmd/admin`, so a self-contained "create 10 users" runner would require out-of-scope backend work.
- `deploy/postgres/002_seed.sql` already defines the exact L2 shape: one admin, one viewer, and nine members in the demo organization.

Implementation shape:
- Runner discovers `user01` through `user10` via `GET /api/admin/users`.
- Runner creates fresh virtual keys through `POST /api/admin/dev/virtual-keys`.
- Runner sets heterogeneous budgets through `PATCH /api/admin/users/{id}/quota`; one selected user gets budget `0`.
- Viewer RBAC uses seeded `user01@democorp.local` credentials.
- Member RBAC requires direct PG fixture setup to assign a temporary password hash to one seeded member, because seeded members currently have no password.

Rejected for T-100: adding admin user creation. That is valid later, but it is a production API change and should be a separate task.

## Decision 2: Upstream Selection

Recommendation: DeepSeek-only.

Reasoning:
- The current deployment direction is DeepSeek-only for this stability pass.
- T-100 is a correctness gate, not a provider fallback or routing diversity test.
- Keeping one provider lowers cost variance and makes ledger closure easier to interpret.

Implementation shape:
- Runner accepts `--deepseek-api-key` / `OMNITOKEN_DEEPSEEK_API_KEY` for compatibility with the task contract and optional direct upstream preflight.
- Runner does not print or persist the key.
- Runner defaults to an already-configured deployed DeepSeek credential pool. If Claude wants the runner to create or rotate admin credentials from `--deepseek-api-key`, that should be explicitly approved because it mutates deployed credential state.

## Decision 3: Ledger Verification

Recommendation: direct Postgres verification.

Reasoning:
- The task-recommended `/api/admin/usage/summary` endpoint does not currently exist.
- Existing `GET /api/admin/users/{id}/usage` is useful for UI summaries, but it does not expose enough data to assert `api_key_id`, `model_routed`, or `upstream_credential_id`.
- Ledger closure needs exact rows from `usage_events`, token breakdown tables, and `cost_ledger`, filtered to the runner's generated virtual keys and execution window.

Implementation shape:
- Add `--database-url` / `OMNITOKEN_TEST_DATABASE_URL`; full T-100 mode requires it.
- Filter assertions by created virtual key IDs plus a `[run_start, run_end]` time window.
- Compare `SUM(cost_ledger.cost_usd)` against usage-derived cost, with the task's `<= 1%` tolerance.
- Sample at least three non-budget-zero users and assert `user_id`, `api_key_id`, `model_routed`, and `upstream_credential_id` are present and match the prepared key/user set.

Rejected for T-100: weakening to admin overview/user-usage APIs. That would leave the attribution acceptance criteria unverified.

## Runner Contract

Flags/env:
- `--gateway-url` / `OMNITOKEN_GATEWAY_URL`
- `--admin-url` / `OMNITOKEN_ADMIN_URL`
- `--admin-token` / `OMNITOKEN_ADMIN_TOKEN`
- `--database-url` / `OMNITOKEN_TEST_DATABASE_URL`
- `--deepseek-api-key` / `OMNITOKEN_DEEPSEEK_API_KEY`
- `--max-requests` / `MAX_REQUESTS`, default `50`
- `--model`, default `chat-fast`
- `--max-tokens`, default `32`

Behavior:
- Refuse `max_requests < 30`, because 10 users need at least three attempts each.
- Print estimated cost before sending upstream requests.
- Run 10 goroutines, one per user, with mixed stream and non-stream calls and at least one stream request per user.
- Treat gateway 5xx as a hard failure.
- Require all budget-zero requests to return 402.
- Require non-budget users to reach at least 90% success, allowing upstream 429 as non-gateway failure.
- Build and run the runner under `-race`.

## E2E Test Shape

`test/e2e/l2_test.go` uses build tag `e2e` and shells out to the runner with the same env/flags. It skips when required env is absent, matching existing e2e style, and remains out of default `go test ./...`.

## Open Approval Points

1. Approve seed-user mode instead of runner-created users for T-100.
2. Approve direct Postgres as required for full ledger/attribution assertions.
3. Confirm `--deepseek-api-key` is parse/preflight-only, not an instruction to mutate deployed credential records.
