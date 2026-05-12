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

---

## DEMO-READY ROUTE (2026-05-11 user-locked)

用户决策：先把 Demo-Ready 路线跑通（gateway 真转发方舟 + 计费落库 + admin 真查 DB + 前端 fetch 真实数据 + 最小种子），**再** push。进度：**75% (6/8)**。剩余：T-006c → T-006d → push。

---

## T-006c 前端 fetch 真实数据 [phase:1] [owner:codex] [status:todo]

**目标**: 把 `测试前端.html` 从 mock 数据改为 fetch 真实 admin API。

**接受标准**:
- [ ] Overview 视图的 KPI 卡片、trend 图、model share 饼图全部从 `GET /api/admin/overview` 拿实时数据。
- [ ] fetch 失败时展示友好的 error state（不是空白、不是 JS 报错）。
- [ ] CORS 与 admin 端口配置保持一致（`OMNITOKEN_ADMIN_CORS_ORIGINS` 包含前端 origin）。
- [ ] 不改后端 API 结构；不新增后端依赖。
- [ ] 手动打开 `测试前端.html` 能看到真实数据（需要本地 Postgres + seed + 至少 1 条 usage 记录）。

**不在范围**: 前端工程化 / React/Next.js 迁移 / 新增页面 / 用户管理视图改造。

**Codex propose 前置**: 不需要，按上述标准直接做。

**依赖**: T-006b approved（已通过）。

---

## T-006d Demo-Ready 端到端验收 [phase:1] [owner:codex] [status:blocked-by-T-006c]

**目标**: 跑通完整 Demo-Ready 链路并记录验收结果。

**接受标准**:
- [ ] 本地起 Postgres → migrate up → seed → gateway + admin → 用 demo virtual key 发 chat completion → 查 admin overview → 前端展示真实数据。
- [ ] 验收表（见下方 DEMO-READY VERIFICATION TABLE）所有项全绿。
- [ ] 录一段 curl + 前端截图作为 Demo 证据。

**依赖**: T-006c approved。

---

## DEMO-READY VERIFICATION TABLE (Claude 验收时填写)

### 1. 端到端功能矩阵

| 功能 | 状态 | 验证方式 | 备注 |
|------|------|---------|------|
| gateway `/healthz` | | curl | |
| gateway `/v1/models` | | curl | |
| gateway `/v1/chat/completions` (非流式) | | curl + virtual key | |
| gateway `/v1/chat/completions` (流式 SSE) | | curl --no-buffer | |
| usage 入账 (usage_events + cost_ledger) | | psql SELECT | |
| admin `/api/admin/overview` (真实数据) | | curl | |
| 前端 Overview (真实数据渲染) | | 浏览器截图 | |
| 虚拟 Key 鉴权 (无 key → 401) | | curl | |
| 虚拟 Key 鉴权 (错误 key → 401) | | curl | |

### 2. 性能基线（单实例 · 本地 Docker Postgres · 单次请求级）

| 指标 | 目标 | 实测 | 通过 |
|------|------|------|------|
| 非流式延迟 (含方舟) | ≤ 3s | | |
| 流式首 chunk (含方舟) | ≤ 2s | | |
| admin overview 查询 | ≤ 100ms | | |
| migrate up (6 versions) | ≤ 5s | | |

### 3. 安全基线核对

| 项目 | 通过 |
|------|------|
| 日志不含 Authorization / API Key / Prompt | |
| 401 不区分 key 不存在 vs disabled | |
| 上游错误不透传到客户端 | |
| 响应不透传 Server / X-Powered-By / Set-Cookie | |

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



## 未来任务 (Phase 1 完整验收后)

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
