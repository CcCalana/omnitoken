# REVIEW.md — OmniToken Review Log

> **归档说明**:
> - R-001 ~ R-007 → `docs/reviews/archive.md`
> - R-006b-prop / R-006b / R-006c / R-006d → `docs/reviews/archive-2026-05-12.md`
> - R-008 ~ R-005b-fix (Phase 2-A 收官 + Phase 2-B 全程) → `docs/reviews/archive-2026-05-19.md`
> - R-INT / R-041-prop ~ R-040 (v1 联调收官 + Phase 3-A Adapter 链路) → `docs/reviews/archive-2026-05-20.md`
>
> 本文件保留最近 review；老的归档到 docs/reviews/。

## R-016 (T-016 实施, impl `c6ee841d` + e2e `8544ce82` + status `9a219c2a`)

**结论: `[+] Approved`** — T-016 接受标准 12/12 全部达成，R-016-prop 留的 4 条债（H-5/M-24/M-25/N-15）+ R-EXT-2026-05-21 折进来的 T-NIT-SSE-CLOSE 全部落地且都有显式断言，无 CRITICAL/HIGH/MEDIUM。3 NIT 不阻塞。v1 最关键的"性价比资源 = 多 upstream key 池"角到位，§零A 第 1 条落地完成。

**正面信号**:
1. ✅ **R-016-prop H-5 教科书级实现**：`copyStreamingResponse:384-386` 在 `n>0` 时 `result.final = true`，`doWithRetries:278` 重试条件包含 `!result.final` —— n>0 即标 final, 无论是否伴 err 都不切。`retry_test.go:97-126 TestArkChatProxyDoesNotRetryPartialFirstRead` 用 mock 返回 `(5 bytes, io.ErrUnexpectedEOF)` 正面断言 `transport.calls == 1`、`body == "part\n"`，三个 assertion 全部命中 R-016-prop H-5 的原文要求。
2. ✅ **T-NIT-SSE-CLOSE 三分支全对称**：`readWithIdle` pre-flight ctx.Done (`proxy.go:433-436`) / 阻塞中 ctx.Done (`456-458`) / timer 触发 (`459-462`) 三个退出分支统一 `_ = body.Close()`。两条 goroutine 回归测试 (`retry_test.go:128-184`) 用 `runtime.NumGoroutine() <= before+2` 在 100ms 内回归基线 —— 完整对齐 T-NIT-SSE-CLOSE 接受标准。
3. ✅ **M-24 / M-25 ops 文档极简但精确**：`docs/operations/master-key-rotation.md` 20 行覆盖两个核心问题——v1 不 unlink/zeroize 取舍 + 4 步 production rotation 流程 + startup-only reload 模型；`deploy/docker-compose.yml` 在 `credential-seed` 与 `gateway` 两处加注释强调"restart gateway after seed change"，运维不会踩坑。
4. ✅ **N-15 双重防漏 Ark body**：`logCredentialRetry` 只输出 `credential_id/attempt/code/upstream_status`，不附响应体；`TestLogCredentialRetryOmitsUpstreamBody` 直接断言 `not Contains("quota_owner")`；`TestArkChatProxyRetries429WithNextCredential` 在 mock 上游 429 body 里塞 `{"quota_owner":"must-not-leak"}` 后再断言响应体不漏 —— log 路径 + client 响应路径都被锁死。
5. ✅ **`WithUpstreamCredentialRecorder` 的指针式设计**：`httpx/virtual_model.go:36-63` 用 mutex 包裹可变 id 解决 "ctx 不可变 vs retry 后归因要更新" 的痛点，proxy retry 切换时 `SetUpstreamCredentialID` 改指针指向值，middleware 收尾读到最终成功的 credential id — 归因到"成功 credential"是正确的语义（429 失败 credential 走 WARN log 而非 usage_events）。`credential_pool_e2e_test.go:198` 用真 DB 聚合 `upstream_credential_id` 验三把 key 全部出现，端到端断言。

**T-CONC-COST-ATTR 合并完成**:
- migration `000012_upstream_credentials_v1.up.sql:32-33` 把 `usage_events.model_routed text NOT NULL DEFAULT ''` 合并到 T-016 同一 migration，无新增 0013（达成 R-016-prop 实现 notes 要求）。
- `cmd/admin/main.go:695/819/831` 三处 SQL 一并切 `COALESCE(NULLIF(ue.model_routed, ''), NULLIF(ue.model_requested, ''), 'unknown')`；`cmd/admin/main_test.go:715/921` 测试断言同步更新。
- `cmd/gateway/main.go:273-278` Decision 1（ctx key 新增 `WithModelRouted` 不动 `WithVirtualModel` 语义）落实，虚拟路径写 `RealModel` / 非虚拟路径写 `modelRequested`，符合 T-CONC-COST-ATTR propose 决策 1 推荐方向。
- **T-CONC-COST-ATTR 任务体可关，无单独实施 commit 必要**。建议 TASKS.md 把它合到 T-016 done 速查表，状态标 ✅。

**N-16 (NIT)**: `internal/credentials/store_postgres.go:135` `loadCredentialsSQL` 硬编码 `WHERE provider = 'ark'` 是 v1 范围正确取舍（任务体明确"多 provider 推 v1.1"），但没注释。多 provider 启动时容易当成 bug 排查很久。可加一行注释 `-- v1: ark-only; expand here for multi-provider`。

**N-17 (NIT)**: `cmd/gateway/main.go:107-110` 当 master key 加载失败但 `cfg.Ark.Enabled()` 时静默 fallback 到 `OMNITOKEN_ARK_API_KEY` 单 key 路径，log level 是 Warn。这在生产环境是合理的 degraded mode，但日志中没说明 "for which reason"（key file missing vs hex decode failed vs invalid length）。可把 LoadMasterKey 的 err 也带进 Warn (现有 line 111 已带，line 108 没带)，让运维一眼分清是哪种失败。

**N-18 (观察, 不立任务)**: usage 归因到"最终成功 credential"是正确语义，但运维想知道"哪把 key 触发 429"只能看 WARN log（grep）。如果 v1.1 admin UI 引入"按 credential 健康度排序"视图，可考虑加 `usage_events.upstream_credential_attempts jsonb` 或独立 `credential_retry_events` 表。**纯设计选项，不预判 v1.1 路线**。

**与 R-EXT-2026-05-21 闭环**: 外部专家 #2（SSE goroutine 泄漏 OOM CRITICAL）被 R-EXT 降级为 NIT 后通过 T-NIT-SSE-CLOSE 完成兜底；专家 #3（单 key 17.1% 成功率）通过本 T-016 完整解决；专家 #4（model_routed 归因）通过 T-CONC-COST-ATTR 合入解决；专家 #6（KMS 主密钥）的 v1 部分通过 Decision 1 file-first + ops 文档解决，KMS 留 vNext。本轮路线判断（"3 命中已跟踪 / 1 NIT / 1 推测降级 / 1 读错代码"）经实测验证全部到位。

---

## R-016-prop (T-016 PROPOSAL, commit `fd9ce8d8`)

**结论: `[+] Approved`** — 5/5 决策直接答了 propose 问题，方向全部采纳。Codex 可开 T-016 实施。1 HIGH + 2 MEDIUM + 1 NIT 是实施期的细节边界，不阻塞 propose 关门。

**正面信号**:
1. ✅ Decision 4 主动核了现有 proxy 行为并据实写进 PROPOSAL：`current proxy already delays downstream header write until after the first SSE read succeeds`。核对 `internal/proxy/proxy.go:293→301-303` 属实（`readWithIdle` 先读，`n>0 && err==nil` 才 `WriteHeader`）。这意味着 T-016 SSE 重试状态机改动量比预期小，retry 包在 read-buffer-first 这段之前/中即可。
2. ✅ Decision 1 给的 "file-first + env-fallback" 直接命中我 propose 的 docker secret 方向，同时保留 env 给单测/local smoke，没多写 KMS 范围。`reads once at startup, validates length, keeps only the decoded bytes in memory` + "logs/error 不打路径内容/key value/前缀"两层 guardrail 到位。
3. ✅ Decision 2 显式拒绝 SIGHUP 给出 "Linux-centric / Windows dev host 难测" 理由，与 AGENTS.md §3.3a 测试环境约束一致；admin CRUD UI / SIGHUP / PG NOTIFY 全部清晰推到 v1.1，不在 T-016 范围越权。
4. ✅ Decision 3 分层做得干净：hot-path 内存 degradation（30s TTL, WARN log + credential_id only）vs DB `status` / `health_state`（operator-controlled, persisted）。拒绝指数退避给出 "2-3 keys, 过度 quarantine 风险" 理由，是 v1 规模下的正确取舍。
5. ✅ 实现 notes 主动提 "T-016 + T-CONC-COST-ATTR 合并 migration"（任务体接受标准早有要求，证明 Codex 真读了）+ "expose deterministic clock in tests so 30s degradation can be asserted without sleeps"（工程素养，避免单测 flake）。

**H-5 (HIGH, 实施期边界)**: Decision 4 SSE retry 状态机没明确 **partial-first-read 情形**——即首次 `readWithIdle` 返回 `n>0 && err != nil`（拿到部分字节但同时报错）。按 `proxy.go:294` 现有判断 `if err != nil && n == 0` 才回错，n>0 即使 err != nil 也会继续走 line 301-303 写 header + 发 partial bytes 给客户端。这种情况下**不能切 credential**（已经动到客户端），属于 PROPOSAL 第 5 步 "once any chunk written, never switch" 的隐含分支。实施 retry 状态机时必须显式：**n>0 即标 final，无论是否伴随 err**。retry_test.go 加一条断言：mock upstream 首 read 返回 `(5_bytes, io.ErrUnexpectedEOF)` → 不切，发出 partial bytes，标 final 返回。

**M-24 (MEDIUM)**: Decision 1 没说 "读完 master key file 后是否 unlink/zeroize 进程内存"。v1 不强求 unlink（docker secret 是只读 tmpfs 挂载，重启还要读，主动 unlink 反而坏运维），但要在 **`docs/operations/master-key-rotation.md`**（任务体已要求新增）写明 v1 取舍："依赖 docker secret 只读挂载 + 进程内存生命周期即可，不主动 unlink；rotation 走 secret 重挂 + gateway restart"。这一句不展开 KMS 是合理的，但必须写。

**M-25 (MEDIUM)**: Decision 2 "startup load into memory, restart-based reload" 意味着 ops 流程是 "seed/migration 添加新 credential → restart gateway 才生效"。要在 **`deploy/docker-compose.yml` 注释 + `docs/operations/master-key-rotation.md` 或单独的 credential ops 文档**写明这一点，避免运维以为改了 DB 就生效。

**N-15 (NIT)**: Decision 3 WARN log 写 "credential id only"，但要在实现时核对：429 切换路径的日志/error envelope **不附带 Ark 上游响应 body**（Ark 429 body 可能含运维联系信息，偶发包含 quota_owner 等可识别字段；与 §十一安全基线一致，不漏到客户端）。

**与外部专家分析（R-EXT-2026-05-21）的交叉**:
- Decision 1 直接对应外部专家 #6（KMS / docker secret 优于裸 env），结论一致。
- Decision 4 SSE retry 边界对应外部专家在 #3 提的 "流式失败重试要在第一 chunk 之前完成"，结论一致。
- 外部专家 #2（SSE goroutine 泄漏 OOM）已在 R-EXT-2026-05-21 降级为 NIT，对应 **T-NIT-SSE-CLOSE 顺带项**（在我刚加进 T-016 任务体的接受标准里，Codex 写 PROPOSAL 时还看不到）。Codex 在实施 T-016 proxy retry 改动时一起做：补 `case <-ctx.Done():` 分支 `_ = body.Close()`，加一条 goroutine 回归断言。

**Codex 下一步**: T-016 可开实施。propose 关门，5 个 propose 决策点视为已锁定；H-5 / M-24 / M-25 / N-15 在 implementation review (R-016) 时核。注意：T-016 任务体在 propose 之后追加了 **T-NIT-SSE-CLOSE** 顺带项（见 TASKS.md 接受标准），是 R-EXT-2026-05-21 外部专家分析触发的，一起做即可。

---

## R-EXT-2026-05-21 外部专家分析核验（analysis_results.md, Gemini brain）

**结论: `[+] 6 条诊断 → 3 命中已跟踪 / 1 NIT 顺带做 / 1 推测降为观察项 / 1 读错代码`**。专家给的优先级表与我们的 v1 路线**不冲突也不需要调整 ETA**。已在 TASKS.md 新增 1 条观察任务 + 1 条 T-016 顺带 NIT。

**正面信号**:
1. ✅ M-23 model_routed 成本归因脱节 — 与 R-CONC-CHECK M-23 一致，方向校准了。
2. ✅ H-3 DSN application_name 失效 — 与 R-CONC-CHECK H-3 一致。
3. ✅ T-016 单 key 17.1% 成功率问题诊断与 ADR 0003 一致，且补提了"SSE 首 chunk 之前完成重试"细节（T-016 propose 决策点 4 已覆盖）。

**核验记录（专家 6 条 vs 实际代码/数据）**:

| # | 专家声称 | 核验 | 结论 |
|---|---|---|---|
| 1 | quota SQL 双 LEFT JOIN 在 1000 RPS 下 PG CPU 100% → CRITICAL | SQL 现状属实（`internal/quota/store_postgres.go:48`），但 R-CONC-CHECK 50×50 实测 gateway **0 panic / 0 5xx**，瓶颈是 Ark 429 不是 PG。1000 RPS 是推测 | 降为 vNext 观察项 **T-QUOTA-CACHE-PROBE**（先量再优化） |
| 2 | SSE `readWithIdle` ctx.Done 分支未关 body → goroutine 泄漏 OOM CRITICAL | 读错代码：外层有 `defer resp.Body.Close()` (`proxy.go:175`)；`resultc` 是 buffered size 1 (`proxy.go:356`)；net/http transport 在 ctx 取消时也会关 body。压测 2500 请求 0 OOM | 降为 NIT，作为 **T-016 顺带项**：补 `case <-ctx.Done():` 分支显式 `body.Close()` 做对称清理 |
| 3 | 单 key 429 占 82.9% | 与 R-CONC-CHECK H-4 一致 | 已在 T-016 in-progress |
| 4 | model_actual 污染成本归因 | 与 R-CONC-CHECK M-23 一致 | 已在 T-CONC-COST-ATTR todo |
| 5 | DSN 无 application_name | 与 R-CONC-CHECK H-3 一致 | 已在 vNext T-CONC-DSN |
| 6 | 主密钥 env 变量泄漏风险 | 与 T-016 propose 决策点 1 一致（env vs docker secret file） | 已在 T-016 propose 待 Codex 拍板 |

**N-14 (NIT)**: 专家在 #1 给出的 Redis + Lua 原子扣减方案细节是有价值的设计参考，但**违反"底座先做最简"原则**。如果未来 T-QUOTA-CACHE-PROBE 量出 PG 真是瓶颈，把这段方案搬进对应 ADR 即可，**当前不立 ADR**。

---

## 未解决项摘要（从所有 review 累积）

| ID | 来源 | 严重度 | 描述 | 状态 |
|----|------|--------|------|------|
| M-13 | R-006a | MEDIUM | gateway 硬依赖 DB，未来考虑 `--auth=stub` | Informational, 留 T-005c |
| M-14 | R-006a | MEDIUM | CreateVirtualKey 不开事务 | Informational, 留 T-005b |
| M-16 | R-008 | MEDIUM | body double-read (1MiB) | Informational, Phase 2 优化 |
| M-17 | R-008 | MEDIUM | SQL fallback 双重防御 | Informational, 设计正确 |
| M-18 | R-006b | MEDIUM | overview 3 条 query 非事务 | Informational, Phase 2 |
| M-19 | R-010 | MEDIUM | 503 admin_auth_not_configured 在生产部署中可能成隐患（默认不放行正确，但运行期需 alerting） | Informational, Phase 2 alerting |
| H-3 | R-CONC-CHECK | HIGH | DSN 未拼 `application_name` → `pg_stat_activity` 采样失效 | 留 vNext T-CONC-DSN |
| H-4 | R-CONC-CHECK | HIGH | 50×50 真 Ark 17.1% 成功率被上游 429 吃了，gateway 真实承载 baseline 未拿到 | 拉回 v1 起 T-CONC-RERUN（与 T-016 同期） |
| M-23 | R-CONC-CHECK | MEDIUM | admin overview 按 `model_actual` 聚合 → Ark backend 名污染成本归因 | ✅ 并入 T-016 (`c6ee841d`) |

---

## R-CONC-CHECK (T-CONC-CHECK 实施, 报告 `04fff8a7`, status `c6c4262f`)

**结论: `[+] Approved with follow-ups`** — 任务形式上完成：报告交付 + V2 candidates 主动列 3 条 + 严格"不修代码"。但**baseline 数据不完整**且**抓到底座成本归因隐患**。T-CONC-CHECK 本身可关；需起 3 个 follow-up（M-23 / H-3 / H-4），其中 M-23 阻塞 Phase 3-A 商业化场景，建议先开。

**正面信号**:
1. ✅ Codex 严格守"只测量不修"边界：所有发现写进报告 §V2 Candidate Fixes，未单边改 DSN / 加索引 / 调连接池 —— 任务体接受标准末条达成。
2. ✅ Preflight 设计严谨：跑前先打 1 发 chat-fast 看实际下游模型，**抓出了 `virtual_models` 表 vs `usage.model_actual` 的不一致**（实质是 Ark backend 模型名披露，gateway 转发逻辑经 `cmd/gateway/main.go:235` + `internal/usage/parser.go:38,41` 证实正确）；同时跑前清零 demo admin 的 monthly budget，避免 quota 抑制 mask 上游真实行为，实验设计强。
3. ✅ Gateway 自身韧性: 50 并发 × 50 真 chat 跑下来 **0 panic / 0 5xx / 0 timeout / 0 client error**，428 成功请求 P95 1.798s / P99 2.415s（含 Ark 上游往返）—— `internal/proxy` SSE 反代 + budget/auth/audit 中间件全链路在压力下没出 race / 没崩。
4. ✅ 报告透明披露所有偏差：vegeta 不可用 → 临时 Go driver（已标出）/ DB sampling filter 失效（peak=0）→ 主动说明"not proof of zero connections"；healthz 实际 996.2 RPS vs 配置 1000 RPS 也照实记。无"成功率掩饰"的迹象。
5. ✅ V2 candidate fixes 三条都精准对症：upstream-aware load profile（解 H-4）/ DSN `application_name` 显式设置（解 H-3）/ loadtest summary 加 429 计数（提升可观测性）—— 不是泛泛的"以后再说"，是可直接立任务的清单。

**M-23 (MEDIUM, 升级建议)**: `model_actual = deepseek-v4-pro` ≠ gateway 重写后 `kimi-k2.6` —— 经查 `cmd/gateway/main.go:235` (`payload["model"] = res.RealModel`) + `internal/usage/parser.go:38` (`ModelActual = response.Model`)，**gateway 转发逻辑正确**，这是 Ark 上游响应里自报的 backend 模型名（与 memory `project_omnitoken_ark_coding_plan` 中"5 模型共用一把 key"的设计契合）。**但**：`cmd/admin/main.go:667/791/803` admin overview 全部按 `model_actual` 聚合成本/请求数 → 用户问"我用了多少 kimi-k2.6"会被答"deepseek-v4-pro"。**这是 OmniToken 底座"性价比资源"角的成本归因路径污染**。建议起 **T-CONC-COST-ATTR**：(a) 复现 Ark 行为并补 ADR 记录预期；(b) `usage_records` 加 `model_routed`（gateway 转发出的模型名，从 `httpx.WithVirtualModel` ctx 取）作为归因 ground truth；(c) admin overview 默认按 `model_routed` 聚合，`model_actual` 保留供审计。**建议 Phase 3-A 之前做**。

**H-3 (HIGH)**: 任务体接受标准第 3 项"DB 连接峰值"形式上未达成（filter `application_name LIKE 'omnitoken%'` peak=0），Codex 已透明说明。建议起 **T-CONC-DSN**：在 `cmd/gateway/main.go` / `cmd/admin/main.go` 的 DSN 构造处显式拼 `application_name=omnitoken-gateway` / `application_name=omnitoken-admin`，并在 `cmd/loadtest/README.md` 把采样 SQL 一并写好。**不阻塞 Phase 3-A**，但底座可观测性短板要补，建议穿插做。

**H-4 (HIGH)**: 2500 真跑 83% 是 Ark 429 → **v1 真实并发上限 baseline 实际上没拿到**。任务体目标"测 v1 真实并发上限"被 Ark rate limit 吃了。Codex 结论"primary bottleneck is Ark upstream rate limiting"诚实，但用户层面"v1 上线后能扛多少 RPS"这个问题没答案。建议起 **T-CONC-RERUN**：(a) mock upstream 跑 50 并发 / 100 并发 / 200 并发 各 1 分钟，量 gateway 真实承载；(b) 真 Ark 跑低 RPS 长时间（如 3 RPS × 600s = 1800 请求，成本约 9 RMB），定位 Ark 429 阈值。**Phase 3-A 启动不强依赖此数**（Agent 适配是离线配置写入，不打 Ark）—— 可推到 v1 真实流量进来前做。

**N-13 (NIT)**: 报告第 30-36 行解释 `model_actual = deepseek-v4-pro` 那一段语义偏弱（"routing target evidence is the virtual_models row"），未点透"这是 Ark backend 模型名披露"。可在后续报告版本加一句"Ark coding plan 5 模型 backend 推理可能共用 —— 这不是 gateway bug"。
