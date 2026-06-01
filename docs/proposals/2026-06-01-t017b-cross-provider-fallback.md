# T-017b Proposal: Cross-Provider Fallback Retry

Task: T-017b
Date: 2026-06-01
Owner: Codex
Status: Proposed

## Summary

Keep `credentials.Selector.NextForProvider` as the provider-ordering and round-robin component, and add provider/model eligibility at the proxy boundary before an upstream request is sent.

The proxy retry loop should ask the selector for the next credential as it does today, then reject only incompatible cross-provider candidates by consulting a small injected model catalog snapshot. This prevents `deepseek-v4-flash` from being sent to Ark when Ark has no matching `model_catalog` row, while preserving the existing retry and degrade behavior.

No `model_catalog` schema change, selector algorithm change, or `cmd/omnitoken-adopt` change is included.

## Decision 1: Model Catalog Query

Recommendation: use interface-injected in-memory catalog lookup.

Implementation shape:
- Add a small proxy-facing interface, for example:

```go
type ModelCatalog interface {
	LookupProviderModel(ctx context.Context, provider string, model string) (ProviderModel, bool)
}

type ProviderModel struct {
	Provider       string
	CanonicalModel string
	ProviderModel  string
}
```

- `ArkChatConfig` receives this interface as `ModelCatalog`.
- Gateway builds the implementation from active `model_catalog` rows at startup or through the same initialization path that constructs the credential selector.
- Lookup is read-only and in-memory during requests; `internal/proxy` does not hold a DB connection and does not issue SQL.
- Matching accepts the routed/request model against `provider_model`, and may also match `canonical_model` for the same provider to support catalog rows where canonical and provider names are identical.
- The guard applies only when `credential.Provider` differs from the routed provider. Same-provider requests keep the current behavior to avoid changing the normal proxy path.

Reasoning:
- This keeps request latency flat and makes tests deterministic.
- It keeps SQL and catalog loading out of the retry loop.
- It avoids coupling `internal/proxy` to Postgres while still letting gateway wire the production catalog.
- It leaves selector ordering untouched; selector still returns candidates, proxy only decides whether the selected cross-provider credential is compatible with the routed model.

Rejected: inline DB lookup or hard-coded validation in `nextCredential`. That would put storage concerns in the hot retry loop and make the selector/proxy boundary harder to test.

## Decision 2: Model Name Conversion Scope

Recommendation: T-017b performs validation only; provider model conversion moves to v1.1.

For this task, cross-provider fallback is allowed only when the fallback provider already has a catalog row that can accept the routed model name. If the selected fallback credential is incompatible, the proxy skips that credential and continues. If no compatible fallback credential remains, it returns HTTP 400 with:

```text
model X is not available on any configured provider
```

Do not rewrite `payload["model"]` in T-017b.

Reasoning:
- Validation closes the current 404 gap with minimal behavior change.
- Name conversion changes upstream request semantics and affects how `model_routed`, `model_actual`, catalog pricing, and usage attribution should be interpreted.
- Conversion needs a separate design for canonical-model equivalence, response model names, and ledger reporting. That is valid v1.1 work, but not necessary to make fallback safe.

## Decision 3: Integration Mock Design

Recommendation: add test-only `httptest.Server` mocks in `internal/proxy` tests instead of reusing `cmd/loadtest/mockark`.

Implementation shape:
- Build two in-process upstream servers in the test: one DeepSeek-shaped server and one Ark-shaped server.
- Configure `credentials.Selector` with one credential per provider and per-server `BaseURL`.
- Use a fake in-memory `ModelCatalog` implementation to declare whether Ark supports the routed model.
- Assert upstream call counts so incompatible fallback providers are skipped before any HTTP request is sent.

Required tests:
- `TestCrossProviderFallbackAllExcluded`: preferred DeepSeek credential returns retryable 429 or 5xx, gets excluded/degraded, catalog says Ark supports the routed model, proxy retries Ark and returns 200.
- `TestCrossProviderFallbackModelNotAvailable`: preferred provider is no longer usable, Ark is selected but catalog says it does not support the routed model, proxy skips Ark and returns 400; Ark upstream call count remains zero.

Reasoning:
- The tests exercise the real proxy retry loop plus selector with no ports, Docker, or external services.
- Provider-specific behavior is explicit in each test.
- No new dependency is needed.

Optional e2e can remain a follow-up behind the existing `e2e` build tag if Claude wants a deployed-chain test. It should not block T-017b closure because the mock integration tests cover the target fallback path deterministically.

## Observability Plan

Emit one WARN log when the proxy actually switches from the routed provider to a compatible fallback provider. Required fields:

- `request_id`
- `from_provider`
- `to_provider`
- `model_requested`
- `model_routed`
- `credential_id`
- `reason`

Use the existing `p.logger.Warn(..., attrs...)` style from `logCredentialRetry`.

Reason values:
- `all_excluded`: the preferred provider has no eligible credential left because attempted credentials are in the retry-loop exclude set.
- `all_degraded`: the preferred provider exists but selector diagnostics show its active credentials are currently degraded.
- `preferred_empty`: no active healthy credential exists for the preferred provider before fallback.

To keep reasons accurate without changing selector ordering, add only a read-only diagnostics helper on `credentials.Selector` if needed. It should report provider availability counts under the same lock used by selection, but must not alter positions, priorities, provider order, or degraded state.

## Retry Loop Behavior

The retry loop should track both attempted credentials and compatibility-skipped credentials so it cannot repeatedly select the same incompatible fallback credential. Compatibility skip is not an upstream failure and should not call `MarkDegraded`.

If catalog lookup rejects a cross-provider credential:
- add its credential ID to the local skip/exclude set;
- continue the loop without sending HTTP;
- return 400 only after no compatible candidate remains.

If a compatible cross-provider credential is selected:
- emit the WARN fallback log once per request/provider switch;
- send the upstream request normally;
- keep existing retry behavior for 429, 5xx, connection errors, and stream read errors.

## Test Plan

- Add proxy integration tests for the two required fallback paths.
- Add selector diagnostics unit coverage only if the read-only helper is introduced.
- Keep default tests mock-only; no real upstream or Docker dependency.
- Run `go vet ./...` and `go test ./...`.

## Dependencies

No new third-party dependencies are needed.
