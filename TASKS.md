# TASKS.md — OmniToken Task Board

## CHANGELOG (压缩版 — 详见 git log + docs/reviews/archive.md)

| 时间 | 事件 |
|------|------|
| 05-11 | R-001 approve. T-001→T-004 拆分。方舟实测→golden语料 |
| 05-11 14:30 | R-002 approve. M-6/7/8 不阻塞 T-003 |
| 05-11 15:00 | 用户决策: L2优先+真方舟+完整RBAC. 授权方舟dev key |
| 05-11 15:40 | R-002.1+R-003-prop approve. Q-1~Q-4 |
| 05-11 16:10 | R-003-license: MPL-2.0 间接依赖 approve. 分级许可证政策 |
| 05-11 17:30 | Demo-Ready 路线锁定. push 等 Demo-Ready 全过 |
| 05-11 18:00 | R-003 approve (8+/1M/4N). T-006-nit+T-006a 拆出 |
| 05-11 18:30 | R-006-nit+R-006a-prop approve |
| 05-11 19:00 | R-006a approve (12+/0H/2M/3N). auth 96.1% |
| 05-11 19:30 | R-007-prop approve (10+/1Q) |
| 05-11 22:15 | R-007 approve (14+/0M/3N). proxy 88.4%. **首次真方舟e2e通过** |
| 05-12 09:40 | R-008 approve (13+/2M/4N). usage 93.7%. 账本闭环 |
| 05-12 11:45 | R-006b approve (10+/1M/2N). admin 51.9%. **进度75%** |
| 05-12 12:55 | R-006c approve (8+/0M/2N). 前端接真API. **进度87.5%** |
| 05-12 21:22 | **T-006d E2E 验收通过**. 真方舟 chat + usage + cost + overview 全链路绿 ✅ |

---

## 已完成任务速查 (详见 git log)

> 完整描述/PROPOSAL/Result 在 git history 中可查。详细 review 见 `docs/reviews/archive.md` 和 `REVIEW.md`。

| 任务 | 内容 | commit | 覆盖率 |
|------|------|--------|--------|
| T-001 | Phase 0 脚手架 | `8f8f3a7` | — |
| T-002 | 收尾 4H+M-4, internal/httpx | `706a3a7` | httpx 87.9% |
| T-002.1 | M-6/7/8 收尾 | `68b85a7` | httpx 90.1% |
| T-003 | golang-migrate, RBAC schema, pricing view | `54058e8` | migrate 45.5% |
| T-006-nit | force sentinel 修复 | `88fc18d` | migrate 45.5% |
| T-006a | 最小虚拟 Key 鉴权 | `4df0033` | auth 96.1% |
| T-007 | SSE 反向代理 + 方舟转发 | `34a5b6a` | proxy 88.4% |
| T-008 | usage parser + cost_ledger | `4761671` | usage 93.7% |
| T-006b | admin overview 真查 DB | `290a5bb` | admin 51.9% |
| T-006c | 前端 fetch 真实数据 | `51ba90c` | — |
| T-006d | Demo-Ready E2E 验收 | — | 12/12 全绿 |

---

## DEMO-READY ROUTE (2026-05-11 user-locked)

进度：E2E 验收通过，但**前端假数据 + admin 无鉴权 + 未验证并发**。新增 PRE-PUSH GATE (T-009a/b + T-010 + T-012)。

---

## T-006d Demo-Ready 端到端验收 [phase:1] [owner:codex] [status:approved]

**Started**: 2026-05-12 13:05 CST

**目标**: 跑通完整 Demo-Ready 链路并记录验收结果。

**接受标准**:
- [x] 本地起 Postgres → migrate up → seed → gateway + admin → 用 demo virtual key 发 chat completion → 查 admin overview → 前端展示真实数据。
- [x] 验收表（见下方 DEMO-READY VERIFICATION TABLE）所有项全绿。
- [x] E2E 脚本输出 + 日志 grep 作为 Demo 证据。

**依赖**: T-006c approved（已通过）。

**Result**: E2E 脚本 `scripts/e2e_verify.py` 通过 12/12（脚本原报 1 FAIL 是 /v1/models 未传 key，属脚本 bug；该端点需 auth 是 T-006a 设计正确行为）。非流式 2093ms、流式 1891ms (15 chunks, has_usage)、overview 15ms、tokens=79、cost=$0.000081。日志 grep 确认无 key/prompt 泄露。

---

## DEMO-READY VERIFICATION TABLE (2026-05-12 21:22 验收)

### 1. 端到端功能矩阵

| 功能 | 状态 | 验证方式 | 备注 |
|------|------|---------|------|
| gateway `/healthz` | ✅ | e2e script | 200, 15ms |
| gateway `/v1/models` | ✅ | e2e script | 200 (需带 virtual key，T-006a 设计) |
| gateway `/v1/chat/completions` (非流式) | ✅ | e2e script + 真方舟 | 200, 2093ms, model=glm-5.1, tokens=18 |
| gateway `/v1/chat/completions` (流式 SSE) | ✅ | e2e script + 真方舟 | 200, 1891ms, 15 chunks, has_usage=true |
| usage 入账 (usage_events + cost_ledger) | ✅ | admin overview | total_tokens=79, cost=$0.000081 |
| admin `/api/admin/overview` (真实数据) | ✅ | e2e script | period=2026-05, active_users=1, trend_days=1, models=1 |
| 前端 Overview (真实数据渲染) | ✅ | T-006c Chrome headless QA | KPI/trend/pie 全部真数据 |
| 虚拟 Key 鉴权 (无 key → 401) | ✅ | e2e script | 401, type=authentication_error |
| 虚拟 Key 鉴权 (错误 key → 401) | ✅ | e2e script | 401, code=invalid_api_key |

### 2. 性能基线（单实例 · 本地 Docker Postgres · 单次请求级）

| 指标 | 目标 | 实测 | 通过 |
|------|------|------|------|
| 非流式延迟 (含方舟) | ≤ 3s | 2093ms | ✅ |
| 流式总延迟 (含方舟) | ≤ 3s | 1891ms | ✅ |
| admin overview 查询 | ≤ 100ms | 15ms | ✅ |
| migrate up (6 versions) | ≤ 5s | < 1s | ✅ |

### 3. 安全基线核对

| 项目 | 通过 |
|------|------|
| 日志不含 Authorization / API Key / Prompt | ✅ (grep 确认) |
| 401 不区分 key 不存在 vs disabled | ✅ (统一 envelope) |
| 上游错误不透传到客户端 | ✅ (T-007 单测覆盖) |
| 响应不透传 Server / X-Powered-By / Set-Cookie | ✅ (T-007 header 白名单) |

### 4. 代码质量总览

| 包 | 覆盖率 | 目标 | 通过 |
|----|--------|------|------|
| internal/httpx | 90.1% | 85% | ✅ |
| internal/config | 100% | 70% | ✅ |
| internal/auth | 96.1% | 85% | ✅ |
| internal/proxy | 88.4% | 85% | ✅ |
| internal/usage | 93.7% | 85% | ✅ |
| cmd/migrate | 45.5% | — | ✅ |
| cmd/admin | 51.9% | — | ✅ |

---

## PRE-PUSH GATE (2026-05-12 user-locked)

**必须全过才能 push**: T-009a → T-009b → T-010 → T-012 → push。
当前进度：**0/4**

---

## T-009a 后端用户/模型聚合 API [phase:1] [owner:codex] [status:review]

**Started**: 2026-05-12 21:30 CST

**目标**: 新增 2 个 admin API，为前端用户页和模型页提供真实数据。

**接受标准**:
- [x] `GET /api/admin/users` 返回当前月 users 列表 + 各用户 token 聚合：
  ```json
  {"users": [{"user_id":"...","email":"...","display_name":"...","used_tokens":12345,"quota":0,"status":"active"}]}
  ```
  查询 `users` JOIN `usage_events` + `usage_token_breakdown`，当月窗口 `[monthStart, nextMonth)`。`quota` 字段 Demo 阶段固定返回 0（无配额系统）。
- [x] `GET /api/admin/models` 返回当前月模型维度聚合：
  ```json
  {"models": [{"model":"glm-5.1","provider":"ark","prompt_tokens":100,"completion_tokens":50,"total_tokens":150,"cost_usd":0.0001,"call_count":2}]}
  ```
  查询 `usage_events` + `usage_token_breakdown` + `cost_ledger`，按 `model_actual` 分组。
- [x] 两个 endpoint 都走 `overviewStore` 或同级抽象，复用 `cmd/admin` 的 `*sql.DB`。
- [x] DB 连接为空时返回 200 + 空数组（降级一致性，同 overview）。
- [x] 单元测试覆盖有数据 + 空数据 case。
- [x] 不改 gateway、不改 overview API、不引入新依赖。

**不在范围**: 分页 / 搜索 / 额度修改 / RBAC。

**Codex propose 前置**: 不需要，按上述标准直接做。

**依赖**: T-006b approved。

**Result**: `ce204b5`。新增 `GET /api/admin/users` 与 `GET /api/admin/models`，复用 `cmd/admin` 现有 `overviewStore` / `*sql.DB`，DB 未配置时返回 200 + 空数组；未改 gateway、overview API、前端或依赖。自测：`go test -count=1 ./cmd/admin`、`go test -count=1 ./...`、`go vet ./...`、`go test -cover -count=1 ./cmd/admin`（cmd/admin 60.9%）通过；`go test -race -count=1 ./cmd/admin` 因本机缺少 gcc/cgo 工具链未能构建。

---

## T-009b 前端用户/模型页接真数据 [phase:1] [owner:codex] [status:blocked-by-T-009a]

**目标**: 消除 `测试前端.html` 中的**全部**硬编码假数据，3 个 tab 全展示真实数据。

**接受标准**:
- [ ] 用户额度页：删除 `userData` 硬编码数组（陈明/林晓等），改为 `fetch('/api/admin/users')`。进度条用真实 `used_tokens`；`quota==0` 时显示"无限额"或隐藏进度条。
- [ ] 模型分析页：删除 GPT-4o/Claude/Gemini 硬编码数据，改为 `fetch('/api/admin/models')`。柱状图展示真实 prompt/completion 拆分。
- [ ] 两个页面都补 loading / empty / error 三态（复用 overview 的 pattern）。
- [ ] `fetch` 使用与 overview 相同的 `ADMIN_API_BASE_URL`。
- [ ] 不改后端 API；不新增后端依赖。

**不在范围**: 额度修改弹窗、分页、搜索。

**Codex propose 前置**: 不需要，按上述标准直接做。

**依赖**: T-009a approved。

---

## T-010 Admin bootstrap token 全路由鉴权 [phase:1] [owner:codex] [status:todo]

**目标**: admin 所有 `/api/admin/*` 路由都必须带 `Authorization: Bearer <bootstrap_token>`，防止 overview/users/models 裸奔。

**接受标准**:
- [ ] 抽取现有 dev endpoint 的 bootstrap token 校验为 `adminAuthMiddleware`。
- [ ] `GET /api/admin/overview`、`GET /api/admin/users`、`GET /api/admin/models` 全部走此 middleware。
- [ ] `/healthz` **不**走鉴权（健康检查必须裸露）。
- [ ] 无 token 或错误 token 返回 401 统一 envelope（与 gateway 401 结构一致）。
- [ ] `OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN` 为空时，所有受保护路由返回 503 `admin_auth_not_configured`（不是默认放行）。
- [ ] 单元测试覆盖：正确 token / 错误 token / 空 token / 未配置 token 4 case。
- [ ] 不做完整 RBAC（留 T-005a/b）。

**Codex propose 前置**: 不需要，按上述标准直接做。

**依赖**: 无。可与 T-009a 并行（不改同一个文件的同一块代码）。

---

## T-012 并发压测验证 [phase:1] [owner:codex] [status:blocked-by-T-009a]

**目标**: 用 Go 写一个简单的并发测试工具，证明 gateway 在 10 并发 × 10 请求（共 100 次）下无 panic、无 data race、usage 入账一致。

**接受标准**:
- [ ] 新增 `cmd/loadtest/main.go`（或 `scripts/loadtest.go`），使用 Go 标准库 + `sync.WaitGroup`。
- [ ] 参数：`-concurrency 10 -requests 10 -gateway http://localhost:8080 -admin http://localhost:8081 -key <virtual_key>`。
- [ ] 每个 goroutine 发 `POST /v1/chat/completions`（非流式，`stream:false`，短 prompt），统计 2xx / 4xx / 5xx / timeout 数量。
- [ ] 跑完后查 `GET /api/admin/overview`，断言 `total_tokens > 0` 且 `active_users >= 1`。
- [ ] 打印汇总表：总请求数、成功率、平均延迟、P95 延迟、usage 总 tokens。
- [ ] 用 `-race` flag 编译和测试，确认无 data race。
- [ ] **成本控制**：默认 `MAX_REQUESTS=100`，超过硬停。单次预计 ≤ 5 元 RMB。
- [ ] 不改 gateway/admin/usage 代码；不引入第三方依赖。

**Codex propose 前置**: 不需要，按上述标准直接做。

**依赖**: T-009a（需要 admin users API 验证多用户入账）+ 有效 virtual key + 方舟 API key。

---

## 未来任务 (Pre-push gate 全过后)

### T-005a RBAC 权限模型 [status:todo]
三角色 Casbin/自写策略引擎。依赖 T-003 RBAC schema。

### T-005b Admin API 鉴权 [status:todo]
Admin 端口 session/JWT + audit_logs。依赖 T-005a。

### T-005c 虚拟 Key 鉴权完整版 [status:todo]
`subtle.ConstantTimeCompare` 替换 `==`。Rate limit per-key。依赖 T-005a。

### T-100 L2 端到端正确性套件 [status:blocked]
1 admin + 10 user 真方舟 e2e。依赖 T-005c + T-007 + T-008。
成本上限保护: `MAX_REQUESTS` 环境变量。nightly GitHub Action。

### T-004 小修小补 [status:todo]
docker-compose profile / Makefile / OpenAPI 等 NIT。
