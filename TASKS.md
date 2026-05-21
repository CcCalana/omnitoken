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
| 2-C 性价比 | **T-016 upstream_credentials + 多 key 池**（ADR 0003 拉回）| schema + 2-3 把 Ark key seed 加密入库 + gateway 轮询/429 重试 + usage 写 credential_id；CRUD UI 推 v1.1 | 5-7d | T-INT ✅ | todo |
| 2-C 验证 | **T-CONC-RERUN 多 key 池真实 baseline**（原 vNext 拉回）| mock upstream 测 gateway 承载 + 多 key 池上线后真 Ark 复跑 50/100 并发 | 1d | T-016 | todo |

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

## T-016 upstream_credentials 多 key 池（v1） [phase:2-C] [owner:codex] [status:in-progress]

Started: 2026-05-21 00:15 +08:00
Proposal: `docs/proposals/2026-05-21-t016-upstream-credential-pool.md`

**目标**: 落实 ADR 0003 决策，把 §零A 第 1 条"性价比资源 = 多 upstream key 池"在 v1 阶段真正落地。具体做：(a) `upstream_credentials` 表 schema + AES-256-GCM envelope encryption；(b) 2-3 把 Ark coding plan key 通过 seed SQL 加密入库；(c) gateway 按 priority/weight 选 credential，429/5xx 自动切池中下一把；(d) usage 流水记录 `upstream_credential_id` 作为 key 维度归因。**admin CRUD UI / KMS / 自动 rotation / 多 provider 推 v1.1**，不在本任务范围。

**涉及**:
- `internal/migrate/migrations/00XX_create_upstream_credentials.sql` + 00YY_add_credential_id_to_usage.sql（新增）
- `internal/crypto/envelope.go`（新增）— AES-256-GCM envelope encryption，主密钥从 env `OMNITOKEN_MASTER_KEY` 注入（hex 编码）
- `internal/credentials/` 包（新增）— credential 加载（启动时从 DB 解密入内存）、selector（按 priority/weight 轮询）、health state（429/5xx 临时降权）
- `cmd/gateway/main.go` — proxy 层接 credential selector；HTTP client 实际用的 `Authorization: Bearer <decrypted_secret>` 替换原硬编码/单 env
- `internal/proxy/`（看具体文件）— 429/5xx 重试切下一 credential 的状态机；SSE 反代要兼容（chunk 已发出后不能切）
- `internal/usage/recorder.go` + `internal/usage/store_postgres.go` + 同期 T-CONC-COST-ATTR 的 schema 合并 — 加 `upstream_credential_id` 列
- `cmd/admin/main.go` — overview 加 "按 credential 维度" 查询能力（v1 后端 API 就行，UI 推 v1.1）
- `deploy/docker-compose.yml` — 加 `OMNITOKEN_MASTER_KEY` env 注入位（用一个固定 demo key，部署文档写明生产换）
- `docs/operations/master-key-rotation.md`（新增）— 主密钥注入和 rotation 流程文档

**接受标准**:
- [ ] **Schema**: `upstream_credentials` 表含字段（按规划.md §185 行）：`id`、`provider`、`base_url`、`encrypted_secret`（bytea）、`region`、`priority`（int）、`weight`（int, 默认 1）、`status`（active/disabled）、`health_state`（healthy/degraded/quarantined）、`last_error`、`quota_hint`、`metadata`（jsonb）、`created_at`、`updated_at`。reverse migration 完整。
- [ ] **加密**: AES-256-GCM envelope，单元测试覆盖 (a) 加密后密文不含明文子串 (b) 错误主密钥解密失败 (c) tag 篡改解密失败 (d) nonce 不复用（每次加密生成新 nonce）。
- [ ] **Seed**: docker-compose 启动时通过 migration 或独立 seed 命令插入 2-3 把 Ark key（明文从 env `OMNITOKEN_ARK_KEYS_*` 读，加密落 DB）；启动日志打印"loaded N upstream credentials"但**不打印任何 key 明文/密文/前后缀**。
- [ ] **Gateway 选 credential**: priority 升序 + weight 加权轮询；status=disabled 跳过；health_state=quarantined 跳过；usage 流水里 `upstream_credential_id` 写选中的。
- [ ] **429/5xx 切换**: 单请求遇 429 或 5xx，**最多重试 2 次**切下一 credential（避免无限重试）；SSE 流式如果 chunk 已发出则不切（标 final）；切到底没 healthy 的返回 503 envelope。**429 自动把当前 credential 标为 degraded 持续 30s**，避免短时间反复打同一 key。
- [ ] **顺手 NIT (T-NIT-SSE-CLOSE)**: 改 `internal/proxy/proxy.go` retry 逻辑时，把 `readWithIdle` 的 `case <-ctx.Done():` 分支补上 `_ = body.Close()`，与 `timer.C` 分支对称（避免依赖外层 defer + buffered channel 间接清理 goroutine）。**单测加一条**：客户端断开后，goroutine 数量在 100ms 内回归基线（用 `runtime.NumGoroutine()` 差值断言或类似手法）。**不阻塞 T-016 主线 review，作为顺带项**。
- [ ] **Usage 归因**: `usage_records.upstream_credential_id` 列（int FK）+ 写入。**与 T-CONC-COST-ATTR 的 model_routed 列同期添加，合并到一个 migration 文件减少冲突**。
- [ ] **Admin 后端 API**: `GET /admin/credentials` 返回 credential 列表（不含密文，仅 id/provider/priority/weight/status/health_state/last_error 等元数据）；用于 v1.1 UI 接入。**UI 不做**，但 API + 单测要做。
- [ ] **关键日志/审计**: credential 添加/更新/禁用走 audit_logs；429 切换 credential 走 WARN log；明文密钥**严禁**出现在任何日志/error message/HTTP response。code review 时 grep `encrypted_secret` 的所有路径。
- [ ] **测试**: `internal/crypto/envelope_test.go`（envelope 加密 4 条）；`internal/credentials/selector_test.go`（priority/weight 排序、disabled/quarantined 跳过、空池 503）；`internal/credentials/load_test.go`（DB 解密 + nonce 唯一）；`internal/proxy/retry_test.go`（429 切换 + SSE 兼容 + 最多 2 次重试）；admin handler test（API 不漏密文）。
- [ ] **覆盖率**: `internal/crypto` + `internal/credentials` ≥85%；`internal/proxy` 不降。
- [ ] **e2e (docker-compose)**: 1 admin user + 3 把 Ark key seed + gateway 接 admin handler；mock upstream 返回 429 让 gateway 切换；usage 流水按 credential 维度聚合可看到 3 把 key 各自的请求数 + 切换次数 > 0。
- [ ] **`go vet ./...` clean；`go test ./...` 全绿；docker race target (`make test-race`) 全绿**。

**Codex propose 前置**: **是**。PROPOSAL 答清 5 点：
1. **主密钥从 env 还是 file？** v1 默认 env (`OMNITOKEN_MASTER_KEY` hex)。但 docker-compose 把 env 写进 yaml 显式存在被 cat 风险。propose 给"docker secret 挂载文件 + 进程 read once"的 v1 方案对比，定取舍。
2. **credential 加载策略**: 启动一次全部解密入内存 vs 每次请求从 DB 解密。**默认推荐启动一次**（避免请求路径解密成本 + 主密钥使用次数最少）；但需要 cache invalidation 机制（admin 改 credential 后 gateway 怎么感知）。propose 给方案：v1 重启感知 vs SIGHUP reload vs 定时 poll vs PG NOTIFY。
3. **429 切换的 backoff 策略**: 当前任务体写"429 标 degraded 30s"。propose 拍 30s 是否合理（Ark 单 key rate limit 实测 ~7 RPS，30s 内不应再打）；也可考虑指数退避（30s/60s/120s）。
4. **SSE 流式中途 429 怎么办**: chunk 已发给客户端，不能切 credential。两路：(a) 流式失败直接 final（用户体验：截断）(b) 流式上游 1 次失败重试要在第一 chunk 之前完成。**默认推荐 (b) 配合 stream 缓冲首 chunk**。propose 拍。
5. **upstream_credentials.metadata jsonb 字段干嘛用**: v1 留着不用？还是塞 `ark_user_id`（key 归属用户，便于运维联系采购）？默认推荐 v1 字段加但不强制写，给 schema 预留扩展位。

**不在范围**:
- admin CRUD **UI**（v1 后端 API 够；UI 推 **v1.1 T-016b**）
- KMS 集成（v1 用 env 主密钥；KMS 推 vNext）
- 自动 rotation（推 vNext T-016c）
- 多 provider（Ark 之外的 OpenAI/Anthropic）（推 vNext T-016d，与 protocol 转换并轨）
- 按虚拟模型分配 credential 池（v1 全局一个池；推 v1.1）
- T-017 priority_fallback policy_id 体系（推 vNext，v1 简化为 priority/weight 静态）
- T-018 故障注入 e2e（推 vNext）

**依赖**: T-INT ✅（v1 联调基线）；T-CONC-COST-ATTR（建议**同期并行**，合并 migration）；ADR 0003 ✅。

**参考**: `docs/adr/0003-multi-key-pool-priority.md` ✅；`规划.md` §零A 第 1 条 + §185 upstream_credentials 字段定义 + §四十六风险表行 468；`REVIEW.md` R-CONC-CHECK M-23 + H-4；memory `project_omnitoken_ark_coding_plan`（已修正）；`docs/release/v1-concurrency-baseline-2026-05-21.md`（验证 baseline 起点）。

---

## T-CONC-COST-ATTR 成本归因路径修复（model_routed） [phase:2-B 后置] [owner:codex] [status:todo]

**目标**: 修复 OmniToken 底座"性价比资源"角的成本归因路径。R-CONC-CHECK M-23 实测：admin overview 当前按 `model_actual`（Ark 上游响应里自报的 backend 模型名）聚合，**与用户/管理员心智里"我请求了哪个模型"完全脱节** —— 例：preflight 跑 `chat-fast → kimi-k2.6`，但 `model_actual = deepseek-v4-pro`（Ark coding plan 多模型共用 backend 推理的预期行为）。本任务把 admin 成本归因切到 **gateway 实际转发的模型**（虚拟模型解析后的 real_model；非虚拟时即用户原 request 的 model），让 admin overview 和审计层数据可信。

**涉及**:
- `internal/migrate/migrations/00XX_add_model_routed_to_usage.sql` (新增) — `usage_records` 加 `model_routed` 列
- `internal/usage/recorder.go` / `internal/usage/store_postgres.go` — `Record` struct 加 `ModelRouted` 字段，写入新列
- `internal/usage/parser.go` — **保持不变**（`ModelActual` 继续取 Ark `response.Model`）
- `cmd/gateway/main.go` — `payload["model"] = res.RealModel` 那段（line 234-244）把 `res.RealModel`（或非虚拟时的 `modelRequested`）传进后续 usage 流水，可走 `httpx.WithVirtualModel` ctx 扩展或新增 ctx key
- `cmd/admin/main.go:667/791/803` — 三处 `COALESCE(NULLIF(ue.model_actual, ''), ...)` 改为 `COALESCE(NULLIF(ue.model_routed, ''), NULLIF(ue.model_requested, ''), 'unknown')`；`model_actual` 列**保留**给审计但 overview 不再用
- `docs/adr/000X-cost-attribution-model-routed.md` (新增) — Context/Decision/Consequences 三段式
- 单测 + admin SQL 断言更新

**接受标准**:
- [ ] **schema**: `usage_records.model_routed text NOT NULL DEFAULT ''`（与 `model_actual` 同 nullability 策略；migration 加 reverse SQL）。
- [ ] **写入**: gateway 把 `res.RealModel`（virtual 路径）或 `modelRequested`（非虚拟路径）写到 `Record.ModelRouted`；recorder/store 透传到 DB。
- [ ] **`internal/usage/parser.go` 行为零变化**：`ModelActual` 继续取 `response.Model`；该字段语义重新定义为"上游自报的 backend 模型名"（在 ADR 里写明）。
- [ ] **admin 聚合切换**: overview / users / models 三处 SQL（`cmd/admin/main.go:667/791/803`）改为 COALESCE `model_routed → model_requested → 'unknown'`；`model_actual` 列保留供审计接口（如果有审计 tab 需要展示 Ark backend 真名，留一个查询点）。
- [ ] **历史数据兜底**: v1 之前 demo 数据 `model_routed=''`，靠 COALESCE 回退到 `model_requested` 即可显示；**不需要回填 SQL**。
- [ ] **ADR `000X-cost-attribution-model-routed.md`**: 必须包含 (a) Ark coding plan 多模型共用 backend 推理的实测证据（R-CONC-CHECK preflight）；(b) 为什么 `model_routed` 是成本归因 ground truth、`model_actual` 退为审计字段；(c) 多 provider 启动后这一策略是否仍成立（前瞻一句话即可）。
- [ ] **测试**:
   - `internal/usage/recorder_test.go` 加 ModelRouted 写入用例（虚拟模型 + 非虚拟模型两路）
   - `internal/usage/store_postgres_test.go` 加 model_routed 列写入断言
   - `cmd/admin/main_test.go` 三处 SQL 断言更新为新 COALESCE
   - 新增 `internal/usage/parser_test.go` 反向断言：parser **不**碰 ModelRouted（防回归）
- [ ] `go vet ./...` clean；`go test ./...`（含 `golang:1.25` Docker race 跑）全绿；`internal/usage` + `cmd/admin` 覆盖率不降。

**Codex propose 前置**: **是**。PROPOSAL 答清 3 点：
1. **`ModelRouted` 数据源**: 复用 `httpx.WithVirtualModel(ctx, modelRequested)` 的 ctx key（已在 `cmd/gateway/main.go:242` 设置，但目前只存 `modelRequested`），还是新增一个 `httpx.WithModelRouted(ctx, res.RealModel)`？前者改语义会破现有用 WithVirtualModel 的地方，后者更干净。**默认推荐后者**，但要 grep 现存 `httpx.WithVirtualModel` 所有 caller 确认无歧义。
2. **schema NOT NULL 还是 nullable**: `model_actual` 当前 schema 是什么（nullable text？），新列保持一致最安全。如果 `model_actual` 是 nullable 且现网数据可能 NULL，`model_routed` 也保 nullable + COALESCE 兜底；如果 `model_actual NOT NULL DEFAULT ''`，新列照搬。Codex propose 时贴 schema 现状定。
3. **`model_actual` 在 admin 是否完全下沉**: overview 切到 model_routed 后，`model_actual` 是否还有展示位（如 audit tab "Ark 上游实际模型"列）？默认推荐**保留供 audit**，overview/users/models 三处 SQL **不展示** model_actual；如果 audit tab 当前没用，propose 给出"未来扩展位"的 stub 设计即可。

**不在范围**:
- DSN `application_name` 修复 → vNext T-CONC-DSN
- v1 真实并发 baseline 复跑 → vNext T-CONC-RERUN
- 跨 provider 的 backend 模型披露差异（OpenAI/Anthropic 是否也有同样行为）→ 多 provider 启动时再看
- Ark 这种 backend 多路复用是否合规/合同问题 → 不是工程范围

**依赖**: T-INT ✅（usage pipeline + admin overview 已就位）；R-CONC-CHECK ✅（M-23 提出）。

**参考**: `docs/release/v1-concurrency-baseline-2026-05-21.md` §Preflight（M-23 实测证据）；`REVIEW.md` R-CONC-CHECK M-23 段；`cmd/gateway/main.go:234-244`（virtual_model 重写）+ `internal/usage/parser.go:38,41`（ModelActual 来源）；memory `project_omnitoken_ark_coding_plan`（5 模型共用 key 的设计背景）；`规划.md` 第八节（schema 变更走 migration）+ §零A（底座三角"性价比资源"）。

---


<!-- T-041 / T-CONC-CHECK / T-MK-RACE / T-042 / T-043 / T-040 任务体已迁出。
     done 状态见 Phase 3-A 表 + 速查表；R-* 详 docs/reviews/archive-2026-05-20.md；
     PROPOSAL 在 docs/proposals/；实现 diff 走 git log。 -->

**Review**: R-040 Approved (REVIEW.md)。无 CRITICAL/HIGH；N-11 (paths() helper 过滤分支永不命中) + N-12 (init() 指针 vs value-receiver 风格混用) 不阻塞，下次顺手裁。T-044 / T-046 解锁。
