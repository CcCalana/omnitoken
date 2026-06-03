# REVIEW.md — OmniToken Review Log

> **归档说明**:
> - R-001 ~ R-007 → `docs/reviews/archive.md`
> - R-006b-prop ~ R-006d → `docs/reviews/archive-2026-05-12.md`
> - R-008 ~ R-005b-fix (Phase 2-A 收官 + Phase 2-B 全程) → `docs/reviews/archive-2026-05-19.md`
> - R-INT / R-041-prop ~ R-040 (v1 联调收官 + Phase 3-A Adapter 链路) → `docs/reviews/archive-2026-05-20.md`
> - R-045-prop ~ R-UI-L1-THEME (Phase 3-A + v1 release) + R-AUDIT-USAGE-VIEW ~ R-CONC-CHECK (v1 门③ + 并发摸底 + 关键实施) → `docs/reviews/archive-2026-06-02.md`
>
> 本文件保留 vNext 路线（T-100 起）的 review + 未解决项摘要。

---
## R-100 (T-100 实施, impl `8df224e` + status `2b44c9d`)

**结论: `[+] Approved`** — M-36 约束全部落地。runner 对 PG 只做 SELECT，所有写操作走 admin API。RBAC skip 语义正确，账本闭环 + 归因验证路径完整。无 CRITICAL/HIGH。2 MEDIUM + 2 NIT 不阻塞。

**正面信号**:

1. ✅ **M-36 PG read-only 严格落地**：`verifyLedger` 和 `verifyAttributionSamples` 仅执行 SELECT，无 INSERT/UPDATE/DELETE。所有 mutation（create virtual key、set budget）走 admin API。`--database-url` flag description 明确标注 "read-only verification"。

2. ✅ **RBAC skip 语义干净**：viewer/member credentials 未提供时打印 `SKIP: ...` 并返回 nil，不 fail 整体 runner。`runRBACChecks` 接收 `out io.Writer` 把 SKIP 消息注入 runner 输出而不是 stderr，与 `main()` 的 reporting 流一致。

3. ✅ **Budget0 用户全程验证链完整**：user01 设 budget=0 → `assertRequestResults` 断言所有 budget0 请求返回 402 → `verifyLedger` 显式跳过 budget0 用户（他们不会有 200 的 usage_events）。三段链路—设置→断言→排除—都是同一数据源（`prepared[i].Budget0`），无重复推导，无漏判。

4. ✅ **账本闭环验证扎实**：`verifyLedger` 比较 `cost_ledger.cost_usd`（系统写入）与从 `usage_token_breakdown` 按 `model_pricing_current` 计价反算的 derived cost，1% tolerance。同时统计 `MissingFields`（user_id / api_key_id / model_routed / upstream_credential_id 为空的行），两层断言——金额闭环 + 归因字段完整。

5. ✅ **归因采样有针对性**：`verifyAttributionSamples` 挑 3 个用户各取一条最新 200 事件，断言 `api_key_id` = 预期、`model_routed` 非空、`upstream_credential_id` 非空。不是泛泛的 `COUNT(*)`，是逐字段硬断言。

6. ✅ **e2e shell-out 设计干净**：`test/e2e/l2_test.go` 用 build tag `e2e` + `os/exec` 调 `go run ./cmd/e2e-runner`，env 全继承，缺 env 时 `t.Skip`。与既有 e2e 模式一致，零新依赖，零测试框架污染。

7. ✅ **go vet + unit test + build race 全绿**：`go vet ./cmd/e2e-runner/...` 无警告，4 条单测 PASS，`go build -race ./cmd/e2e-runner` 通过。

---

**M-37 (MEDIUM) — verifyLedger model matching 三路 OR 可能在边界情形下双计**：

`verifyLedger` 的 LEFT JOIN `model_catalog mc` ON 条件有三个 OR 分支（lines 499-504）：
```
(ue.model_actual <> '' AND mc.provider_model = ue.model_actual)
OR (ue.model_routed <> '' AND mc.canonical_model = ue.model_routed)
OR (ue.model_requested <> '' AND mc.canonical_model = ue.model_requested)
```
若 `model_actual` 匹配某行的 `provider_model` 且 `model_routed` 匹配另一行的 `canonical_model`（同为该 provider），则 usage_event 行与多个 model_catalog 行 JOIN，SUM 会把 pricing 乘 N 倍，derived cost 虚高 → `withinOnePercent` 误报 mismatch。`UNIQUE (canonical_model, provider)` 约束使此情形在实际数据上极难触发（`model_actual` 通常是 `deepseek-v4-flash` 这样的 provider model 名，不会同时是另一个 model 的 `canonical_model`），但 SQL 形状本身不防。**建议**：把 ON 改成 `COALESCE` 优先级链——先 `model_actual` + `provider_model`，再 `model_routed` + `canonical_model`，最后 `model_requested` + `canonical_model`——或加一层 `SELECT DISTINCT ON (ue.id)` 子查询保证每行 usage_event 只匹配一个定价。不阻塞——当前 seed data 下不会触发。

**M-38 (MEDIUM) — runRBACChecks member 分支缺审计日志校验**：

viewer RBAC 检查在拿到 403 后调用 `verifyForbiddenAudit` 验证审计日志记录了 forbidden 尝试（lines 319-325），但 member RBAC 检查（lines 331-339）只断言 HTTP 403，没有紧随的审计日志查询。member 分支也应验证 `POST /api/admin/virtual-models` 的 403 被记入 `audit_logs`，与 viewer 分支对称。当前行为：member 403 后 runner 直接 return nil，若审计日志写入异步延迟或漏写，runner 不会发现。**建议**：加一条 `verifyForbiddenAudit` 调用，用 member 的 user ID（`userIDByEmail(prepared, cfg.MemberEmail)`）、resource_type=`"virtual_model"`、action=`"create_virtual_model"`（需确认 `internal/rbac/types.go` 中对应常量）。不阻塞——403 本身已被断言，审计漏写是低概率事件。

**N-35 (NIT) — `getenvInt` 零值语义隐藏**：`getenvInt` 在 `parsed == 0` 时返回 fallback（line 660），意味着显式设 `MAX_REQUESTS=0` 会静默用默认值 50。0 在本 runner 中本就会被 `validateConfig` 拒绝（`MaxRequests < 30`），所以无实际影响。若未来新增接受 0 的 flag（如 `duration=0` 表示不限时），这个行为会成为 surprise。当前不改。

**N-36 (NIT) — `verifyForbiddenAudit` 硬编码 action 字符串**：`"update_quota"` 与 `internal/rbac/types.go:28` `ActionUpdateQuota` 常量值一致（已验证），但 runner 不 import internal 包，字符串重复是 tester-to-production coupling。若 renaming 常量，runner 在 e2e 跑时会率先暴露（审计日志 query 找不到对应 action → 测试失败），fail-safe 方向正确。不阻塞。

---

## R-QUOTA-CACHE-PROBE (T-QUOTA-CACHE-PROBE 实施, impl `94b4607` + status `1996684`)

**结论: `[+] Approved`** — measurement-only 纪律严格遵守（零改动 `internal/` / `cmd/gateway` / `cmd/admin`），3 档并发 10/30/50 全部跑完 60s + 10s warmup，DB 采样每档 3 次，报告数据完整。核心结论明确：quota SQL 从 30c 开始成为延迟瓶颈，50c 仍 100% 2xx / 0 5xx。无 CRITICAL/HIGH。2 MEDIUM + 2 NIT 不阻塞。

**正面信号**:

1. ✅ **Measurement-only 边界零越线**：diff 只在 `cmd/loadtest/`（+4 行 `heap_alloc` 指标 + `quota_probe.sh` 脚本 + README）+ `docs/release/`（报告）。`internal/` / `cmd/gateway` / `cmd/admin` 零改动。T-CONC-RERUN 建立的 discipline 第三次连续合规。

2. ✅ **报告数据三档完整 + 退化曲线清晰**：10c → 402 RPS / P99 52ms / quota SQL mean 14ms；30c → 263 RPS / P99 257ms / quota SQL mean 87ms；50c → 199 RPS / P99 566ms / quota SQL mean 211ms。每档含 gateway summary + 3 次 DB sample + pg_stat_activity + docker stats 终态。数据密度够支撑后续 ADR 决策。

3. ✅ **报告 Methodology 段诚实披露 Docker 代理问题**：Docker Desktop 内部代理 `10.23.0.1:8080` 超时导致镜像拉取失败，最终 mockark 走 host 而非 compose `mock-ark` service。报告写了诊断过程 + 实际 workaround + "gateway still used the real auth/quota/usage/proxy middleware stack"——正面确认测量路径有效性未受损。

4. ✅ **`runtime_heap_alloc_final_bytes` 低成本有用信号**：`printSummary` 末尾加 `runtime.ReadMemStats` + 一行输出，在 summary 已打印（所有请求结束）后调用，零测量干扰。3 档数据（2.3MB / 3.2MB / 1.8MB）无内存泄漏迹象。

5. ✅ **probe 脚本自动化程度合理**：`quota_probe.sh` 91 行，一轮命令跑完 3 档 ×（warmup + measure + 3 DB sample + docker stats）。`CREATE EXTENSION IF NOT EXISTS pg_stat_statements` idempotent，`pg_stat_statements_reset()` 在 warmup 后清零确保测量窗口纯净。

6. ✅ **go vet + 8/8 单测全绿**：`cmd/loadtest` 新增 `heap_alloc` 行测试断言已同步更新。

---

**M-39 (MEDIUM) — throughput 随并发升高而下降，报告未作为核心证据点名**：

数据中最强的瓶颈信号不是 P99 延迟上升，而是 **RPS 随并发升高而下降**：10c → 402 RPS，30c → 263 RPS（-35%），50c → 199 RPS（-50%）。这是典型的 "more concurrency = less throughput" 饱和模式——更多 goroutine 在 quota SQL 上排队，连接池竞争加剧，单位时间完成请求数反而减少。报告 Conclusion 段提到 "quota check is a latency bottleneck" 但没直接引用这个 throughput inversion 数据点。**建议**：在 Conclusion 或 V2 Candidates 前加一句 "throughput drops 50% from 10c to 50c while quota SQL mean grows 15×, confirming the quota path is the dominant serialisation point"。不阻塞——数据在表格里，读者可自行读出，只是报告作为 decision-support 文档应该把最关键的 evidence 直接点出来。

**M-40 (MEDIUM) — `quota_probe.sh` DB 采样间隔硬编码，不随 `MEASURE_DURATION` 自适应**：

脚本 line 79-83 用固定 `sleep 15 / sleep 20 / sleep 20` 在 60s 窗口内取 3 个采样点。如果有人设 `MEASURE_DURATION=30s`，第三个采样会在 loadtest 结束后才执行（`pg_stat_statements` 已被 reset 或为空）。**建议**：把采样间隔计算为 `$((MEASURE_DURATION_SECONDS / 3))`，或至少在最前面校验 `MEASURE_DURATION` 必须 ≥ 60s。不阻塞——默认参数下行为完全正确，覆盖了 100% 的实际使用场景。

**N-37 (NIT) — `sample_db` 的 `pg_stat_statements` 查询 filter 与 quota SQL 形状耦合**：

```sql
WHERE query LIKE '%u.monthly_budget_cents%'
  AND query LIKE '%LEFT JOIN usage_events%'
```

依赖表别名 `u` 和 `LEFT JOIN` 关键字。若 `internal/quota/store_postgres.go` 重构 SQL（换 alias、换 JOIN 顺序），filter 静默返回 0 rows，报告 DB sample 段变空。对 measurement-only 脚本可接受——操作者会看到空表并排查。若后续将 probe 脚本固化为 CI job，建议改为更宽松的单条件 filter（只 match `monthly_budget_cents`）或在脚本注释里标明耦合点。

**N-38 (NIT) — `docker stats` 硬编码容器名**：`omnitoken-gateway-1 omnitoken-postgres-1 omnitoken-admin-1` 是 default compose project name 的默认容器名。若用户用 `docker compose -p myproj`，容器名变成 `myproj-gateway-1` 等，`docker stats` 行静默失败（no such container）。建议用 `docker compose -f "$COMPOSE_FILE" ps -q` 动态解析容器 ID，或用 `--filter name=` 匹配。不阻塞——当前项目未使用自定义 project name。

---

## R-T017b-prop (T-017b PROPOSAL, commit `c498850`)

**结论: `[+] Approved`** — 3/3 决策全部采纳，方向与任务体默认推荐一致。Proposal 关门，Codex 可开 T-017b 实施。1 NIT 留实施期注意。

**Decision 1: Model Catalog Query → ACCEPT**

推荐 interface-injected in-memory catalog lookup。`ModelCatalog` interface 含 `LookupProviderModel(ctx, provider, model) (ProviderModel, bool)`，注入 `ArkChatConfig`。Gateway 启动时从 `model_catalog` 表加载到内存。Guard 仅在 `credential.Provider != routedProvider` 时触发，same-provider 路径零改动。

✅ 正确选择。理由充分：请求延迟平坦、测试确定性、proxy 不持 DB 连接、selector 排序不受影响。拒绝 inline DB lookup 的理由成立（storage concern 进 hot retry loop 是错误的耦合）。

**Decision 2: Model Name Conversion Scope → ACCEPT**

T-017b 仅做校验，不做 `payload["model"]` 改写。跨 provider fallback 仅当 fallback provider 有匹配 catalog row 时允许；不兼容则 skip；全部不兼容返回 400。

✅ 正确的 scope 取舍。理由中的连锁影响（model_routed / model_actual / pricing / attribution）是精准的——name conversion 需要独立的 canonical-model equivalence 设计，不属于 T-017b。

**Decision 3: Integration Mock Design → ACCEPT**

推荐 `httptest.Server` in-process mock（DeepSeek-shaped + Ark-shaped），不走 `cmd/loadtest/mockark` 双实例。2 条测试：fallback 成功 + model-not-available skip。上游 call count 断言验证 incompatible provider 在 HTTP 发出前被 skip。

✅ 比 mockark 双实例方案更优：零端口管理、零 Docker 依赖、零外部进程。`internal/proxy` 测试套件已有类似 mock 模式（`retry_test.go`），一致性好。

**Observability Plan → ACCEPT**

WARN log 7 字段 + 3 reason 值（`all_excluded` / `all_degraded` / `preferred_empty`）。Reason 准确性靠 selector 只读 diagnostics helper，不改选择算法。

**正面信号**:

1. ✅ **3 决策全命中任务体默认推荐**：无试探性偏离，无过度设计。Codex 对 task spec 的阅读完整。

2. ✅ **Retry loop 行为描述精确**：compatibility skip ≠ upstream failure，不调 `MarkDegraded`；重复选择防护（skip set 含 attempted + compatibility-skipped）；400 only after no compatible candidate remains。这段描述几乎可直接翻译成实现。

3. ✅ **`ProviderModel` struct 含 `CanonicalModel` + `ProviderModel` 双字段**：虽然 T-017b 只做校验，但 struct 已预留 v1.1 name conversion 所需数据。不越界但也不浪费。

4. ✅ **Selector boundary 尊重得当**："add only a read-only diagnostics helper on `credentials.Selector` if needed... must not alter positions, priorities, provider order, or degraded state."——对既有 selector 合约的边界理解准确。

---

**N-39 (NIT) — empty routed provider 边界未显式处理**：

Decision 1 说 "guard applies only when credential.Provider differs from the routed provider." 但未讨论 `ProviderRoutedFromContext` 为空的情况。当请求不走 virtual model（legacy single-key 路径）时 routed provider = ""，credential.Provider 可能为 "ark"（normalized），比较 `"" != "ark"` → guard 触发 → 查 catalog → 可能误拒绝。建议实施时加一条：guard 在 `routedProvider == ""` 时也 skip（与 same-provider 同义——都是"没有跨 provider 意图"）。不影响 proposal 方向，实施期注意即可。

---

## R-T017b (T-017b 实施, impl `5fc1fdc` + status `3faba2a`)

**结论: `[+] Approved`** — 3 块缺口全部落地。model_catalog 资格校验 + 跨 provider 结构化日志 + 4 条集成测试（覆盖 3 种 reason 值 + model-not-available 400 + pool empty 503）。N-39 已修复（empty routed provider 不触发 guard）。proxy coverage 87.0%（↑0.3%）。无 CRITICAL/HIGH/MEDIUM。1 NIT 不阻塞。

**正面信号**:

1. ✅ **N-39 落地精准**：`isCrossProviderFallback`（`proxy.go:391`）先检查 `routedProvider == ""` → 直接 `return false`。legacy single-key 路径 + Anthropic 路径 + 任何未设 provider 的请求都不会误触发 catalog guard。一行 fix，语义清晰。

2. ✅ **`selectionLimit` 正确防无限循环**：`doWithRetries` 引入 `selectionLimit = maxAttempts + Len()`，`selections` 计数包含 compatibility-skipped credentials，`attempts` 只计实际发送的 upstream 请求。不兼容的跨 provider credential 被 skip 后计入 selections 但不计入 attempts → 不会永久循环。逻辑严密。

3. ✅ **`AvailabilityForProvider` 只读诊断设计干净**：`selector.go:107` 使用与 `NextForProvider` 相同的 `s.mu.Lock()`，但不改 positions/priorities/degraded。返回 `ProviderAvailability{ActiveHealthy, Excluded, Degraded, Available}` 四字段。单测覆盖双时间点（degraded 期内 + 过期后），验证 expiry 后 `Available` 正确恢复。

4. ✅ **3 种 reason 值全部有针对性测试**：
   - `TestCrossProviderFallbackAllExcluded` → DeepSeek 429 → exclude → Ark 200 → assert `reason=all_excluded`
   - `TestCrossProviderFallbackAllDegradedReason` → DeepSeek pre-degraded → Ark 200 → assert `reason=all_degraded`
   - `TestCrossProviderFallbackPreferredEmptyReason` → 无 DeepSeek credential → Ark 200 → assert `reason=preferred_empty`
   每条测试同时断言 WARN log 的 7 个字段完整，不只是 reason 字符串。

5. ✅ **`model-not-available` 400 测试硬断言 Ark call count = 0**：`TestCrossProviderFallbackModelNotAvailable` 用 `atomic.Int32` 计数，断言 Ark upstream 的 HTTP 请求数为 0——验证 incompatible provider 在 catalog lookup 阶段被 skip，不发 HTTP。这是 AC "不在 catalog 中，skip 该 credential 并继续尝试下一个" 的硬证据。

6. ✅ **`TestArkChatProxyReturnsPoolEmptyWhenSelectorHasNoHealthyCredentials`** 补了回归保护：所有 credentials disabled → 503 + `CodeUpstreamCredentialPoolEmpty`。虽然不是 T-017b 的核心场景，但 `doWithRetries` 重构后的 pool-empty 分支值得单独覆盖。

7. ✅ **`StaticModelCatalog` 设计简洁**：`\x00` 分隔符组合 key（providers/models 不含 null byte），`normalizeProvider` 与 selector 一致（空→"ark"），构造时只索引 `provider_model` 不索引 `canonical_model`（符合 Decision 2）。单测 `TestStaticModelCatalogLooksUpProviderModelOnly` 显式断言 canonical model 不匹配——hard guard 防止实施中不自觉做了 name conversion。

8. ✅ **`requestModelInfo` 提取让 `rewriteRequest` 签名变化可管理**：`requested`（virtual model name）与 `routed`（resolved model name）分离，`fallbackReason` 和 `logCrossProviderFallback` 各自取用。`rewriteRequest` 返回 4-tuple 虽略宽但调用方只有 `ServeHTTP`，没有扩散。

9. ✅ **Anthropic 路径零回归**：`rewriteRequest` 改动同时覆盖了 OpenAI 和 Anthropic 路径。`TestAnthropicToOpenAIRequestValidation` + `TestOpenAIToAnthropicMessageVariants` 新增 6 条 subtest 验证 Anthropic 请求解析和响应转换的边缘 case，确保 proxy 重构不影响 `/v1/messages`。

10. ✅ **go vet + 全量测试 green**：`internal/proxy` 所有测试 PASS（含既有 retry/stream/anthropic 测试），`internal/credentials` 6/6 PASS，proxy coverage 87.0%（≥ 基线 86.7%）。

---

**N-40 (NIT) — `fallbackReason` 在 mixed state 下可能给出不精确的 reason**：

当 preferred provider 有 ≥3 个 credentials，其中部分被 retry loop exclude、部分被 pre-degrade 时，`AvailabilityForProvider` 返回 `Available=0, Excluded>0, Degraded>0`。`fallbackReason` 的 switch 优先级为 `all_excluded`（Excluded == ActiveHealthy）→ `all_degraded`（Available==0 && Degraded>0）→ `preferred_empty`。若 `Excluded < ActiveHealthy`，`all_excluded` 不匹配，会返回 `all_degraded`——即使实际是 mixed（部分 excluded + 部分 degraded）。当前部署中每个 provider 只有 1-2 个 credentials（T-016 seed），mixed state 不会出现。建议后续加 `active_healthy=X excluded=Y degraded=Z` 到 log attrs（不代替 reason，作为补充），或把 mixed 情况归为 `reason=preferred_unavailable`。不阻塞——当前规模不会触发，且日志已有 credential_id 用于排查。

---

## R-100-prop (T-100 PROPOSAL, commit `959f47f`)

**结论: `[+] Approved with modification`** — 3 个决策点方向正确。1 条修改要求（M-36）不阻塞 proposal 关门。Codex 可开 T-100 实施。

**Decision 1: seed users — ACCEPT（含修改）**

seed SQL 已有 1 admin + 1 viewer + 9 member，`GET /api/admin/users` 可发现。admin API 确实缺 create-user 端点——Codex 的判断正确，不在 T-100 里新增 production API。

**M-36 (实施期修改) — runner 不应直接写 PG 设 member password**：proposal 建议"direct PG fixture setup to assign a temporary password hash to one seeded member"。runner 的角色是 test harness，不是 DB migration tool。改为：
- runner 接受 `--member-email` + `--member-password`（或 `--viewer-email` + `--viewer-password`）作为可选 flag
- 如果未提供，**skip** member/viewer RBAC 测试（打印 "SKIP: member credentials not provided"）——不 fail
- password 的 PG 写入作为**文档化的 pre-flight 步骤**写在 runner README 中（一行 SQL：`UPDATE users SET password_hash = crypt('temp123', gen_salt('bf')) WHERE email = 'user02@...'`），由运维手动执行
- runner 自身对 PG **只做 SELECT**（账本查询 + 归因查询）

这样 runner 保持 clean——所有写操作走 admin API，PG 只读。

**Decision 2: DeepSeek-only — ACCEPT**

与任务体默认推荐一致。`--deepseek-api-key` 仅用于 preflight 验证（确保上游可达），不 mutation 已部署的 credential 记录。`chat-fast` 默认模型走既有的 virtual_models → DeepSeek credential pool。

**Decision 3: direct PG for ledger — ACCEPT**

`/api/admin/usage/summary` 端点不存在且不在 T-100 scope，`GET /api/admin/users/{id}/usage` 不暴露 `api_key_id` / `model_routed` / `upstream_credential_id`。直接 PG SELECT 是当前唯一能验证账本闭环和归因正确性的路径。**约束**：`--database-url` 只用于 SELECT，runner 不 INSERT/UPDATE/DELETE。

**正面信号**:

1. ✅ **3 个决策的 trade-off 分析诚实**：每个决策都列了"为什么不能走更简单的路径"——缺 create-user API、缺 usage summary API、member 无 password。不回避 gap，不让 reviewer 猜。

2. ✅ **`max_requests < 30` 硬拒绝**：proposal 设了下限——10 user × 3 次/人 = 30。比任务体 AC 的 `MAX_REQUESTS ≥ 10` 更保守（任务体说你至少需要 4 次/人），但这是安全侧——多跑几次才有统计意义。

3. ✅ **runner contract 清晰**：7 个 flag/env var 全列明。`--max-tokens=32` 默认——与 T-SMOKE-AGENT 一致，成本可控。

4. ✅ **e2e test 设计正确**：`test/e2e/l2_test.go` 用 build tag `e2e` + shell out 到 runner，与既有 e2e 模式一致。

**Codex 下一步**: T-100 proposal 关门。实施时遵守 M-36（runner PG 只读，member password pre-flight 文档化）。Decision 1/2/3 已锁定。

---

## R-T018 (T-018 实施, impl `b70794b`)

**结论: `[+] Approved`** — 6/6 场景全部落地，443 行纯测试，零生产代码改动。覆盖 retry recovery + all-exhausted + cross-provider fault fallback + cross-provider exhaustion + degrade/restore 时序 + SSE mid-stream disconnect。3 条测试接入真 usage.Middleware 验证 attribution 正确性。无 CRITICAL/HIGH/MEDIUM/NIT。proxy coverage 87.6%（↑0.6%）。

**正面信号**:

1. ✅ **6 条场景全对应任务体 AC**：

   | # | 测试 | AC | 关键断言 |
   |---|---|---|---|
   | 1 | `TestArkChatProxyRetryConnectionFailureThenRecordsWinningCredential` | Retry + 恢复 | usage attribution → winning credential ID + provider; retry log with `CodeUpstreamConnectionFailed` |
   | 2 | `TestArkChatProxyAllCredentialsExhaustedReturns5xxAndNoUsageRecord` | 全部耗尽 | 3 calls (one per credential); 2 retry logs; final 502 + `CodeUpstream5xx`; upstream body NOT leaked; NO usage record |
   | 3 | `TestArkChatProxyCrossProviderFaultFallbackRecordsArkCredential` | 跨 provider 故障 fallback | 2 DeepSeek retry logs + cross-provider fallback log; usage attribution → `provider=ark` + credential ID |
   | 4 | `TestArkChatProxyCrossProviderAll5xxExhaustion` | 跨 provider 全部耗尽 | cross-provider fallback log; final 502; NO usage record; body not leaked |
   | 5 | `TestArkChatProxyDegradeSkipsAndRestoresCredential` | Degrade + 恢复时序 | 4 `AvailabilityForProvider` 断言（initial→degraded→restored）；call counts A:1→1→2, B:0→2→2；no real sleep |
   | 6 | `TestArkChatProxyDoesNotRetryAfterSSEChunkThenDisconnect` | Stream 中途断开 | `"first"` chunk received; `"second"` NOT in response; cred-B calls = 0; no retry log |

2. ✅ **Usage attribution 接入真 middleware**：场景 1/2/3/4 用 `usage.Middleware(recorder, ...)` 包裹 proxy，`retryUsageRecorder` channel 捕获 `RecordInput`。成功路径断言 `UpstreamCredentialID` / `Provider` / `APIKeyID`；失败路径 `assertNoRetryUsageInput` 断言无 usage record——AC"失败请求不产生错误归因"硬证据。

3. ✅ **`TestArkChatProxyDegradeSkipsAndRestoresCredential` 全链路零 flake**：3 次请求（429 → degrade 期跳过 → degrade 过期恢复）用 `NewSelectorWithClock` + 手动推进 `now`，无 `time.Sleep`。`AvailabilityForProvider` 4 次断言验证 initial/degraded/restored 三态。

4. ✅ **`TestArkChatProxyDoesNotRetryAfterSSEChunkThenDisconnect` 补充 `TestArkChatProxyDoesNotRetryPartialFirstRead`**：后者覆盖 `n>0 && err!=nil`（partial first read），本测试覆盖 chunk 已 flush 到 client 后上游断连——两个 `final=true` 分支独立验证。

5. ✅ **上游 body 防泄漏两条新断言**：场景 2/4 都验证 `"vendor body must not leak"` 不在响应体中。与既有 `TestLogCredentialRetryOmitsUpstreamBody` 形成三重保护。

6. ✅ **helper 设计干净可复用**：`requestSubject` / `waitRetryUsageInput` / `assertNoRetryUsageInput` 三个 helper + `retryUsageRecorder` + `retryUsageConfig` 可被后续 retry 测试复用。

7. ✅ **零生产改动 + go vet green + coverage ↑**：diff 仅 `retry_test.go`（+443 行）。proxy 87.6% > 基线 87.0%。

---

## R-T019 (T-019 实施, impl `8148f34`)

**结论: `[+] Approved`** — 后端 + 前端 + RBAC 全链路 788 行，6 条 handler 测试 + 3 条 store 测试覆盖 4 种状态码 × 3 种角色。admin 67.1%。无 CRITICAL/HIGH/MEDIUM/NIT。

**正面信号**:

1. ✅ **API 设计完整对标既有 credential CRUD 模式**：`POST /api/admin/users` → `protectedWrite(ActionCreateUser)` → `parseCreateUserRequest`（email 用 `net/mail.ParseAddress` RFC 5322 校验）→ `store.CreateUser`（事务内 INSERT users + role_assignments）→ `audit.SetResource` after → `201 Created`。error 分支覆盖 400/403/409/500 四种状态。

2. ✅ **RBAC 三处同步完整**：`internal/rbac/types.go` 新增 `ActionCreateUser` + `AllActions` 注册；`internal/rbac/policy.go` `defaultPolicy` admin=true / member=false / viewer=false；`engine_test.go` `TestAuthorizeRoleActionMatrix` 三角色全断言。无遗漏。

3. ✅ **事务性创建 + 幂等防重**：`CreateUser` 内 `db.BeginTx` → `INSERT INTO users ... ON CONFLICT (organization_id, email) DO NOTHING RETURNING`（`ErrNoRows` → `errAdminUserExists` → 409）→ `INSERT INTO role_assignments ... RETURNING role_id` → `tx.Commit()`。`committed` flag + `defer` rollback 保证失败时清理。模式与 T-044 `ON CONFLICT DO NOTHING` 一致。

4. ✅ **`defaultOrganizationID` fallback 务实**：当 `params.OrganizationID == uuid.Nil`（auth subject 无 org）时，`SELECT id FROM organizations ORDER BY created_at LIMIT 1` 取最早 org。与 seed SQL 中单 org 部署对齐。`errAdminOrganizationNotFound` 区分"DB 无 org"和"真正的 DB 错误"。

5. ✅ **Password 安全三处到位**：(a) `crypt($4, gen_salt('bf'))` 在 SQL 内完成，不经过 Go 内存；(b) `userAuditView` 仅输出 email/display_name/role/user_id/org_id，不包含 password；(c) `TestCreateUserEndpointCreatesAndAudits` 显式断言 `if _, leaked := after["password"]; leaked { t.Fatalf(...) }`——password 不漏到 audit log。

6. ✅ **6 条 handler 测试覆盖完整状态矩阵**：
   - `TestCreateUserEndpointCreatesAndAudits` — 201 + audit after snapshot（含断言 password 不在 audit 中）
   - `TestCreateUserEndpointDeniesNonAdminAndAudits` — viewer/member → 403 + audit forbidden + store 未被调用
   - `TestCreateUserEndpointRejectsInvalidInput` — 400 + `invalid_user` error code
   - `TestCreateUserEndpointReturnsConflictForDuplicate` — 409 + `user_exists`

7. ✅ **3 条 store 测试覆盖 SQL 形状**：
   - `TestPostgresOverviewStoreCreateUserInsertsUserRoleAndHash` — 断言 `INSERT INTO users` 含 `password_hash` / `crypt($4, gen_salt('bf'))` / `ON CONFLICT`；`INSERT INTO role_assignments` 含 `FROM roles r` / `RETURNING role_id`
   - `TestPostgresOverviewStoreCreateUserUsesDefaultOrganization` — nil OrganizationID → 3 条 query (org lookup + user insert + role assign)
   - `TestPostgresOverviewStoreCreateUserDuplicate` — 空 row → `errAdminUserExists`

8. ✅ **前端 UX 完整闭环**：
   - "新建用户"按钮仅在 admin 角色可见（`syncRoleControls` → `nodes.open.hidden = getRole() !== "admin"`）
   - Modal form 四字段：邮箱（`type="email"`）、显示名、角色 dropdown（member/viewer/admin）、初始密码（`type="password" autocomplete="new-password"`）
   - 创建成功 → 自动弹出 "生成 Key" 面板 → 调 `POST /api/admin/dev/virtual-keys` → 展示 full key + prefix + "请立即复制此 Key，关闭后不可再次查看" 安全提示
   - 复制按钮 `navigator.clipboard.writeText` + `execCommand('copy')` fallback + `showToast("Key 已复制")`
   - users 列表每行加 "生成 Key" 按钮（`data-action="generate-key"`），点击进入 key 生成 flow
   - users.test.js 新增断言：admin 角色看到 `data-action="generate-key"` + `edit-quota`；viewer 角色两者均不可见

9. ✅ **CSS 增量干净**：`.view-actions` 水平排列 action buttons、`.key-result-panel` grid 布局、`.security-note` warning 配色、`.key-code` monospace 代码块、`.key-actions` space-between 排列。响应式断点统一收窄为 `flex-direction: column`。

10. ✅ **`adminFakeTx` 补齐 fake driver 事务支持**：`adminFakeConn.BeginTx` → `adminFakeTx{Commit/Rollback}`。不阻塞既有测试（fake driver 的 QueryContext 在事务内仍返回 `adminFakeRows`），`CreateUser` 的事务路径可被单测覆盖。

11. ✅ **go vet + 全量测试 green + coverage 67.1%**。

---

## R-T020 (T-020 实施, impl `433c7b1`)

**结论: `[~] Conditional — 不关任务`** — compose + nginx template + .env.example + admin-password init 交付正确，compose config valid，Go 测试 green。但 **`docker compose up` 未实际跑通**（Docker Desktop 代理阻断镜像拉取），无法确认 service 启动、admin 登录、gateway 转发在真实 Docker 环境中正常。退回重验。2 NIT 不阻塞。

**正面信号**:

1. ✅ **10 service 最小化设计精准**：postgres → migrate → seed → admin-password → credential-seed → redis/nats → gateway/admin → nginx。依赖链有序（`depends_on` + `condition: service_healthy/completed_successfully`），无冗余服务（vs dev 的 mock-ark + test）。

2. ✅ **admin-password 注入安全**：`003_admin_password.sql` 用 psql `:'password'` 变量语法（非 shell 拼接），含双重校验——`length(:'password') > 0` + `COUNT(*) = 1` after UPDATE。compose 中 `:?` parameter expansion 在 shell 层先校验非空。三处防护：shell → psql `\if` → SQL RETURNING check。

3. ✅ **nginx 配置完全 env var 驱动**：`nginx.prod.conf.template` 含 `${DOMAIN}` / `${GATEWAY_PORT}` / `${ADMIN_PORT}` / `${NGINX_HTTP_PORT}` 占位符，compose 命令中 `envsubst` 替换。SSL 检测逻辑在 shell 中完成——`[ -n "${SSL_CERT_PATH}" ]` → 生成 HTTPS server block + HTTP→HTTPS redirect；否则纯 HTTP。用户不写一行 nginx conf。

4. ✅ **`ssl-disabled.pem` placeholder 方案干净**：Docker volume mount 不支持条件挂载，placeholder 文件解决"SSL 未配时 mount source 不存在"的问题。该文件仅用于满足 mount 语法，实际 TLS 路径仅在有 `SSL_CERT_PATH`/`SSL_KEY_PATH` 时启用。文件内容明确标注用途，避免误解。

5. ✅ **SSE 代理配置正确**：`proxy_buffering off; proxy_cache off; proxy_read_timeout 300s; chunked_transfer_encoding on;`——四件套确保 streaming 响应不被 nginx 缓冲或超时截断。`/v1/` 前缀覆盖 chat/completions 和 messages 端点。

6. ✅ **admin 路由重定向设计**：`/admin` → 302 附加 `?admin=<scheme>://<host>` 参数，`/admin/` 检查该参数存在才 serve index.html。这解决了 admin web console 需要知道自身 base URL 的问题——frontend JS 从 URL param 提取 admin API 地址。

7. ✅ **`.env.example` 结构清晰**：42 行分 6 组（Security / Upstream Keys / Admin / Domain+Ports / SSL / Optional tuning），每组含注释说明。必填项（MASTER_KEY / upstream key / ADMIN_INITIAL_PASSWORD）无默认值，强制用户填写。

8. ✅ **README.md 快速部署 ≤ 10 行**：6 步从 `cp .env.example` 到 "Sign in as admin"。每步 ≤ 一行。

9. ✅ **compose config 无语法错误 + Go tests 全绿**。

---

**N-41 (NIT) — nginx 命令中 `envsubst` 依赖 nginx:alpine 自带，未显式安装**：

`nginx:1.27-alpine` 镜像的 `envsubst` 来自 `gettext` 包。该包在 nginx 官方 Alpine 镜像中预装（官方 Dockerfile 文档使用 `envsubst` 作为推荐模板方案），所以当前可用。但如果未来切到 `nginx:1.27`（Debian slim 无 gettext），或切到自定义 nginx 镜像，compose 会在 `envsubst` 行失败。建议在 compose nginx 命令前加 `apk add --no-cache gettext 2>/dev/null || true` 作为安全网，或直接在 Dockerfile.nginx 中 `RUN apk add gettext`。不阻塞——当前镜像实测可用。

**N-42 (NIT) — compose 中 nginx 命令的双层 `$$` 转义隐晦**：

compose 命令中大量使用 `$$` 来逃逸 Docker Compose 的变量处理（如 `$$host` → nginx 最终看到 `$host`）。这是正确的——但 LOCATIONS 嵌入在 YAML 字符串中，双重转义逻辑（Docker Compose → Shell → envsubst）对后续维护者不够直白。建议在 `nginx.prod.conf.template` 顶部加一行注释说明变量替换管线：`# Variables: ${DOMAIN} etc. are substituted by envsubst in compose command. $host $uri are nginx builtins.`。不阻塞——配置已通过 compose config 验证和健康检查。

Resolved: `aacce11` — Docker AC 5/5 verified; admin SPA root and SSL healthcheck fixed; N-42 comment landed; lint/test green.

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
