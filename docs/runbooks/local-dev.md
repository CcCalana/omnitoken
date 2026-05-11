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
