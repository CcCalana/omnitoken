# cmd/loadtest

Small local smoke-load tool for Demo-Ready gateway checks. It sends bounded non-stream chat requests, then verifies admin usage aggregation.

## Postgres sampling

T-CONC-DSN sets `application_name` on gateway/admin Postgres clients so concurrent runs can sample the right sessions:

```sql
SELECT application_name, state, count(*) AS connections
FROM pg_stat_activity
WHERE application_name IN ('omnitoken-gateway', 'omnitoken-admin')
GROUP BY application_name, state
ORDER BY application_name, state;
```
