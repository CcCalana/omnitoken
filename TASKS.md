# TASKS.md — OmniToken Task Board

## CHANGELOG (压缩版 — 详见 git log + docs/reviews/)

| 时间 | 事件 |
|------|------|
| 05-11 ~ 05-12 | Phase 0/1 全部 ✅: T-001~T-008 + T-006a~T-006d (Demo-Ready E2E 通过)。详见 `docs/reviews/archive.md` + `archive-2026-05-12.md` |
| 05-18 | T-010 / T-012 实施 + R-010/R-012 approve. **Pre-push gate 4/4 ✅** |
| 05-18 | **设计理念锁定**: 性价比 / 权限额度 / 安全审计 三角；定位"先搭底座再叠应用"。Phase 2 重组为 2-A/2-B/2-C |
| 05-19 | **Phase 2-A 完成**: T-013 (audit 95.9%) + T-014 (anomaly 82.1%) approve |
| 05-19 | **路线重排**: 上线优先 + 一次联调。Phase 2-C 推 vNext，v1 ETA ~1w |
| 05-19 | **AGENTS.md §3.3 新增**: 测试环境强制 docker-compose；禁本地装 PG/Redis/NATS |
| 05-19 | **Ark coding plan 洞察**: 单 key 5 模型（doubao/deepseek/glm/kimi/minimax）。T-016 多 key 加密推迟；T-017a 虚拟模型解析抽出加进 v1 |
| 05-19 | **Phase 2-B 完成 5/5 ✅**: T-005a (rbac 89.9%) + T-015 (`0041c11`) + T-017a (`0021e37`) + T-005b (`62504b9` + 修复 `670dc60`) + **T-INT (`4db5057`) v1 release candidate 就绪** |
| 05-19 | **REVIEW 归档**: R-008~R-005b-fix 全部沉到 `docs/reviews/archive-2026-05-19.md`；REVIEW.md 仅留 R-INT |
| 05-19 | **AGENTS.md §3.3a/§3.3/§7 收紧**: `-race` 统一 Docker/CI 跑；Windows 缺 gcc 是预期，禁汇报 |
| 05-19 22:07 | **URGENT triaged**: T-042 smoke 误读真 `~/.codex/auth.json` → 印到 Codex transcript。低 sev。结构修复 → AGENTS.md §9.5 落 smoke 方法学（必须 `--home <temp>` + 禁 cat auth 文件） |
| 05-19 ~ 05-20 | **Phase 3-A Adapter 链路 4/4 ✅**: T-041 `a6d1d09` / T-042 `ceb123c` / T-043 `5254c48` / T-040 `147502da`。R-* 详 docs/reviews/archive-2026-05-20.md。Codex 三次主动纠 spec（golang:1.25 / provider singular / Result-vs-CLI 互斥）|
| 05-20 | **R-CONC-CHECK approve w/ follow-ups**。`04fff8a7` 2500 真 Ark: 428/2500 2xx, 2072 上游 429, gateway 自身 0 panic/0 5xx/0 timeout。抓出 M-23/H-3/H-4 三 follow-up |
| 05-20 | **T-CONC-COST-ATTR 任务体写好**（status:todo, propose 前置=是）。`model_routed` 列 + admin SQL 切换 + ADR；3 propose 决策点 |
| 05-20 | **路线反思 + ADR 0003**: 用户讯问"50×50 单 key 不符合中转站行业玩法"，回溯 §零A 第 1 条，**T-016 多 key 池 v1 必做**。从 vNext 拉回 Phase 2-C；admin CRUD UI / KMS / rotation 推 v1.1；T-CONC-RERUN 也拉回与 T-016 同期。v1 ETA "~1 周" → "~2 周"。memory `project_omnitoken_ark_coding_plan` 需修正 |
| 05-20 | **T-016 任务体写好**（status:todo, propose 前置=是）。schema + envelope encryption + 2-3 Ark key seed + gateway 轮询 + 429 切池；5 propose 决策点：主密钥来源 / credential 加载策略 / 429 backoff / SSE 流式中途切换 / metadata 字段用途 |
| 05-21 | **R-EXT-2026-05-21 外部专家分析核验**：6 条诊断 → 3 命中已跟踪 / 1 NIT 挂到 T-016 顺带做（SSE ctx.Done 分支补 body.Close + 单测）/ 1 推测（quota Redis 缓存）降为 vNext 观察项 **T-QUOTA-CACHE-PROBE** / 1 读错代码（goroutine 泄漏 OOM）。v1 ETA 不动 |
| 05-21 | **R-016-prop approve** (`fd9ce8d8`)：5/5 propose 决策直接采纳；H-5 (partial-first-read) + M-24/M-25 (ops 文档) + N-15 (WARN log 不漏 Ark body) 留实施期核 |
| 05-21 | **R-016 approve** (impl `c6ee841d` + e2e `8544ce82`)：12/12 接受标准全达成；H-5/M-24/M-25/N-15 + T-NIT-SSE-CLOSE 五条债全部落地且有显式断言；proxy 86.7% / crypto 87.8% / credentials 92.0%；T-CONC-COST-ATTR 合并到 000012 migration + admin 三处 SQL 切到 model_routed 一并完成。**v1 §零A 第 1 条"性价比资源 = 多 upstream key 池"落地**。3 NIT (provider='ark' 缺注释 / master key fallback log 缺 reason / usage_events 不追溯 retry 链路) 不阻塞 |
| 05-21 | **T-CONC-RERUN 任务体写好**（status:todo, propose 前置=是）。mock baseline + 真 Ark 多 key 池验证 + DB/quota 观察；5 propose 决策点：mock 形式 / 并发档位 / T-CONC-DSN 是否前置 / pg_stat_statements 是否启 / 报告位置。严格 measurement-only |

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
| T-015 | 用户月度 budget + quota 编辑 (全栈) | `0041c11` | — |
| T-017a | 虚拟模型解析（单 key 5 模型） | `0021e37` | — |
| T-005b | admin auth 升级 + 登录页 + RBAC 落地 (含修复) | `62504b9` + `670dc60` | — |
| T-INT | 前后端联调 + v1 release prep | `4db5057` | 17/17 真方舟 + 14/14 control-plane |
| T-MK-RACE | Makefile race 移入 Docker (golang:1.25 + named volumes) | `a44f27a` | — |
| T-041 | Claude Code 适配（配置写入） | `a6d1d09` | agent_adapter 81.9% |
| T-042 | Codex 适配 (config.toml + auth.json 无损 patch) | `ceb123c` | agent_adapter 82.6% |
| T-043 | OpenCode 适配 (XDG 三档解析) | `5254c48` | agent_adapter 82.2% |
| T-040 | Registry + AgentConfig interface 抽象 | `147502da` | agent_adapter 83.6% |
| T-CONC-CHECK | v1 并发摸底报告 + 3 follow-up | `04fff8a7` / `c6c4262f` | — |
| **T-016** | upstream_credentials 多 key 池 v1 (envelope + selector + retry + ops 文档) | `c6ee841d` + e2e `8544ce82` | proxy 86.7% / crypto 87.8% / credentials 92.0% |
| **T-CONC-COST-ATTR** | model_routed 列 + admin 三处 SQL 切换（**并入 T-016 同一 migration 000012**） | `c6ee841d` | usage/admin 不降 |

> 详细 review 见 `docs/reviews/archive-2026-05-20.md`（Phase 2-B 收官 + Phase 3-A Adapter 链路）；T-006d 验收明细见 R-006d 归档；T-009a/T-009b 详情见 R-009a / git。

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

> **2026-05-19 用户决策**: 上线优先 + 一次前后端联调。
> **2026-05-20 决策更新 (ADR 0003)**: T-CONC-CHECK 实测单 key 路径 17.1% 成功率，与 §零A 第 1 条"性价比资源 = 多 upstream key 池"脱节。T-016 多 key 池从 vNext 拉回 Phase 2-C，作为 v1 必做项；admin CRUD UI / KMS / 多 provider 推 v1.1。v1 ETA "~1 周" → "~2 周"。

| 子轨 | 任务 | 一句话 | 估时 | 依赖 | 状态 |
|---|---|---|---|---|---|
| 2-A 审计 | T-013 audit_logs schema + middleware | admin 写操作落 audit | 2d | — | ✅ |
| 2-A 审计 | T-014 audit 查询 + 异常告警 | audit tab + 阈值 WARN | 2d | T-013 | ✅ |
| 2-B 权限 | T-005a RBAC 引擎 | 三角色硬编码 matrix | 3d | T-013 | ✅ |
| 2-B 额度 | T-015 用户月度 budget + quota 编辑（全栈） | usage 入账前 budget 校验 → 402；admin Users 页可改 quota | 3d | T-005a | ✅ |
| 2-B 权限 | T-005b admin auth 升级 + 登录页 + RBAC 落地（全栈） | 替换 bootstrap → session/JWT；RBAC 挂 admin 写；前端登录/退出 | 3d | T-005a + T-013 | ✅ |
| 2-B 性价比 | **T-017a 虚拟模型解析（单 key 5 模型）** | gateway 接 `chat-fast/balanced/quality/...` 改写为真实 Ark model；含 admin 列表 + 前端展示 | 1-1.5d | T-008 ✅ | ✅ |
| 2-B 联调 | **T-INT 前后端联调 + v1 release prep** | admin + viewer 双账号走通全流程；env / docker-compose / 部署文档；含虚拟模型路由演示 | 1d | T-015 + T-005b + T-017a | ✅ |
| 2-C 性价比 | **T-016 upstream_credentials + 多 key 池**（ADR 0003 拉回）| schema + 2-3 把 Ark key seed 加密入库 + gateway 轮询/429 重试 + usage 写 credential_id；CRUD UI 推 v1.1 | 5-7d | T-INT ✅ | ✅ |
| 2-C 验证 | **T-CONC-RERUN 多 key 池真实 baseline**（原 vNext 拉回）| mock upstream 测 gateway 承载 + 多 key 池上线后真 Ark 复跑 50/100 并发 | 1d | T-016 ✅ | propose |

**v1 上线 ETA（2026-05-20 调整）**: ADR 0003 拉回 T-016 后，v1 ETA 从原 "~1 周" 调整为 **"~2 周"**。T-016 是关键路径（5-7d），T-CONC-RERUN 验收 1d。Phase 3-A Agent 适配相应延后 ~1 周。

> **2026-05-19 用户决策（Ark coding plan 洞察）**: 5 个模型共用一把 key（doubao-seed-code / deepseek-v3.2 / glm-5.1 / kimi-k2.6 / minimax-m2.7），T-016 多 key 加密管 v1 不需要，留到第二家 provider 时再启。智能路由的"虚拟模型解析"部分（T-017a）已抽出加进 v1。

### vNext（v1 后再做）

- ~~**T-016 upstream_credentials + 加密 + admin CRUD**~~ → **2026-05-20 拉回 Phase 2-C v1，见 ADR 0003**。admin CRUD UI / KMS / 自动 rotation 推 v1.1
- **T-017b fallback retry on 5xx/429**（2d，含 SSE 中途切换状态机）
- **T-018 故障注入 e2e**（与 T-017b 配套，1-2d）
- **T-100 L2 端到端正确性套件**（1 admin + 10 user 真方舟 e2e）
- **T-CONC-DSN**（R-CONC-CHECK H-3）：gateway/admin DSN 显式拼 `application_name=omnitoken-gateway`/`omnitoken-admin`，让 `pg_stat_activity` 采样可用；同步更新 `cmd/loadtest/README.md` 采样 SQL。可观测性短板，不阻塞 Phase 3-A。
- ~~**T-CONC-RERUN**~~ → **2026-05-20 拉回 Phase 2-C v1，与 T-016 验收同期**（ADR 0003）
- **T-QUOTA-CACHE-PROBE**（2026-05-21 外部专家提，启动门槛 = T-CONC-RERUN 完成）：跑 mock upstream 高并发后，量 `monthlyBudgetStatusSQL`（`internal/quota/store_postgres.go:48` 双 LEFT JOIN + SUM）在真实 gateway 承载下的 PG CPU / 慢查询 / 连接池占用。如果发现是瓶颈，候选解：(a) `usage_events(organization_id, user_id, created_at)` + `cost_ledger(usage_event_id)` 加索引（低风险）；(b) Redis 月度额度缓存 + 异步入账后增量更新（架构性变更，需 ADR）。**先量再写实现**，本条只是观察任务。
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

<!-- Phase 2-B 5/5 ✅。T-013~T-INT 任务体全部 approved 后迁出；详见速查表 + git log + R-INT。 -->

---

## 🎯 Phase 3-A Agent 适配 Epic (2026-05-19 user-locked)

> **路径决策**: 先写具体（Claude Code）后抽抽象（T-040 后置），符合 AGENTS.md §3.1 "三处重复再抽象"。tingly-box 参考文档实现节奏一致。**跳过 Phase 2-C vNext** 直接开 Phase 3-A：Agent 接入是产品演示价值更高的下一步，下游单 provider Ark 仍可工作，不强依赖 fallback。

| # | 任务 | 一句话 | 估时 | 依赖 | 状态 |
|---|---|---|---|---|---|
| 1 | T-041 Claude Code 适配（配置写入） | `omnitoken adopt claude-code` 改 `~/.claude/settings.json` 指向 OmniToken；带备份 | 2d | — | ✅ `a6d1d09` |
| 2 | T-042 Codex 适配 | `~/.codex/config.toml` + `auth.json` 无损 toml_edit | 2d | T-041 | ✅ `ceb123c` |
| 3 | T-043 OpenCode 适配 | `~/.config/opencode/opencode.json` 加 XDG 路径解析 | 1d | T-042 | ✅ `5254c48` |
| 4 | T-040 抽象层提取（后置） | 三处重复后抽 `Registry` + `AgentConfig` interface | 1d | T-043 | ✅ `147502da` |
| 5 | T-045 Anthropic → OpenAI 协议转换 | gateway 多挂 `/v1/messages`；让 Claude Code 真正能跑 | 4d | T-041 | todo (Phase 3-A 后置, T-016 完成后) |
| 6 | T-044 路由规则联动 | apply 配置 = 同步生成 OmniToken 内部 `virtual_models` + admin 端可视化 | 2d | T-040 | todo |
| 7 | T-046 一键 onboard CLI 收口 | `omnitoken adopt <agent>` 统一入口 + 退出 / restore | 1d | T-044 | todo |

**Phase 3-A ETA**: **2–2.5 周**（顺序）；T-045 是真正 demoable 时刻。

**实施前必读**: `docs/references/agent-adapter/agent-adapter-pattern.md` §3.3（Claude Code 完整源码模板）+ `agent-adapter-projects-reference.md` §4.1（token_proxy 无损 JSON 编辑模式）。

### 提案（Phase 2 候选，不阻塞当前 gate）

- 智能 Key 池与配额感知模型路由（2026-05-13）→ [`docs/proposals/2026-05-13-smart-key-pool-routing.md`](docs/proposals/2026-05-13-smart-key-pool-routing.md)
- 智能路由 + 性能指标 + Elastic 远景（2026-05-14）→ [`docs/proposals/2026-05-14-smart-routing-elastic-cache.md`](docs/proposals/2026-05-14-smart-routing-elastic-cache.md)

---

<!-- T-041 / T-CONC-CHECK / T-MK-RACE / T-042 / T-043 / T-040 任务体已迁出。
     done 状态见 Phase 3-A 表 + 速查表；R-* 详 docs/reviews/archive-2026-05-20.md；
     PROPOSAL 在 docs/proposals/；实现 diff 走 git log。

     T-016 + T-CONC-COST-ATTR 任务体已迁出（2026-05-21 done）。done 状态见
     Phase 2 表 + 速查表；R-016-prop / R-016 详 REVIEW.md；PROPOSAL 在
     docs/proposals/2026-05-21-t016-upstream-credential-pool.md；
     ops 文档 docs/operations/master-key-rotation.md；
     migration 000012；实现 diff 走 git log (c6ee841d + 8544ce82)。 -->

**Review**: R-040 Approved (REVIEW.md)。无 CRITICAL/HIGH；N-11 (paths() helper 过滤分支永不命中) + N-12 (init() 指针 vs value-receiver 风格混用) 不阻塞，下次顺手裁。T-044 / T-046 解锁。

---

## T-CONC-DSN pg_stat_activity application_name [phase:2-C] [owner:codex] [status:done]

Started: 2026-05-21 19:29 +08:00

Review: R-CONC-RERUN-prop Decision 3 approved.
Scope: gateway/admin Postgres DSN `application_name=omnitoken-gateway` / `omnitoken-admin` plus `cmd/loadtest/README.md` sampling SQL. No config or internal DB interface changes.
Acceptance: `go vet ./...` and `go test ./...` clean; independent commit before T-CONC-RERUN.
Result: `9b44f98b` — gateway/admin application_name + sampling SQL landed; all green, no deviation.

---

## T-CONC-RERUN 多 key 池真实并发 baseline（v1 收官） [phase:2-C] [owner:codex] [status:done]

Started: 2026-05-21 18:30 +08:00
Proposal: `docs/proposals/2026-05-21-t-conc-rerun-baseline.md`

**目标**: 拿到 v1 上线前 OmniToken gateway 的**真实并发承载 baseline**——T-CONC-CHECK 上次跑 17.1% 成功率被 Ark 单 key rate limit 吃了（H-4），无法回答用户"v1 能扛多少 RPS"。本任务通过两路验证拿到可信数据：(a) mock upstream 跑高并发，量 gateway 自身处理能力（不被上游 rate limit 干扰）；(b) T-016 多 key 池真 Ark 复跑，验证切换机制是否真把 Ark 总速率上限提高到 N 倍。报告产物作为 v1 release 一部分。**严格"不修 internal/* 与 cmd/gateway 代码"**，与 T-CONC-CHECK 同性质 measurement-only。

**涉及**（read-only + 报告写入）:
- `cmd/loadtest/`（可能要加 profile 配置 / 429 计数）— **propose 时拍是否允许修**；如果改 loadtest 工具则改动隔离在 `cmd/loadtest/` 内，不动 internal/* 与 cmd/gateway / cmd/admin
- `deploy/docker-compose.yml`（可能要起 mock upstream service）— propose 时拍 mock 形式
- `docs/release/v1-concurrency-rerun-2026-05-22.md`（新增，或追加到 `v1-concurrency-baseline-2026-05-21.md`，propose 时拍）
- `docs/proposals/2026-05-21-t-conc-rerun-*.md`（propose 落地）

**接受标准**:
- [ ] **mock upstream baseline**: 给出 gateway 在 mock upstream（< 1ms 响应延迟、永不 429）下的承载曲线——至少覆盖 3 个并发档位（建议 50 / 100 / 200，propose 拍最终值），每档至少 60s 稳定段。报告含：RPS（实际 vs 配置）、P50/P95/P99 延迟、gateway 5xx 数、超时数、客户端错误数、**memory/goroutine 走势**（至少进程级 `runtime.NumGoroutine` 终值 + `/proc/self/status` 或 docker stats 一次抓取）。**目标**：gateway 5xx ≤ 0.1% 且 P99 ≤ 100ms（mock 路径下基线门槛，未达则在报告里点明 follow-up）。
- [ ] **真 Ark 多 key 池验证**: 用 T-016 已 seed 的 3 把 Ark key（docker-compose `credential-seed` 跑过），跑一档真请求（建议 30 并发 × 30s，约 900 请求，成本 < 5 元）。报告含：成功率、上游 429 次数、**429 切换发生次数**（grep gateway log "upstream credential retry"）、`usage_events` 按 `upstream_credential_id` 聚合的三把 key 各自请求数。**目标**：成功率 > 80%（vs T-CONC-CHECK 17.1%），证明 T-016 切池机制把单 key rate limit 转成池级 rate limit。
- [ ] **DB 观察**（连接池 + 慢查询）: 跑期间至少采样 3 次 `pg_stat_activity` 和 `pg_stat_statements`（如 enabled）的 omnitoken 相关连接。**前置门槛**：DSN 是否有 `application_name`，见 propose 决策点 3。如果 propose 决定**不**做 T-CONC-DSN 前置，报告必须像 T-CONC-CHECK 一样透明披露"采样失效，连接池数据缺失"；如果做了前置，报告含 gateway / admin 各自连接峰值 + 是否触发 max_conn 上限。
- [ ] **额度路径观察（T-QUOTA-CACHE-PROBE 输入）**: 跑期间记录 `pg_stat_statements` 里 `monthlyBudgetStatusSQL`（如可识别）或 quota path 慢查询的 mean/max 延迟，作为 T-QUOTA-CACHE-PROBE 的 baseline 数据。如果 `pg_stat_statements` 没开，propose 决定是开（改 docker-compose postgres conf）还是用 EXPLAIN 离线粗估。**至少给一条结论**："quota check 路径在 N RPS 下平均延迟 X ms / 是否成瓶颈"。
- [ ] **报告产物**: 一份 release 文档（propose 拍位置），含：methodology / mock baseline 数据 / 真 Ark 数据 / DB 观察 / V2 candidate fixes（与 T-CONC-CHECK 同结构）/ 与 T-CONC-CHECK 17.1% 的对照说明。
- [ ] **严格 measurement-only**: 不改 internal/* 与 cmd/gateway / cmd/admin 代码。如果发现新瓶颈（如 quota DB 真成瓶颈、selector lock 争用、proxy goroutine 异常增长），**写进报告 V2 candidates，不当场修**，开 follow-up 任务交给后续。
- [ ] `go vet ./...` clean；`go test ./...`（含 docker race）全绿（如果 propose 决定改 loadtest 工具，这条强制）；如果完全不改代码，仅断言"runtime 行为不变"。

**Codex propose 前置**: **是**。PROPOSAL 答清 5 点：
1. **mock upstream 形式**: (a) 在 docker-compose 起一个独立 mock service（如 `mock-ark` nginx + lua / 一个 30 行 Go binary 返 stub completion）；(b) `cmd/loadtest` 内 in-process httptest server 然后从 loadtest 直打 mock + gateway 旁路（**不经 docker network**）；(c) 起一个 wiremock 容器。**默认推荐 (a)**——最接近真实部署网络拓扑，但要新建一个 ~50 行 Go binary。propose 拍。
2. **并发档位 + 时长**: 默认建议 50 / 100 / 200 各 60s 稳定段（前 10s warmup 不计入）。propose 是否覆盖；要不要加 spike test（500 并发 30s 看是否优雅 degrade）；真 Ark 一档是否 30×30 还是 10×120。
3. **T-CONC-DSN 是否前置**: T-CONC-DSN 是修代码（DSN 加 `application_name=omnitoken-gateway`/`omnitoken-admin`），独立 vNext 任务。propose 拍：(a) 前置一个 < 1h 的 T-CONC-DSN 实施 commit，本任务才开跑，让 DB 观察数据可信；(b) 不前置，本任务报告里透明披露 sampling 失效（与 T-CONC-CHECK 同处理），T-CONC-DSN 留 vNext。**两个方案各自 trade-off**，propose 给推荐 + 理由。
4. **`pg_stat_statements` 是否启用**: 默认 docker-compose postgres 没开。propose 拍 (a) 改 docker-compose postgres command 加 `-c shared_preload_libraries=pg_stat_statements`（要重启 PG，是改 deploy 不是改 internal/*，属本任务允许范围）；(b) 不开，用 EXPLAIN ANALYZE 离线粗估 monthlyBudgetStatusSQL 延迟。**默认推荐 (a)**——给 T-QUOTA-CACHE-PROBE 留好 baseline。
5. **报告位置**: (a) 新建 `docs/release/v1-concurrency-rerun-2026-05-22.md`，与 T-CONC-CHECK 报告并列；(b) 追加到 `v1-concurrency-baseline-2026-05-21.md` 作为 "Rerun" 章节。**默认推荐 (a)**——两次跑性质不同（单 key vs 多 key 池），分文件更清晰；T-CONC-CHECK 那份留作 history snapshot。

**不在范围**:
- T-CONC-DSN 实施（如果 propose 决定前置，独立一个 < 1h commit，**不并入本任务 commit**）
- T-QUOTA-CACHE-PROBE 实施（依赖此任务输出的 baseline 数据，本任务只产出输入数据 + 结论建议）
- 任何 `internal/*` 与 `cmd/gateway / cmd/admin` 代码修复（与 T-CONC-CHECK 同性质 measurement-only）
- v1.1 admin CRUD UI 配套压测（v1.1 范围）
- 多 provider（OpenAI/Anthropic）并发对照（多 provider 启动后再做）
- DDoS / 异常输入压测（vNext T-018 故障注入 e2e 范围）

**依赖**: T-016 ✅（多 key 池就绪，3 把 Ark seed key 可用）；R-CONC-CHECK ✅（17.1% 对照基线）；可选前置：T-CONC-DSN（见 propose 决策点 3）。

**参考**: `docs/release/v1-concurrency-baseline-2026-05-21.md`（T-CONC-CHECK 报告 + V2 candidates）；`REVIEW.md` R-CONC-CHECK H-4 段；`docs/adr/0003-multi-key-pool-priority.md`（多 key 池设计）；`cmd/loadtest/README.md`（loadtest 工具用法）；`规划.md` §零A 第 1 条（性价比资源验收门槛）。

Result: `abc98a05` — mock rerun captured PG saturation; true Ark 30x30 blocked by missing 3-key seed/master-key env, no undeclared deviation.
Result(rerun): `dff69844` — true Ark 43.0%, switch 216, no undeclared deviation.


**Superseded 2026-05-23**: 候选 B 由用户直接确认（Ark coding plan 一账号一 key，三把里两把不是 coding plan）。Probe 不跑。H-6 二审改走 T-MP-DEEPSEEK：multi-provider 池跨 Ark + DeepSeek 重测 30×30，>80% 验收门槛在新形态下达成才能签。原 Ark 单 provider 路径**永久无法达成**（物理约束）。详 ADR 0004。本任务保持 status:review，等 T-MP-DEEPSEEK rerun 数据落进 `docs/release/v1-concurrency-rerun-2026-05-22.md` 新增段后一并签。

---

## T-MP-DEEPSEEK Multi-provider 池接入 DeepSeek (v1 收官) [phase:2-C] [owner:codex] [status:done]

Started: 2026-05-23 00:00 Asia/Shanghai

⚠️ **Docker-only (AGENTS.md §3.3a)**：所有 `go vet` / `go test -race` / migration / loadtest 一律在 docker-compose 容器内跑。**禁 Windows host 跑 make / golangci-lint / `go test -race`**。本机用 `docker compose run --rm <service>` 模式触发。违反一次=R-* 直判 HIGH 退回。

**目标**: ADR 0004 落地——把 multi-provider 池能力接进 v1，把 DeepSeek 官方 API 作为 v1 第二个 upstream provider。让 §零A 第 1 条"性价比资源"验收能在跨 provider 形态下过 >80% 门槛。`.env` 已落 `OMNITOKEN_DEEPSEEK_KEYS_1/2/3` + `OMNITOKEN_DEEPSEEK_BASE_URL=https://api.deepseek.com/v1`。

**涉及**:
- migration 000013（新增 `upstream_credentials.base_url text NOT NULL DEFAULT ''` + `virtual_models.provider text NOT NULL DEFAULT 'ark'`；Ark 旧行回填 base URL 与 provider）
- `cmd/upstream-credential-seed`（识别 `OMNITOKEN_DEEPSEEK_KEYS_*` + `OMNITOKEN_DEEPSEEK_BASE_URL`；跨 provider 单一全局 priority 序列；audit snapshot 加 provider/base_url）
- `internal/proxy/`（按 credential.base_url 构建上游 URL；OpenAI 兼容协议复用 Ark non-stream 路径；selector 跨 provider 轮询）
- `deploy/postgres/002_seed.sql`（virtual_models 加 provider 列；`chat-fast` 改 provider=deepseek + real_model=deepseek-v4-flash 作为 v1 测试主路径，其它 chat-* 保 Ark fallback）
- `docs/release/v1-concurrency-rerun-2026-05-22.md`（新增 "Multi-provider Rerun" 段，不覆盖既有段）
- e2e 测试加一份 cross-provider credential pool 用例

**接受标准** (propose 跳过——本任务范围由 ADR 0004 锁定):
- [ ] migration 000013 up/down 双向跑过；Ark 已有数据回填后 `SELECT COUNT(*) FROM upstream_credentials WHERE provider='ark' AND base_url=''` = 0
- [ ] `credential-seed` 跑后 `SELECT provider, COUNT(*) FROM upstream_credentials WHERE active GROUP BY provider` 给出 `ark` ≥ 1 + `deepseek` = 3
- [ ] 跨 provider e2e: 一个测试用例验证 selector 在 Ark credential 全部 429/degraded 时 fallback 到 DeepSeek credential，反之亦然；usage_events 写入正确 provider+credential_id
- [ ] `chat-fast` 通过 gateway 打 DeepSeek，返回 OpenAI 兼容 chat completion；usage_events 行的 `model_routed` = `deepseek-v4-flash`，`upstream_credential_id` 指向 DeepSeek 行
- [ ] **多 provider rerun (并发档位可降)**: 起步 `cmd/loadtest -concurrency 30 -duration 30s -allow-failures` + `MAX_REQUESTS=900` + `model chat-fast`（打 DeepSeek）。**成功率 > 80% 是硬门槛、并发量无下限**——若 30 conc <80% 则降到 10 conc × 30s 再跑；仍 <80% 降到 5 conc × 30s；找到达成 >80% 的最大稳定并发即可，**不为冲 30 conc 牺牲成功率**。报告 "Multi-provider Rerun" 段每个尝试档位记 timestamp / conc / RPS / 2xx / 4xx / 5xx / upstream_429 / P50/P95/P99 / 三把 DeepSeek key 各自 `count(*)` / 切换次数；最后一行宣告 "v1 上线档位 = N conc / X% 成功率" + 与 17.1% / 43.0% 三层对照。如果 5 conc 仍 <80%，写未声明偏差 stop 报给 Claude（不当场调代码）
- [ ] DeepSeek 跑测成本 ≤ ¥1（按 ¥1/M input + ¥2/M output 算，900 请求 × max_tokens=8 ≈ ¥0.03，加上一次 e2e ≈ ¥0.1，硬上限 ¥1 截止）
- [ ] **Docker-only 自报**：每个 commit message 附一行 "verification: go vet via `docker compose run --rm test`" 或同等表述；review 时核
- [ ] `go vet ./...` clean；e2e 全绿；migration up/down 验证

**不在范围**:
- 完整 Provider interface 抽象 / factory（推 v1.1）
- Anthropic 协议适配（T-020，v1.1+）
- OpenAI 官方 API key（T-OAI-*，v1.1+）
- admin UI per-provider 视图（v1.1）
- DeepSeek cache hit 优化路径（按 cache miss 算成本是 v1 安全侧默认）
- KMS / rotation 自动化

**Codex 实施建议**（不强制，speed lever 5：你拍最优切分）:
- 单 commit 把 migration + seed + virtual_models seed 落地 → impl review
- 单 commit 把 proxy.base_url 路径 + e2e + cross-provider rerun 落地 → impl review
- 或一次性大 commit，由你判断 review 时一次看完还是分两次

**禁动**:
- ADR 0003（保留为历史决策；ADR 0004 通过 status 注引用）
- 既有 Ark 路径行为（Ark credential 仍按 priority 序列工作）
- `docs/release/v1-concurrency-baseline-2026-05-21.md`、`v1-concurrency-rerun-2026-05-22.md` 既有 mock/True Ark 段
- T-016 / T-CONC-COST-ATTR 既有代码（增量在 schema + seed + proxy.base_url）

**异常处理**:
- DeepSeek API 返非 OpenAI 兼容形态 → 写未声明偏差停跑，不当场抽 Provider interface
- 三把 DeepSeek key 任意一把无效 → 跑测前 health probe（curl 单 ping）跑 3 把都通过才进 loadtest；任何一把不通 stop + 报告给 Claude，不当场删 key
- 跑测成功率仍 < 80% → 在报告里诚实写数据 + V2 candidate，**不当场调 selector 算法**，开 follow-up 给 Claude 决策

**依赖**: T-016 ✅；T-CONC-DSN ✅；T-CONC-RERUN 既有数据作历史对照；`.env` 三把 DeepSeek key ✅

Result: `026f90ca` — multi-provider 30 conc 100.0%, DeepSeek 767/767, no undeclared deviation.

---

## T-016b-MIN Admin Credential CRUD UI (Min) + 30s Polling Hot Reload (v1 上线必备) [phase:2-C] [owner:codex] [status:in-progress]

Started: 2026-05-23 00:00 Asia/Shanghai

⚠️ **Docker-only (AGENTS.md §3.3a)**：所有 `go vet` / `go test -race` / migration / e2e / 前端构建一律 `docker compose run --rm`。**禁 Windows host 跑 make / golangci-lint / `go test -race` / npm**。违反一次 = R-* 直判 HIGH 退回。

**目标**: 让 v1 上线后运维通过 admin web 加 / 禁用 upstream credential，无需 SSH 改 `.env`。配套 30s PG 轮询热加载，新池自动生效（无需手动 restart gateway）。ADR 0005 落地。

**涉及**:
- `cmd/admin`（POST/PATCH 端点 + master key env 加载 + validation + audit_logs 写入）
- `cmd/gateway`（启动后台 polling goroutine；env `OMNITOKEN_CREDENTIAL_POLL_INTERVAL` 默认 `30s`，`0` 关闭）
- `internal/credentials`（selector atomic Replace 方法 + 单测；不变 existing API）
- `web/src/`（Upstream Credentials tab + add modal + disable button + 30s banner）
- `deploy/docker-compose.yml`（admin service 加 `OMNITOKEN_MASTER_KEY` / `OMNITOKEN_MASTER_KEY_FILE` env，与 gateway 共享）
- `docs/operations/master-key-rotation.md`（一段说明：v1 admin + gateway 共享 master key，trust boundary 显式；KMS 仍 v1.1+）

**接受标准** (propose 跳过 — 范围由 ADR 0005 锁定):
- [ ] `POST /admin/credentials` 端点：input `{provider, alias, priority, base_url, key}`，输出 `PublicCredential`（密文不出）。validation：provider ∈ {`ark`, `deepseek`}、`base_url` 必须 `http(s)://`、alias 同 provider 内唯一、priority ≥ 1、key 非空。失败 4xx + 标准 error envelope。
- [ ] `PATCH /admin/credentials/:id/disable` 端点：把 row `active=false`（不删，保审计）；幂等（已 disabled 返 200 不报错）。
- [ ] 两端点都走 RBAC admin role（T-005a 现成 middleware）+ 写一行 `audit_logs`（T-013 现成中间件）。viewer / user role 一律 403。
- [ ] **gateway polling**: 启动 30s tick（可 env 配 interval），每 tick `SELECT FROM upstream_credentials WHERE updated_at > $last_seen`（或全量 reload，性能差异在 v1 规模可忽略）+ decrypt + 原子 swap selector 内部 slice。新增 credential 在 ≤ interval+1s 内可被路由命中。日志 INFO 一行 `credential pool reloaded count=N delta=±M`。
- [ ] selector 增加 `Replace([]Credential)` 或 `Refresh(ctx)` 方法（看 Codex 哪个干净）；并发安全（既有 mutex 之上原子 swap）；不破坏 `Next/NextForProvider/MarkDegraded` 既有语义；单测覆盖 swap 时正在跑的请求不 panic / 不读到中间态。
- [ ] **前端**: admin web 加 "Upstream Credentials" tab（参考既有 Audit Logs tab 结构）。列表显示 provider / alias / priority / base_url / status / health_state。"新增" 按钮弹 modal 表单。"禁用" 按钮带 confirm。Mutation 成功后顶部 red banner："已写入 DB，gateway 在 30s 内自动加载新池；如需即时生效请手动 `docker compose restart gateway`。"banner 30s 后自动消失。
- [ ] **docker-compose**: `admin` service env 加 `OMNITOKEN_MASTER_KEY` 与 `OMNITOKEN_MASTER_KEY_FILE`（passthrough，与 gateway 同源；docker-compose.yml 写注释强调"admin 加密路径需与 gateway 同一把 master key"）。
- [ ] e2e 测试：在 `cmd/gateway/credential_pool_e2e_test.go` 或同等位置加一条用例——admin POST 一条新 ark credential → 跑 polling tick → gateway 通过新 credential serve 请求 → usage_events 写入新 credential_id。
- [ ] `docs/operations/master-key-rotation.md` 加一段"admin 进程也要注入 master key；rotation 时两处同步"，3-5 行即可。
- [ ] **Docker-only 自报**：每个 commit message 附 verification 行列 `docker compose run --rm` 命令链。

**不在范围**:
- UPDATE / 编辑现有 credential（disable + 新 add 等价覆盖）
- DELETE（disable 是审计友好的取代）
- master key 前端管理（环境变量管，trust boundary 文档化）
- KMS / 自动 rotation / 健康检查 worker / 多 admin role 细分
- LISTEN/NOTIFY 热加载（用 polling 规避复杂度，v1.1 再上 LISTEN）
- 多 provider whitelist 扩展（v1 只 ark + deepseek；新 provider 加 enum 是 v1.1 工作）
- cost-aware routing UI / 跨 provider 优先级编辑

**禁动**:
- 既有 `GET /admin/credentials` handler 行为（增量在新增 POST/PATCH）
- T-016 / T-MP-DEEPSEEK proxy / selector 核心路径（增量是 Replace + polling，不是改 Next 语义）
- ADR 0003 / 0004（保留为历史，ADR 0005 引用）
- master key 加载逻辑（复用 envelope.LoadMasterKey）

**异常处理**:
- 加 polling 后 gateway 启动期 reload 顺序不变（先 initial load 再启 ticker）
- 如果 polling tick 时 DB 不可达：log WARN，**不 crash gateway**，下一 tick 再试
- admin POST 时 master key 未注入：返 500 + 明确 error_code `master_key_missing`，不静默成功
- 测试时 polling interval 不可硬编 30s，必须经 env / config（让单测能短路 1ms tick）

**Codex 实施建议**（speed lever 5，切分你拍）:
- 单 commit 把 selector Replace + gateway polling + admin POST/PATCH 后端 + docker-compose env 落地 → 后端 review
- 单 commit 把 frontend tab + modal + banner + e2e 测试落地 → frontend review
- 或一次性大 commit；review 时分块看

**依赖**: T-016 ✅；T-MP-DEEPSEEK ✅；ADR 0005

**参考**: `docs/adr/0005-admin-credential-crud-min.md`；`cmd/admin/main.go:512-529` 既有 list handler；`internal/credentials/selector.go` 既有 mutex；T-013 audit middleware；T-005a RBAC middleware

