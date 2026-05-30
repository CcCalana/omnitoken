# PROPOSAL: T-CONC-RERUN Concurrency Baseline Rerun

refs T-CONC-RERUN.

## Decision 1: mock upstream form

Choose **docker-compose mock service**.

Add a small standard-library Go binary under `cmd/loadtest/mockark` and wire it
as a `mock-ark` service in `deploy/docker-compose.yml`. The service returns a
minimal OpenAI-compatible non-stream chat completion, sleeps only if configured,
never emits 429, and reports a lightweight `/healthz`.

Rationale: the measurement must exercise gateway HTTP, Docker networking,
connection pooling, usage parsing, and quota paths in the same topology as the
real stack. An in-process `httptest` server would mostly measure the load
driver, while a WireMock container adds a dependency and configuration surface
without improving this baseline.

Scope guardrail: the mock exists only for measurement. It must not change
`internal/*`, `cmd/gateway`, or `cmd/admin`, and it must not become production
routing code.

## Decision 2: concurrency levels and duration

Choose **50 / 100 / 200 concurrency, each with 10s warmup + 60s measured
steady-state**.

Update only `cmd/loadtest/` as needed so it can run by duration, discard warmup
samples, report P50/P95/P99, classify 429 separately, and write a machine-readable
summary. Keep the existing bounded request mode for T-CONC-CHECK reproducibility.

Do not add a 500-concurrency spike in the first rerun. The acceptance gate asks
for a stable curve, and a spike would blur whether failures are startup surge,
gateway headroom, quota SQL, or client-side saturation. If 200 concurrency passes
with P99 well under 100ms and gateway 5xx at 0, list a 500-concurrency spike as a
V2 candidate rather than spending this task on it.

For real Ark, choose **30 concurrency x 30s** with `MAX_REQUESTS` capped to the
observed request budget before starting the run. This directly answers whether
T-016 pool retry improved the 17.1% result without turning the test into a long
rate-limit search.

## Decision 3: T-CONC-DSN ordering

Recommend **front-load T-CONC-DSN as a separate small commit before the rerun**.

The change should only append `application_name=omnitoken-gateway` and
`application_name=omnitoken-admin` to the gateway/admin DSNs, plus document the
sampling query in `cmd/loadtest/README.md`. It should remain its own
T-CONC-DSN commit and must not be folded into T-CONC-RERUN.

Rationale: T-CONC-CHECK already lost the DB connection-pool sample because the
`pg_stat_activity` filter had no stable application name. Repeating the rerun
with known-broken sampling would produce another partial report precisely where
T-QUOTA-CACHE-PROBE needs baseline data.

Trade-off: this is a code change outside the measurement-only task, so it needs
explicit review acknowledgement before running T-CONC-RERUN. The benefit is that
the measurement report can answer gateway/admin connection peaks instead of
carrying the same H-3 caveat forward.

## Decision 4: pg_stat_statements

Choose **enable pg_stat_statements in docker-compose Postgres for this run**.

Update `deploy/docker-compose.yml` Postgres settings to preload
`pg_stat_statements`, then have the measurement procedure create the extension
after migrations. Capture at least the quota-path query mean/max timings and
call counts during the mock run.

Rationale: this is deploy/test infrastructure, not production code, and it gives
T-QUOTA-CACHE-PROBE a real baseline for `monthlyBudgetStatusSQL`. Offline
`EXPLAIN ANALYZE` is acceptable as a fallback only if the extension fails to
load, because it cannot reflect concurrent gateway pressure or connection-pool
behavior.

Operational note: enabling the extension requires a Postgres restart and may
require recreating the local compose volume if preload state is stale. The report
should state the exact reset/restart path used.

## Decision 5: report location

Choose **new release report**:
`docs/release/v1-concurrency-rerun-2026-05-22.md`.

Keep `docs/release/v1-concurrency-baseline-2026-05-21.md` as the historical
single-key/Ark-rate-limit snapshot. The rerun has a different purpose: mock
gateway capacity plus multi-key Ark validation. Splitting the files keeps the
comparison explicit and avoids rewriting the evidence trail behind R-CONC-CHECK.

## Implementation notes

- Expected code-write scope for T-CONC-RERUN: `cmd/loadtest/` and
  `deploy/docker-compose.yml` only.
- Expected report scope: `docs/release/v1-concurrency-rerun-2026-05-22.md`.
- Any new bottleneck goes into the report's V2 candidates, not into
  `internal/*`, `cmd/gateway`, or `cmd/admin`.
- No new third-party dependency is needed.
- The real Ark run must use the existing local secret path without printing key
  material, request headers, or `.env` contents.
