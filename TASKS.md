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
| 05-30 | **T-044 impl (`8fb054e`) → R-044 Approved**。10 文件 1044 行：virtual_models CRUD API + RBAC + audit + `omnitoken-adopt --admin-url` auto-ensure + 前端 edit/toggle。Phase 3-A 6/7 done，T-046 解锁 |
| 05-30 | **T-046 任务体写好**（status:todo）。交互式 prompts + status + dry-run + restore 确认 + 错误 polish。Phase 3-A 最后一块拼图 |
| 05-30 | **T-046 impl (`00c9f4a`) → R-046 Approved**。638 行 CLI 层改动：交互式 prompts + status 三 agent + dry-run + restore 确认 + 中文 actionable 错误。**Phase 3-A 7/7 收官** |
| 05-30 | **T-FIX-AUTH-XAPIKEY (`9958f47`)**：`RequireVirtualKey` 加 `x-api-key` fallback + `/v1/messages` 路径返回 Anthropic error 格式。Claude Code 请求此前因 auth header 不匹配被 401 阻塞 |
| 05-30 | **T-SMOKE-AGENT 任务体写好**（status:todo）。两层测试：集成测试（mock 上游 + 完整 middleware 栈，CI 必跑）+ e2e 测试（真上游 + agent 格式请求） |
| 06-01 | **服务器部署规划**：用户决定服务器部署测试（5-10 人、DeepSeek-only、暂无域名）。Claude 产出 runbook + nginx config + docker-compose.server.yml。T-DEPLOY 和 T-SMOKE-AGENT 任务体下给 Codex |
| 06-01 | **T-DEPLOY impl (`31adccf`) → R-DEPLOY Approved**。preflight + smoketest 脚本就绪，4 条 runbook 偏差（master-key 挂载/DB user/Codex URL/nginx -t）已修 |
| 06-01 | **T-SMOKE-AGENT impl (`9bb08f0` + `cd118c9`) → R-SMOKE-AGENT Approved**。519 行测试：5 条集成（mock + 真 middleware）+ 4 条 e2e（真上游 + agent 格式）。Claude Code + Codex 全链路验证闭环 |
| 06-01 | **T-UI-L1-THEME impl (`8c8790c`) → R-UI-L1-THEME Approved**。537 行纯前端：design tokens + dark theme + FOUC 防护 + Toast + Modal + 6 view alert/confirm 替换。零新依赖 |
| 06-01 | **vNext 稳定系统路线确定**：优先级 T-100 > T-QUOTA-CACHE-PROBE > T-017b > T-018。T-100 + T-QUOTA-CACHE-PROBE 任务体已写好下发 |
| 06-01 | **T-100 proposal (`959f47f`) → R-100-prop Approved w/ M-36**。3 决策：seed users + DeepSeek-only + direct PG ledger。M-36：runner PG 只读，member password pre-flight 文档化 |
| 06-01 | **T-100 impl (`8df224e`) → R-100 Approved**。676 行 runner + 67 行单测 + 30 行 e2e shell-out。M-36 PG read-only 严格落地；RBAC skip 语义正确；账本闭环 + 归因验证路径完整。M-37/M-38 2 MEDIUM 不阻塞 |
| 06-01 | **T-QUOTA-CACHE-PROBE impl (`94b4607`) → R-QUOTA-CACHE-PROBE Approved**。10/30/50 mock probe 报告落地；quota SQL 从 30c 起成延迟瓶颈，50c 仍 100% 2xx / 0 5xx。M-39/M-40 2 MEDIUM 不阻塞 |
| 06-01 | **T-017b 任务体下发**。3 缺口：model_catalog 资格校验 + 跨 provider 结构化日志 + 集成测试；propose 前置（3 决策点） |
| 06-01 | **T-017b proposal (`c498850`) → R-T017b-prop Approved**。3/3 决策采纳：in-memory catalog interface + validation-only + httptest mocks。1 NIT (empty routed provider) 留实施期 |
| 06-01 | **T-017b impl (`5fc1fdc`) → R-T017b Approved**。700 行变更：catalog guard + fallback log + selector diagnostics + 5 条集成测试。N-39 empty provider handled。proxy 87.0%。1 NIT (mixed state reason) 不阻塞 |
| 06-01 | **T-018 任务体下发**。6 场景故障注入：retry recovery + all-exhausted + cross-provider fault fallback + degrade/restore + SSE disconnect。propose 跳过 |
| 06-02 | **T-018 impl (`b70794b`) → R-T018 Approved**。443 行纯测试，6/6 场景全落地，3 条接入真 usage.Middleware。零生产改动。proxy 87.6%。零 issue |
| 06-02 | **T-019 任务体下发**。admin 用户创建 API + 前端新建用户 modal + virtual key 生成/展示/复制。propose 跳过 |
| 06-02 | **T-019 impl (`8148f34`) → R-T019 Approved**。788 行全栈：POST /api/admin/users + RBAC + 事务创建 + 前端 modal + Key 展示/复制。6 handler + 3 store 测试。admin 67.1%。零 issue |

---

### 部署收尾（2026-06-02）

| 优先级 | 任务 | 估时 | 成本 | 说明 |
|---|---|---|---|---|
| 1 | **T-019** Admin 用户与 Key 管理收口 | 1-1.5d | 零 | ✅ `8148f34` R-T019 Approved |

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

### 近期优先（2026-06-01）

| 优先级 | 任务 | 状态 | 说明 |
|---|---|---|---|
| 1 | **T-SMOKE-AGENT** | ✅ `9bb08f0` | 5 条集成 + 4 条 e2e，全链路 auth→proxy 覆盖 |
| 2 | **T-DEPLOY** | ✅ `31adccf` | 预检 + smoketest 脚本就绪；4 条 runbook 偏差已修 |
| 3 | **T-UI-L1-THEME** | ✅ `8c8790c` | design tokens + dark theme + Toast + Modal，纯 vanilla 零构建 |

### vNext 稳定系统路线（2026-06-01，优先级排序）

| 优先级 | 任务 | 估时 | 成本 | 说明 |
|---|---|---|---|---|
| 1 | **T-100** L2 多租户正确性套件 | 2-3d | ¥2-5/次 | ✅ `8df224e` R-100 Approved |
| 2 | **T-QUOTA-CACHE-PROBE** | 1d | 零（mock） | ✅ `94b4607` R-QUOTA-CACHE-PROBE Approved |
| 3 | **T-017b** cross-provider fallback retry | 2d | 零（mock） | ✅ `5fc1fdc` R-T017b Approved |
| 4 | **T-018** 故障注入 e2e | 1-2d | 零（mock） | ✅ `b70794b` R-T018 Approved |

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

## 已完成任务归档

> Phase 3-A (T-045/T-044/T-046) / v1 release (T-DEPLOY/T-SMOKE-AGENT/T-UI-L1-THEME) / v1 关键实施 (T-CONC-DSN/T-CONC-RERUN/T-MP-DEEPSEEK/T-016b-MIN/T-AUDIT-USAGE-VIEW) / vNext 已完成 (T-100/T-QUOTA-CACHE-PROBE/T-017b) 的任务体已移至 → `docs/tasks/archive-2026-06-02.md`

---

## T-018 Proxy 故障注入与恢复测试 [phase:vNext] [owner:codex] [status:done]

Start: 2026-06-02 00:15 +08:00
Result: `b70794b` — 6 scenarios, real usage middleware attribution, 87.6% coverage; all green; zero production change.

**目标**: 补齐 proxy retry/fallback 链路在**多步故障 + 恢复**场景下的集成测试覆盖。现有 `retry_test.go` + `proxy_test.go` 已覆盖单次故障（timeout / 502 / connection refused / 429 retry / stream idle），T-017b 加了跨 provider fallback 正常路径。缺失的是：retry 多步后成功、全部 credential 耗尽后的最终错误、跨 provider 故障 fallback、以及错误 envelope 格式在故障场景下的一致性。

**背景**: `规划.md` §10 定义 L2 包含"故障注入 e2e"，但 proxy 层的集成测试已经有 httptest-based mock 基础设施可做到这个——不需要 Docker。本任务聚焦在**故障在 retry loop 内的多步行为**，而非基础设施级 chaos（kill container 等留 v1.1）。

**涉及**:
- `internal/proxy/retry_test.go` — 新增故障注入测试
- `internal/proxy/proxy_test.go` — 可能补 error envelope 格式断言
- 不改 `internal/proxy/proxy.go` 或任何非测试文件

**测试场景**:

| # | 场景 | 注入 | 预期 |
|---|---|---|---|
| 1 | **Retry + 恢复**：cred-1 连接失败 → cred-2 成功 | cred-1 返回 `net.OpError`（关闭 server）；cred-2 正常 200 | 200 OK；usage 归因到 cred-2 |
| 2 | **全部耗尽**：3 个 credentials 全部 502，max retries=2 | 3 个 upstream 全返 502 | 最终 502 BadGateway + `CodeUpstream5xx`；WARN log 含 2 次 retry |
| 3 | **跨 provider 故障 fallback**：DeepSeek cred 全部连接失败 → Ark 成功 | 2 个 DeepSeek creds 连接失败；Ark 正常 | 200 OK；cross-provider fallback log + `reason=all_excluded` |
| 4 | **跨 provider 全部耗尽**：DeepSeek + Ark 全部 5xx | 所有 creds 502 | 最终 400（若 Ark 无 catalog match）或 502（若 Ark 有 match 但上游 502） |
| 5 | **Degrade + 恢复时序**：cred-1 429 → degraded 30s → 后续请求跳过 cred-1 → degrade 过期后 cred-1 恢复可用 | `MarkDegraded` + clock advance | credential 在 degrade 期内被跳过；过期后恢复参与选择 |
| 6 | **Stream 中途上游断开**：上游在发出第一个 chunk 后关闭连接 | mock 写 1 chunk → 关连接 | 客户端收到该 chunk；no retry（`final=true`）；gateway 不 crash |

**接受标准**:
- [ ] 6 条场景全部有集成测试，使用 `httptest.Server` + `credentials.Selector` mock，不走真上游或 Docker
- [ ] 每条测试有显式断言：(a) HTTP status code、(b) error code in response body、(c) WARN log 存在/内容（适用时）、(d) upstream call counts per credential（适用时）
- [ ] 场景 1/2/3/4 验证 Usage Recorder attribution：成功请求的 `api_key_id` / `upstream_credential_id` 指向实际发请求的 credential
- [ ] 场景 5 验证 `AvailabilityForProvider` 在 degrade 期/恢复后返回正确的 `Available` 计数
- [ ] 场景 6 验证 streaming `final` flag 阻止 retry（已有 `TestArkChatProxyDoesNotRetryPartialFirstRead`，本条聚焦 SSE chunk 发送后上游主动断连）
- [ ] `go vet ./...` + `go test ./internal/proxy/...` 全绿
- [ ] proxy 单测覆盖率不下降（当前基线 87.0%）

**不在范围**:
- ❌ Docker / docker-compose e2e（当前 `test/e2e/` 只有 `l2_test.go`，T-018 不做 compose-level chaos）
- ❌ 改 `internal/proxy/proxy.go` 或任何生产代码
- ❌ 改 selector 核心算法
- ❌ Kill gateway/admin container（v1.1 基础设施 chaos）
- ❌ PG/Redis/NATS 故障注入（v1.1）
- ❌ Circuit breaker / exponential backoff（v1.1 T-018-follow-up）

**Codex propose 前置**: **跳过**。场景列表 = 接受标准，纯测试代码，无生产改动。T-QUOTA-CACHE-PROBE 已建立 test-only skip-propose 范式。

**依赖**: T-017b ✅（selector diagnostics `AvailabilityForProvider` 可用于 degrade 恢复断言）；T-016 ✅（selector + multi-credential pool）

**参考**:
- 现有故障测试：`internal/proxy/proxy_test.go:299-457`（`TestArkChatProxyScenarios` 含 timeout/502/conn-failed 单次故障）
- T-017b 新增测试：`internal/proxy/retry_test.go:63-271`（`TestCrossProviderFallback*` 系列，httptest + selector 模式）
- Usage recorder test helper：`internal/usage/recorder_test.go`（如有 mock recorder 可复用）
- Error codes：`internal/proxy/proxy.go:25-32`

Result: `b70794b` — 6 proxy failure/recovery tests landed; proxy coverage 87.6%; all green.

---

## T-019 Admin 用户与 Key 管理收口 [phase:deploy] [owner:codex] [status:done] [started:2026-06-02 10:31 CST]

Result: `8148f34` — POST /api/admin/users + RBAC + transactional create + frontend modal + key copy; all green; admin 67.1%.

**目标**: 补齐 admin 控制台的用户创建 + virtual key 生成/下发链路，使 admin 可在网页上完成"新建用户 → 生成 key → 复制下发"全流程，不再需要手动 SQL 或 curl。

**背景**: 当前 `GET /api/admin/users` 可列用户、`PATCH /api/admin/users/{id}/quota` 可改额度，但缺少 `POST /api/admin/users` 创建端点。`POST /api/admin/dev/virtual-keys` 后端已有但无前端 UI。admin 创建用户需手动 INSERT SQL，生成 key 需手动 curl——这是部署后自助管理的最大缺口。

**涉及**:
- `cmd/admin/main.go` — 新增 `POST /api/admin/users` handler + `ActionCreateUser` RBAC action
- `internal/rbac/types.go` — 新增 `ActionCreateUser`，加入 `AllActions` + default policy（admin 可写）
- `web/index.html` — users tab 加"新建用户"按钮 + modal
- `web/src/views/users.js` — 加 create user + generate key 交互逻辑
- `web/src/api.js` — 可能需要加 `api.createUser` / `api.createVirtualKey` 方法（视现有 `api.post` 通用性）

**接受标准**:
- [ ] `POST /api/admin/users` 端点：接受 `{email, display_name, role, password}`，校验 email 格式 + role 合法性（admin/member/viewer），在同一事务内 INSERT users + INSERT role_assignments + INSERT password_hash（`crypt(..., gen_salt('bf'))`），返回 `{user_id, email, display_name, role}`
- [ ] RBAC：新增 `ActionCreateUser = "create_user"`，仅 admin 可调用。viewer/member 调 → 403 + audit_logs 记 forbidden
- [ ] users tab 加"新建用户"按钮 → 弹出 modal（email / display_name / role dropdown / password 四字段），提交后调 `POST /api/admin/users`
- [ ] 用户创建成功后，在用户行或 modal 内展示"生成 Key"按钮 → 调已有 `POST /api/admin/dev/virtual-keys` → 展示 full virtual_key + key_prefix，支持一键复制（`navigator.clipboard.writeText`）
- [ ] Key 展示区域含安全提示："请立即复制此 Key，关闭后不可再次查看"——不存 localStorage，不展示已生成的 key 列表
- [ ] audit_logs：创建用户 + 生成 virtual key 两次操作分别写入审计日志
- [ ] `make lint` + `make test` 全绿
- [ ] admin 包单测覆盖率不下降（当前需先查基线）

**不在范围**:
- ❌ 批量导入用户（CSV 等）
- ❌ 邮件/IM 下发 key（v1.1）
- ❌ Key 回收/轮换 UI（v1.1）
- ❌ 用户删除/禁用 UI（`PATCH /api/admin/users/{id}` 暂不做，status toggle 可后续加）
- ❌ Password 修改/重置（v1.1）

**设计约束**:

1. **复用既有模式**：参考 T-016b-MIN credential create handler 的结构——`parse → validate → store call → audit before/after → JSON response`。参考 T-044 `ON CONFLICT DO NOTHING RETURNING` 的幂等创建模式（email + organization_id UNIQUE 约束天然防重）。

2. **Password 安全**：`password_hash = crypt($password, gen_salt('bf'))` 走 pgcrypto，与既有 login handler 的 `crypt(password, password_hash) = password_hash` 校验一致。不存明文，不在日志/audit 中输出 password 或 hash。

3. **事务性**：user INSERT + role_assignment INSERT + password_hash UPDATE 在同一 `sql.Tx` 内完成。若 organization 只有一个（seed 默认 org），`organization_id` 可从 auth subject 或配置读取。

4. **前端风格一致**：modal 复用 `OmniTokenModal` 组件（`web/src/components/modal.js`），表单 field 命名与既有 users tab 一致。Key 展示区用 monospace `<code>` + copy icon，与 credential tab 的 key prefix 展示风格一致。

**Codex propose 前置**: **跳过**。模式明确（credential CRUD + virtual key create 均已实现），AC 完整，无架构决策点。

**依赖**: T-005a ✅（RBAC 引擎）；T-016b-MIN ✅（credential CRUD 参考实现）；T-044 ✅（virtual key 创建端点已存在）

**参考**:
- User schema：`migrations/000004_rbac_schema.up.sql`（users + role_assignments）
- Virtual key create：`cmd/admin/main.go:525` `POST /api/admin/dev/virtual-keys`
- Credential create handler 模式：`cmd/admin/main.go` `makeCreateCredentialHandler`
- RBAC actions：`internal/rbac/types.go:22-26`
- Modal 组件：`web/src/components/modal.js`（`confirmModal` 快捷方法）

Result: `8148f34` — cmd/admin 67.1% (HEAD baseline 66.4%); create-user + key handoff landed; all green; no undeclared deviation.

---
