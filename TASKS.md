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
| 05-23 | **README 用户化重写 + 公开仓库上线**：READMEs 去草稿,LICENSE Apache-2.0,`.omnitoken-master-key` 加入 gitignore;`master` → `main`;新建 https://github.com/CcCalana/omnitoken (public) |
| 05-23 | **T-UI-L1-THEME 任务体写好**（status:todo）。借鉴 metapi (MIT) 前端设计语言 L1 档：design tokens CSS + Toast + Modal + dark theme,守住 vanilla JS 不引入 React/Tailwind/build |
| 05-23 | **上线门评估**：用户列三条 release-gate ①前端看报 ✅(overview 趋势+模型环图,无时间切换是已知非阻塞)/②管理员分配额度 ✅(users tab 月度预算编辑 + RBAC)/③审计查看用户使用场景 ❌ 缺口 → 落地为 T-AUDIT-USAGE-VIEW |
| 05-23 | **T-AUDIT-USAGE-VIEW 任务体写好**（status:todo）。audit tab 加 tab 切换"管理操作流水 / 用户使用流水"；后者按 user_id 聚合 usage_events，含模型 top-N、小时分布、近 N 次调用详情 |
| 05-30 | **用户决策: v1 发布后优先走 Phase 3-A 收尾 (T-045 协议转换)，再走 vNext 基础设施**。T-AUDIT-USAGE-VIEW H-8/M-33 已在 `e9d878a` 修复，用户确认无需二审。T-CONC-RERUN 经 T-MP-DEEPSEEK 100% 验证、H-6 关闭，签字 done。v1 底座三角完整就绪 |
| 05-30 | **T-045 proposal (`b61600a`) → R-045-prop Approved**。5/5 决策采纳；handler 分层 `anthropic.MessagesHandler → usage.Middleware → proxy` 用 transforming ResponseWriter 让 usage 捕获 OpenAI bytes 不改 parser。Codex 开实施 |
| 05-30 | **T-045 impl (`d17a430`) → R-045 Approved**。1070 行纯 stdlib，4 文件零改 usage 包。transforming ResponseWriter + SSE 状态机 + X-1/X-2/X-3 全部落地。Phase 3-A "demoable moment" 达成——Claude Code 可通过 OmniToken `/v1/messages` 调用任意 OpenAI-compatible 上游 |
| 05-30 | **T-044 任务体写好**（status:todo）。virtual_models CRUD API + UI + `omnitoken adopt --ensure-model` 联动；抄 T-016b-MIN credential CRUD 模式，跳过 propose |

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
| 2-C 性价比 | **T-MP-DEEPSEEK Multi-provider 池接入 DeepSeek**（ADR 0004）| cross-provider 池 + DeepSeek 3 key seed + proxy base_url 路由 + 30 conc 100.0% 验收 | 2d | T-016 ✅ | ✅ |
| 2-C 运维 | **T-016b-MIN Admin Credential CRUD + 30s Polling**（ADR 0005）| admin POST/PATCH credential + gateway 热加载 + 前端 tab | 2d | T-MP-DEEPSEEK ✅ | ✅ |
| 2-C 验证 | **T-CONC-RERUN 多 key 池真实 baseline**（原 vNext 拉回）| mock upstream 测 gateway 承载 + 真 Ark/DeepSeek 复跑；经 T-MP-DEEPSEEK 100.0% 验证 H-6 关闭 | 1d | T-016 ✅ | ✅ |
| v1 门③ | **T-AUDIT-USAGE-VIEW 用户使用流水** | audit tab 双 tab + per-user usage 聚合 API + 前端图表 | 2d | T-013 ✅ + T-015 ✅ | ✅ |

**v1 底座三角状态（2026-05-30）**: 全部 ✅。性价比（T-016 + T-MP-DEEPSEEK + T-016b-MIN）/ 权限额度（T-005a + T-015 + T-005b）/ 安全审计（T-013 + T-014 + T-AUDIT-USAGE-VIEW）。release gate ①②③ 全部满足。

### vNext（v1 发布后再做）

- **T-017b fallback retry on 5xx/429**（2d，含 SSE 中途切换状态机）
- **T-018 故障注入 e2e**（与 T-017b 配套，1-2d）
- **T-100 L2 端到端正确性套件**（1 admin + 10 user 真方舟 e2e）
- **T-QUOTA-CACHE-PROBE**（2026-05-21 外部专家提）：跑 mock upstream 高并发后，量 `monthlyBudgetStatusSQL`（`internal/quota/store_postgres.go:48` 双 LEFT JOIN + SUM）在真实 gateway 承载下的 PG CPU / 慢查询 / 连接池占用。如果发现是瓶颈，候选解：(a) `usage_events(organization_id, user_id, created_at)` + `cost_ledger(usage_event_id)` 加索引（低风险）；(b) Redis 月度额度缓存 + 异步入账后增量更新（架构性变更，需 ADR）。**先量再写实现**，本条只是观察任务。
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
| 5 | T-045 Anthropic → OpenAI 协议转换 | gateway 多挂 `/v1/messages`；Anthropic↔OpenAI 双向转换，让 Claude Code 真正能跑 | 4d | T-041 ✅ + T-016 ✅ | ✅ `d17a430` |
| 6 | T-044 路由规则联动 | apply 配置 = 同步生成 OmniToken 内部 `virtual_models` + admin 端可视化 | 2d | T-040 | todo |
| 7 | T-046 一键 onboard CLI 收口 | `omnitoken adopt <agent>` 统一入口 + 退出 / restore | 1d | T-044 + T-045 | todo |

**Phase 3-A 当前焦点**: T-045 协议转换——这是"真正 demoable 时刻"。完成后 Claude Code 可以走 OmniToken 调用任意 OpenAI-compatible 上游。T-044/T-046 在 T-045 之后。

---

## T-045 Anthropic → OpenAI 协议转换 [phase:3-A] [owner:codex] [status:done] [started:2026-05-30 10:27 CST]

**目标**: gateway 新增 `POST /v1/messages` 端点，接收 Anthropic Messages API 格式请求，转换为 OpenAI chat completions 格式后走既有 proxy + middleware 管道，上游响应转换回 Anthropic 格式返回客户端。使 Claude Code 能通过 OmniToken 调用任意 OpenAI-compatible 上游（Ark / DeepSeek / 未来 provider）。

**背景**: T-041~T-043 已让 Claude Code / Codex / OpenCode 的配置指向 OmniToken，但 Claude Code 使用 Anthropic Messages API 协议，当前 gateway 只有 OpenAI-compatible `/v1/chat/completions`。不做协议转换，Claude Code 的请求无法到达上游。

**涉及**:
- `internal/proxy/anthropic.go` (新增) — Anthropic↔OpenAI 请求/响应转换器 + SSE 事件映射
- `internal/proxy/anthropic_test.go` (新增) — 转换正确性测试 + golden fixture 回放
- `internal/proxy/anthropic_e2e_test.go` (新增) — 端到端：Claude Code 格式请求 → gateway → 真上游 → Anthropic 格式响应
- `cmd/gateway/main.go` — 注册 `POST /v1/messages` 路由，复用既有 middleware 栈
- `internal/usage/middleware.go` — `snapshotRequestMetadata` 可能需适配 Anthropic 请求体格式（提取 model / stream）
- `testdata/golden/anthropic/` (新增) — Anthropic 请求/响应 golden fixtures

**架构指引**（Codex 在 PROPOSAL 中拍板具体方案）:

Anthropic 转换器作为一个 `http.Handler` wrapper，包裹既有的 OpenAI proxy handler。转换器负责：
1. **请求转换**（Anthropic → OpenAI）：解析 Anthropic Messages API 请求体，构造等效的 OpenAI chat completions 请求体
2. **委托上游**：将转换后的请求交给既有 proxy 管道（credential selection / retry / upstream 转发全部复用）
3. **响应转换**（OpenAI → Anthropic）：非流式 buffer 完整响应后转换；流式逐 SSE chunk 转换

请求体字段映射（核心）：
- `model` → `model`（直接透传，virtual model 解析在转换前完成）
- `system`（string 或 array）→ `messages` 数组头部插入 system role message
- `messages[].role` → `messages[].role`（user/assistant 直通）
- `messages[].content`（string 或 content block array）→ `messages[].content`（string 或 OpenAI 多模态数组）
- `max_tokens` → `max_tokens`
- `stream` → `stream`
- `temperature` / `top_p` / `stop_sequences` → `temperature` / `top_p` / `stop`

响应体字段映射（非流式，OpenAI → Anthropic）：
- `id` → `id`，`model` → `model`
- `choices[0].message.content` → `content: [{type: "text", text: "..."}]`
- `choices[0].message.reasoning_content`（如有）→ `content` 数组前置 `{type: "thinking", thinking: "..."}`
- `choices[0].finish_reason` → `stop_reason`（映射：`stop`→`end_turn`, `length`→`max_tokens`, `tool_calls`→`tool_use`）
- `usage.prompt_tokens` → `usage.input_tokens`
- `usage.completion_tokens` → `usage.output_tokens`
- `usage.prompt_tokens_details.cached_tokens` → `usage.cache_read_input_tokens`
- `usage.total_tokens` → `usage.input_tokens + output_tokens`（Anthropic 没有 total 字段，由 client 自行加总）

流式 SSE 事件映射（OpenAI → Anthropic）：
- **首个 chunk**（含 role + content 或空 delta）→ `message_start` event（含 message 元信息 + `usage.input_tokens` 如果已知）
- **首个有内容的 chunk** → `content_block_start` + `content_block_delta` events
- **后续 content chunk** → `content_block_delta` events
- **reasoning chunk** → `content_block_start`(type:thinking) + `content_block_delta`(type:thinking_delta)
- **chunk 含 `finish_reason`** → `content_block_stop` + `message_delta`（含 `stop_reason` + `usage.output_tokens`）
- **`[DONE]`** → `message_stop` event

**接受标准**:
- [ ] `POST /v1/messages` 非流式请求 → gateway 返回合法 Anthropic Messages API 响应（`type: "message"`，含 `content`/`stop_reason`/`usage`）
- [ ] `POST /v1/messages` 流式请求（`stream: true`）→ gateway 返回合法 Anthropic SSE 事件流（`message_start` → `content_block_start/delta/stop` → `message_delta` → `message_stop`）
- [ ] 既有 middleware 全部复用：protectGatewayRoute（virtual key 鉴权）/ enforceMonthlyBudget（额度）/ resolveVirtualModel（虚拟模型）/ usage 记录（`usage_events` + `cost_ledger` 正确写入）
- [ ] `snapshotRequestMetadata` 正确提取 Anthropic 请求中的 `model` 和 `stream`（适配 Anthropic 请求体格式）
- [ ] 上游错误（4xx/5xx）返回 Anthropic 格式 error（`{"type": "error", "error": {"type": "api_error", "message": "..."}}`），不透传上游 stack trace
- [ ] `usage` 字段正确映射：`input_tokens` ← `prompt_tokens`，`output_tokens` ← `completion_tokens`，`cache_read_input_tokens` ← `prompt_tokens_details.cached_tokens`
- [ ] 非流式 golden fixture 回放测试：`anthropic_nonstream_default.json` 作为请求 → 转换 → 转换回 → 验证语义等价
- [ ] 流式 golden fixture 回放测试：mock upstream 返 OpenAI SSE → 逐 chunk 转换为 Anthropic SSE → 验证事件类型和字段
- [ ] e2e 测试：用 `testdata/golden/ark/anthropic_nonstream_default.json` 作为请求体 → 打真上游 → 响应为非流式 Anthropic 格式 → `usage.input_tokens + output_tokens` 正确
- [ ] credential selector 正确工作：`provider=ark` 走 Ark upstream，`provider=deepseek` 走 DeepSeek upstream；`upstream_credential_id` 写入 `usage_events`
- [ ] RBAC：viewer 不可写，member 可调用；session 失效 401
- [ ] `go vet ./...` + `go test -race ./...`（Docker 内）全绿；`internal/proxy` 覆盖率不退化

**不在范围**:
- ❌ 完整 Anthropic Tools/Function calling ↔ OpenAI tools mapping（请求体中如有 tools 字段，v1 可透传或丢弃，但不可 500 crash）
- ❌ Anthropic prompt caching 请求头（`anthropic-beta: prompt-caching-*`）→ v1.1
- ❌ 多模态 content block 映射（Anthropic `type: "image"` / `type: "document"` → OpenAI `image_url` / `file_url`）→ v1.1
- ❌ thinking budget_tokens → reasoning_effort 语义映射（v1 直通 `thinking` 字段）
- ❌ Anthropic `computer_use` / `bash_use` tool 类型 → v1.1
- ❌ 上游 Anthropic-compatible 直通（如 Ark `/v1/messages` 直接转发，不做转换）—— v1 统一走 OpenAI 转换路径，减少双路径复杂度
- ❌ admin UI 改动
- ❌ T-044 路由规则联动 / T-046 CLI 收口

**Codex propose 前置**: **是**。PROPOSAL 答清 5 点：
1. **转换层放置位置**: (a) 新 `http.Handler` wrapper 包裹既有 proxy，放在 middleware 栈内层（推荐——最大化复用 credential/retry/usage）；(b) 独立 handler 直接调 upstream（不共享 proxy 代码路径）。**默认推荐 (a)**。
2. **流式 SSE 转换策略**: (a) 逐 chunk 实时转换（低延迟，但需维护 SSE 状态机——当前 chunk 是首个/中间/最后）；(b) buffer 全部 OpenAI chunks 后一次性构造 Anthropic SSE 流（简单但 TTFB 差）。**默认推荐 (a)**。
3. **usage 解析路径**: (a) 转换后使用既有 OpenAI usage parser（转换器输出 OpenAI 格式 → middleware 捕获 → `ParseStream/ParseNonStream` 直接工作）；(b) middleware 捕获 Anthropic 格式 → 新增 Anthropic usage parser。**默认推荐 (a)**——让 middleware 看到转换前的 OpenAI 中间格式，不改 usage 包。
4. **`snapshotRequestMetadata` 适配**: Anthropic 请求体中 model 字段在顶层 `"model"`，stream 也是 `"stream"`（布尔）。但 Anthropic 没有 `messages[0].content` 这种简单结构。propose 拍：(a) 在 middleware 内 detect Anthropic vs OpenAI 请求体格式（按 URL path 或 Content-Type 或 body 内 `"messages"` 结构的差异）；(b) 转换器在 request context 里预注入 model/stream 元信息，middleware 从 ctx 读。**默认推荐 (b)**——解耦。
5. **SSE content block index 管理**: Anthropic SSE 要求每个 content block 有单调递增的 `index`（0, 1, 2...）。OpenAI SSE 的 chunk index 是 choice index。propose 拍如何从 OpenAI chunks 推导 Anthropic content block 边界（text 开始=新 block start，reasoning→text 切换=新 block，等等）。

**依赖**: T-016 ✅（多 key 池 + credential selector）；T-MP-DEEPSEEK ✅（多 provider 验证通过）；T-041 ✅（Claude Code 配置写入就绪，可以 e2e 验证）

**禁动**:
- 既有 `/v1/chat/completions` 路径行为（零 regression）
- 既有 proxy retry / credential selection 逻辑
- `internal/credentials/` 接口
- `internal/usage/parser.go` 中 OpenAI 格式解析（如果 propose Decision 3 选 a）

**异常处理**:
- Anthropic 请求体中 `messages` 为空 → 400 + Anthropic error format
- `stream: true` 但上游返回非流式 → 当作非流式处理，不 panic
- 转换过程中上游 SSE 格式异常（缺 usage / 非 JSON data）→ 尽最大努力 emit 已接收内容 + log WARN，不崩 gateway
- `content` 为 null 的 assistant message（Anthropic 合法，表示 tool_use）→ v1 当作空 text 处理，不 500

**参考**:
- 既有 proxy 实现：`internal/proxy/proxy.go`（`ServeHTTP` / `rewriteRequest` / `doWithRetries` / `copyStreamingResponse`）
- 既有 middleware 栈：`cmd/gateway/main.go:116-121`（`newMux`）
- Anthropic golden fixture：`testdata/golden/ark/anthropic_nonstream_default.json`
- OpenAI golden fixtures：`testdata/golden/ark/openai_*.json` / `openai_stream_*.txt`
- Anthropic Messages API 文档：`https://docs.anthropic.com/en/api/messages`
- Claude Code adapter：`internal/agent_adapter/claude_code.go`（`ANTHROPIC_BASE_URL` / `ANTHROPIC_MODEL` 生成逻辑）

Result: `d17a430` + `fd35718` — 1070 行纯 stdlib，4 文件零改 usage 包；transforming ResponseWriter + SSE 状态机 + X-1/X-2/X-3 全部落地；make lint/test/test-race + proxy e2e 全绿；no undeclared deviation.

---

<!-- T-041 / T-CONC-CHECK / T-MK-RACE / T-042 / T-043 / T-040 任务体已迁出。
     done 状态见 Phase 3-A 表 + 速查表；R-* 详 docs/reviews/archive-2026-05-20.md；
     PROPOSAL 在 docs/proposals/；实现 diff 走 git log。

     T-016 + T-CONC-COST-ATTR 任务体已迁出（2026-05-21 done）。done 状态见
     Phase 2 表 + 速查表；R-016-prop / R-016 详 REVIEW.md；PROPOSAL 在
     docs/proposals/2026-05-21-t016-upstream-credential-pool.md；
     ops 文档 docs/operations/master-key-rotation.md；
     migration 000012；实现 diff 走 git log (c6ee841d + 8544ce82)。 -->

---

## T-044 路由规则联动 — virtual_models CRUD + Agent 配置同步 [phase:3-A] [owner:codex] [status:review] [started:2026-05-30 11:27 CST]

**目标**: 补齐 virtual_models 的写路径（API + UI），让 `omnitoken adopt` 在写入 agent 配置前自动确保目标 virtual model 存在于服务端。打通"admin 新建 virtual model → omnitoken adopt 引用 → gateway 解析 → 正确 provider 路由"的完整链路。

**背景**: 当前 virtual_models API 只读（`GET /api/admin/virtual-models`），表数据靠 `deploy/postgres/002_seed.sql` 手工维护。admin UI 的 virtual_models 页是纯展示表格，无新建/编辑入口。`omnitoken adopt` 接受 `--model chat-fast` 但从不验证服务端是否存在该 virtual model——如果不存在，gateway 会把它当真实模型名透传，provider 上下文丢失。

**涉及**:
- `cmd/admin/main.go` — 新增 `POST /api/admin/virtual-models` + `PATCH /api/admin/virtual-models/{name}` 端点
- `cmd/admin/main_test.go` — handler 测试（create / update / conflict / RBAC）
- `cmd/omnitoken-adopt/main.go` — adopt 前调 admin API 确保 virtual model 存在
- `internal/agent_adapter/` 或 `cmd/omnitoken-adopt/` — `EnsureVirtualModel` HTTP client helper
- `web/src/views/virtual_models.js` — 加 create form + edit modal + disable 按钮
- `web/src/api.js` — 加 `createVirtualModel` / `updateVirtualModel` 方法

**接受标准**:
- [ ] `POST /api/admin/virtual-models` 端点：input `{name, real_model, provider, description?}`；validation：`name` 非空且唯一、`real_model` 非空、`provider` ∈ `{ark, deepseek}`、`description` 可选。成功 201。同名冲突 409 + `error_code: "virtual_model_exists"`。RBAC admin role + audit_logs
- [ ] `PATCH /api/admin/virtual-models/{name}` 端点：input 全部可选 `{real_model?, provider?, status?, description?}`。至少一个字段非空。`status` 只接受 `active` / `disabled`。幂等——重复 PATCH 同值 200。name 不存在 404。RBAC + audit_logs 同上
- [ ] 既有 `GET /api/admin/virtual-models` 行为零 regression
- [ ] `cmd/omnitoken-adopt` 加 `--admin-url` / `--real-model` / `--provider` 三个可选 flag。三者全提供时：在写 agent 配置前调 admin API 查 virtual model 是否存在；不存在则 `POST` 创建；存在且 provider+real_model 匹配则静默继续；存在但不匹配则 error 退出（不覆盖）
- [ ] `cmd/omnitoken-adopt` 加 `--ensure-model` flag（bool，默认 true 当 `--admin-url` 提供）。设为 false 跳过 ensure
- [ ] admin API client 用标准 `net/http`，10s 超时，非 2xx 返明确 error
- [ ] `web/src/views/virtual_models.js` 加"新建"按钮 → modal 表单（name / real_model / provider dropdown / description）。提交走 `POST`，成功后 toast + 刷新
- [ ] 列表每行加"编辑"按钮 → 同 modal 预填现有值、name disabled（不可改名）。提交走 `PATCH`
- [ ] 列表每行加"启用/禁用"toggle。禁用后 status badge 变灰
- [ ] 现有 seed SQL 的 6 个虚拟模型可编辑/禁用，不破坏已有展示
- [ ] `web/src/views/virtual_models.test.js` 不破坏
- [ ] **Docker-only**：Go test `docker compose run --rm test` + 前端 `node --test`
- [ ] `go vet ./...` + `go test -race ./...` 全绿；`cmd/admin` 覆盖率不退化

**不在范围**:
- ❌ `routes` 表（fallback chains / canary weights）—— v1 路由 = `virtual_models.provider`，高级路由留 v1.1
- ❌ `DELETE /api/admin/virtual-models/{name}` —— disable 是审计友好的替代
- ❌ agent 类型到 virtual model 的显式关联表 —— v1 隐式关联（model name 匹配）足够
- ❌ `omnitoken adopt --list-models` 或交互式模型选择器 —— 推 T-046
- ❌ 修改 gateway `resolveVirtualModel` 或 credential selector 逻辑
- ❌ `internal/router/` 重构

**Codex propose 前置**: **跳过**（范围由 TASKS 锁定，类似 T-016b-MIN）。实施时如遇未声明偏差，停并向 Claude 报告。

**依赖**: T-040 ✅（Registry + CLI 骨架）；T-045 ✅（gateway `/v1/messages` 走 virtual model 解析）

**参考**:
- 既有 credential CRUD：`cmd/admin/main.go` POST/PATCH credential handler + `web/src/views/credentials.js` modal（T-016b-MIN，可逐块复用）
- 既有 virtual_models 只读 handler：`cmd/admin/main.go:565-581`
- virtual_models schema：`migrations/000009_virtual_models.up.sql` + `000013` provider 列
- 既有 UI：`web/src/views/virtual_models.js`（~70 行纯展示）
- Agent adapter CLI：`cmd/omnitoken-adopt/main.go`（`--gateway-url` / `--token` / `--model` 已存在）
- RBAC：`internal/auth/rbac.go`；Audit：`internal/audit/middleware.go`

**Hints**:
- **代码抄 T-016b-MIN**：credential CRUD 的 handler 结构 / modal 表单 / validation 几乎可逐块复用，virtual_models 更简单（字段少、无加密），2d 充裕
- **HTTP client 位置**：放 `cmd/omnitoken-adopt/` 做私有 helper 或新 `internal/adminclient/` package。不放 `internal/agent_adapter/`（关注点不同）
- **`--admin-url` 默认**：不强制推导。docker-compose 约定 `8081`，但让用户显式传
- **冲突处理**：virtual model 已存在但 provider/real_model 不同 → error 退出，不静默覆盖（安全选择——覆盖可能影响其他 agent 用户）
- **PATCH 语义**：只更新 provided 字段。未提供的字段保持原值
- **前端 modal**：用现有 inline form pattern（与 credentials.js 风格一致），不新建通用 modal 组件（留给 T-UI-L1-THEME）

**禁动**:
- 既有 `GET /api/admin/virtual-models` handler 行为
- `internal/router/resolver.go` 接口
- gateway `resolveVirtualModel` 中间件
- `deploy/postgres/002_seed.sql`
- T-016b-MIN credential CRUD 路径

**Result**: `8fb054e` — virtual_models CRUD + UI + adopt ensure-model landed; lint/test/race/node all green, no deviation.

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


**Superseded 2026-05-23 → resolved 2026-05-30**: 原 Ark 单 provider 路径永久无法达成 >80%（物理约束，详 ADR 0004）。H-6 经 T-MP-DEEPSEEK 跨 provider 形态验证 30 conc / 100.0%，二审关闭。本任务 done。

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

## T-016b-MIN Admin Credential CRUD UI (Min) + 30s Polling Hot Reload (v1 上线必备) [phase:2-C] [owner:codex] [status:done]

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

Result: `4b3d6b32` — admin credential add/disable + 30s polling hot reload landed; Docker-only all green, no undeclared deviation.

---

## T-UI-L1-THEME 前端视觉对齐 L1 + Toast/Modal + Dark Theme [phase:post-v1] [owner:codex] [status:todo]

**目标**: 借鉴 [cita-777/metapi](https://github.com/cita-777/metapi)（MIT 许可）的前端设计语言到 `web/`，**只做 L1 视觉对齐**：design tokens CSS + dark theme 切换 + Toast 组件 + Modal 组件。**不引入** React/Tailwind/Vite/任何构建工具，守住 OmniToken `web/` 当前的纯静态、零构建形态。

**背景**: metapi 是 AI router aggregator（产品定位与 OmniToken 不同），但其前端 design tokens / 组件模式可借鉴。L1 = 视觉与基础组件，不抄页面结构。L2/L3（组件库迁移 / React 重写）作为独立任务后续讨论。

**涉及**:
- `web/styles.css` (现 899 行) — 重构成 `:root` + `[data-theme="dark"]` CSS 变量驱动，所有现有色值改成变量引用
- `web/index.html` — 在 `<html>` 上加 `data-theme` 属性接入点，topbar 加主题切换按钮
- `web/src/app.js` (现 114 行) — 主题切换 state + localStorage 持久化 + system preference 监听
- `web/src/components/toast.js` (新增) — 全局 Toast 容器 + `showToast(message, kind)` 函数，kind ∈ {info, success, warning, danger}
- `web/src/components/modal.js` (新增) — `openModal({title, body, actions, onClose})` 函数，焦点陷阱 + ESC 关闭 + 背景点击关闭
- 6 个现有 view (`overview/users/models/virtual_models/credentials/audit`) — 把现有 `alert()` / `confirm()` / 手写 banner 调用替换为 toast / modal

**接受标准**:
- [ ] `web/styles.css` 头部含完整 design tokens block：color (primary / success / warning / danger / info + *-soft 浅底变体) / radius (sm/md/lg/xl) / shadow (sm/md/lg/card) / topbar-height / sidebar-width / z-index 层级
- [ ] `[data-theme="dark"]` 覆盖所有 color 变量；浅色为默认；切换实时生效无闪烁（FOUC 用 inline `<script>` 在 `<head>` 内提前应用 stored theme）
- [ ] 主题切换按钮三态：system / light / dark；选 system 时跟随 `prefers-color-scheme` 媒体查询
- [ ] localStorage key `omnitoken.theme`（独立命名空间，不撞 metapi）
- [ ] Toast 组件：自动 4s 消失（hover 暂停），可堆叠最多 3 条，含 4 种 kind 样式
- [ ] Modal 组件：键盘可访问（Tab 焦点陷阱、ESC 关闭、初始焦点落在第一个可聚焦元素）、点击背景关闭、`aria-modal="true"` + `role="dialog"`
- [ ] 现有 6 个 view 全部在 dark theme 下视觉无破损（手测一遍 overview 图表、users 表格、credentials modal、audit 列表）
- [ ] 移除现有 view 里所有 `alert()` / `confirm()` 调用，全部走 toast/modal
- [ ] `web/src/views/*.test.js` 既有测试不破坏（如有断言依赖 DOM 结构需同步更新）
- [ ] **无新增 npm/构建依赖**；`web/` 仍可用 `python -m http.server 3000` 直接 serve
- [ ] commit message 体里注明 "Design tokens inspired by metapi (MIT) — github.com/cita-777/metapi"

**不在范围**（L2/L3 留独立任务）:
- ❌ Tailwind / React / Vite / TypeScript / 任何构建工具
- ❌ 移动端响应式（MobileDrawer / ResponsiveFilterPanel 等 L2 组件）
- ❌ 新增页面（ModelTester / Monitors / ProgramLogs 等）
- ❌ 图表库（@visactor/react-vchart 等）；overview 现有图表保持现状或最多换色不换实现
- ❌ i18n 框架（双语切换）
- ❌ 全局 `⌘K` 搜索 / 通知中心 / DiceBear 头像 等高级特性
- ❌ 修改 admin / gateway 后端代码

**依赖**: 无（纯前端任务，可任意时间起）

**Hints**:
- **MIT attribution**: metapi LICENSE 是 MIT，借鉴 CSS 设计 token 与组件形状不需要复制源代码本身。commit message 提一句出处即可，**不需要**复制 metapi 整份 LICENSE。
- **关键参考文件**:
  - `metapi/src/web/index.css` 头部约 1-70 行 — design tokens 含 dark theme，直接抄结构与命名（color/radius/shadow/topbar-height 等命名风格）
  - `metapi/src/web/components/Toast.tsx` — Toast Provider + useToast hook 的 React 实现，**思路**可借鉴（容器 + 自动消失 + 堆叠），**实现**必须改成 vanilla JS（DOM API + closure，不用 React）
  - `metapi/src/web/components/CenteredModal.tsx` — 居中 modal 的实现思路（焦点陷阱、ESC、aria 属性），同上需改 vanilla
- **FOUC 防闪烁**: theme 切换的关键技巧——在 `<head>` 内放一段 inline `<script>` 先读 localStorage 再 `document.documentElement.setAttribute('data-theme', ...)`，必须在 `styles.css` 之前执行。否则刷新会先闪一下浅色再切到 dark。
- **不要把 `styles.css` 重写成一个新文件**：在原 file 上 incremental 改，保留现有 CSS class 命名（views 已经在用），仅把硬编码色值替换成 `var(--color-*)`。这样能控制 diff 大小，review 更快。
- **测试方法**: 跑 `cd web && python -m http.server 3000` 然后浏览器开 `http://localhost:3000/?admin=http://localhost:8081`，分别在 light/dark 下点过 6 个 tab。可在 PR 里贴 2 张截图（light + dark）证明视觉无破损。
- **Docker-only 范围说明**: 本任务纯前端，不动 PG/Redis/NATS；review 时若要端到端验证（用 console 调一次 chat 看 toast 提示），仍走 `make up` 起后端，不要本地装 PG/Redis。
- **review 单 commit 还是拆**: 推荐拆两个 commit —
  - C1: design tokens 重构 + dark theme + FOUC 防护（纯 CSS/HTML，零功能改动）
  - C2: Toast + Modal 组件 + 6 个 view 替换 alert/confirm
  方便 review 时第一个 commit 看视觉，第二个看行为。一次性大 commit 也接受，但 review 会要求逐块解释。

**参考**:
- 上游灵感：`github.com/cita-777/metapi` (MIT) — 仅 `src/web/index.css` + `src/web/components/Toast.tsx` + `src/web/components/CenteredModal.tsx`
- 本地基线：`web/styles.css` (899 行) / `web/src/app.js` (114 行) / `web/src/views/*.js` (6 个 view, 共 ~1100 行)
- README 已说明 web console 启动方式：`README.md` § "Open the web console"

---

## T-AUDIT-USAGE-VIEW Audit Tab 加用户使用流水（上线门 ③） [phase:v1-release] [owner:codex] [status:done]

Start: 2026-05-23 17:16 +08:00

**目标**: 把 audit tab 拓展为双 tab：「管理操作流水」（现有 audit_logs 视图）/「用户使用流水」（**新**，按用户聚合 usage_events）。后者展示每个用户主要使用了哪些模型、什么时段、最近调用了什么。**上线门 ③** —— 用户已明确这是 release-gate 要求。

**背景**: 数据已经齐全（`usage_events.user_id` 在 000006 加列，`model_routed` 在 000012 加列，token breakdown 在 `usage_token_breakdown`），但 admin API 没暴露 per-user 聚合 endpoint，UI 也没视图。

**涉及**:
- 后端
  - `cmd/admin/main.go` —— 新增 `GET /api/admin/users/{user_id}/usage`（或 `/api/admin/usage/by-user/{user_id}`，命名 Codex 自决）
  - SQL 聚合查询：`usage_events JOIN usage_token_breakdown` on user_id，分别返回
    - `model_top`: model_routed → total_tokens / call_count，按 tokens DESC 取前 N（建议 N=10，可 query param 调）
    - `hourly_distribution`: 24 小时×日历日的二维 heatmap 输入，或者退一步只给"过去 7 天 hour-of-day 24 槽位"的一维分布（建议先做一维，复杂度低）
    - `recent_calls`: 最近 50 条（含 created_at / model_routed / status_code / total_tokens / streaming），按 created_at DESC
  - RBAC：admin + viewer 都可读（沿用 audit-logs 的策略，read-only 操作 viewer 不阻塞）
  - 时间范围：默认本月（与 overview 一致），支持 `since` / `until` query param（RFC3339）
- 前端
  - `web/index.html` —— audit pane 顶部加 tab 切换 UI（两个 button-like tab）
  - `web/src/views/audit.js` 重构或拆分：保留 admin-write 视图，加 user-usage 视图
  - 新建 `web/src/views/audit_usage.js`（或合并到 audit.js 看 file 体积）—— 用户使用流水的渲染、模型 top 表 + 小时分布柱状图（用现有 Chart.js）+ 近 N 次详情表
  - "用户使用流水" 入口的交互：默认展示 users 列表（用户名 + email + 月度 tokens）；点击某用户展开右侧详情（模型 top / 小时分布 / 近 50 次），或下拉切换用户
  - 复用 design tokens（待 T-UI-L1-THEME 落地后；如果先做本任务，先用现有色值，theme 任务落地时再统一替换）

**接受标准**:
- [ ] 后端 endpoint 返回 JSON envelope 含三个字段：`{ "user_id", "period", "model_top": [...], "hourly_distribution": [24 个 number], "recent_calls": [{...}] }`
- [ ] SQL 单查询或最多 3 个查询合成响应；避免 N+1
- [ ] 聚合按 `model_routed`，**不是** `model_requested` —— 用户实际命中的真实模型才是"主要使用场景"
- [ ] 时间范围默认 `period = current_month`；`since`/`until` 覆盖时返回真实窗口
- [ ] RBAC：admin 与 viewer 均可读；非登录态 401；session 失效 401
- [ ] 单测：`cmd/admin` 至少 3 个测试（happy path / 空用户 / 时间窗 filter）；coverage 不退化
- [ ] 前端 audit pane 默认开"管理操作流水"（保持现有行为，向后兼容），切换到"用户使用流水"加载新数据
- [ ] 用户使用流水的"用户选择"交互至少能用：列表点击 or 下拉切换
- [ ] 模型 top 表展示模型名 / token 数 / 调用次数 / 占比百分比
- [ ] 小时分布渲染为柱状图（24 槽位 0-23 时），空槽位也要显示零柱
- [ ] 近 N 次详情表至少含 5 列：时间 / 模型 / 状态码 / tokens / 流式标记
- [ ] 视觉上与现有 overview / users 视图风格一致（不要引入新组件库）
- [ ] 现有 `web/src/views/audit.test.js` 不破坏，必要时同步更新
- [ ] **Docker-only**：所有 Go test 在 docker 内跑（`docker compose --env-file .env -f deploy/docker-compose.yml run --rm test ...` 或 `make test`）；Codex 在 PR 描述中显式自报"已在 docker 内跑通"

**不在范围**:
- ❌ 虚拟 Key 维度的贡献分析（用户讨论已排除）
- ❌ 时段热图二维（周×小时）——本任务只做一维（hour-of-day）
- ❌ 导出 CSV（与 overview 看报扩展一起留 v1.1）
- ❌ 单用户级别的告警 / 阈值配置
- ❌ 与 `audit_logs` 表合并查询（两类数据语义不同，分开看更清晰）
- ❌ 后端引入新依赖（继续 sqlc/pgx 既有路径）

**依赖**: 无强依赖。**软依赖** T-UI-L1-THEME（如果 T-UI-L1-THEME 先落地，本任务的新组件直接吃 design tokens；如果本任务先落地，theme 任务那边把新组件一起搬到变量）

**Hints**:
- **SQL 聚合查询提示**: 
  - model_top: `SELECT u.model_routed, SUM(t.prompt_tokens + t.completion_tokens + ...) AS tokens, COUNT(*) FROM usage_events u JOIN usage_token_breakdown t ON ... WHERE u.user_id = $1 AND u.created_at >= $2 GROUP BY 1 ORDER BY 2 DESC LIMIT $3`
  - hourly_distribution: `SELECT EXTRACT(HOUR FROM created_at AT TIME ZONE 'UTC') AS h, COUNT(*) FROM usage_events WHERE user_id = $1 AND created_at >= $2 GROUP BY 1` —— 注意时区，建议**统一存储侧用 UTC，展示侧 web 用浏览器时区做小时偏移**（Codex 实施时确认或自报）
  - recent_calls: 直接 `SELECT ... ORDER BY created_at DESC LIMIT 50`
- **路径选择**：endpoint 路径建议 `/api/admin/users/{user_id}/usage` —— 与已有 `/api/admin/users/{user_id}/quota` 平行。**不要**用 `/api/admin/usage/by-user/...`，避免引入新顶层资源
- **frontend tab 切换 UI**：metapi 的 ResponsiveBatchActionBar 之类组件不需要引入；用两个 `<button>` 模拟 tab 即可，class 切换 active 态，纯 vanilla
- **小时分布柱状图**：复用现有 Chart.js（overview 已经在用，无需新依赖），type:'bar'，24 个柱
- **用户选择 UX**：第一版**推荐下拉**（简单）而不是抽屉/列表选中。Codex 可以自决，但 commit body 说明理由
- **测试**: 后端 happy path 用 seed 用户 `00000000-0000-0000-0000-000000000201`；空用户造一个新建 + 不调用 gateway；时间窗 filter 造一条早 1 天的数据测过滤
- **Review 单 commit 还是拆**：推荐拆两个 commit —— C1 后端 endpoint + sqlc + unit test；C2 frontend tab 切换 + 视图 + Chart.js 整合。或单 commit 看体积

**参考**:
- 现有 admin handler 模式：`cmd/admin/main.go` 的 overview / users / audit-logs 三个 handler
- `migrations/000006_usage_events_user_id.up.sql` 已加 user_id 列与 `(user_id, created_at)` 复合索引
- `migrations/000012_upstream_credentials_v1.up.sql` 加 model_routed 列；T-CONC-COST-ATTR 已切到这一列
- `internal/usage/` 既有 usage_events 写入路径，参考它的字段命名
- 前端基线：`web/src/views/audit.js` (159 行) / `web/src/views/overview.js` (203 行，含 Chart.js 用法)
Result: `7b9b0653` + `57775cae` — cmd/admin 67.4%; Docker vet/test + web view tests all green, no undeclared deviation; browser smoke skipped because in-app browser was unavailable.
