# TASKS.md — OmniToken Task Board

## CHANGELOG 2026-05-11

- Claude completed R-001 (see `REVIEW.md`). T-001 通过验收，但产生 4 个 HIGH + 5 个 MEDIUM 跟进项。
- 新增 T-002 / T-003 / T-004，请按 ID 顺序由 Codex 接手。Ark provider 实测纪录与 golden 语料已落到 `testdata/golden/ark/`，`.env.example` 与 seed 已经把 `ark-code-latest` 写入，可被前端联调与 T-007 SSE 代理引用。

---

## T-002 收尾 Phase 0 HIGH 项 + 引入 internal/httpx + Ark provider 占位 [phase:0->1] [owner:codex] [status:in-progress]

**目标**: 落地 R-001 中的 4 个 HIGH 与 1 个 MEDIUM (M-4)，为 Phase 1 代理实现搭好基础设施；同时把火山方舟 provider 写入配置结构（先占位、不真正转发）。

**涉及**:
- `internal/httpx/` (新增): `RequestLogger`、`StatusRecorder`、`WriteJSON`、`CORS(origins []string)`、`RequestID(next)`。每个中间件附单测，覆盖率 ≥ 85%。
- `cmd/gateway/main.go`、`cmd/admin/main.go`: 接入 `internal/httpx`，删除重复代码。
- 新增 `internal/httpx/server.go`：`Run(ctx, srv)` 提供 SIGINT/SIGTERM 优雅停机，`shutdown_timeout` 默认 15s。
- `internal/config/config.go`：扩展为结构体 `Load() (Config, error)`，覆盖 `OMNITOKEN_ADMIN_CORS_ORIGINS`、`OMNITOKEN_ARK_*` 一组变量。`config_test.go` 覆盖所有新增字段的默认值与解析。
- `cmd/admin/main.go`：CORS 改为读取允许列表；非白名单 origin 不发 ACAO 头；允许 `Authorization` 仅在白名单 origin 时回显。
- `cmd/gateway/main.go`：`/v1/chat/completions` 暂仍未实现，但用 `httpx.WriteJSON` 返回统一 502 envelope 占位，便于 T-007 直接替换。
- 文档：`docs/runbooks/local-dev.md` 补一节"配置火山方舟 API Key 做本地联调"。
- `cmd/gateway/main.go` 与 `cmd/admin/main.go` 错误判断改为 `errors.Is(err, http.ErrServerClosed)`。

**接受标准**:
- [ ] `internal/httpx` 单测覆盖 ≥ 85%，含 request_id 透传（请求头未带时生成，带时直接复用）。
- [ ] gateway/admin 日志结构化字段包含 `request_id`、`duration_us`（不再用 `duration_ms`，毫秒精度太粗）。
- [ ] 注入 `Authorization: Bearer SECRET_*` 后日志/响应均不泄漏（已是基线，必须保留）。
- [ ] 优雅停机：发送 SIGINT 后进程在 ≤ 15s 内退出，期间正在处理的 GET 请求被允许完成。
- [ ] CORS 白名单: 非白名单 Origin 不会得到 `Access-Control-Allow-Origin`。两条 case (in-list / not-in-list) 各一个测试。
- [ ] `Load() Config` 读取所有 `OMNITOKEN_ARK_*` 变量并暴露 `cfg.Ark.OpenAIBaseURL` 等字段；缺失时返回结构体零值不报错（key 留空），但暴露 `cfg.Ark.Enabled() bool`。
- [ ] `go test -count=1 ./...`、`go vet ./...`、`go fmt -l .` 全绿；docker build 仍通过。
- [ ] 截图或日志样本贴在 Result 区，证明 request_id 与 duration_us 出现在 stdout。

**不在范围**:
- 实际把请求转发到方舟（留给 T-007）。
- 数据库索引与 migrate 工具改造（T-003）。
- compose depends_on / openapi 改动（T-004）。

**依赖**:
- 仍不引入新的第三方库。所有变更可用标准库完成（`os/signal`、`errors`、`crypto/rand` 即可生成 request_id）。

---

## T-003 数据层治理：索引、golang-migrate、pricing 版本 [phase:1] [owner:codex] [status:todo]

**目标**: 让数据库从"能建表"升级到"能查询、能演化"。本任务必须早于任何写真实查询的 SQL。

**涉及**:
- 新增 `migrations/000002_indexes.up.sql` / `.down.sql`：补齐 R-001 M-1 列出的索引集合。
- 新增 `migrations/000003_pricing_window.up.sql` / `.down.sql`：`model_pricing` 增加 `effective_to timestamptz`，并创建视图 `model_pricing_current`（取每个 model_id 最新生效行）。
- 引入 `github.com/golang-migrate/migrate/v4`（首个第三方 Go 依赖，需要在 PR 描述中显式备案版本与替代品比较）。
- 新增 `cmd/migrate/main.go`：薄包装，支持 `up` / `down` / `version` / `force`，环境变量 `OMNITOKEN_DATABASE_URL`。
- 修改 `deploy/docker-compose.yml`：去掉 `docker-entrypoint-initdb.d` 挂载 `000001_init.up.sql` 的方式，改为单独的 `migrate` service（一次性 job），seed.sql 保留。
- `docs/runbooks/local-dev.md` 增补"如何手动执行迁移"段落。

**接受标准**:
- [ ] `migrate up && migrate down && migrate up` 在干净 Postgres 上 idempotent。
- [ ] 启动 docker-compose 后 `schema_migrations` 表存在且记录最新版本。
- [ ] 视图 `model_pricing_current` 在多版本价格数据中仅返回最新行（含一条 SQL 集成测试）。
- [ ] `model_pricing` 历史行不会被删除，便于审计。
- [ ] 任何新增 Go 依赖锁定到具体 minor 版本，并在 PR 描述列出 "Why not <alternative>"。

---

## T-004 小修小补 [phase:1] [owner:codex] [status:todo]

**目标**: 收尾 R-001 的 NIT 与 1 个 MEDIUM (M-2)，避免遗漏。

**涉及**:
- `deploy/docker-compose.yml`: 删除 gateway/admin 对 postgres/redis/nats 的 `depends_on`，改为 docker-compose profile `infra` 控制是否启基础设施。
- `docs/api/openapi.yaml`: `total_tokens` 类型加 `format: int64`；新增 ark 模型条目；新增 `/api/admin/overview` 中 `model_usage` 与 `trend` 结构定义。
- `Makefile`: `lint` 与 `vet` 拆分为两个 target；新增 `make smoke` (跑 `go test ./cmd/...`)。
- `cmd/gateway/main.go`: 静态模型列表改为按 `OMNITOKEN_STATIC_MODELS=<csv>` 可覆盖（仅 Phase 0 兼容，长远走 model_catalog 表）。

**接受标准**:
- [ ] `make help` 列出新 target。
- [ ] OpenAPI 规范在 `swagger-cli validate docs/api/openapi.yaml` 下通过（CI 加一步）。
- [ ] `docker compose -f deploy/docker-compose.yml --profile infra up -d` 仅起 pg/redis/nats，主程序由开发者本机 `go run` 启动，方便排错。

---

## T-001 Phase 0 脚手架 [phase:0] [owner:codex] [status:approved]

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
