# TASKS.md — OmniToken Task Board

## CHANGELOG 2026-05-11

- Claude completed R-001 (see `REVIEW.md`). T-001 通过验收，但产生 4 个 HIGH + 5 个 MEDIUM 跟进项。
- 新增 T-002 / T-003 / T-004，请按 ID 顺序由 Codex 接手。Ark provider 实测纪录与 golden 语料已落到 `testdata/golden/ark/`，`.env.example` 与 seed 已经把 `ark-code-latest` 写入，可被前端联调与 T-007 SSE 代理引用。
- **2026-05-11 14:30** — Claude completed R-002. T-002 approve → status:approved。新增 3 个 MEDIUM (M-6 request_id 信任、M-7 CORS Allow-Methods、M-8 Ark URL 默认值) 不阻塞 T-003，但**必须在 T-005 虚拟 Key 鉴权动工前清掉**；建议 Codex 在做 T-003 前花 30 分钟扫掉这三项（小改）。
- **2026-05-11 14:35** — 用户提出引入"1 admin + 10 user"为基准测试场景。Claude 起草并发测试矩阵草案，等用户确认 L1-L4 哪些进入 Phase 1 验收。等用户回复后再写正式任务条目（暂记为 T-CONC 占位）。
- **2026-05-11 15:00** — 用户决策三件事: (a) 测试侧重 L2 正确性优先并作为 Phase 1 验收门; (b) L2/L3 上游使用**真火山方舟**; (c) admin 鉴权采用**完整 RBAC** (admin / member / viewer)。同步落地：`规划.md` 第十节增补 10.1 并发测试矩阵；本文件新增 T-002.1 / T-003 范围扩展 / T-005 拆 a/b/c / T-100 L2 套件。
- **2026-05-11 15:00** — 用户授权方舟 dev key `4dc1b1a3-…`（详见 `.env`，已 git-ignored）。`AGENTS.md §9` 给出调用规则与成本边界，**严禁** 把真 key 写入源文件 / fixture / commit / 日志。
- **2026-05-11 15:05** — User feedback: Claude 在此项目里要严格守边界，**不预先做 Codex 范围内的工作**（API 探测、golden 语料、seed 数据、配置文件、过度细化的任务断言）。已存入 Claude 长期记忆。本轮起所有任务条目仅写"目标 + 接受标准 + 不在范围"，实施细节均要求 Codex 在条目下 `## PROPOSAL` 区块给出，再由 Claude review。

---

## T-002 收尾 Phase 0 HIGH 项 + 引入 internal/httpx + Ark provider 占位 [phase:0->1] [owner:codex] [status:approved]

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

**Result**: Implementation commit `706a3a7`. Verification passed:
`go test -count=1 ./...`, `go vet ./...`, `gofmt -l .`, `go test -count=1 -cover ./internal/httpx`
(87.9%), gateway/admin `docker build`, compose restart with `--no-build`, and smoke checks for
`/healthz`, `/v1/chat/completions`, and admin CORS allow/deny.

Log sample:

```json
{"msg":"http request","request_id":"req-smoke-gateway","method":"GET","path":"/healthz","status":200,"duration_us":44}
{"msg":"http request","request_id":"req-smoke-chat","method":"POST","path":"/v1/chat/completions","status":502,"duration_us":51}
{"msg":"http request","request_id":"req-smoke-cors-deny","method":"GET","path":"/api/admin/overview","status":200,"duration_us":112}
```

---

## T-002.1 R-002 三个 MEDIUM 收尾 [phase:0->1] [owner:codex] [status:review]

**目标**: 把 R-002 中 M-6 / M-7 / M-8 三个 MEDIUM 在进入 T-005 之前一次清掉。预计 30–60 分钟。

**接受标准**:
- [x] M-6 解决：`internal/httpx.RequestID` 始终生成内部权威 `request_id`；如客户端传 `X-Request-Id`，作为 `upstream_request_id` 字段同时落日志与上下文，但**不**覆盖内部 ID。`request_id` 字段不可由客户端伪造。
- [x] M-7 解决：`internal/httpx.CORS` 签名扩展为 `CORS(origins []string, methods []string)`；admin 默认 `[]string{"GET","POST","PATCH","DELETE","OPTIONS"}`，gateway 仍按需配置。
- [x] M-8 解决：`internal/config.Load()` 中 `Ark.OpenAIBaseURL` / `AnthropicBaseURL` / `DefaultModel` 三个字段在环境变量未设置时回退到方舟已知公开地址与 `ark-code-latest`，使得仅设置 `OMNITOKEN_ARK_API_KEY` 即可让 `Ark.Enabled()=true`。
- [x] 三项均补单测；不引入新依赖；覆盖率不下降。

**不在范围**: 任何真实上游转发或 RBAC 逻辑。

**Codex propose 前置**: 不需要 propose，按上述标准直接做。

**Result**: Implementation commit `68b85a7`. Verification passed:
`go test -count=1 ./...`, `go vet ./...`, `gofmt -l .`,
`go test -count=1 -cover ./internal/httpx ./internal/config` (`internal/httpx` 90.1%, `internal/config` 100.0%),
and gateway/admin `docker build`. No real Ark upstream calls were made.

---

## T-003 数据层治理：索引、golang-migrate、pricing 版本、RBAC schema [phase:1] [owner:codex] [status:review]

**Short note**: Do T-003 first; migrate shapes T-004 infra profile.

**目标**: 让数据库从"能建表"升级到"能查询、能演化"。同时为 T-005 RBAC 鉴权预埋 schema。本任务必须早于任何写真实查询的 SQL。

**接受标准**:
- [ ] golang-migrate 引入并锁版，PR 描述列出 "Why not goose / atlas / dbmate"。
- [ ] `migrate up && migrate down && migrate up` 在干净 Postgres 上 idempotent。
- [ ] 启动 docker-compose 后 `schema_migrations` 表存在且记录最新版本；DDL 不再依赖 `docker-entrypoint-initdb.d`，seed.sql 保留。
- [ ] 索引补齐至少覆盖：`usage_events(created_at)`、`usage_events(organization_id, created_at)`、`usage_events(api_key_id, created_at)`、`audit_logs(created_at)`、`cost_ledger(usage_event_id)`。
- [ ] `model_pricing` 增加 `effective_to timestamptz` + 视图 `model_pricing_current`（每个 model_id 最新一行），含至少 1 个 SQL 集成测试。
- [ ] RBAC schema 预埋：新增 `roles`(系统级三角色 `admin` / `member` / `viewer`，by canonical name) 与 `role_assignments`(user × organization × role)；DDL 已经入库但**不**包含鉴权 hook（hook 留 T-005a/b/c）。
- [ ] migrations 全部双向，down 不留孤儿。

**不在范围**:
- 鉴权 / 权限检查逻辑（T-005a/b/c）。
- 真正的查询代码（sqlc 引入留待后续任务）。

**Codex propose 前置 (必须)**:
在本条目下追加 `## PROPOSAL` 区块，至少包含：(1) `roles` 与 `role_assignments` 的表结构与外键设计选项及取舍；(2) `model_pricing_current` 用 VIEW 还是 materialized view 的依据；(3) `cmd/migrate` 的 CLI flag 集合；(4) 是否需要拆分多个 migration 文件以及顺序。Claude 在 `REVIEW.md` 给出 `[+] approved` 后再开工。

## PROPOSAL

1. RBAC schema 设计

建议新增 `users`、`roles`、`role_assignments` 三张表，而不是只新增 `roles` / `role_assignments`。原因是当前 `api_keys` 和未来 admin token 都需要稳定的 user 归属；如果 `role_assignments` 直接存裸 `user_id uuid` 而无外键，T-005b/c 会留下孤儿权限风险。

拟定结构：

- `users(id uuid pk default gen_random_uuid(), organization_id uuid not null references organizations(id) on delete cascade, email text, display_name text, status text check active/disabled, created_at timestamptz default now(), unique(organization_id, email))`。`email` 允许后续接 SSO；Phase 1 seed 一个 admin + 10 users。
- `roles(id uuid pk default gen_random_uuid(), canonical_name text not null unique, description text not null, created_at timestamptz default now())`。系统级三角色用 `canonical_name in ('admin','member','viewer')`，不按组织复制，避免多租户下角色语义漂移。
- `role_assignments(id uuid pk default gen_random_uuid(), organization_id uuid not null references organizations(id) on delete cascade, user_id uuid not null references users(id) on delete cascade, role_id uuid not null references roles(id) on delete restrict, created_at timestamptz default now(), unique(organization_id,user_id,role_id))`。

取舍：`role_assignments.organization_id` 与 `users.organization_id` 存在可推导关系，但显式保留，便于后续跨组织成员与 tenant-scoped 查询。为避免错配，增加复合外键 `(organization_id,user_id)` → `users(organization_id,id)`，并给 `users` 加 `unique(organization_id,id)`。

2. `model_pricing_current` 视图形态

建议使用普通 `VIEW`，不使用 materialized view。价格表规模小、写入低频，普通 view 每次查询用 `row_number() over (partition by model_id order by effective_from desc, created_at desc)` 过滤最新行即可。materialized view 需要刷新策略，Phase 1 没有调度器接入真实价格同步，容易产生"价格已写入但 current 视图未刷新"的隐性错误。

拟定语义：只返回 `effective_from <= now()` 且 `(effective_to is null or effective_to > now())` 的每个 `model_id` 最新一行。这样历史价格不删除，审计可追溯。

3. `cmd/migrate` CLI flags

建议首版保持薄包装：

- `omnitoken-migrate up`
- `omnitoken-migrate down [-steps N]`，默认 `N=1`，避免误把全部 schema 回滚。
- `omnitoken-migrate version`
- `omnitoken-migrate force -version N`
- 全局 flag: `-database-url`，默认读取 `OMNITOKEN_DATABASE_URL`；`-path` 默认 `file://migrations`。

错误策略：缺少 database URL 直接返回非零；`down` 不提供 `-steps` 时只回滚一步；`force` 必须显式传 `-version`，避免误操作。

依赖选择：使用 `github.com/golang-migrate/migrate/v4`，建议锁到当前稳定 minor（实施时用 `go get github.com/golang-migrate/migrate/v4@v4.x.y` 解析实际 patch）。Why not goose: goose 支持 Go migration，项目已要求 SQL 文件迁移。Why not atlas: atlas 更偏 schema diff/声明式，当前任务只需要可逆版本迁移。Why not dbmate: dbmate CLI 很轻，但 Go 内嵌薄包装和库级测试能力弱于 golang-migrate。

4. Migration 文件拆分与顺序

建议拆分为 3 个 migration，便于 review 和失败定位：

- `000002_indexes.up/down.sql`: 只补索引，覆盖 usage/admin 查询路径。
- `000003_pricing_window.up/down.sql`: `model_pricing.effective_to`、约束/索引、`model_pricing_current` view。
- `000004_rbac_schema.up/down.sql`: `users`、`roles`、`role_assignments`、基础三角色 seed。seed roles 放在 migration 内，因为系统角色属于 schema invariant；demo users 仍留 `deploy/postgres/002_seed.sql` 或后续测试 fixture，不放 invariant migration。

`docker-compose.yml` 调整顺序：Postgres 只负责建空库；新增一次性 `migrate` service 依赖 Postgres healthy，执行 `cmd/migrate up`；`seed` 可在 migrate 之后执行，或暂保留为手动/后续 T-004 profile 配置。T-003 只移除 DDL 的 `docker-entrypoint-initdb.d` 挂载，保留 seed.sql。

**Result**: `54058e8` — implemented `cmd/migrate`, reversible migrations 000002-000004, compose migrate/seed ordering, local-dev baseline runbook, and license ledger/CI gate. Self-test: `go test ./...`; `go vet ./...`; `gofmt -l .`; `go-licenses check ./...`; isolated Postgres `migrate up -> version 4 -> down -steps 1 -> up -> down -steps 4 -> up`; SQL integration test for `model_pricing_current`; compose `migrate seed` produced `schema_migrations=4,false` and 11 demo users; gateway/admin/migrate Docker builds passed. Local `go test -race ./...` blocked by missing Windows `gcc`.

## LICENSE EXCEPTION PROPOSAL

T-003 implementation is blocked by the dependency license gate. Attempted dependency:
`github.com/golang-migrate/migrate/v4@v4.18.3`.

License scan from local module cache:

- `github.com/golang-migrate/migrate/v4@v4.18.3`: MIT
- `go.uber.org/atomic@v1.7.0`: MIT
- `github.com/hashicorp/errwrap@v1.1.0`: MPL-2.0
- `github.com/hashicorp/go-multierror@v1.1.1`: MPL-2.0

Because MPL-2.0 is outside the allowed MIT/BSD/Apache-2.0 set in the T-003 implementation constraints, Codex stopped before writing implementation code and reverted `go.mod` / `go.sum`.

Decision requested from Claude:

1. Approve MPL-2.0 transitive dependencies for golang-migrate and continue T-003 as proposed.
2. Or replace the library approach with an external migrate CLI/container strategy that does not add MPL-2.0 Go dependencies to this module.
3. Or re-propose a different migration library with only MIT/BSD/Apache-2.0 dependencies.

---

## DEMO-READY ROUTE (2026-05-11 user-locked)

用户决策：先把 Demo-Ready 路线跑通（gateway 真转发到方舟 + 计费落库 + admin 真查 DB + 前端 fetch 真实数据 + 最小种子），**再** push 到 GitHub。T-005a/T-005b/T-005c 完整版、T-009 限流、T-100 L2 套件**全部推后**到 Phase 1 完整验收阶段。

Demo-Ready 任务顺序：**T-006-nit → T-006a → T-007 → T-008 → T-006b → T-006c → T-006d**。本轮先正式拆 T-006-nit 与 T-006a 两条；T-007 等 T-006a approve 后再拆，避免一次性塞过多任务造成并发会话状态打架（参考 2026-05-11 17:00 异常状态根因）。

---

## T-006-nit `cmd/migrate force` sentinel 修复 [phase:1] [owner:codex] [status:review]

**目标**: 修 R-003 M-9。`runForce` 当前用 `-version -2` 作为 sentinel 判断"未传"，用户合法传 `-2` 会被误识别。

**接受标准**:
- [ ] 改用 `flag.Visit` 或显式 `valueSet bool` 检测；保留"force 必须显式传 -version 否则拒绝"语义。
- [ ] 增 unit test：`force -version -2 -database-url ...` 应当走到正常 force 路径（执行 `m.Force(-2)`，stderr 输出可来自 golang-migrate；不应在 CLI 层报 "force requires -version"）。
- [ ] `force` 不带 `-version` 仍然报错 + exit 2。

**不在范围**: 其他 CLI 改动。

**预计**: 15-30 分钟。

**Codex propose 前置**: 不需要，按上述标准直接做。

**Result**: `88fc18d` — replaced the `-version -2` sentinel with `flag.Visit` detection and added a unit test proving explicit `force -version -2` calls `Force(-2)`. Self-test: `go test ./cmd/migrate`.

---

## T-006a 最小虚拟 Key Gateway 鉴权（Demo-Ready 版） [phase:1] [owner:codex] [status:approved]

**目标**: 让 gateway 数据面有"最低限度的身份概念" —— 数据库里查到 active 的虚拟 Key 才让请求继续。**这是 Demo-Ready 版**，不做完整 RBAC、不做模型白名单、不做配额、不做 timing-attack 防护以上的复杂度。完整版留 T-005c。

**接受标准**:
- [ ] gateway `/v1/chat/completions` 与 `/v1/models` 强制带 `Authorization: Bearer <virtual_key>`；缺失或非法返回 401 统一 envelope（不区分"key 不存在"和"key 存在但 disabled"）。
- [ ] Key 校验流程：取 `Authorization` → 解析 `Bearer xxxxxxxx_<secret>` → 通过 `key_prefix` 查 `api_keys` → 比较 `sha256(secret)` 是否等于 `key_hash`，状态 == 'active'。
- [ ] 一次"创建虚拟 Key"的内部辅助路径：`cmd/seed` 或 `cmd/admin` 加一个 dev-only 命令/HTTP endpoint（**不**接入 RBAC，admin token 暂用 env var `OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN` 简单匹配），生成 Key，返回明文一次。Demo-Ready 阶段允许这种简化。
- [ ] Subject 写入 `r.Context()`：`Subject{UserID uuid.UUID, OrgID uuid.UUID, APIKeyID uuid.UUID}`，供 T-007/T-008 消费。**不**含 Role 字段（留 T-005a）。
- [ ] 单元测试覆盖率 `internal/auth` ≥ 85%；含 invalid token / disabled key / valid key 三条 case。
- [ ] **不**接入 timing-equal 字符串比较（标准库 `subtle.ConstantTimeCompare` 留 T-005c）。Demo-Ready 阶段接受 timing attack 风险，需在代码注释明确标注。
- [ ] **不**接入 Redis cache、in-process LRU（留 T-005c）。每次请求查 DB。Demo-Ready 阶段不在乎 1ms 级延迟。

**不在范围**:
- 完整 RBAC（T-005a/b）。
- 模型白名单检查（T-005c）。
- 配额预扣 / 限流（T-009）。
- 实际转发到上游（T-007）。
- timing-attack 防护、缓存（T-005c 完整版）。

**Codex propose 前置 (必须)**:
在本条目下追加 `## PROPOSAL` 区块，覆盖：
1. Virtual Key 字符串格式 —— `key_prefix` 长度与字符集、随机段长度与字符集、明文展示形式（如 `omt_<prefix>_<secret>`）。注意 Phase 0 已落 `api_keys.key_prefix varchar(16)`、`key_hash bytea`，propose 要兼容。
2. Key 创建路径选哪种：cmd/seed 一次性命令 vs cmd/admin 加 dev HTTP endpoint。给出取舍。
3. `internal/auth` 包目录结构与暴露的 API（接口签名）。
4. 401 envelope 的统一格式（与 gateway `/v1/chat/completions` 现有占位 envelope 对齐：`{"error":{"message":..., "type":..., "code":...}}`）。
5. Admin bootstrap token 的注入路径：`OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN` env var 是否足够、是否需要在 config 包接入。

**依赖**: T-003 approved（已通过）。

**Demo-Ready 衔接**: T-007 接 T-006a 后能拿到 Subject 进入实际转发；后续 T-005a/b/c 完整版会把"模型白名单 / RBAC / cache / timing-equal"加上去，T-006a 不需要返工。

## PROPOSAL

1. Virtual Key 字符串格式

采用 `omt_<prefix>_<secret>`，其中 `prefix` 为 12 位 lowercase base32 字符（`a-z2-7`），写入现有 `api_keys.key_prefix varchar(16)`；`secret` 为 32 bytes CSPRNG 后的 base64url no-padding 字符串（约 43 字符）。`Authorization` 只接受 `Bearer omt_<prefix>_<secret>`，解析失败、prefix 查不到、key disabled、hash 不匹配都返回同一个 401。

存储策略：只存 `key_prefix` 与 `sha256(secret)` 的 `key_hash`，明文 Key 只在创建时返回一次。Demo-Ready 简化版暂用普通 byte/string 比较，不做 `subtle.ConstantTimeCompare`，实现处加注释明确标注"Demo-Ready 简化版；timing-equal 留 T-005c"。

2. Key 创建路径

选择 `cmd/admin` 加 dev-only HTTP endpoint，而不是 `cmd/seed` 一次性命令。路径建议 `POST /api/admin/dev/virtual-keys`，仅当 `OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN` 非空时注册；请求必须带 `Authorization: Bearer <bootstrap token>`。这样 Demo 调试时无需重跑 seed，就能为现有 demo users 创建 key，也能让前端/脚本拿到一次性明文。

接口只覆盖 Demo-Ready 最小字段：`organization_id`、`user_id`、可选 `project_id`、可选 `expires_at`；不接 RBAC、不接模型白名单、不接额度。写入 `api_keys.status='active'`、`key_prefix`、`key_hash`、`organization_id`、`project_id`。是否存在 user/project 的校验只做数据库外键级失败处理，复杂业务校验留后续。

3. `internal/auth` 包目录与 API

建议新增：

- `internal/auth/subject.go`: `Subject{UserID uuid.UUID, OrgID uuid.UUID, APIKeyID uuid.UUID}`、`WithSubject(ctx, Subject)`、`SubjectFromContext(ctx) (Subject, bool)`。
- `internal/auth/virtual_key.go`: `ParseVirtualKey(raw string) (prefix string, secret string, ok bool)`、`HashSecret(secret string) []byte`、`AuthenticateVirtualKey(ctx context.Context, store VirtualKeyStore, raw string) (Subject, bool, error)`。
- `internal/auth/postgres_store.go`: Demo-Ready 简化版 `database/sql` 查询实现，接口为 `LookupVirtualKey(ctx, prefix string) (VirtualKeyRecord, error)`。
- `internal/auth/middleware.go`: `RequireVirtualKey(store VirtualKeyStore, unauthorized func(http.ResponseWriter)) func(http.Handler) http.Handler`。

UUID 类型使用 `github.com/google/uuid`（BSD-3-Clause，permissive，新增时同步 `THIRD_PARTY_LICENSES.md`）。查询层暂不用 sqlc/pgx，继续用标准库 `database/sql` + `github.com/lib/pq`，避免把完整数据访问层提前带进来。

4. 401 envelope

统一返回 HTTP 401：

```json
{"error":{"message":"unauthorized","type":"authentication_error","code":"invalid_api_key"}}
```

缺失 header、非 Bearer、格式错误、不存在、disabled、hash mismatch 全部相同 envelope，不暴露探测信息。结构沿用 gateway 现有 `/v1/chat/completions` 占位 `errorEnvelope/errorDetail`，T-006a 可以先在 gateway 内复用或迁出小 helper；不改 502 upstream 占位语义，T-007 再替换转发逻辑。

5. `OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN` 注入路径

在 `internal/config.AdminConfig` 增加 `BootstrapToken string`，由 `OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN` 注入；`.env.example` 只放占位值，不放真实 token。`cmd/admin` 启动时如果 token 为空，不注册 dev endpoint，并记录一条不含 token 的结构化日志；如果非空，注册 dev endpoint 并用常规字符串比较验证 `Authorization: Bearer <token>`。这是 Demo-Ready 简化版，完整 admin token/RBAC 留 T-005b。

边界确认：T-006a 不做完整 RBAC、不做模型白名单、不做配额预扣/限流、不做 timing-equal、不做 Redis/in-process cache、不做真实上游转发。代码注释会明确标注"Demo-Ready 简化版"及后续归属任务。

**Result**: `4df0033` — implemented Demo-Ready virtual key auth, admin-port-only dev key creation, `internal/auth` Subject/context helpers, `api_keys.user_id` migration, runbook/env/compose wiring, and `github.com/google/uuid` license ledger update. Self-test: `go test ./...`; `go vet ./...`; `go test -cover ./internal/auth` (96.1%); `gofmt -l cmd internal migrations`; `go-licenses check/report`; `docker compose config --quiet`; isolated Postgres `migrate up -> version 5 -> down -steps 1 -> up -> version 5` plus seed; gateway/admin/migrate Docker builds passed.

---

## T-007 SSE 反向代理 + 方舟 OpenAI-compat 转发 [phase:1] [owner:codex] [status:todo]

**目标**: 让 gateway `/v1/chat/completions` 真正把请求转发到火山方舟 OpenAI-compat 端点，**非流式与 SSE 流式都通**。返回内容透传给客户端，**usage 必须在响应里携带**（用于 T-008 计费解析）。这是 Demo-Ready 路线的核心一块。

**接受标准**:
- [ ] `POST /v1/chat/completions` 经 T-006a 鉴权后，把请求体（含 `messages`、`model`、`stream` 等字段）转发到 `cfg.Ark.OpenAIBaseURL`。
- [ ] 路由确定方舟为唯一上游：先用 `cfg.Ark.DefaultModel` 替换 client-请求的 model（Demo-Ready 简化），完整版多 provider 路由留 routes 表 / T-005c 之后。
- [ ] 上游 `Authorization: Bearer <OMNITOKEN_ARK_API_KEY>` 由 gateway 注入；客户端的 `Authorization`（虚拟 Key）**绝不**透传到上游。
- [ ] 非流式：`stream:false` 或缺省时，等上游响应完整 → 透传 status code + body 给客户端。
- [ ] 流式：`stream:true` 时透传 SSE chunk 不缓冲（Flush 每个 chunk），且**强制注入 `stream_options.include_usage: true`**（即使客户端没传也加）。最后一条 `choices:[]` chunk 必须 reach 客户端，承载 usage。
- [ ] 推理控制：当 `cfg.Ark.DisableThinking == true` 时，请求体**强制注入** `thinking: {"type": "disabled"}`（顶层字段）。已在 `.env` 默认 true，目的是把延迟从 ~2.8s 降到 ~1.8s。
- [ ] `request_id`：内部权威 `request_id` 通过 `X-Request-Id` 注入到上游请求头（方便上游/方舟侧日志关联）。
- [ ] 上游错误统一包装：upstream 5xx / 超时 / 连接失败 → 返回 502 统一 envelope（沿用现有 `errorEnvelope/errorDetail` 结构），**不**透传 upstream 错误体。code 字段标准化：`upstream_timeout` / `upstream_5xx` / `upstream_connection_failed` / `upstream_invalid_response`。
- [ ] 安全：上游响应头中**不**透传 `Server`、`X-Powered-By`、`Set-Cookie` 等可能泄漏厂商信息或不安全的字段；日志/metric 中**不**出现 `Authorization` / 真实 API Key / Prompt 全文。
- [ ] `Subject` 通过 `SubjectFromContext(r.Context())` 读出，写入日志（user_id / org_id / api_key_id），便于 T-008 后续从 ctx 拿到。
- [ ] 配置兜底：`cfg.Ark.Enabled() == false`（API Key 为空）时，`/v1/chat/completions` 返回 503 envelope `code:"upstream_not_configured"`（沿用 Phase 0 占位的 code 名字）。
- [ ] 测试：
  - 单元测试 `internal/proxy` 覆盖率 ≥ 85%；用 `httptest.NewServer` 起假上游，测非流式、流式、上游 timeout、上游 502、上游连接失败、缺失 API Key 五种 case。
  - 至少 1 个 e2e/integration test 走真实方舟（用 `.env` 的真 key），跑非流式 + 流式各一次，验证 usage 字段被透传给客户端、`stream_options.include_usage` 被注入、`thinking.disabled` 被注入。**用 build tag `e2e`，默认 skip，CI 不跑**。
- [ ] golden 语料：复用 `testdata/golden/ark/openai_stream_no_thinking_with_usage.txt` 作为流式断言基线；解析这份 golden 校验"最后一个 chunk 含 usage" 的逻辑路径。
- [ ] runbook：`docs/runbooks/local-dev.md` 增"如何用 demo virtual key 发一次 chat completion 到方舟"段落，含两条 curl 命令（非流式 + `--no-buffer` 流式）。

**不在范围**:
- 多 provider / 多模型路由（routes 表、T-005c 之后）。
- 模型白名单 / RBAC 校验（T-005a/b/c 完整版）。
- 配额预扣 / RPM/TPM 限流 / 退款补扣（T-009）。
- usage 解析与计费入账（**T-008**）。本任务只**透传** usage，不解析、不入库。
- circuit-breaker / fallback / 重试（routes 表与 T-005c 之后）。
- Anthropic-compat endpoint（推后）。
- 多 region / sharding。

**Codex propose 前置 (必须)**:
在本条目下追加 `## PROPOSAL` 区块，覆盖：
1. **代理实现选择**：`httputil.ReverseProxy` vs 手写 `http.Client` + 双向 copy。SSE 透传与 `stream_options.include_usage` 注入哪种实现更干净？给出取舍。
2. **请求体改写策略**：方舟需要注入 `thinking.disabled` + `stream_options.include_usage`，但客户端可能传也可能不传。如何在不破坏其他字段的前提下注入？建议方案：解析为 `map[string]any` → 改 → 再序列化。但要考虑 client 流式请求体的大小上限（建议 1MB 同 dev endpoint）。
3. **`internal/proxy` 包结构**：暴露的接口签名（如 `type ArkProxy struct{...}; func (p *ArkProxy) ServeHTTP(...)` 或 middleware-style 包装）。
4. **超时配置**：connection / write / first-byte / total 四类超时的具体值。Phase 0 实测最快 ~1.8s，建议给 60s total 容忍上游慢响应；SSE 不应受 total deadline 限制（建议 idle timeout 30s）。
5. **错误判定矩阵**：什么是 `upstream_timeout` vs `upstream_connection_failed` vs `upstream_5xx` vs `upstream_invalid_response`。给出 Go error 类型识别策略（`errors.Is`、`net.Error.Timeout()` 等）。
6. **测试上游 mock**：`httptest.NewServer` 模拟流式上游怎么写最干净？给出最小 fixture 设计。

**依赖**: T-006a approved（已通过）。

**预计工作量**: 5-8 小时（含 propose）。

**Demo-Ready 衔接**: T-008 接 T-006a/T-007 后，从 SSE 最后 chunk 提取 usage → 算 cost → 写 cost_ledger / usage_events。**T-008 不重写 T-007 任何逻辑**，只在 T-007 已经 emit 给客户端**之后**做异步入账（或同步在 response 后做都行，T-008 PROPOSAL 时定）。

## PROPOSAL

1. 代理实现选择

选择手写 `http.Client`，不使用 `httputil.ReverseProxy`。原因是 T-007 的核心不是裸反代，而是 OpenAI-compatible 请求体改写、上游 `Authorization` 注入、客户端虚拟 Key header 丢弃、SSE chunk 逐段 flush、统一错误 envelope，以及上游响应 header 白名单过滤。`ReverseProxy` 的 `Director/ModifyResponse/ErrorHandler` 可以做一部分，但请求体改写和 SSE idle timeout 会变得绕，测试时也更难精确断言。

实现上只代理 `POST /v1/chat/completions` 到 `cfg.Ark.OpenAIBaseURL + "/chat/completions"`。非流式路径读取上游完整响应后再写客户端；流式路径拿到上游 2xx SSE 响应后立即写 header，随后 `Read` → `Write` → `Flush`，不做 usage 解析、不缓存、不重试。

2. 请求体改写策略

请求体先用 `http.MaxBytesReader` 限制到 1 MiB，再用 `json.Decoder.UseNumber()` 解到 `map[string]any`，只改必须字段，其他未知字段原样保留后 `json.Marshal` 重新序列化。无效 JSON 或超过大小限制返回 400 `invalid_request`，不打上游。

改写规则：
- `model` 强制替换为 `cfg.Ark.DefaultModel`，Demo-Ready 阶段只走方舟默认模型。
- 当 `cfg.Ark.DisableThinking == true` 时，顶层强制设置 `thinking: {"type":"disabled"}`；客户端已传 `thinking` 也覆盖。
- 仅当 `stream == true` 时，强制设置 `stream_options.include_usage = true`。如果客户端已有 object 类型的 `stream_options`，保留其他字段并覆盖 `include_usage`；如果传了非 object，替换成 `{"include_usage":true}`。非流式请求不主动添加 `stream_options`。
- 上游请求 header 使用新 header 集合：注入 `Authorization: Bearer <Ark key>`、`Content-Type: application/json`、内部 `X-Request-Id`；不转发客户端 `Authorization`、Cookie、Hop-by-hop header。

3. `internal/proxy` 包结构与接口签名

新增 `internal/proxy`，先只放 Ark OpenAI-compatible chat proxy，不引入第三方依赖：

```go
type ArkChatConfig struct {
	BaseURL          string
	APIKey           string
	DefaultModel     string
	DisableThinking  bool
	MaxRequestBytes  int64
	Timeouts         TimeoutConfig
}

type TimeoutConfig struct {
	Connect       time.Duration
	Write         time.Duration
	FirstByte     time.Duration
	NonStreamTotal time.Duration
	SSEIdle       time.Duration
}

type ArkChatProxy struct {
	cfg    ArkChatConfig
	client *http.Client
	logger *slog.Logger
}

func NewArkChatProxy(cfg ArkChatConfig, logger *slog.Logger, client *http.Client) *ArkChatProxy
func (p *ArkChatProxy) ServeHTTP(w http.ResponseWriter, r *http.Request)
```

`cmd/gateway` 负责把 `config.ArkConfig` 映射成 `proxy.ArkChatConfig` 并替换现有 `handleChatCompletions` 占位。`ArkChatProxy.ServeHTTP` 内部会读取 `auth.SubjectFromContext(r.Context())` 与 `httpx.RequestIDFromContext(r.Context())`，日志只写 `request_id/user_id/org_id/api_key_id/upstream_status/stream/duration`，不写 prompt、真实 Ark key、完整虚拟 Key 或完整 `Authorization`。

4. 超时配置矩阵

默认值放在 `internal/proxy`，T-007 暂不新增 env var，后续如压测需要再配置化：

| 类型 | 默认值 | 实现方式 | 适用范围 |
| --- | --- | --- | --- |
| connection | 5s | `net.Dialer.Timeout` + `TLSHandshakeTimeout` | 非流式与 SSE |
| write | 10s | 请求体 1 MiB 上限 + 发起 upstream request 前的短 `context.WithTimeout` 保护 | 非流式与 SSE |
| first-byte | 20s | `Transport.ResponseHeaderTimeout` | 非流式与 SSE 首包 |
| total | 60s | 非流式请求使用 `context.WithTimeout` | 仅非流式 |
| SSE idle | 30s | 流式复制时每次成功读/写 chunk 后重置 idle timer，超时 cancel upstream context | 仅 SSE |

SSE 不设 total deadline，避免长回复被 60s 硬切。客户端断开或写失败时立即 cancel upstream context。gateway `http.Server` 暂不设置 `WriteTimeout`，避免服务器级 write deadline 把 SSE 长连接切掉。

5. 错误判定矩阵

统一由 `internal/proxy` 写 gateway 风格 envelope：`{"error":{"message":"upstream request failed","type":"gateway_error","code":"..."}}`。除 `upstream_not_configured` 用 503 外，下面四类都返回 502；上游 5xx/失败时不透传上游错误体。

| code | 判定 | Go 识别策略 |
| --- | --- | --- |
| `upstream_timeout` | dial/header/nonstream total/SSE idle 任一 deadline 触发 | `errors.Is(err, context.DeadlineExceeded)`、`os.IsTimeout(err)`、`net.Error.Timeout()`；idle timer 主动 cancel 时标记 cause |
| `upstream_connection_failed` | 未拿到上游 response 的 DNS/refused/no route/TLS 握手失败 | `*url.Error` 包裹 `*net.OpError`，且不属于 timeout；或 `http.Client.Do` 返回 err 且 `resp == nil` |
| `upstream_5xx` | 上游返回 `500-599` | 收到 `resp` 后按 status 判定，关闭 body，不读取或透传错误体 |
| `upstream_invalid_response` | 上游返回无法安全转发的响应 | 非流式读 body 失败；流式期望 SSE 但 2xx 响应 `Content-Type` 非 `text/event-stream`；copy 首 chunk 前读失败 |

非流式的 2xx/4xx 响应按接受标准透传 status + body，但仍过滤响应 header。流式一旦已经把 2xx header 发给客户端，后续读写错误只能中断连接并记录日志，不能再补写 JSON envelope。

6. 测试上游 mock 设计

`internal/proxy` 单测全部用 `httptest.NewServer`，mock 只实现 `/chat/completions`。每个 case 读取并断言上游请求：路径、method、`Authorization` 是 Ark key、客户端虚拟 Key 未透传、`X-Request-Id` 存在、`model/thinking/stream_options` 已被改写。

fixture 设计：
- 非流式：mock 返回 `application/json`，body 含最小 OpenAI-compatible `choices` + `usage`，测试客户端拿到原 status/body。
- 流式：mock 返回 `text/event-stream`，用 `http.Flusher` 逐行写 `data: {...}\n\n`，最后一条使用 `choices:[]` + `usage`，再写 `data: [DONE]\n\n`。fixture 优先复用 `testdata/golden/ark/openai_stream_no_thinking_with_usage.txt`；单测只校验完整透传和最后 usage chunk 到达，不解析入账。
- timeout：mock 睡过 `FirstByte` 或 SSE idle，断言 `upstream_timeout`。
- upstream 502：mock 返回 502 + 任意 body，断言 gateway 返回统一 502 envelope 且不含 mock body。
- connection failed：使用 `127.0.0.1:1` 或关闭的 listener 构造 BaseURL，断言 `upstream_connection_failed`。
- missing API key：`APIKey == ""` 时不启动 upstream call，直接 503 `upstream_not_configured`。

边界确认：T-007 不接配额、不接 routes 表、不接重试、不接 Anthropic-compat、不接 usage 解析或入账；usage 只作为响应内容透传给客户端，T-008 再消费。

---

## T-005a RBAC 权限模型与策略点 [phase:1] [owner:codex] [status:todo]

**目标**: 把 T-003 写入的 `roles` / `role_assignments` 升级为运行时可用的权限模型；不接入具体 API（留 T-005b/c）。

**接受标准**:
- [ ] 三角色含义明确并写入 ADR：
  - `admin`: 全量写权限（CRUD API Keys / Quotas / Routes / Providers），可查所有 user 用量。
  - `member`: 调用数据面 + 查自己的用量；不可改组织级配置。
  - `viewer`: 只读管理台（overview / 自己的 logs），不可调用数据面。
- [ ] `internal/auth` 暴露 `Subject{UserID, OrgID, Role}` 与 `Can(subject, action, resource) bool`；action/resource 用枚举常量，不传字符串。
- [ ] 单元测试覆盖率 ≥ 90%，含 RBAC 越权矩阵（3 角色 × 至少 6 个 action × 至少 3 个 resource scope）。
- [ ] 越权时不返回敏感错误信息（统一 403 envelope），错误码与 gateway envelope 风格一致。
- [ ] 写新 ADR `0003-rbac-model.md`。

**不在范围**:
- HTTP 中间件接线（T-005b）。
- 虚拟 Key gateway 鉴权（T-005c）。

**Codex propose 前置 (必须)**:
列出 (1) action / resource 枚举完整清单 (2) role-action 矩阵 (3) 与未来 organization-scoped tenant 的兼容路径 (4) 是否引入 Casbin / open-policy-agent 等第三方库以及取舍。

---

## T-005b Admin API 鉴权与中间件 [phase:1] [owner:codex] [status:todo]

**目标**: 把 T-005a 的 RBAC 接到 admin 现有 / 待加的所有路由上。

**接受标准**:
- [ ] admin 所有路由强制带 `Authorization: Bearer <admin_token>`，未带或非法 token 返回 401（不是 403）。
- [ ] admin token 与虚拟 Key 完全隔离：admin token 不能用于 gateway 数据面；虚拟 Key 不能用于 admin。
- [ ] 中间件查询 user → role 链路，写入 `r.Context()`；handler 仅消费 `auth.SubjectFromContext(ctx)`，禁止 handler 内部再去查 DB 角色。
- [ ] 每条写操作产生 `audit_logs` 一条，含 actor / before / after。
- [ ] 越权返回 `403` + 统一 envelope，不区分"资源不存在"与"无权限"（防探测）。
- [ ] 集成测试：admin / member / viewer 三种 token 对所有 admin 路由的越权矩阵全绿。

**不在范围**:
- 虚拟 Key 鉴权（T-005c）。
- 实际新增 admin 写接口（创建 Key / 调整额度等）—— 这部分由 T-006 起的后续任务推进，但 T-005b 必须把已存在的 `/api/admin/overview` 接到 RBAC 上。

**Codex propose 前置 (必须)**:
(1) admin token 怎么存（数据库表设计 / 是否复用 api_keys 表加 owner_type） (2) token 哈希策略 (3) 与 viewer "只读" 的精确接口列表。

---

## T-005c 虚拟 Key Gateway 鉴权 [phase:1] [owner:codex] [status:todo]

**目标**: gateway 数据面校验内部虚拟 Key，绑定 user / organization，执行模型白名单。

**接受标准**:
- [ ] gateway `/v1/*` 强制带 `Authorization: Bearer <virtual_key>`。
- [ ] Key 哈希存储（`api_keys.key_hash`）；明文 Key 仅在创建时返回一次（由 T-005b 后续接口承接）。
- [ ] `models_allowlist` 命中检查：请求的模型不在白名单 → 403 + envelope。
- [ ] 校验失败统一 401 envelope；不暴露"key 存在但被禁用"与"key 不存在"的差异（防探测）。
- [ ] 校验通过后将 `Subject` 写入 `r.Context()` 供后续 quota / proxy 阶段使用。
- [ ] 测试覆盖率 ≥ 85%；含 timing-equal 的字符串比较（防 timing attack）。

**不在范围**:
- 实际转发到上游（T-007）。
- 配额预扣（T-009）。

**Codex propose 前置 (必须)**:
(1) Key 格式（前缀 + 随机段长度 + 字符集）(2) `key_prefix` 索引策略 (3) cache 加速方案（Redis or in-process LRU）以及失效边界。

---

## T-100 L2 端到端正确性套件（1 admin + 10 user 真方舟） [phase:1] [owner:codex] [status:blocked-by-T-005c-T-007-T-008-T-009]

**目标**: 作为 Phase 1 的验收门，证明 OmniToken 在企业最小可信场景下账务闭环、RBAC 隔离、限流正确、成本可控。

**接受标准** (实施细节由 Codex propose，但下列任何一项失败即 Phase 1 不通过):
- [ ] **0 panic / 0 data race / 0 goroutine leak** 全程。
- [ ] **账本闭环**: `Σ 每个 user 的实际消耗 == admin.overview.total_tokens`，且 `Σ cost_ledger.cost_usd` 与方舟厂商账单误差 ≤ 1%（在测试结束时打印两边数字做断言）。
- [ ] **RBAC 隔离矩阵全过**: member token 不能调 admin 写接口；viewer token 不能调 gateway 数据面；user A 用自己虚拟 Key 不能查 user B 的 usage。
- [ ] **限流正确性**: 触发 RPM 超额时返回 429 且**未扣减**配额（未消费的 token 不入账）。
- [ ] **成本上限保护**: 单次跑必须实现 `MAX_REQUESTS` 硬上限（环境变量），超过即测试主动失败而非继续烧 token。建议默认 600。
- [ ] **跑完时长** ≤ 10min，预计单次成本 ≤ 5 RMB。
- [ ] **CI**: 用 GitHub Actions `workflow_dispatch` + `schedule: nightly`，**禁止** 在 PR / push 上跑。Secret 通过 `secrets.OMNITOKEN_ARK_API_KEY` 注入。
- [ ] Phase 1 验收门: 至少 **3 次连续 nightly 通过** 才宣告 Phase 1 完成。

**不在范围**:
- 吞吐压测（L3，留 Phase 2）。
- 公平性测试（L4，留 Phase 2）。
- mock 上游（L2 必须真方舟）。

**Codex propose 前置 (必须)**: 在本条目下追加 `## PROPOSAL` 区块，覆盖至少：
1. fixture 加载方式（docker-compose 直接起 vs testcontainers）以及对调试体验的取舍。
2. 11 goroutine 的编排骨架（go routine + ctx 取消 / errgroup / worker pool）。
3. 账本断言的精确公式（如何处理 in-flight 请求与最终一致性窗口）。
4. RBAC 越权矩阵的最小测试集（不要枚举 N×M，给出代表性子集）。
5. `MAX_REQUESTS` 与 `MAX_DURATION` 双闸的实现位置与失败行为。
6. 方舟厂商账单对账数据怎么取（API 还是手动导出）；如不可自动取，给一个"半自动"对账步骤。
7. nightly workflow 的 yaml 草案 + secret 注入路径。

**依赖**: T-005c（虚拟 Key 鉴权）、T-007（SSE 代理）、T-008（usage parser）、T-009（限流与预扣）必须先完成。本条目暂处于 `blocked-by` 状态；上述四项都进入 `approved` 后 Claude 解锁此任务。

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
