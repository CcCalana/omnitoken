# TASKS.md — OmniToken Task Board

## T-001 Phase 0 脚手架 [phase:0] [owner:codex] [status:review]

**目标**: 落地 Phase 0 仓库脚手架，使新人可以快速运行最小 gateway/admin 服务，并具备本地基础设施、CI、ADR 与迁移入口。

**涉及**:
- `go.mod`, `Makefile`, `.gitignore`, `.env.example`
- `cmd/gateway`, `cmd/admin`
- `deploy/docker-compose.yml`, `deploy/Dockerfile.gateway`, `deploy/Dockerfile.admin`
- `migrations/`
- `docs/adr/`, `docs/api/`, `docs/runbooks/`
- `.github/workflows/ci.yml`

**接受标准**:
- [x] 第九节目录结构落地，含关键 README / 占位文件。
- [x] `make help` 可展示标准命令。
- [x] `deploy/docker-compose.yml` 可一键启动 Postgres 16 / Redis 7 / NATS JetStream，并构建 gateway/admin。
- [x] `cmd/gateway` 提供 `/healthz` 与 `/v1/models`。
- [x] `cmd/admin` 提供 `/healthz` 与 `/api/admin/overview` Mock。
- [x] GitHub Actions 覆盖 lint、test、docker build。
- [x] 首批 ADR: `0001-tech-stack.md`, `0002-monorepo-layout.md`。
- [x] 本地 `go test ./...` 通过；`go test -race ./...` 因本机 Windows 缺少 `gcc` 无法执行，CI Ubuntu 仍保留 race detector。

**不在范围**:
- 真实 API Key 鉴权、额度预扣、上游转发、数据库访问。
- 正式前端工程化迁移。
- 引入未审批的第三方 Go 依赖。

**依赖**:
- 无新增第三方 Go 依赖。Phase 0 先使用标准库，`chi` / `viper` / `sqlc` / `pgx` 等按后续任务备案并锁版。

**Started**: 2026-05-11 Asia/Shanghai.

**Result**: Implementation commit `8f8f3a7`. Phase 0 scaffold ready for Claude review. Verification:
`go fmt ./...`, `go vet ./...`, `go test ./...`, `go build ./cmd/gateway ./cmd/admin`,
gateway/admin `docker build`, `docker compose -f deploy/docker-compose.yml config`,
`docker compose -f deploy/docker-compose.yml up -d --no-build`, and HTTP smoke checks for
`/healthz`, `/v1/models`, `/api/admin/overview` passed. Local `make` is not installed on this
Windows machine, so `scripts/dev.ps1` mirrors core Makefile targets.
