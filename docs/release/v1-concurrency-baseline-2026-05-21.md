# V1 Concurrency Baseline - 2026-05-21

## Scope

T-CONC-CHECK measured the current v1 gateway under local Docker Compose. This
run did not change production code or tune database/runtime settings.

Environment:

- Stack: `deploy/docker-compose.yml`
- Gateway: `http://localhost:8080`
- Admin API: `http://localhost:8081`
- Real upstream path: `chat-fast`
- Health path: `GET /healthz`

## Preflight

Before the paid run, `virtual_models` was checked in Postgres:

```text
chat-fast -> kimi-k2.6 (active)
```

A single `chat-fast` request completed successfully in `1347ms`. The latest
usage row recorded:

```text
model_requested = chat-fast
model_actual    = deepseek-v4-pro
total_tokens    = 15
status_code     = 200
```

`model_actual` is the model string returned by Ark. The routing target evidence
for the gateway is the `virtual_models` row above; the gateway rewrites
`chat-fast` to that `real_model` before forwarding.

The demo admin user's monthly budget was cleared before the run so quota did
not mask upstream behavior.

## Results

### Real upstream: 50 concurrency x 50 requests

Command shape:

```powershell
$env:MAX_REQUESTS = "2500"
go run .\cmd\loadtest `
  -concurrency 50 `
  -requests 50 `
  -gateway http://localhost:8080 `
  -admin http://localhost:8081 `
  -model chat-fast `
  -timeout 60s
```

Summary:

| Metric | Value |
| --- | ---: |
| Total requests | 2500 |
| 2xx | 428 |
| 4xx | 2072 |
| 5xx | 0 |
| Timeouts | 0 |
| Other client errors | 0 |
| Success rate | 17.1% |
| P50 latency | 737ms |
| P95 latency | 1.798s |
| P99 latency | 2.415s |
| Max latency | 4.021s |
| Usage tokens recorded by loadtest | 7396 |

Gateway logs classify all 4xx responses as upstream `429` from Ark. No gateway
panic, timeout, or 5xx was observed during this run.

### Gateway healthz: 1000 RPS for 60s

Vegeta was not installed on this Windows host, so a temporary Go load driver in
`C:\tmp` was used against the same target shape.

Summary:

| Metric | Value |
| --- | ---: |
| Configured rate | 1000 RPS |
| Duration | 60s |
| Completed requests | 59770 |
| Actual throughput | 996.2 RPS |
| 2xx | 59770 |
| Non-2xx | 0 |
| Errors | 0 |
| P50 latency | <1ms |
| P95 latency | <1ms |
| P99 latency | 556us |

### DB connection sampling

During the 2500-request real-upstream run, Postgres was sampled every second
with:

```sql
SELECT count(*)
FROM pg_stat_activity
WHERE application_name LIKE 'omnitoken%';
```

Summary:

| Metric | Value |
| --- | ---: |
| Samples | 150 |
| Peak `omnitoken%` connections | 0 |

The requested `application_name` filter produced zero for every sample. This is
a measurement limitation of the current DSN/application-name setup, not proof
that the gateway used no database connections.

## Conclusion

v1 currently supports the local gateway health path at roughly `~1000 RPS` with
no observed errors. The real upstream path did not validate 50 concurrent
successful chat traffic: only `428/2500` requests succeeded and `2072/2500`
were upstream `429`. The primary bottleneck observed in this run is Ark upstream
rate limiting, not local gateway CPU, DB exhaustion, or timeout behavior.

DB connection peak under the required `application_name LIKE 'omnitoken%'`
sampling rule was `0`, but that filter is not currently a useful proxy for real
gateway/admin DB connections.

## V2 Candidate Fixes

- Add an upstream-aware load profile with lower RPS or bounded concurrency for
  paid Ark tests so baseline runs distinguish gateway capacity from provider
  rate limits.
- Set an explicit Postgres `application_name=omnitoken-gateway` and
  `application_name=omnitoken-admin` in local/CI DSNs, or update the sampling
  query to match the actual lib/pq defaults.
- Record upstream `429` counts in the loadtest summary so failures can be
  classified without parsing container logs.
