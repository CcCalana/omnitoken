# e2e-runner

Runs the T-100 deployed L2 multi-tenant correctness suite against live gateway/admin URLs. It creates virtual keys and budgets through admin HTTP APIs, but uses Postgres only for read-only ledger and attribution SELECTs.

Example:

```bash
go run ./cmd/e2e-runner \
  --gateway-url http://localhost:8080 \
  --admin-url http://localhost:8081 \
  --admin-token "$OMNITOKEN_ADMIN_TOKEN" \
  --database-url "$OMNITOKEN_TEST_DATABASE_URL"
```

Optional viewer/member RBAC checks require `--viewer-email/--viewer-password` and `--member-email/--member-password`. If member login is needed in a seeded environment, prepare it outside the runner, for example: `UPDATE users SET password_hash = crypt('temp123', gen_salt('bf')) WHERE email = 'user02@democorp.local';`.
