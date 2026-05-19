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

## T-CONC-CHECK v1 并发摸底（不修代码）[phase:2-B 后置] [owner:codex] [status:todo]

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

---

## T-MK-RACE Makefile: race 验证移入 Docker [phase:infra] [owner:codex] [status:in-progress]

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
