# Local Development Runbook

## Start Services

```powershell
make up
```

Gateway listens on `http://localhost:8080`; admin listens on
`http://localhost:8081`. Local infrastructure uses high host ports by default:
Postgres `15432`, Redis `16379`, NATS `14222`, and NATS monitor `18222`.

On Windows machines without `make`, run:

```powershell
.\scripts\dev.ps1 up
```

## Smoke Checks

```powershell
curl http://localhost:8080/healthz
curl http://localhost:8081/healthz
curl http://localhost:8081/api/admin/overview
```

`/v1/models` and `/v1/chat/completions` require a Demo-Ready virtual key after
T-006a.

## Stop Services

```powershell
make down
```

Windows fallback:

```powershell
.\scripts\dev.ps1 down
```

## Move From initdb To golang-migrate

New local databases start from an empty Postgres database. The compose
`migrate` service runs all SQL files in `migrations/`, creates
`schema_migrations`, and then the `seed` service applies
`deploy/postgres/002_seed.sql`.

Existing dev volumes that already ran `docker-entrypoint-initdb.d` have schema
version 1 in the database but no `schema_migrations` row. Baseline those
volumes once before running new migrations:

```powershell
$env:OMNITOKEN_DATABASE_URL="postgres://omnitoken:omnitoken@localhost:15432/omnitoken?sslmode=disable"
go run ./cmd/migrate force -version 1
go run ./cmd/migrate up
```

If a local migration is left dirty after an interrupted experiment, inspect the
database first, then repair the version explicitly:

```powershell
go run ./cmd/migrate version
go run ./cmd/migrate force -version 1
go run ./cmd/migrate up
```

For a disposable dev database, run
`docker compose -f deploy/docker-compose.yml down -v` to remove the old volume
and let compose migrate from scratch on the next `make up`. Production
databases are not baselined from initdb in this project; they should run the
migrate service from an empty database.

## Create A Demo Virtual Key

The Demo-Ready key creation endpoint is registered on the admin service at
`http://localhost:8081`, not on the gateway data-plane port `8080`. It is a
server-to-server dev endpoint protected by `OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN`;
full admin RBAC and audit table writes are left to T-005b. The admin CORS
allow-list still applies to this path, but local scripts should call it as a
server-to-server endpoint rather than from browser code.

Set a local bootstrap token before starting admin:

```powershell
$env:OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN="<set-me-or-disable>"
```

Create a virtual key for the demo admin user:

```powershell
curl -X POST http://localhost:8081/api/admin/dev/virtual-keys `
  -H "Authorization: Bearer <set-me-or-disable>" `
  -H "Content-Type: application/json" `
  -d '{"organization_id":"00000000-0000-0000-0000-000000000001","user_id":"00000000-0000-0000-0000-000000000201"}'
```

The response includes `dev_only: true` and returns the plaintext virtual key
once. The admin service logs `organization_id`, `user_id`, `key_prefix`, and
`created_at`, but never logs the secret.

## Configure Volcano Ark For Local Integration

T-002 only loads Ark provider configuration; it does not forward requests yet.
For local experiments, set the variables below in your shell or `.env` file:

```powershell
$env:OMNITOKEN_ARK_API_KEY="<dev key>"
$env:OMNITOKEN_ARK_OPENAI_BASE_URL="https://ark.cn-beijing.volces.com/api/coding/v3"
$env:OMNITOKEN_ARK_ANTHROPIC_BASE_URL="https://ark.cn-beijing.volces.com/api/coding"
$env:OMNITOKEN_ARK_DEFAULT_MODEL="ark-code-latest"
$env:OMNITOKEN_ARK_DISABLE_THINKING="true"
```

The fastest observed demo recipe is the OpenAI-compatible endpoint with
`thinking: {"type": "disabled"}` and `stream_options: {"include_usage": true}`.
Do not commit real Ark keys or captured request headers.
