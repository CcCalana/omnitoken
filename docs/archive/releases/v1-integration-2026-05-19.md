# V1 Integration Report - 2026-05-19

## Scope

T-INT stitched together admin login, viewer RBAC, budget editing, virtual-key
creation, virtual model routing, usage recording, quota blocking, audit logs,
Docker Compose, and the static admin console.

## Environment

- Stack: `deploy/docker-compose.yml`
- Admin API: `http://localhost:8081`
- Gateway: `http://localhost:8080`
- Web console: local `web/index.html` against the admin API
- Local secret: `OMNITOKEN_ARK_API_KEY` written only to ignored `.env`

`git check-ignore -v .env` confirmed `.env` is ignored by `.gitignore:7`.

## Verification

### Automated smoke

Command:

```powershell
$env:OMNITOKEN_RUN_REAL_UPSTREAM="1"
python scripts\v1_integration.py
```

Result: `17/17` non-failing checks.

Covered:

- admin health and gateway health
- admin login as `admin@democorp.local`
- viewer login as `user01@democorp.local`
- overview, users, models, virtual models, and audit tabs via admin API
- viewer direct quota PATCH denied with `403`
- admin creates a dev virtual key through session auth
- `chat-fast` non-streaming real upstream request succeeds
- `chat-fast` streaming real upstream request succeeds
- usage is visible in Users after deferred ledger write
- budget forced below used amount returns quota block on the next chat
- audit contains `create_virtual_key` and `update_quota`

Observed timings from the real upstream run:

- non-stream `chat-fast`: `1047ms`
- stream `chat-fast`: `687ms`

### Database spot checks

Latest usage rows after the real run:

```text
model_requested = chat-fast
model_actual    = deepseek-v4-pro
status_code     = 200
```

Virtual model seed table remains:

```text
chat-fast -> kimi-k2.6
```

Note: the gateway mapping table is correct; the Ark response reported
`deepseek-v4-pro` as the actual model. Usage records preserve both fields.

### Screenshot

Archived screenshot:

- `docs/release/v1-integration-admin-overview.png`

The Browser plugin timed out twice during local-page automation, so the
screenshot was captured with local Chrome headless as a fallback.

## Fixes Landed During T-INT

- Seeded `user01@democorp.local` as a viewer login account with a valid bcrypt
  password hash.
- Carried the authenticated role through admin sessions and `/api/admin/me`.
- Wired the frontend Users tab to `/api/admin/me` so viewer accounts hide quota
  editing.
- Enforced session-role admin checks on admin write routes; viewer PATCH now
  returns `403` and records audit.
- Kept bootstrap as an optional local fallback instead of the primary v1 path.
- Added `OMNITOKEN_ADMIN_SESSION_TTL` and anomaly threshold envs to Compose and
  `.env.example`.
- Updated Dockerfiles from Go 1.23 to Go 1.25 to match `go.mod`.
- Added `scripts/v1_integration.py`.
- Updated README v1 deployment and smoke-test instructions.

## Known Limits

- `make lint` could not run because `make` is not installed on this Windows
  host. Equivalent `go vet ./...` passed.
- The first Docker Compose build failed with Docker BuildKit session metadata;
  retrying with `DOCKER_BUILDKIT=0` succeeded.
