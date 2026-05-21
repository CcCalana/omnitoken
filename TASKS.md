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
| 05-19 18:02 | T-041 PROPOSAL `3187fb2` — 独立 CLI / 显式 token / 全备份保留 + Windows-safe 时间戳 |
| 05-19 | **R-041-prop approve** (5+/1Q/2N)。Codex 可开实施 |
| 05-19 | **AGENTS.md §3.3a/§3.3/§7 收紧**: `-race` 统一 Docker/CI 跑；Windows 缺 gcc 是预期，禁汇报。配套 T-MK-RACE |
| 05-19 | **R-041 approve** (5+/2M/2N + gcc 规则提醒)。`a6d1d09` agent_adapter 81.9%；M-20/M-21 (env 单行 + 非原子写) 不阻塞 v1，并入 T-042 修 |
| 05-19 | **Codex 下一步队列**: T-MK-RACE (infra, <1h) → T-042 Codex 适配 (含 M-20/M-21 修 + helper 抽出, propose 前置=是) |
| 05-19 | **R-MK-RACE approve** (4+/1N)。`a44f27a` golang:1.25 + named volumes + cmd/migrate slashPath。new race policy 立刻生效（Result 无 gcc 措辞） |
| 05-19 | **R-042-prop approve** (5+/1Q/1N)。`34ea18f` 决策 1/2/3 全采纳推荐方向；Codex 可开 T-042 实施 |
| 05-19 22:07 | **URGENT triaged**: T-042 smoke 误读真 `~/.codex/auth.json` → 印到 Codex transcript。定性: 低 sev（中转站 key / 无外发 / 不轮换）。结构修复 → AGENTS.md §9.5 落 smoke 方法学（必须 `--home <temp>` + 禁 cat auth 文件）。T-042 实施代码本身无问题，可继续 commit |
| 05-20 | **R-042 approve** (5+/1N)。`ceb123c` agent_adapter 82.6%；Q-1 三个 edge case 全覆盖 + N-6 加分；R-041 的 M-20/M-21/N-3 一次性修完。T-043 OpenCode 可启动 |
| 05-20 | **T-043 任务体写好**：OpenCode 适配（第三个 adapter）。XDG 路径解析 + 复用 fileio.go；propose 前置=是（managed 字段集 / Windows XDG / `--home` 旗标）。落地后开 T-040 抽象 |
| 05-20 | **R-043-prop approve** (5+/1Q/1N)。`d3088d3` Codex 主动纠 spec（`provider` singular，非任务体里的复数）+ XDG 三档清晰。Q-1: 写几个 model 实施时拍板，默认推荐单一 |
| 05-20 | **R-043 approve** (5+/1N)。`5254c48` agent_adapter 82.2%；Q-1 取单 model + N-8 文件 comment 落地；XDG 三档全测 + plural `providers` 用户数据保留有专测。**T-040 trigger 达成**，可直接开抽象层提取 |
| 05-20 | **T-040 任务体写好**：Registry + AgentConfig interface 抽象。propose 前置=是（接口签名 3 路线 / Result 收敛 / 注册时机）。CLI 零改动，留 T-046；nonEmptyStrings 顺手收 R-043 N-9 |
| 05-20 | **R-040-prop approve** (5+/1N)。`fd8310a` Codex 第三次纠 Claude spec（Result 删字段 × CLI 零改互斥），给分阶段 compat shim 方案。Codex 可开 T-040 实施 |
| 05-20 | **R-040 approve** (5+/2N)。`147502da` agent_adapter 83.6% (+1.4pp)；CLI 零改实锤；canonical + legacy 双向断言到位；N-10 用 `DefaultRegistry.MustRegister(...)`。**Phase 3-A Adapter 抽象层落地**，T-044 / T-046 解锁 |
| 05-20 | **R-CONC-CHECK approve w/ follow-ups** (5+/3 follow-ups)。`04fff8a7` 报告；2500 真 Ark 428/2500 2xx (2072 上游 429), gateway 自身 0 panic/0 5xx/0 timeout。抓出 **M-23**（model_actual=Ark backend 名 ≠ 路由意图，污染 admin 成本归因）+ H-3（DSN application_name）+ H-4（真实并发 baseline 未拿到）。M-23 建议起 T-CONC-COST-ATTR 在 Phase 3-A 前做 |
| 05-20 | **T-CONC-COST-ATTR 任务体写好**（status:todo, propose 前置=是）。usage_records 加 `model_routed` 列；admin overview/users/models 三处 SQL 切到 model_routed；ADR 记录 Ark backend 复用行为。3 propose 决策点：ctx key 复用 vs 新增 / schema NOT NULL 选型 / model_actual 是否下沉 |
| 05-20 | **路线反思 + ADR 0003**: 用户讯问"50×50 单 key 不符合中转站行业玩法"，回溯 T-CONC-CHECK 数据 + 规划.md §零A 第 1 条，确认 T-016 多 key 池 v1 必做。**T-016 从 vNext 拉回 Phase 2-C**，admin CRUD UI / KMS / 自动 rotation 推 v1.1；T-CONC-RERUN 也拉回与 T-016 同期。v1 ETA "~1 周" → "~2 周"。memory `project_omnitoken_ark_coding_plan` 需修正（混淆"计费简化"vs"运行时 key 池"两件事） |
| 05-20 | **T-016 任务体写好**（status:todo, propose 前置=是）。schema + envelope encryption + 2-3 Ark key seed + gateway 轮询 + 429 切池；admin CRUD UI / KMS / 多 provider / policy_id 体系全部推 v1.1+。5 propose 决策点：主密钥来源 / credential 加载策略 / 429 backoff / SSE 流式中途切换 / metadata 字段用途 |

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

| # | 任务 | 一句话 | 估时 | 依赖 |
|---|---|---|---|---|
| 1 | **T-041 Claude Code 适配（配置写入）** | `omnitoken adopt claude-code` 改 `~/.claude/settings.json` 指向 OmniToken；带备份 | 2d | — |
| 2 | T-042 Codex 适配 | `~/.codex/config.toml` + `auth.json` 无损 toml_edit | 2d | T-041 |
| 3 | T-043 OpenCode 适配 | `~/.config/opencode/opencode.json` 加 XDG 路径解析 | 1d | T-042 |
| 4 | **T-040 抽象层提取（后置）** | 三处重复后抽 `Registry` + `AgentConfig` interface | 1d | T-043 |
| 5 | T-045 Anthropic → OpenAI 协议转换 | gateway 多挂 `/v1/messages`；让 Claude Code 真正能跑 | 4d | T-041 |
| 6 | T-044 路由规则联动 | apply 配置 = 同步生成 OmniToken 内部 `virtual_models` + admin 端可视化 | 2d | T-040 |
| 7 | T-046 一键 onboard CLI 收口 | `omnitoken adopt <agent>` 统一入口 + 退出 / restore | 1d | T-044 |

**Phase 3-A ETA**: **2–2.5 周**（顺序）；T-045 是真正 demoable 时刻。

**实施前必读**: `docs/references/agent-adapter/agent-adapter-pattern.md` §3.3（Claude Code 完整源码模板）+ `agent-adapter-projects-reference.md` §4.1（token_proxy 无损 JSON 编辑模式）。

### 提案（Phase 2 候选，不阻塞当前 gate）

- 智能 Key 池与配额感知模型路由（2026-05-13）→ [`docs/proposals/2026-05-13-smart-key-pool-routing.md`](docs/proposals/2026-05-13-smart-key-pool-routing.md)
- 智能路由 + 性能指标 + Elastic 远景（2026-05-14）→ [`docs/proposals/2026-05-14-smart-routing-elastic-cache.md`](docs/proposals/2026-05-14-smart-routing-elastic-cache.md)

---

## T-041 Claude Code 适配（配置写入） [phase:3-A] [owner:codex] [status:done]

Started: 2026-05-19 18:02 +08:00
Proposal: `docs/proposals/2026-05-19-t041-claude-code-adapter.md`

**目标**: 让企业员工跑 `omnitoken adopt claude-code` 一次，Claude Code 之后所有调用都走 OmniToken。本任务只做**配置文件写入 + CLI 入口 + 备份/恢复**，**不做协议转换**（留 T-045）；run 后用户的 Claude Code 此时还跑不通端到端（因 OmniToken 尚不接 Anthropic 格式），但配置层完整，给 T-045 留好接口。

**接受标准**:
- [ ] 新增包 `internal/agent_adapter`（暂不抽 Registry，三个 Agent 都写完再抽——见 T-040）。导出 `WriteClaudeCodeSettings(opts) (Result, error)` + `RestoreClaudeCodeSettings() (Result, error)`。
- [ ] `~/.claude/settings.json` 用**无损 JSON merge**（读取 → patch `env` 字段 → 写回），保留用户既有非 `env` 字段；写之前备份到 `~/.omnitoken/backups/claude-code/settings.json.<RFC3339>.bak`。
- [ ] env 字段集对齐 tingly-box `agent-adapter-pattern.md` §3.3：`ANTHROPIC_BASE_URL` / `ANTHROPIC_AUTH_TOKEN` / `ANTHROPIC_MODEL` / `ANTHROPIC_DEFAULT_*_MODEL` / `CLAUDE_CODE_SUBAGENT_MODEL`；默认 model 填 `chat-balanced`（T-017a 已 seed → `glm-5.1`）。
- [ ] 新增独立 CLI `cmd/omnitoken-adopt`。子命令 `adopt claude-code --gateway-url <URL> --token <virtual_key>` / `restore claude-code`。CLI 用 `flag` 标准库，**不引第三方 CLI 框架**（cobra / urfave 都 propose 拒）。
- [ ] 跨平台路径: `$HOME` / Windows `%USERPROFILE%` / 显式 `--home` 覆盖。
- [ ] 测试 ≥ 6 case：首次写 / 合并保留用户字段 / 重复幂等 / 备份命名时间戳唯一 / restore 最新备份 / `--home` 覆盖生效。
- [ ] `internal/agent_adapter` 覆盖率 ≥ 80%。

**Codex propose 前置**: **是**。PROPOSAL 答清 3 点：
1. **CLI 二进制布局**: 独立 `cmd/omnitoken-adopt` vs 挂在 `cmd/admin-cli`（若有）。推荐独立——员工机器只装这一个工具。
2. **`virtual_key` 来源**: CLI `--token` 显式传 vs 自动从 admin URL 拉。v1 推荐显式，admin 创建 key 后给员工。
3. **备份保留策略**: 全部保留 vs 只留最新 N 个。v1 推荐全部保留（磁盘成本低，可回溯）。

**不在范围**:
- 协议转换 Anthropic ↔ OpenAI → **T-045**
- Codex / OpenCode 写入 → T-042 / T-043
- 实际端到端调通 → T-045 闭环
- 抽象层 `Registry` / `AgentConfig` → **T-040 后置**

**依赖**: 无。T-005b 已可创 virtual_key，CLI 接 `--token` 即可。

**参考**: `docs/references/agent-adapter/agent-adapter-pattern.md` §3.3（Apply 完整源码模板）+ `agent-adapter-projects-reference.md` §4.1（token_proxy 无损 JSON 模式）。

**Result**: `a6d1d09` — agent_adapter 81.9%; Q-1 keys: ANTHROPIC_BASE_URL, ANTHROPIC_AUTH_TOKEN, ANTHROPIC_MODEL, ANTHROPIC_DEFAULT_HAIKU_MODEL, ANTHROPIC_DEFAULT_OPUS_MODEL, ANTHROPIC_DEFAULT_SONNET_MODEL, CLAUDE_CODE_SUBAGENT_MODEL, API_TIMEOUT_MS, DISABLE_TELEMETRY, DISABLE_ERROR_REPORTING, CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC, CLAUDE_CODE_MAX_OUTPUT_TOKENS.
N-1 compact UTC `20060102T150405.000000000Z`; N-2 invalid existing config exits 2 with one-line stderr; all green except race blocked by missing gcc.

---

## T-CONC-CHECK v1 并发摸底（不修代码）[phase:2-B 后置] [owner:codex] [status:done]

Started: 2026-05-20 22:38 +08:00

**目标**: Phase 3-A 启动前用半天测 v1 真实并发上限，**只测量不修**。发现红线再开 T-019 / DB 调优。

**接受标准**:
- [ ] 用现有 `cmd/loadtest` 跑 **50 并发 × 50 请求 = 2500 次**（成本约 12 元 RMB，先 confirm；用 `chat-fast → kimi-k2.6` 走便宜模型）。记录 P50/P95/P99/错误率/usage 总 tokens。
- [ ] 用 vegeta 跑 **1000 RPS 非流式 60s**，**打 `/healthz` 或 mock-upstream 而非真方舟**——验 gateway 自身瓶颈，不烧 token。记 P95 / error rate / DB 连接曲线。
- [ ] 跑测试同时 admin 端 `SELECT count(*) FROM pg_stat_activity WHERE application_name LIKE 'omnitoken%'` 每秒采样，记录峰值连接数。
- [ ] 输出 `docs/release/v1-concurrency-baseline-2026-05-XX.md`，含三组数据 + 一段结论"v1 验证支持 ~N 并发，DB 连接峰值 M，主要瓶颈是 X"。
- [ ] **不修任何代码**。发现 panic / data race / DB 耗尽 → 单独在报告里列 "v2 candidate fixes" 清单交 Claude。

**Codex propose 前置**: **否**（按上述清单跑即可）。

**不在范围**: 修代码 / 调 DB 连接池 / 加 Redis。结果出来后单独开 T-019。

**依赖**: T-INT ✅（`cmd/loadtest` + admin 鉴权已就位）。

**参考**: `规划.md` §十.L3（1000 RPS / P95<80ms 验收门）+ `cmd/loadtest/README.md`。

**Result**: `04fff8a7` — 2500 real chat: 428/2500 2xx, 2072 upstream 429, P95 1.798s/P99 2.415s; healthz ~996 RPS, 0 errors.
DB sample peak by required `application_name LIKE 'omnitoken%'` filter: 0; report: `docs/release/v1-concurrency-baseline-2026-05-21.md`; all green.

**Review**: R-CONC-CHECK Approved with follow-ups (REVIEW.md)。无 CRITICAL。任务形式上完成；**抓出 3 个 follow-up**：M-23（成本归因 model_actual vs model_routed）/ H-3（DSN application_name）/ H-4（baseline 真实并发上限未拿到）。**M-23 起 T-CONC-COST-ATTR；H-4 由 ADR 0003 决策拉回 v1 起 T-CONC-RERUN（与 T-016 同 phase）；H-3 留 vNext T-CONC-DSN**。

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

## T-MK-RACE Makefile: race 验证移入 Docker [phase:infra] [owner:codex] [status:done]

Started: 2026-05-19 20:11 +08:00

**目标**: 让 Windows 主机不必装 gcc 也能完成提交前的 race 验证；同时把"本地 `make test` 跑 `-race`"这条隐性要求从默认路径移除（已与 AGENTS.md §3.3a/§3.3/§7 对齐）。

**涉及**:
- `Makefile` (现 `test` target 含 `-race`)
- 可能新增 `make test-race` target

**接受标准**:
- [ ] `make test`（默认）在 Windows 主机不带 `-race` 跑通，不依赖 gcc。
- [ ] 新增 `make test-race`：在 `golang:1.23`（或与 CI 一致的版本）容器里跑 `go test -race ./...`，挂载工作树 + Go module cache，**不要**把缓存目录写进工作树（参考 AGENTS.md §3.3 末条）。
- [ ] `make help` 文案同步更新。
- [ ] CI（`.github/workflows/ci.yml:21`）保持 `go test -race ./...`，**不动**——CI 是 Ubuntu，本来就能跑 race。
- [ ] 在新 target 的实现里，Docker 镜像版本/缓存挂载路径要在 commit message 写清楚（便于以后升 Go 版本时 grep）。

**不在范围**:
- 改 CI workflow（已经覆盖 race）。
- 在 Windows 装 MinGW/TDM-GCC（明确不做）。
- 改 Dockerfile.* 生产镜像（与测试无关）。

**Codex propose 前置**: **否**。直接做即可，方案空间很小（一条 docker run 命令）。

**依赖**: 无。

**参考**: AGENTS.md §3.3a 第 5 条、§3.3 第 3 条、§7 第 1 条；`docs/reviews/archive.md:308`（既往已知 Windows race 缺 gcc 的归档说明）。

**Result**: `a44f27a` - default test target no-race; Docker race target on `golang:1.25` with named module/build caches; all green.

---

## T-042 Codex 适配（配置 + 凭据写入） [phase:3-A] [owner:codex] [status:done]

Started: 2026-05-19 20:41 +08:00
Proposal: `docs/proposals/2026-05-19-t042-codex-adapter.md`

**目标**: 让企业员工跑 `omnitoken-adopt adopt codex` 一次，Codex CLI 之后所有调用都走 OmniToken。仿 T-041 的形状（写本地配置 + 备份 + restore），把第二个 Agent 落进 `internal/agent_adapter`，**同时收 R-041 的两条 MEDIUM** —— 这是抽 helper 的天然时机。

**涉及**:
- `internal/agent_adapter/`（新增 `codex.go` + `codex_test.go`；抽 `fileio.go` helper 同时用在 Claude Code 上）
- `cmd/omnitoken-adopt/main.go`（加 `adopt codex` / `restore codex` 分支）
- 现有 `claude_code.go` 要回归 helper（M-20 / M-21 修复）

**接受标准**:
- [ ] `internal/agent_adapter` 导出 `WriteCodexSettings(opts) (Result, error)` + `RestoreCodexSettingsWithOptions(opts) (Result, error)`。
- [ ] **抽出文件 I/O helper**: `writeAtomic(path, data)` (tmp + rename) 和缩进保形的 JSON merge 工具，**Claude Code 路径回归使用**——R-041 M-20 / M-21 由此一并解决。Claude Code 现有测试不准退化（覆盖率不降 + indent 保形断言新增）。
- [ ] 跨平台 `~/.codex/config.toml` 无损编辑 + `~/.codex/auth.json` 无损 merge；备份到 `~/.omnitoken/backups/codex/<filename>.<RFC3339-compact>.bak`。
- [ ] env / 配置字段集对齐 `docs/references/agent-adapter/agent-adapter-pattern.md` §3.4（Codex 部分）；managed key 白名单常量化，CLI stdout 打印 `managed_env` + `managed_toml` 两行。
- [ ] CLI 入口 `adopt codex --gateway-url <URL> --token <virtual_key> [--model <name>] [--home <path>]` / `restore codex [--home <path>]`；`flag` 标准库，不引第三方 CLI 框架。
- [ ] 失败语义对齐 T-041：invalid 现有 config → exit 2 + 单行 stderr + 原文件不改、备份不建（auth.json 走 JSON、config.toml 走所选 TOML lib 的 parse 路径）。
- [ ] 测试 ≥ 8 case：① 首次写 ② 合并保留用户 toml 注释/字段 ③ 合并保留 auth.json 非 managed key ④ 重复幂等 ⑤ 备份命名唯一 ⑥ restore 同时恢复 toml + json ⑦ `--home` 覆盖 ⑧ invalid existing config 双路径（toml 坏 + auth.json 坏）。
- [ ] `internal/agent_adapter` 包覆盖率 ≥ 80%（含 Claude Code + Codex 合并后）。

**Codex propose 前置**: **是**。PROPOSAL 答清 3 点：
1. **TOML 无损编辑方案**: Go 生态没有 Rust `toml_edit` 的直接对等物。`pelletier/go-toml/v2` Marshal 不保注释/空白；可选 (a) 引入它接受"非 managed key 字段值保留 + 注释丢失"的小代价；(b) 手写 minimal TOML patcher，**仅修改 managed key 行**（line-based regex/parser），其他字节不动；(c) 等用户没意见的话直接全文重写。各方案 license / 维护活跃度 / 复杂度对照。**默认推荐 (b)** —— 与 T-041 "管已知白名单、保其他字节" 哲学一致。
2. **`config.toml` vs `auth.json` 分工**: 哪些字段写 toml、哪些写 json？对照 tingly-box `agent-adapter-pattern.md`，给出最终 managed-keys 清单（两个文件分别列）。
3. **helper 抽法**: 是否值得现在就抽 `Registry` / `AgentConfig` interface？我的判断 **否** —— Phase 3-A 还要落 T-043 OpenCode 才到"三处重复"，现在抽是过早抽象，违反 AGENTS.md §3.1。**只抽 `fileio.go` 文件级 helper**，interface 等 T-040 后置。

**不在范围**:
- 协议转换 → T-045
- OpenCode 适配 → T-043
- `Registry` / `AgentConfig` interface 抽象 → T-040 后置
- Codex auth.json 里 OAuth refresh token 自动续期 → 留 Phase 3-B
- 端到端打通 Codex 走 OmniToken 实际能跑 → 等 T-045

**依赖**: T-041 ✅。T-MK-RACE 优先实施（让 Codex 不再需要在 chat 里解释 race）—— **不阻塞**，T-042 完全可以并行。

**参考**: `docs/references/agent-adapter/agent-adapter-pattern.md` §3.4（Codex Apply 模板）；`agent-adapter-projects-reference.md` §4.1（token_proxy `toml_edit` 无损编辑模式，但是 Rust，Go 需重新调研）；R-041 M-20 / M-21（REVIEW.md）。

---

## T-043 OpenCode 适配（配置写入） [phase:3-A] [owner:codex] [status:done]

Started: 2026-05-20 09:52 +08:00
Proposal: `docs/proposals/2026-05-20-t043-opencode-adapter.md`

**目标**: 让企业员工跑 `omnitoken-adopt adopt opencode` 一次，OpenCode IDE 插件之后所有调用都走 OmniToken。**第三个 adapter** —— 落地后即满足"三处重复"，T-040 抽象层提取解锁。

**涉及**:
- `internal/agent_adapter/opencode.go` + `opencode_test.go`（新增）
- `cmd/omnitoken-adopt/main.go`（加 `adopt opencode` / `restore opencode` 分支）
- **不动** `fileio.go` —— 现有 `readJSONObject` / `writeJSONAtomic` / `uniqueBackupPath` / `latestNamedBackupPath` 已够用（XDG 路径解析在 opencode.go 里写）

**接受标准**:
- [ ] `internal/agent_adapter` 导出 `WriteOpenCodeSettings(opts) (Result, error)` + `RestoreOpenCodeSettingsWithOptions(opts) (Result, error)`，签名风格对齐 T-041/T-042。
- [ ] **XDG 路径解析**: `$XDG_CONFIG_HOME/opencode/opencode.json` 优先；未设则回退 `$HOME/.config/opencode/opencode.json`（含 Windows `%USERPROFILE%\.config\opencode\opencode.json` 回退）。`--home <path>` 覆盖时按 `<home>/.config/opencode/opencode.json` 解析。
- [ ] **JSON 无损 merge**: 复用 `readJSONObject` + `writeJSONAtomic`；OmniToken 接管 `providers.omnitoken` 整个子对象（**整体替换**，与 T-042 `[model_providers.omnitoken]` table 替换语义一致），其他 provider 和 root 字段（`$schema` / 用户自定 key）保留。**M-20 indent 保形** 自然继承 fileio.go 行为。
- [ ] 备份到 `~/.omnitoken/backups/opencode/opencode.json.<compact-UTC>.bak`；首次写无备份；重复幂等。
- [ ] managed key 白名单 `managedOpenCodeProviderKeys` 常量化，CLI stdout 打印 `managed_provider providers.omnitoken,...`（**或** `managed_keys`，措辞由 propose 答）。
- [ ] CLI 入口 `adopt opencode --gateway-url <URL> --token <virtual_key> [--model <name>] [--home <path>]` / `restore opencode [--home <path>]`；`flag` 标准库。
- [ ] 失败语义对齐 T-041/T-042：invalid 现有 opencode.json（非 JSON object / `providers` 非 object）→ exit 2 + 单行 stderr + 原文件不改、备份不建。
- [ ] 测试 ≥ 7 case：① 首次写（含建空目录） ② 合并保留 `$schema` 和其他 provider ③ 重复幂等 ④ 备份命名唯一 ⑤ restore 最新备份 ⑥ `--home` 覆盖 ⑦ XDG_CONFIG_HOME env 覆盖（含设置 vs 未设置两种）。**所有测试 100% 用 `t.TempDir()` + `t.Setenv("XDG_CONFIG_HOME", ...)` / `t.Setenv("HOME", ...)`** —— AGENTS.md §9.5 硬约束。
- [ ] `internal/agent_adapter` 包覆盖率 ≥ 80%（合并三个 adapter 后）。

**Codex propose 前置**: **是**。PROPOSAL 答清 3 点：
1. **OpenCode managed 字段集**: opencode.json 的 `providers.omnitoken` 子对象需要哪些字段？对照 `agent-adapter-projects-reference.md` 第 233/329-333 行（token_proxy Rust 实现）和 OpenCode 官方 schema（https://opencode.ai/config.json）给最终清单 —— 至少含 `base_url` / `models` 数组 / 是否需要 `apiKey` 字段。**注意**：OmniToken 的 token 应放 OpenCode 的 secret 字段（如 `apiKey` 或独立 `auth` 字段），不要明文进 opencode.json 如果 OpenCode 支持外置 secret。如不支持就明文写但 stdout 不回显（同 T-041/T-042 安全纪律）。
2. **XDG fallback 在 Windows 的行为**: Windows 上 `XDG_CONFIG_HOME` 一般未设，OpenCode 实际查 `%APPDATA%\opencode\` 还是 `%USERPROFILE%\.config\opencode\`？给出选择依据 + 1 个测试覆盖 Windows fallback。**默认推荐**：与 Linux 行为对齐 `<home>/.config/opencode/`（OpenCode 文档若另说则跟官方）。
3. **`--home` vs `--config-home` 旗标**: 是否需要单独的 `--config-home` 覆盖 XDG_CONFIG_HOME？我的判断 **不需要** —— `--home` 一刀切（`<home>/.config/opencode/`）测试足够，advanced 用户可以直接设 `XDG_CONFIG_HOME` env。Codex 给反对意见再 propose。

**T-040 trigger**: T-043 commit 落地 + R-043 approve 后，`internal/agent_adapter` 有三个具象 adapter（Claude Code / Codex / OpenCode）+ 共享 fileio helper。**这是开 T-040 的信号** —— 抽 `Registry` + `AgentConfig` interface 时机成熟。T-043 任务本身**不**做抽象抽取（属 T-040 范围），但 commit message 里点一下"三 adapter 全齐，T-040 可启"。

**不在范围**:
- 协议转换 / 真实端到端调通 → T-045
- `Registry` / `AgentConfig` interface 抽象 → **T-040**（T-043 完成后启）
- IDE 插件本体注入（VSCode extension 端）→ Phase 3-B 视市场反馈
- 多 OpenCode 实例 / 用户 namespace 隔离 → 暂停区

**依赖**: T-042 ✅（`fileio.go` 已抽出且包含 M-20/M-21 修复）。

**参考**: `docs/references/agent-adapter/agent-adapter-pattern.md` §3.4（含 OpenCode 完整模板，但 tingly-box 把 Config 留作 opaque `map[string]any`，managed 字段需 propose 自定）；`agent-adapter-projects-reference.md` 第 233 / 329-352 行（token_proxy Rust 写 opencode.json + `resolve_opencode_config_dir` XDG 解析模式 —— Go 等价实现是本任务核心参考）；R-042 N-7（`firstString` BackupPath 语义可顺手改名 / 删除，**可选**，不强求）。

**Result**: `5254c48` — agent_adapter 82.2%; Q-1 选单一 `--model`，多模型留 T-044；N-8 文件注释已落. all green.

---

## T-040 抽象层提取（Registry + AgentConfig interface） [phase:3-A] [owner:codex] [status:done]

Started: 2026-05-20 12:33 +08:00
Proposal: `docs/proposals/2026-05-20-t040-agent-registry.md`

**目标**: 三个具象 adapter（Claude Code / Codex / OpenCode）已就位，凑齐 AGENTS.md §3.1 "三处重复再抽象"门槛。本任务**只做内部抽象提取**，外部行为零变化、CLI 入口零变化、所有现有测试零退化。是 T-044（路由规则联动）和 T-046（一键 onboard 收口）的前置基础。

**涉及**:
- `internal/agent_adapter/`（新增 `registry.go` + `interface.go` 或合一；refactor `claude_code.go` / `codex.go` / `opencode.go` 让 `*XxxConfig` 实现 interface）
- 现有 `WriteXxxSettings(opts XxxOptions) (Result, error)` / `RestoreXxxSettingsWithOptions(opts) (Result, error)` 函数**保留**作为 thin wrapper，**不动 CLI** `cmd/omnitoken-adopt/main.go`（CLI 切到 registry 是 T-046 的事）。
- `nonEmptyStrings` 从 `claude_code.go:193` 挪到 `fileio.go` 或新 helper 模块（**收 R-043 N-9**）。

**接受标准**:
- [ ] 定义 `AgentConfig` interface，导出 `Write(opts) (Result, error)` 和 `Restore(opts) (Result, error)` 两个方法；签名由 propose 拍板（type-safe 路线还是 `interface{}` 路线，见 propose 第 1 条）。
- [ ] 定义 `Registry`：注册 / 查询 / 枚举三个能力。**默认 Registry 实例**包级可访问；同时支持 `NewRegistry()` 构造私有实例（便于测试）。
- [ ] `*ClaudeCodeConfig` / `*CodexConfig` / `*OpenCodeConfig` 三个类型实现 interface，在 `init()` 里注册到 default registry，agent type 常量 `AgentTypeClaudeCode` / `AgentTypeCodex` / `AgentTypeOpenCode` 导出。
- [ ] **现有 `WriteXxxSettings` / `RestoreXxxSettingsWithOptions` 导出函数保留**，内部 delegate 到 `(&XxxConfig{}).Write(opts)` —— 零 backward-compat 破坏，CLI 不需要改。
- [ ] **`Result` struct 收敛**：当前 9 字段（`SettingsPath`/`ConfigPath`/`AuthPath`/`BackupPath`/`BackupPaths`/`RestoredFrom`/`RestoredFromPaths`/`ManagedKeys`/`ManagedEnvKeys`/`ManagedTomlKeys`/`Warnings`）冗余明显。propose 给瘦身方案（建议方向：保 `BackupPaths`/`RestoredFromPaths`/`ManagedKeys`/`Warnings`，弃单值 `BackupPath`/`RestoredFrom`，路径角色用 `Paths map[string]string` 或保 `ConfigPath`+`AuthPath` 两个具名字段）。CLI 同步收敛，**测试断言相应更新但语义不变**。
- [ ] `nonEmptyStrings` 挪到 `fileio.go`（R-043 N-9 收口）；不引入新的工具文件。
- [ ] 测试：① interface 三实现各自单测可通过（沿用现有测试，仅签名挪位） ② 新增 `registry_test.go`：注册 / 查询 / 枚举 / 重复注册行为 ③ end-to-end 检查 `Registry.Get(AgentTypeClaudeCode).Write(...)` 与现有 `WriteClaudeCodeSettings(...)` 路径**结果完全一致**（建议表驱动）。
- [ ] `internal/agent_adapter` 包覆盖率**不降**（当前 82.2%，目标 ≥80%）。
- [ ] `cmd/omnitoken-adopt/main.go` **零修改**（CLI 切 registry 留 T-046）。如果 propose 觉得本任务顺手把 CLI 切了更合算，**先 propose 说服 Claude**，不要单边决定。

**Codex propose 前置**: **是**。PROPOSAL 答清 3 点：
1. **interface 方法签名**: 三个路线对比：(a) tingly-box `Apply(paramsInterface interface{}) (*Result, error)` —— 动态类型，每个实现内部 type-assert；(b) Go 1.21+ 泛型 `type AgentConfig[Opts any] interface { Write(Opts) (Result, error) }` —— 类型安全但 Registry 没法 homogeneous（无法 `map[AgentType]AgentConfig`）；(c) **base options struct**（`type BaseOptions struct { Home, GatewayURL, Token, Model string; Now func()time.Time }`），interface 用 `Write(BaseOptions) (Result, error)`，每个具象 Options 类型嵌入 BaseOptions 但目前三个 adapter 字段一致没差异。**默认推荐 (c)** —— 类型安全且 Registry 同构。
2. **Result 收敛方案**: 给最终 struct，包括哪些字段保哪些删；测试中 `BackupPath` 单值断言怎么平迁到 `BackupPaths[0]`；OmniToken adopt CLI 输出格式是否变（建议**不变**，单值字段在 CLI 层从 Paths[0] 现取）。
3. **Registry 注册时机**: package `init()` 隐式注册 vs 显式 `RegisterDefaults()` 函数调用。**默认推荐 init()**——零样板代码，但测试要能 `NewRegistry()` 拿到空 registry（用于 isolated test）。

**不在范围**:
- CLI 切 registry 入口 → **T-046**
- 路由规则联动（adopt 配置 = 同步生成 admin 端 virtual_models）→ **T-044**
- 协议转换（Anthropic ↔ OpenAI）→ **T-045**
- 新的 adapter（第四个 agent）→ 出现具体需求再开

**依赖**: T-043 ✅（三个 adapter 齐了）。

**参考**: `docs/references/agent-adapter/agent-adapter-pattern.md` §1（Registry 接口模板，但 Go 用 `interface{}` params 不够 idiomatic，自定签名）+ §3.3/§3.4（三个 Apply/Restore 实现模板对照）；R-043 N-9（`nonEmptyStrings` 归并）。

**Result**: `147502da` — agent_adapter 83.6%; N-10 用 `DefaultRegistry.MustRegister(...)`; compat shim 保 CLI 零改. all green.

**Review**: R-040 Approved (REVIEW.md)。无 CRITICAL/HIGH；N-11 (paths() helper 过滤分支永不命中) + N-12 (init() 指针 vs value-receiver 风格混用) 不阻塞，下次顺手裁。T-044 / T-046 解锁。
