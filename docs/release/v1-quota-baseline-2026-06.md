# V1 Quota Baseline - 2026-06-01

## Scope

T-QUOTA-CACHE-PROBE measures the current Postgres quota check path only. It did
not change `internal/`, `cmd/gateway`, or `cmd/admin`.

## Methodology

- Stack: `deploy/docker-compose.yml`
- Gateway/admin image: local build from current tree
- Postgres: compose `postgres:16-alpine` with `pg_stat_statements`
- Mock upstream: `cmd/loadtest/mockark` on the host, reached by gateway through
  `http://host.docker.internal:8090`
- Test key: seed demo user virtual key created through admin dev endpoint
- Model: `ark-code-latest`
- Profiles: concurrency 10 / 30 / 50, each with 10s warmup and 60s measurement
- Request budget: `MAX_REQUESTS=200000`
- Raw local outputs: `/private/tmp/omnitoken-quota-probe`

Docker note: Docker Desktop initially failed to pull images because its internal
proxy attempted `10.23.0.1:8080` and timed out. Host and container direct
registry checks were healthy. Restarting Docker Desktop and retrying pulls
restored postgres/redis/nats images. `alpine:3.20` still hit the proxy path, so
mockark was run from the host instead of the compose `mock-ark` image. Gateway
still used the real auth/quota/usage/proxy middleware stack.

## Gateway Results

| Concurrency | Requests | RPS | 2xx | 5xx | Success | P50 | P95 | P99 | Max |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 10 | 24,143 | 402.3 | 24,143 | 0 | 100.0% | 24ms | 43ms | 52ms | 84ms |
| 30 | 15,801 | 263.0 | 15,801 | 0 | 100.0% | 106ms | 211ms | 257ms | 393ms |
| 50 | 11,965 | 198.9 | 11,965 | 0 | 100.0% | 237ms | 482ms | 566ms | 776ms |

Loadtest runtime end-state:

| Concurrency | Runtime goroutines | Runtime heap alloc |
| ---: | ---: | ---: |
| 10 | 7 | 2,313,760 bytes |
| 30 | 7 | 3,168,168 bytes |
| 50 | 7 | 1,778,328 bytes |

## DB Samples

`monthlyBudgetStatusSQL` was sampled three times per measurement window after
resetting `pg_stat_statements` at the end of warmup.

| Concurrency | Sample | Calls | Mean | Max |
| ---: | ---: | ---: | ---: | ---: |
| 10 | 1 | 4,683 | 24.218ms | 66.723ms |
| 10 | 2 | 14,595 | 14.040ms | 66.723ms |
| 10 | 3 | 22,489 | 14.080ms | 66.723ms |
| 30 | 1 | 4,542 | 73.027ms | 217.544ms |
| 30 | 2 | 10,029 | 80.626ms | 300.322ms |
| 30 | 3 | 14,857 | 87.310ms | 310.953ms |
| 50 | 1 | 3,234 | 193.067ms | 554.396ms |
| 50 | 2 | 7,335 | 202.572ms | 554.396ms |
| 50 | 3 | 11,237 | 210.586ms | 670.831ms |

Peak sampled gateway connections:

| Concurrency | Active | Idle | Idle in transaction | Admin |
| ---: | ---: | ---: | ---: | ---: |
| 10 | 11 | 3 | 3 | 1 |
| 30 | 30 | 6 | 5 | 1 |
| 50 | 49 | 9 | 6 | 1 |

Final container snapshots:

| Concurrency | Gateway memory / PIDs | Postgres memory / PIDs | Admin memory / PIDs |
| ---: | --- | --- | --- |
| 10 | 22.65MiB / 19 | 66.29MiB / 9 | 6.98MiB / 8 |
| 30 | 21.95MiB / 19 | 94.04MiB / 9 | 6.98MiB / 8 |
| 50 | 22.11MiB / 19 | 99.09MiB / 9 | 6.977MiB / 8 |

## Conclusion

Quota check is a latency bottleneck, but not a correctness or availability
failure in the tested 10/30/50 profiles.

The bottleneck begins to matter at 30 concurrency: gateway P99 moves from 52ms
to 257ms while `monthlyBudgetStatusSQL` mean rises to 87ms. At 50 concurrency it
dominates the profile: gateway P99 is 566ms and quota SQL mean reaches 211ms.
Despite that, all three profiles completed with 100% 2xx and 0 gateway 5xx, so
the current implementation is stable through 50 mock concurrency on this local
environment.

## V2 Candidates

- Add a Redis-backed monthly budget cache or pre-aggregated usage counter.
- Rework quota SQL away from per-request `LEFT JOIN usage_events` plus
  `cost_ledger` aggregation.
- Add a lightweight gateway diagnostics endpoint if future probes need real
  gateway goroutine counts instead of process/PID-level snapshots.
