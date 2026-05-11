# OmniToken

OmniToken is an enterprise AI API gateway scaffold for managing virtual API keys,
provider routing, quotas, billing metadata, and audit trails.

## Quick Start

```powershell
go test -race ./...
make up
curl http://localhost:8080/healthz
curl http://localhost:8080/v1/models
curl http://localhost:8081/api/admin/overview
```

On Windows machines without `make`, use:

```powershell
.\scripts\dev.ps1 up
```

The canonical task and planning documents are `规划.md`, `TASKS.md`,
`CLAUDE.md`, and `AGENTS.md`.
