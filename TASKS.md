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
| 05-18 22:14 | T-010 admin auth + ConstantTimeCompare 实施 (`760020f`) |
| 05-18 22:29 | T-012 cmd/loadtest 实施 (`d7e78e8`, 73.6%) |
| 05-18 23:xx | **R-010 / R-012 approve. Pre-push gate 4/4 ✅** 可 first push |
| 05-18 23:xx | **设计理念锁定**: 性价比 / 权限额度 / 安全审计 三角；定位"先搭底座再叠应用"。Phase 2 重组为 2-A/2-B/2-C |
| 05-18 23:56 | T-013 PROPOSAL by Codex (`f444ba2`) — JSONB 演进 + ActorResolver DI + expvar metric |
| 05-19 | **R-013-prop approve** (10+ / 0C / 0H / 1M-Q / 2L-Q / 4N)。Codex 可开实施 commit |
| 05-19 00:24 | T-013 实施 (`d5b379d`, audit 95.9%) — migration 000007 + internal/audit + cmd/admin 接线 |
| 05-19 | **R-013 approve** (5+/0C/0H/0M/3N)。Phase 2-A 进度 1/2，T-014 可起 |
| 05-19 00:38 | T-014 告警段 PROPOSAL by Codex (`e2a9d98`) |
| 05-19 00:58 | T-014 audit 查询 API + 前端 Audit tab 实施 (`ff2b7a6`, cmd/admin 71.4%) |
| 05-19 | **R-014-prop approve** (5+/2N) + **R-014a approve partial** (5+/3N)。告警段待实施 |
| 05-19 10:04 | T-014 告警段实施 (`db425c7`, anomaly 82.1%) — Monitor + PostgresStore + admin 接线 |
| 05-19 | **R-014b approve** (5+/0C/0H/0M/2N)。**Phase 2-A 完成 2/2 ✅**，T-005a 可起 |
| 05-19 10:25 | T-005a PROPOSAL by Codex (`519086e`) — 硬编码 map / 单 action key / 不 cache / 低基数 reason |
| 05-19 | **R-005a-prop approve** (5+/2N)。Codex 可开实施 |
| 05-19 10:46 | T-005a 实施 (`89ef188`, rbac 89.9%) — Engine + 硬编码 policy + PostgresStore + fall-through 测试 |
| 05-19 | **R-005a approve** (5+/0C/0H/0M/2N)。Phase 2-B 进度 1/3，T-015 / T-005b 可起 |
| 05-19 | **路线重排**: 上线优先 + 一次联调。Phase 2-B = T-015 + T-005b + T-INT 三卡；Phase 2-C 推 vNext。v1 ETA ~1w |
| 05-19 11:05 | T-015 PROPOSAL by Codex (`7462418`) — USD cents / DB SUM / 402 envelope / inline 编辑 + role provider |
| 05-19 | **R-015-prop approve** (5+/3N)。Codex 可开实施 |
| 05-19 | T-015 实施 commit `0041c11` (`feat: enforce monthly user budget`). tmp-go-cache 已清 |
| 05-19 | **R-015 approve** (5+/0C/0H/0M/3N)。联调手册落 `docs/release/v1-t015-quota-walkthrough.md`。**Phase 2-B 进度: 2/5** |
| 05-19 | **AGENTS.md §3.3 新增**: 测试环境强制 docker-compose；禁本地装 PG/Redis/NATS |
| 05-19 | **Ark coding plan 洞察**: 单 key 5 模型（doubao/deepseek/glm/kimi/minimax）。T-016 多 key 加密推迟至第二家 provider；**T-017a 虚拟模型解析**抽出加进 v1 |

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
| T-009a | admin users/models 聚合 API | `ce204b5` | admin 60.9% |
| T-009b | web/ 正式静态前端三页接真 API | `10177c8` | — |
| T-010 | admin bootstrap token 全路由鉴权 + ConstantTimeCompare | `760020f` | admin 63.2% |
| T-012 | cmd/loadtest 标准库并发压测工具 | `d7e78e8` | loadtest 73.6% |
| T-013 | audit_logs schema + middleware (Phase 2-A) | `d5b379d` | audit 95.9% |
| T-014 | audit 查询 API + 前端 tab + 异常 key 告警 (Phase 2-A 收官) | `ff2b7a6` + `db425c7` | anomaly 82.1% / admin 72.0% |
| T-005a | RBAC 三角色策略引擎 (Phase 2-B 起点) | `89ef188` | rbac 89.9% |

> T-006d 完整验收明细见 R-006d 归档；T-009a/T-009b 详情见 R-009a / git；T-010/T-012/T-013/T-014/T-005a 详情见 `REVIEW.md`。

---

## DEMO-READY ROUTE (2026-05-11 user-locked)

E2E 验收通过，但**前端假数据 + admin 无鉴权 + 未验证并发**。Pre-push gate: T-009a → T-009b → T-010 → T-012。

---

## PRE-PUSH GATE (2026-05-12 user-locked)

**结果**: **4/4 ✅** — T-009a / T-009b / T-010 / T-012 全部 approve。下一步: Codex 执行 first push（`git push origin master` 或建议改成 PR-based main 分支）。

> 所有 4 个任务的详细 acceptance + Result 已迁出，仅留速查表 + Review 归档。

---

<!-- 完成任务体（T-009a / T-009b / T-010 / T-012）已全部删除；速查表 + R-009a/R-010/R-012 + git log 是单一信息源。 -->

---

## 🎯 Phase 2 底座三角路线 (2026-05-18 user-locked)

> **设计理念锁定** (见 `规划.md` §零A): OmniToken 的最简底座 = 「**性价比资源** + **权限/额度** + **安全/审计**」。定位为"先搭底座，再叠应用"——Phase 2 只做控制面 + 数据面，不做 chat UI / playground / agent。
>
> **最简底座 ETA**: 顺序 2.5–3 周；可并行后乐观 ~2 周。

### 总览（v1 上线路线，2026-05-19 重排）

> **2026-05-19 用户决策**: 上线优先 + 一次前后端联调。Phase 2-B 收尾即视为 v1；Phase 2-C 多 key 池 / fallback 推到 **vNext**（已具备底座理念三角的两角即可对外说"企业 RBAC + budget + 审计"）。

| 子轨 | 任务 | 一句话 | 估时 | 依赖 | 状态 |
|---|---|---|---|---|---|
| 2-A 审计 | T-013 audit_logs schema + middleware | admin 写操作落 audit | 2d | — | ✅ |
| 2-A 审计 | T-014 audit 查询 + 异常告警 | audit tab + 阈值 WARN | 2d | T-013 | ✅ |
| 2-B 权限 | T-005a RBAC 引擎 | 三角色硬编码 matrix | 3d | T-013 | ✅ |
| 2-B 额度 | T-015 用户月度 budget + quota 编辑（全栈） | usage 入账前 budget 校验 → 402；admin Users 页可改 quota | 3d | T-005a | todo |
| 2-B 权限 | T-005b admin auth 升级 + 登录页 + RBAC 落地（全栈） | 替换 bootstrap → session/JWT；RBAC 挂 admin 写；前端登录/退出 | 3d | T-005a + T-013 | todo |
| 2-B 性价比 | **T-017a 虚拟模型解析（单 key 5 模型）** | gateway 接 `chat-fast/balanced/quality/...` 改写为真实 Ark model；含 admin 列表 + 前端展示 | 1-1.5d | T-008 ✅ | todo |
| 2-B 联调 | **T-INT 前后端联调 + v1 release prep** | admin + viewer 双账号走通全流程；env / docker-compose / 部署文档；含虚拟模型路由演示 | 1d | T-015 + T-005b + T-017a | todo |

**v1 上线 ETA**: T-015 / T-005b / T-017a 三条线可并行 → 整体 **~5-6 天**（瓶颈是 T-005b 3d）；T-INT 收官 +1d → **~1 周内**可达发布候选。

> **2026-05-19 用户决策（Ark coding plan 洞察）**: 5 个模型共用一把 key（doubao-seed-code / deepseek-v3.2 / glm-5.1 / kimi-k2.6 / minimax-m2.7），T-016 多 key 加密管 v1 不需要，留到第二家 provider 时再启。智能路由的"虚拟模型解析"部分（T-017a）已抽出加进 v1。

### vNext（v1 后再做）

- **T-016 upstream_credentials + 加密 + admin CRUD**（**仅在接第二家 provider 如 OpenAI/Anthropic 时启动**；Ark coding plan 单 key 5 模型不需要）
- **T-017b fallback retry on 5xx/429**（2d，含 SSE 中途切换状态机）
- **T-018 故障注入 e2e**（与 T-017b 配套，1-2d）
- **T-100 L2 端到端正确性套件**（1 admin + 10 user 真方舟 e2e）
- Phase 3-A Agent 适配 Epic（见 `规划.md` §十四）

### 旧任务状态同步

- **T-005c** [废]: `ConstantTimeCompare` 已由 T-010 实现；rate limit per-key 推到 vNext。
- **T-005a / T-005b**: 已合入 2-B，原条目作废；T-005b 现含前端登录。
- **T-100 / T-016 / T-017 / T-018**: 推 vNext，不阻塞 v1 上线。
- **T-004** 小修小补：维持 todo，机会主义穿插。

### 暂停区（不进 Phase 2，留给 Phase 3+）

- chat UI / playground / prompt 模板：定位是底座，不做应用层
- Anthropic / Gemini 协议转换：已在 docs/proposals 中规划，等多 provider 真正有需求再启
- 多模态 token 计算 / cache_creation：方舟 + OpenAI 系列优先，其他厂商按需
- Agent 适配层（Claude Code / Codex / OpenCode）：见 `规划.md` §十四，Phase 3 Epic

---

<!-- T-013 / T-014 / T-005a 已 approved 并迁出至速查表。Phase 2-A 完成 2/2 ✅；Phase 2-B 进度 1/5，下一棒 T-015 (review) / T-005b / T-017a（三条并行）→ T-INT 联调。 -->

---

## T-015 用户月度 budget + quota 编辑（全栈） [phase:2-B] [owner:codex] [status:review]

**Started**: 2026-05-19 11:05 +08:00 by Codex.

**目标**: 给 v1 加上"管理员能给员工划线"的能力——usage 入账前 budget 校验、超额硬拦截 + 402、admin Users 页可看可改 quota。这是底座三角"权限/额度"一角的兑现。

**接受标准**:
- [ ] 用户表新增月度 budget 字段（单位由 PROPOSAL §1 定：USD 美分整型 vs token 数）；migration 编号 `000008`。
- [ ] gateway 路径在 usage 入账逻辑前查当前用户当月已用量 + budget；超额返回 402 envelope（与 401 同 shape，code 由 PROPOSAL §2 定）。
- [ ] admin API: `GET /api/admin/users` 返回字段增加 `budget` + `used`；新增 `PATCH /api/admin/users/:id/quota` 写动作，走 `auditMiddleware` + 落 `update_quota` action。
- [ ] 前端 Users tab：每行展示 used / budget 进度条；admin 角色看到编辑按钮（小弹窗 / inline input），保存后 fetch 刷新；viewer 不显示编辑。
- [ ] 测试 ≥ 6 case：budget 充足放行 / budget 不足 402 / budget 字段 nil 视为无限制 / quota update SQL 断言 / 前端 admin/viewer 渲染差异 / 超额 audit 落库（action=`update_quota`）。
- [ ] gateway / admin 覆盖率不下降；新增逻辑覆盖率 ≥ 80%。

**Codex propose 前置**: **是**。PROPOSAL 4 点：

1. **budget 字段单位**: USD 美分（`bigint`，与 cost_ledger 对齐）vs token 数（`bigint`）。给推荐 + 与现有 cost_ledger / usage_token_breakdown 的对齐成本。
2. **检查时机 / 一致性**: usage middleware 落库前查 vs gateway 入口预查 vs Redis Lua 原子扣减（规划 §三.2 建议 Redis，但 v1 是否引入 Redis？）。给推荐——v1 倾向无 Redis（用 DB 当月 SUM，接受小并发竞态）。
3. **402 envelope 形状**: 与 401 一致 `{error: {message, type, code}}`，`type=quota_exceeded`，code 命名（`monthly_budget_exhausted`?）。
4. **前端编辑形态**: inline cell edit vs 弹窗。考虑 viewer 角色不显示按钮的实现：前端读 `/api/admin/me` 角色 vs 后端按角色返回不同字段集。给推荐。

**不在范围**:
- per-key RPM/TPM 限流（T-019 / vNext）
- 周/日级 budget、组织级 budget、跨月结转（vNext）
- budget 超额后的"宽限期 / 软告警"（vNext）
- 多币种（Phase 3）

**依赖**: T-005a approved ✅（用 RBAC vocabulary `update_quota`，前端按角色显隐）。可与 T-005b 并行（前端编辑按钮的角色判断暂时通过 bootstrap token 假定 admin；T-005b 完成后会自然替换）。

**参考**: `internal/usage` (commit `4761671`) usage 入账 + cost_ledger 模式；`web/src/views/users.js` 是 Users tab 改动起点。

**Result**: `0041c11` (feat: enforce monthly user budget)。清理了本地暂存区和缓存，测试了 quota update 和 budget 限流机制。`grep "monthly_budget_usd"` 结果为 no matches，确认为孤儿遗留字段。前端暂留手动指定角色的口子，在 T-005b 中完整接入。

---

## T-005b admin auth 升级 + 登录页 + RBAC 落地（全栈） [phase:2-B] [owner:codex] [status:todo]

**目标**: 把 admin 从"全局 bootstrap token"换成"用户登录态"。后端: 用户密码 + session/JWT + RBAC engine 落 admin 写路由 + audit `actor_id` 切到真实 user UUID。前端: 登录页 / 退出 / 401 拦截重定向 / Authorization header 自动注入。

**接受标准**:
- [ ] users 表新增 `password_hash`（bcrypt cost ≥ 10）；migration 编号 `000009`；seed 至少 1 个 admin + 1 个 viewer 账号供联调。
- [ ] `POST /api/admin/login` 接收 email + password；成功返回 session token（PROPOSAL §1 决方案：cookie session vs JWT bearer）。
- [ ] `POST /api/admin/logout` 失效当前 session。
- [ ] `adminAuthMiddleware` 改为解析 session/JWT → 解析出 `Actor{OrgID, UserID}` → 注入 ctx；`BootstrapActorResolver` 替换为真实 resolver。
- [ ] admin 所有写路由（当前 `POST /api/admin/dev/virtual-keys` + T-015 加的 `PATCH /api/admin/users/:id/quota`）调 `rbac.Engine.Authorize`；拒绝时返回 403 envelope + 落 audit（`status_code=403, after={reason}`）。
- [ ] 审计 actor 来源切换: `actor_id = user UUID 字符串`，`actor_type = "user"`；audit middleware 不动，只替换 resolver。
- [ ] 前端: 新增 `/login` 路由；token 存 localStorage（如 JWT 方案）或浏览器自带 cookie（如 session 方案）；所有 fetch 自动带 token；401/403 跳登录页或显示错误。
- [ ] 旧 bootstrap token 路径保留为可选 dev fallback（env 开关 `OMNITOKEN_ALLOW_BOOTSTRAP_TOKEN=1`），默认关；T-INT 联调时只用真实账号。
- [ ] 测试 ≥ 6 case：登录成功 / 密码错 / disabled 用户拒登 / RBAC deny admin 写 → 403 + audit / 真实 actor_id 落 audit / 前端 401 自动跳登录。

**Codex propose 前置**: **是**。PROPOSAL 5 点：

1. **session vs JWT**: 单 admin 实例 + 简单部署 → 倾向 server-side session（HTTP-only cookie，SameSite=Lax，store in DB or in-memory）；JWT 加复杂度但 stateless。给推荐 + 部署假设。
2. **密码存储**: bcrypt（`golang.org/x/crypto/bcrypt`）确认；cost 选 10 还是 12（v1 用户量小，可选 12）。
3. **session 存哪**: in-memory map (重启失效，需重新登录) vs 新 `admin_sessions` 表。给推荐（v1 倾向 in-memory + 短 TTL）。
4. **登录端点是否自审计**: 登录成功 / 失败是否落 audit_logs？审计 actor 在登录前还无身份——可用 `actor_type=anonymous` + IP 标识。给推荐。
5. **前端 token 管理形态**: localStorage（JWT）vs cookie（session）。是否需要 refresh 机制 / 长 TTL（v1 倾向 24h session，过期重新登录）。

**不在范围**:
- 找回密码 / 邮箱验证 / 2FA / 第三方 IdP（vNext）
- 用户自助注册（v1 全部 seed + admin 创建）
- 跨域 admin（v1 同源部署）
- session revoke 主动失效列表（v1 靠 TTL 自然过期）

**依赖**: T-005a approved ✅（RBAC engine 接进路由）+ T-013 approved ✅（audit middleware 已就位）。可与 T-015 并行；前端登录页交付后 T-015 的"按角色显隐编辑按钮"自然生效。

**参考**: 现有 `adminAuthMiddleware` 在 `cmd/admin/main.go`（commit `760020f`）；`internal/audit.ActorResolver` 替换点（commit `d5b379d`）；前端 fetch 封装在 `web/src/api.js`。

---

## T-017a 虚拟模型解析（单 key 5 模型） [phase:2-B] [owner:codex] [status:todo]

**目标**: 让 gateway 接受虚拟模型名（如 `chat-fast` / `chat-balanced`），后台解析为真实 Ark 模型名后转发上游，并把 `model_requested` / `model_actual` 双字段都落 `usage_events`。**不做** retry/fallback（留 T-017b vNext）。此任务兑现底座三角"性价比"一角的最简演示。

**接受标准**:
- [ ] 新 migration `000009_virtual_models.up/down.sql`：`virtual_models(name PK text, real_model text NOT NULL, status text DEFAULT 'active', description text, created_at, updated_at)`；CHECK 非空非空串。
- [ ] seed 至少 5 条映射到 `deploy/postgres/002_seed.sql`：`chat-fast → kimi-k2.6` / `chat-balanced → glm-5.1` / `chat-quality → deepseek-v3.2` / `chat-code → doubao-seed-code` / `chat-experimental → minimax-m2.7`（具体真实 model 名以方舟 API 文档为准；PROPOSAL 锁定时一并核实）。
- [ ] 新增 `internal/router` 包，提供 `Resolver.Resolve(ctx, requested) (Resolution, error)`；`Resolution` 含 `RealModel string`、`IsVirtual bool`。请求模型不在 `virtual_models` 表中 → 透传不报错（视为真实模型直发，向后兼容）；表中标记 `status != 'active'` → 400 `virtual_model_disabled`。
- [ ] gateway middleware 在 `enforceMonthlyBudget` 之后、reverse proxy 之前插入解析步骤；解析成功后改写请求 body 的 `model` 字段并把虚拟原名注入 ctx 供 usage middleware 读。
- [ ] `usage_events.model_requested` 落虚拟名（用户传入的）；`model_actual` 落真实 Ark 模型名。两字段都已存在（T-008 commit `4761671`），本任务只确保填对。
- [ ] admin `GET /api/admin/virtual-models` 走 `adminAuthMiddleware`，返回 `{virtual_models: [{name, real_model, status, description}, ...]}`；空数组降级与 overview 一致。
- [ ] 前端在左导航第 5 项加 "Virtual Models" 视图（或在 Models tab 顶部插一块），只读表格展示映射。loading/empty/error 三态齐备。
- [ ] 测试 ≥ 6 case：unit (resolver 命中/未命中/disabled/db error) + admin handler (success/db nil) + gateway 集成 (body model 重写正确、ctx 注入虚拟原名) + 真实模型透传不破坏现有 e2e。
- [ ] `internal/router` 覆盖率 ≥ 80%；`cmd/gateway` 不下降。

**Codex propose 前置**: **是**。PROPOSAL 答清 3 点:

1. **Schema 弹性**: v1 用单列 `real_model text` vs 用 `real_models text[]` 数组为 T-017b retry 提前留位。给推荐 + 数据迁移成本估计。建议：先单列；T-017b 时加一列 `fallback_models text[]` 不破坏。
2. **解析失败语义**: 未在表中的模型名透传上游 vs 400 `unknown_virtual_model`。推荐**透传**（向后兼容、`/v1/chat/completions` 现有调用方继续工作）；`disabled` 状态才报错。
3. **请求 body 重写位置**: 在 gateway 自己重写 vs 在 `internal/proxy` 的 reverse-proxy `Director` 重写。推荐前者——保留 `internal/proxy` 的协议无感性，重写在 router 层完成。

**不在范围**:
- retry / fallback / 主模型失败切备用 → **T-017b vNext**（含 SSE 中途切换状态机）
- 加权路由 / cost-aware / latency-based 策略 → vNext
- admin 可改虚拟模型映射的 CRUD → vNext（v1 用 seed 写死，配合 migration 增减）
- 跨 provider 路由（OpenAI / Anthropic）→ T-016 触发后再启
- 客户端通过 `/v1/models` 看到虚拟模型清单 → 本任务不动 `/v1/models`，仅在 admin 端暴露

**依赖**: T-008 ✅（`usage_events.model_actual` 字段已就位）+ T-007 ✅（chat proxy）。**与 T-015 / T-005b 完全独立**，三条线可并行。

**参考**:
- `internal/proxy`（commit `34a5b6a`）— gateway middleware 链插入位置
- `internal/usage` middleware（commit `4761671`）— 怎么从 ctx 拿虚拟原名落 `model_requested`
- `规划.md` §零A "性价比" 一角 + `docs/proposals/2026-05-13-smart-key-pool-routing.md`（智能路由长线方向，T-017a 是其最简前哨）

---

## T-INT 前后端联调 + v1 release prep [phase:2-B 收官] [owner:codex] [status:todo]

**目标**: 把 T-013 / T-014 / T-005a / T-015 / T-005b / **T-017a** 所有改动拼起来，做一次真账号 + 真 DB + 真方舟 的端到端走查；同时把部署相关的零碎落地（env、docker-compose、README）。重点演示：admin 登录 → 改员工 budget → 员工用 `chat-fast` 虚拟模型发请求 → 后台路由到 `kimi-k2.6` → audit 全程留痕。

**接受标准**:
- [ ] 联调脚本：用 admin 账号登录 → Overview 看到真实数据 → Users 看 budget / 改 quota → Models 看 → Audit 筛过滤 → 退出。再用 viewer 账号登录，确认编辑按钮不可见、PATCH 直接 403、audit 看得见但不能改。脚本可用 Playwright 也可 `node --test fetch mock`，但**至少一次手动走查截图存档**。
- [ ] 真方舟链路：用 admin 账号建一个 virtual key → 用该 key 发非流式 + 流式 chat → admin Users 看到 used 增加 → 把该 user budget 改成已用量之下 → 再发 chat 收到 402 → 看 audit 完整记录 `update_quota` + `create_virtual_key` 两条。
- [ ] 部署落地：`deploy/docker-compose.yml` 起 pg + admin + gateway 一次成功；`.env.example` 补齐 v1 新增 env（admin secret / session TTL / budget 默认值 / anomaly 阈值）；`README.md` 加"v1 部署 30 分钟走完"章节。
- [ ] 修联调中发现的 bug；如果 bug 超过一个 commit 能改的范围，开 sub-task 而非塞进 T-INT。
- [ ] 联调报告：单独 markdown `docs/release/v1-integration-2026-05-XX.md`，含验证清单 + 截图 + 已知问题（如 race test 在本机缺 gcc 跑不了 — 已是历史问题）。
- [ ] CI 不阻塞：`go vet`、`go test -count=1 ./...`、`make lint`（如果环境支持）全绿；race test 在能跑的环境跑一次。

**Codex propose 前置**: **否**——联调按上述清单走，发现新问题再 propose。

**不在范围**:
- 性能压测（vNext T-100）
- 多实例 / Kubernetes 部署（v1 单实例 docker-compose 够用）
- 监控告警接 webhook（v1 留 expvar + WARN log）

**依赖**: T-015 + T-005b approved。

**参考**: `cmd/loadtest`（commit `d7e78e8`）有 e2e 调用 pattern；`scripts/e2e_verify.py`（T-006d 用过）可作为真方舟链路脚本起点。

- 智能 Key 池与配额感知模型路由（2026-05-13）→ [`docs/proposals/2026-05-13-smart-key-pool-routing.md`](docs/proposals/2026-05-13-smart-key-pool-routing.md)
- 智能路由 + 性能指标 + Elastic 远景（2026-05-14）→ [`docs/proposals/2026-05-14-smart-routing-elastic-cache.md`](docs/proposals/2026-05-14-smart-routing-elastic-cache.md)


