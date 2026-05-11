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
curl http://localhost:8080/v1/models
curl http://localhost:8081/healthz
curl http://localhost:8081/api/admin/overview
```

## Stop Services

```powershell
make down
```

Windows fallback:

```powershell
.\scripts\dev.ps1 down
```

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
