# V1 Concurrency Rerun - 2026-05-21

## Scope

T-CONC-RERUN re-ran the v1 gateway after T-016 and the T-CONC-DSN preflight.
This run added only loadtest/mock measurement tooling; it did not change
`internal/*`, `cmd/gateway`, or `cmd/admin`.

## Methodology

- Stack: `deploy/docker-compose.yml`
- Mock upstream: `cmd/loadtest/mockark`, built by `deploy/Dockerfile.mockark`
- Mock upstream URL: `http://mock-ark:8090/api/coding/v3`
- Gateway/admin DSN tagging: `9b44f98b`
- `pg_stat_statements`: enabled through compose Postgres preload and
  `CREATE EXTENSION IF NOT EXISTS pg_stat_statements`
- Mock run seed path: migrate -> seed -> credential-seed -> gateway restart
- PG container was recreated to apply preload; the existing named volume was
  reused. No volume deletion was performed.
- Mock credential seed used a deterministic non-secret dev master key and
  three fake mock keys. These rows were removed after the mock run.
- Planned paid Ark budget: `MAX_REQUESTS=900` for `30 x 30s`.

M-26 note: if the compose Postgres volume is recreated for a future paid Ark
rerun, repeat migrate -> seed -> credential-seed -> gateway restart, and keep
`OMNITOKEN_MASTER_KEY_FILE`/`OMNITOKEN_MASTER_KEY` identical to the T-016 seed
key. Otherwise existing encrypted Ark credentials cannot be decrypted.

## Mock Baseline

Each profile ran for 60s through the real gateway/auth/quota/usage path.

| Concurrency | Requests | RPS | 2xx | 5xx | Success | P50 | P95 | P99 | Max |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 50 | 12,681 | 210.6 | 12,672 | 9 | 99.9% | 211ms | 484ms | 639ms | 1.202s |
| 100 | 17,016 | 281.5 | 9,742 | 7,274 | 57.3% | 229ms | 1.052s | 1.443s | 2.467s |
| 200 | 21,393 | 352.1 | 8,746 | 12,647 | 40.9% | 437ms | 1.496s | 2.033s | 3.428s |

Gateway 5xx was caused by Postgres connection exhaustion, not mock upstream
429s. Gateway logs include `pq: sorry, too many clients already` from quota
checks and async usage writes. `upstream_429=0` in all three profiles.

## DB Observation

`pg_stat_activity` tagging worked after T-CONC-DSN. During the mock runs:

| Profile | Peak observed gateway conns | Admin conns | Notes |
| ---: | ---: | ---: | --- |
| 50 | 77 | 1 | includes active + idle + idle in transaction |
| 100 | 96 | 1 | one sample failed later with too many clients |
| 200 | 91 | 1 | sampling itself can be refused at saturation |

`monthlyBudgetStatusSQL` was the dominant hot query:

| Profile | Calls | Mean | Max |
| ---: | ---: | ---: | ---: |
| 50 | 12,672 | 170.100ms | 975.578ms |
| 100 | 9,859 | 391.165ms | 2.265s |
| 200 | 8,908 | 464.518ms | 2.062s |

Container stats after the 200 profile: gateway `35.88MiB` and 98 PIDs,
Postgres `107.6MiB`, mock-ark `9.94MiB`.

## True Ark Rerun

The paid true Ark multi-key profile did not run. The local `.env` contains one
fallback `OMNITOKEN_ARK_API_KEY`, but it does not contain
`OMNITOKEN_MASTER_KEY_FILE`/`OMNITOKEN_MASTER_KEY` or any
`OMNITOKEN_ARK_KEYS*` values required by `credential-seed`.

Running a paid `30 x 30s` test against a single fallback key would repeat the
old T-CONC-CHECK shape and would not validate T-016 multi-key pooling. Required
preconditions before retrying:

1. Set the same master key source used for credential seeding.
2. Set three Ark seed keys through `OMNITOKEN_ARK_KEYS` or numbered
   `OMNITOKEN_ARK_KEYS_1..3`.
3. Run `credential-seed`.
4. Restart gateway.
5. Run loadtest with `MAX_REQUESTS=900`.

## Comparison With T-CONC-CHECK

T-CONC-CHECK found `428/2500` success (`17.1%`) on a single Ark key with zero
gateway panic/timeout/5xx. This rerun proves a different bottleneck: when Ark
is removed via mock upstream, the current gateway stack saturates Postgres
quota/usage paths before it meets the mock target of P99 <= 100ms and 5xx <=
0.1%.

## V2 Candidate Fixes

- Add explicit `sql.DB` pool limits and backpressure so gateway cannot open
  enough concurrent Postgres work to hit server `max_connections`.
- Optimize or cache `monthlyBudgetStatusSQL`; it is already a confirmed input
  for T-QUOTA-CACHE-PROBE.
- Revisit upstream credential priority semantics: the mock run had three
  active credentials, but usage aggregated to one credential ID because all
  seeded rows use increasing priorities and the selector exhausts priority 1
  before lower-priority fallback rows.
- Retry true Ark multi-key validation after the three seed keys and master key
  source are configured.
