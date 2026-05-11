# REVIEW.md — OmniToken Review Log

## R-001 (对应 T-001, commit `8f8f3a7` + `6d76909`)

> 本地验证: `go vet ./...`、`go test ./...` 全绿；`go build -o gateway.exe ./cmd/gateway` & `admin.exe` 成功；
> 启动后 `/healthz`、`/v1/models`、`/api/admin/overview`、OPTIONS 预检均按预期返回；未实现的 `/v1/chat/completions` 返回 404；
> 主动注入 `Authorization: Bearer SECRET_*` 后日志未泄漏（slog 仅记录 method/path/status/duration_ms）。
> 额外用用户提供的火山方舟 API key (`ark-code-latest`) 实测了两条 upstream 端点（详见 §1.2），结果将用于 T-002+ 的 golden 语料。

### 1. 总体结论

**通过 Phase 0 验收，可继续推进 Phase 1**，但有 4 个 HIGH 与 5 个 MEDIUM 需在进入 Phase 1 之前清掉（见 §3 跟进任务）。脚手架质量高于第一周通常水平，主要扣分点在"Phase 0 已做但做得不彻底"的几处可观测性与安全细节，而不是新增功能债。

### 1.1 正面信号 [+]

- **依赖管理克制**：Phase 0 完全用标准库，避免提早锁版 `chi`/`viper`/`pgx` 等，与 ADR-0001 一致；符合 AGENTS.md §3.1 "三处重复再抽象"。
- **distroless + nonroot + CGO_ENABLED=0**：镜像最小化与运行时安全做得到位。
- **migrations up/down 双向**：表结构覆盖了第十一节安全基线（`api_keys.key_hash bytea`、`upstream_credentials.encrypted_secret bytea`、`audit_logs` 含 before/after_state jsonb）。
- **usage 维度完整**：`usage_token_breakdown` 把 `reasoning_tokens` / `cached_tokens` / `cache_creation_tokens` / `cache_read_tokens` / `image_tokens` / `audio_*` 都拆开列了。今天实测方舟 `ark-code-latest`（实际是 GLM-5.1，reasoning 模型）的 usage 字段刚好命中这套表设计，证明数据模型方向正确。
- **端口避让**：Postgres `15432` / Redis `16379` / NATS `14222` 都避开了主机常见端口，对开发者友好。
- **CORS preflight 测试**虽然简单但确实写了，且日志没有把 Authorization 打到 stdout。
- **Codex 在自己跑不了 race detector 时主动备注，并保留 CI Linux 端 `-race`**：风险沟通到位。

### 1.2 火山方舟实测结果（写在这里供后续 T-002 golden 语料引用）

| 维度 | OpenAI-compat | Anthropic-compat |
| --- | --- | --- |
| Endpoint | `POST https://ark.cn-beijing.volces.com/api/coding/v3/chat/completions` | `POST https://ark.cn-beijing.volces.com/api/coding/v1/messages` |
| Auth header | `Authorization: Bearer <key>` | `x-api-key: <key>` + `anthropic-version: 2023-06-01` |
| 配置模型名 | `ark-code-latest` | `ark-code-latest` |
| 实际后端 | `glm-5.1`（response `model` 字段 = "glm-5.1"，确认是 reasoning 模型） | `glm-5.1` |
| 非流式延迟 | 2.8s（默认开 reasoning） | 4.8s（默认开 thinking，输出全是 thinking） |
| 关 reasoning 后延迟 | **1.77s** 含完整 SSE 往返 | 未测 |
| 关 reasoning 字段 | `"thinking": {"type": "disabled"}`（请求体顶层） | 同 |
| usage in SSE | 默认 `chunk.usage = null`；开 `stream_options.include_usage:true` 后**最后一个 `choices:[]` 的 chunk 携带完整 usage** | usage 在最终 message_stop 事件 |
| usage 字段 | `prompt_tokens` / `completion_tokens` / `total_tokens` / `prompt_tokens_details.cached_tokens` / `completion_tokens_details.reasoning_tokens` | `input_tokens` / `output_tokens` / `cache_read_input_tokens` |
| reasoning 内容字段 | 非标准: `message.reasoning_content` | 标准: content array 中 `type:"thinking"` block |
| 计费坑 | reasoning_tokens 会吃 max_tokens 配额，若不关 thinking、max_tokens 太小，会出现 `content:""` + `finish_reason:"length"` —— 必须在 usage mapper 里把 reasoning_tokens 单独计入 `usage_token_breakdown.reasoning_tokens`，否则成本对不上。 | 同上 |

**最快响应配方（写进未来文档与 demo 默认）**：OpenAI-compat 端点 + `thinking:{type:"disabled"}` + `stream_options:{include_usage:true}`，可稳定 1.7-2.0s 内返回含 usage 的完整 SSE。

实测的 4 份原始响应已保存到 `testdata/golden/ark/`，供 T-007 SSE 反向代理与 T-010 usage mapper 回放使用。

### 2. 问题清单

#### CRITICAL — 0 项

无。Phase 0 没有触发数据安全或上线阻断风险。

#### HIGH — 4 项（进入 Phase 1 前必须修复）

- **H-1 `cmd/admin/main.go:106 withCORS` 把 `Access-Control-Allow-Origin` 硬编码成 `*`**，且同时允许 `Authorization` 头。在 Phase 0 只服务 mock GET 数据时勉强可接受，一旦下个任务接入鉴权与写接口，这等于把管理 API 暴露给任何 Origin 携凭据访问。**修法**：从 `OMNITOKEN_ADMIN_CORS_ORIGINS` 环境变量读取白名单（默认 `http://localhost:3000`）；非白名单 origin 不写 ACAO 头。落到下方 T-002。
- **H-2 `cmd/{gateway,admin}/main.go` 错误判断使用 `err != http.ErrServerClosed`**。Go 1.20+ 起规范要求 `errors.Is(err, http.ErrServerClosed)`，否则一旦上游用 wrap 过的 error（后续接入 TLS / autocert 时常见）就会被误判为 fatal 并 `os.Exit(1)`。两处都要改。
- **H-3 没有 graceful shutdown**。当前 `server.ListenAndServe` 没有 SIGINT/SIGTERM 拦截。Phase 1 一接 SSE，进程被 docker stop 直接杀掉，会让客户端拿到截断的流而我们 usage 落库丢失。`cmd/gateway` 必须先做 graceful shutdown 才能开始写代理。
- **H-4 `requestLogger` 没有 `request_id`**。规划第七节明确点过"尽早设计 request_id / trace_id / session_id"；实测日志显示 `duration_ms` 为 0（毫秒级精度不够，但更严重的是无法把同一请求的入站日志、上游调用日志、usage 事件串起来）。Phase 0 末就该加：入口生成或读取 `X-Request-Id`、写入 `r.Context()`、所有 log/响应头都带上。

#### MEDIUM — 5 项（建议在 Phase 1 内顺手修）

- **M-1 `migrations/000001_init.up.sql` 完全没有索引**。`usage_events(created_at)`、`usage_events(organization_id, created_at)`、`usage_events(api_key_id, created_at)`、`audit_logs(created_at)`、`cost_ledger(usage_event_id)` 是后台查询和管理台分页的必需路径，先建索引比后建便宜。
- **M-2 `docker-compose.yml` 中 `gateway`/`admin` 都 `depends_on: postgres/redis/nats healthy`**，但目前两个进程并不连这些组件，会让本地 dev 启动时间多 30-60 秒。在真正接入存储前先去掉这些 depends_on，或把它们改为可选 profile。
- **M-3 `migrations/` 被挂载到 Postgres `docker-entrypoint-initdb.d`，绕开了 `golang-migrate` 的 `schema_migrations` 表**。Phase 1 引入 golang-migrate 时会出现"migration 已应用但 Postgres 不知道"的状态。当前应明确：dev 容器初始化只跑 seed.sql，DDL 一律走 migrate up（启动脚本里执行）。
- **M-4 `cmd/gateway` 和 `cmd/admin` 重复了 `healthResponse` / `statusRecorder` / `requestLogger` / `writeJSON` 四个结构**。这是同一概念在两处出现的"第二次重复"，下次再出现就必须抽到 `internal/httpx`。可以放进 T-002 一起做。
- **M-5 `model_pricing` 缺 `effective_to`**，并且没有"取最新"视图。后续 Phase 2 价格变更时会出现"同一模型同时间多条 active 价格"的歧义。

#### NIT — 3 项

- N-1 `openapi.yaml` 中 `total_tokens` 类型 `integer` 应加 `format: int64`。
- N-2 `Makefile` 的 `lint vet:` 把两个 target 合并是少见写法，可读性差；拆成两个 target 更友好。
- N-3 `cmd/gateway/main.go` 的硬编码模型列表把日期都写死（`Created: 1715558400` 等），且没包含 `ark`。改成从 seed 后的 `model_catalog` 读取是 Phase 1 工作，Phase 0 至少把 `ark` 加进静态列表以便前端联调。

### 3. 跟进任务（已写入 `TASKS.md`）

| 任务 | 等级 | 说明 |
| --- | --- | --- |
| T-002 | HIGH | 修复 H-1/H-2/H-3/H-4，并完成 M-4 抽取 `internal/httpx`；加入方舟 provider 配置 |
| T-003 | MEDIUM | 数据库索引与迁移工具治理（M-1/M-3/M-5） |
| T-004 | NIT | docker-compose depends_on / Makefile / OpenAPI 等小项（M-2/N-1/N-2） |

请 Codex 先做 T-002，完成后在本条目下追加 `**Resolved**:` 与逐条回应；其余按 ID 顺序推进。
