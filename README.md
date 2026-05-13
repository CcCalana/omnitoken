# OmniToken

OmniToken is a Demo-Ready enterprise AI API gateway for issuing virtual API
keys, proxying OpenAI-compatible chat requests to upstream providers, and
recording token/cost usage for an admin console.

The current local demo flow is:

1. Register a demo tenant and user from seed data.
2. Fill in an upstream provider key.
3. Create a virtual key for that user.
4. Let the user call the gateway.
5. View real usage in the admin API and web console.

> Status note: this is Phase 1 Demo-Ready software. The dev virtual-key endpoint
> is for local demos only. Full admin RBAC, production signup, quota changes, and
> admin-route authentication hardening are tracked in later tasks.

## Architecture

```mermaid
flowchart LR
  User["User with virtual key"] --> Gateway["Gateway :8080"]
  Gateway --> Auth["Virtual key auth"]
  Gateway --> Ark["Volcano Ark OpenAI-compatible API"]
  Gateway --> Usage["usage_events + token breakdown + cost_ledger"]
  Admin["Admin API :8081"] --> Usage
  Web["web/ static console"] --> Admin
  Postgres[(Postgres)] --> Auth
  Postgres --> Usage
```

## What Works Today

- OpenAI-compatible `GET /v1/models`
- OpenAI-compatible `POST /v1/chat/completions`
- Demo virtual-key creation through the admin service
- Postgres migrations and seed data through Docker Compose
- Usage and cost ledger writes after chat completions
- Admin APIs for overview, users, and models
- Static admin console in `web/`

## Prerequisites

- Docker Desktop or a compatible Docker engine
- Go 1.23+ for local tests and tools
- Python 3 for serving the static web console
- `curl`
- `make` optional; Windows users can use `scripts/dev.ps1`

## Quick Start

### 1. Create `.env`

PowerShell:

```powershell
Copy-Item .env.example .env
notepad .env
```

Bash:

```bash
cp .env.example .env
${EDITOR:-vi} .env
```

Fill in at least these values:

```dotenv
OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN=dev-bootstrap-token-change-me
OMNITOKEN_ARK_API_KEY=<your-volcano-ark-dev-key>
OMNITOKEN_ARK_DEFAULT_MODEL=ark-code-latest
OMNITOKEN_ADMIN_CORS_ORIGINS=http://localhost:3000
```

Do not commit `.env`. It is intentionally ignored by git.

### 2. Start the local stack

```powershell
make up
```

Windows fallback:

```powershell
.\scripts\dev.ps1 up
```

This builds the gateway/admin/migrate images, starts Postgres/Redis/NATS, runs
database migrations, applies `deploy/postgres/002_seed.sql`, and starts:

| Service | URL |
| --- | --- |
| Gateway | `http://localhost:8080` |
| Admin API | `http://localhost:8081` |
| Postgres | `localhost:15432` |
| Redis | `localhost:16379` |
| NATS | `localhost:14222` |

Check health:

```powershell
curl.exe http://localhost:8080/healthz
curl.exe http://localhost:8081/healthz
```

### 3. Use the seeded demo tenant

The seed file creates one organization, one project, and eleven demo users.
Use the demo admin user for the first trial:

| Field | Value |
| --- | --- |
| Organization | `Demo Organization` |
| Organization ID | `00000000-0000-0000-0000-000000000001` |
| Project | `Default Project` |
| Project ID | `00000000-0000-0000-0000-000000000101` |
| Demo Admin | `admin@democorp.local` |
| Demo Admin User ID | `00000000-0000-0000-0000-000000000201` |

There is no public signup endpoint yet. For local demos, "registration" means
using the seeded tenant or inserting another user into Postgres.

### 4. Create a virtual key for the user

PowerShell:

```powershell
$AdminToken = "dev-bootstrap-token-change-me"
$Body = @{
  organization_id = "00000000-0000-0000-0000-000000000001"
  user_id = "00000000-0000-0000-0000-000000000201"
} | ConvertTo-Json

$KeyResponse = Invoke-RestMethod `
  -Method Post `
  -Uri "http://localhost:8081/api/admin/dev/virtual-keys" `
  -Headers @{ Authorization = "Bearer $AdminToken" } `
  -ContentType "application/json" `
  -Body $Body

$VirtualKey = $KeyResponse.virtual_key
$KeyResponse | ConvertTo-Json
```

Bash:

```bash
curl -sS -X POST http://localhost:8081/api/admin/dev/virtual-keys \
  -H "Authorization: Bearer dev-bootstrap-token-change-me" \
  -H "Content-Type: application/json" \
  -d '{"organization_id":"00000000-0000-0000-0000-000000000001","user_id":"00000000-0000-0000-0000-000000000201"}'
```

The response contains `virtual_key`. Copy it now; the plaintext secret is only
returned in this response. It starts with `omt_`.

### 5. User trial: call the gateway

List available models:

```powershell
curl.exe http://localhost:8080/v1/models `
  -H "Authorization: Bearer $VirtualKey"
```

Send a non-streaming chat completion:

```powershell
curl.exe -X POST http://localhost:8080/v1/chat/completions `
  -H "Authorization: Bearer $VirtualKey" `
  -H "Content-Type: application/json" `
  -d '{"model":"ark-code-latest","messages":[{"role":"user","content":"Output exactly: pong"}],"stream":false,"max_tokens":32}'
```

Streaming SSE example:

```powershell
curl.exe --no-buffer -X POST http://localhost:8080/v1/chat/completions `
  -H "Authorization: Bearer $VirtualKey" `
  -H "Content-Type: application/json" `
  -d '{"model":"ark-code-latest","messages":[{"role":"user","content":"Count from 1 to 5."}],"stream":true,"max_tokens":64}'
```

The gateway keeps the virtual key local, injects the real Ark key upstream, and
records usage after the response.

### 6. Verify usage

Wait a moment for the deferred ledger write, then query admin APIs:

```powershell
Start-Sleep -Seconds 2
curl.exe http://localhost:8081/api/admin/overview
curl.exe http://localhost:8081/api/admin/users
curl.exe http://localhost:8081/api/admin/models
```

Expected signals:

- `total_tokens > 0`
- `active_users >= 1`
- `model_usage` includes the Ark-backed model
- `users` shows the demo admin user with non-zero usage

Current Phase 1 note: only the dev key-creation endpoint requires the bootstrap
token today. Admin read endpoints are intentionally simple until the T-010 admin
auth hardening task lands.

### 7. Open the web console

Serve `web/` on `localhost:3000`, which matches the default admin CORS allow
list:

```powershell
cd web
python -m http.server 3000
```

Open:

```text
http://localhost:3000/?admin=http://localhost:8081
```

The console has three views:

- Overview: monthly tokens, estimated cost, active users, trend, model share
- Users: per-user token usage and quota placeholder
- Models: prompt/completion token split, cost, and call count

## Adding Another Local Demo User

Open a psql shell:

```powershell
docker compose --env-file .env -f deploy/docker-compose.yml exec postgres `
  psql -U omnitoken -d omnitoken
```

Insert a user and grant the seeded member role:

```sql
INSERT INTO users (organization_id, email, display_name)
VALUES (
  '00000000-0000-0000-0000-000000000001',
  'new.user@democorp.local',
  'New Demo User'
)
RETURNING id;

INSERT INTO role_assignments (organization_id, user_id, role_id)
SELECT
  '00000000-0000-0000-0000-000000000001',
  users.id,
  roles.id
FROM users
JOIN roles ON roles.canonical_name = 'member'
WHERE users.email = 'new.user@democorp.local'
ON CONFLICT (organization_id, user_id, role_id) DO NOTHING;
```

Use the returned `users.id` in the virtual-key creation request.

## Common Commands

| Goal | Command |
| --- | --- |
| Start stack | `make up` |
| Stop stack | `make down` |
| Follow logs | `make logs` |
| Windows start | `.\scripts\dev.ps1 up` |
| Reset local volumes | `docker compose --env-file .env -f deploy/docker-compose.yml down -v` |
| Run Go tests | `go test -count=1 ./...` |
| Run vet | `go vet ./...` |
| Race tests | `make test` |

## Troubleshooting

### Gateway returns `401 invalid_api_key`

Use the full `virtual_key` returned by `/api/admin/dev/virtual-keys`, not the
`key_prefix`. The key should start with `omt_`.

### Gateway cannot reach the upstream provider

Check that `.env` contains `OMNITOKEN_ARK_API_KEY`, then recreate the gateway:

```powershell
make up
```

### Web console shows a CORS error

Serve the console from `http://localhost:3000` or add your origin to
`OMNITOKEN_ADMIN_CORS_ORIGINS` in `.env`, then restart admin:

```dotenv
OMNITOKEN_ADMIN_CORS_ORIGINS=http://localhost:3000,http://127.0.0.1:3000
```

### Admin charts are empty

Run at least one successful `/v1/chat/completions` request and wait one or two
seconds for usage recording. Then reload the web console.

### Start from a clean database

```powershell
docker compose --env-file .env -f deploy/docker-compose.yml down -v
make up
```

## Repository Layout

| Path | Purpose |
| --- | --- |
| `cmd/gateway` | OpenAI-compatible gateway |
| `cmd/admin` | Admin API and dev virtual-key endpoint |
| `cmd/migrate` | golang-migrate wrapper |
| `internal/auth` | Virtual-key generation and auth middleware |
| `internal/proxy` | Ark chat-completions proxy |
| `internal/usage` | Usage parsing, recording, and cost ledger writes |
| `migrations` | Database schema migrations |
| `deploy` | Dockerfiles, Compose, and seed SQL |
| `web` | Static admin console |
| `docs/runbooks/local-dev.md` | Detailed local development notes |

## Security Notes

- Never commit `.env`, provider keys, virtual keys, or full Authorization
  headers.
- Local demo pricing in `deploy/postgres/002_seed.sql` is placeholder data and
  must not be used for commercial quotes.
- The dev virtual-key endpoint is not a production admin API.
- Logs are designed not to print provider keys, virtual keys, or prompt bodies.
