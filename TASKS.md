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

## T-CONC-DSN pg_stat_activity application_name [phase:2-C] [owner:codex] [status:review]

Started: 2026-05-21 19:29 +08:00

Review: R-CONC-RERUN-prop Decision 3 approved.
Scope: gateway/admin Postgres DSN `application_name=omnitoken-gateway` / `omnitoken-admin` plus `cmd/loadtest/README.md` sampling SQL. No config or internal DB interface changes.
Acceptance: `go vet ./...` and `go test ./...` clean; independent commit before T-CONC-RERUN.
Result: `9b44f98b` — gateway/admin application_name + sampling SQL landed; all green, no deviation.

---

## T-CONC-RERUN 多 key 池真实并发 baseline（v1 收官） [phase:2-C] [owner:codex] [status:review]

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

**Hints (R-CONC-RERUN H-6 follow-up, 2026-05-22)**: 用户已把 master key + 三把真 Ark key 落进本机 `.env`（`OMNITOKEN_MASTER_KEY` 64-hex + `OMNITOKEN_ARK_KEYS_1/2/3`），接受标准 #2 的前置条件齐了。请执行真 Ark 30×30 那一档并把结果**追加进同一份 release 文档**（`docs/release/v1-concurrency-rerun-2026-05-22.md` 的 "True Ark Rerun" 段，**不新开 release 文件、不改 mock 段已有数字**），步骤约束：
1. `docker compose -f deploy/docker-compose.yml up -d` 让 `credential-seed` 用新 env 跑——必要时 `docker compose stop gateway credential-seed && docker compose up -d` 触发 credential-seed 重跑；不要 `-v` 抹卷（per R-CONC-RERUN M-26）。
2. 跑前查 `upstream_credentials`：`SELECT provider, COUNT(*), MIN(priority), MAX(priority) FROM upstream_credentials WHERE active GROUP BY provider;` 应 `ark` = 3 行（mock-ark 假行 = 0；否则 M-28 清理证据缺失，先 `DELETE FROM upstream_credentials WHERE provider <> 'ark'` 并把这条命令写进报告 cleanup 段）。
3. 用 mock 段同款命令形状跑：`cmd/loadtest -concurrency 30 -duration 30s -allow-failures` + `MAX_REQUESTS=900` + `model chat-fast` + 复用现有 demo virtual key + admin-token。完整形状参考 `cmd/loadtest/README.md`；**不修 cmd/loadtest 代码**。
4. 跑后查证据：
   - `usage_events` 按 `upstream_credential_id` 聚合，确认三个 credential_id 都出现；若集中在单一 priority 1，记 V2 candidate 而**不**当 defect（ADR 0003 priority-based fallback 是设计，仅 429 触发切换）。
   - gateway log `grep "upstream credential retry"` 看 429 切换次数（观察是否发生即可，不要求下限）。
   - 与 T-CONC-CHECK 17.1% 对照：成功率必须 > 80%（任务体接受标准 #2 硬门槛）。未达则在 True Ark Rerun 段透明披露原因 + V2 candidate，**不当场修**。
5. 写完 release 文档 + **新 commit（不要 amend `abc98a05`）**，TASKS.md 本条 Result 行后**再追加一行** `Result(rerun): <hash> — true Ark <成功率>%, switch <次数>, no undeclared deviation.`，状态保持 `status:review` 等二审。
6. 报告 True Ark Rerun 段必须含：跑期 timestamp / 实际 RPS / 总请求数 / 2xx / upstream_429 / gateway 5xx / P50/P95/P99 / 三把 key 各自 `count(*)` / 切换次数 / 与 T-CONC-CHECK 17.1% 对照一行结论。
7. **禁动**：`internal/*`、`cmd/gateway`、`cmd/admin`、`cmd/loadtest` 任何代码；`docs/release/v1-concurrency-baseline-2026-05-21.md`；mock 段已有数字；ADR 0003；R-CONC-RERUN M-27 / M-28 顺手在报告里修就行，不另开 commit。
8. **Cost 约束**：`MAX_REQUESTS=900` 是硬上限（≈ 5 元预算）；真 Ark 提前 429 / 网络异常则直接中止并把已得数据写进报告，**不重跑超 budget**。
