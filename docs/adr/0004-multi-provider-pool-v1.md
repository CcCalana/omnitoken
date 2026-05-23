# ADR 0004: Multi-Provider Pool 拉进 v1

Status: Accepted, 2026-05-23

## Context

ADR 0003 决定 v1 做"Ark 多 key 池"。2026-05-22 T-CONC-RERUN 真测把这条假设打穿了：

- Ark coding plan **一账号只能绑一把 key**（用户 2026-05-23 确认）。
- 把"2-3 把同 plan key"塞进 seed 是 ADR 0003 没核实就写的假设。
- 真测 3 把 key：1 把是 coding plan（裸 UUID 格式，老 T-CONC-CHECK 用的），2 把是普通 Ark API key（`ark-...-<5char>` 格式），对 `kimi-k2.6` 没 entitlement → 319/588 (54.2%) upstream 404，43.0% 成功率，未达 §零A 第 1 条 v1 验收 >80% 硬门槛。
- 失败成因不是 T-016 设计缺陷：0 × upstream 429 + 三把 key 都进 usage_events 证明多 key 池**机制**有效，瓶颈在"physical pool 在单 provider 单 plan 下无法 > 1 把 key"的现实约束。

memory `project_omnitoken_ark_coding_plan` 当初判断"5 model 共用 1 key"是计费简化是对的，但漏掉了"v1 实际 RPS = 单 plan 单 key 的 ~7 RPS 上限"这层 —— v1 在 §零A 第 1 条上无法用"同 provider 多 key"路径过门槛。

## Decision

**把 multi-provider pool 从 v1.1+ / T-016b 拉进 v1**，作为 v1 §零A 第 1 条"性价比资源"验收的实际形态。

v1 阶段的可裁剪范围：

- ✅ **必须包含**: `upstream_credentials.provider` 已经是字段（T-016 已落），扩展 `credential-seed` 接受 DeepSeek key + 通用 OpenAI-compatible HTTP 路径；proxy 按 credential.base_url（**新增字段**）转发；selector 跨 provider 轮询/429 切池；usage 流水记录 provider + credential_id；virtual_models 加 `provider` 维度（决定 chat-* 路由到哪家）。
- ✅ **DeepSeek 作为 v1 第二个 provider**：官方 API (`api.deepseek.com/v1`)，OpenAI 兼容协议，多 key 同账号合法，`deepseek-v4-flash` 成本 ¥1/M input + ¥2/M output 远低于 Ark coding plan；用户已采购 3 把 key，已落 `.env` 的 `OMNITOKEN_DEEPSEEK_KEYS_1/2/3` + `OMNITOKEN_DEEPSEEK_BASE_URL`。
- ✅ **"开放式" v1 形态**: provider 在 schema 是 text 字段 + 一个生成新 provider key seed env var 的约定（`OMNITOKEN_<PROVIDER>_KEYS_*` + `OMNITOKEN_<PROVIDER>_BASE_URL`），proxy 走通用 OpenAI-compatible adapter。新加 provider 不需要改 internal 接口，只加 seed + 改 virtual_models 路由表即可。Anthropic 这种独立协议留 v1.1+。
- ⏭ **推 v1.1+**: 完整 Provider interface 抽象（factory pattern）；admin CRUD UI 跨 provider；Anthropic 协议适配 (T-020)；OpenAI 官方 API key（T-OAI-*）；KMS；admin per-provider 视图。

**为什么这次裁剪能成立**：

- DeepSeek 是 OpenAI 兼容协议，**adapter 复用现成 proxy 链路**，新增 ~50 行代码（schema 一列 + seed 一份 + base URL 解析）。
- 跨 provider 真正达成"多 key 物理池"语义：3 把 DeepSeek key 同时跑可叠加 RPS，绕开 Ark 单 plan 单 key 约束。
- "开放但不通用"：不做 Provider interface 抽象（factory 模式 = 2-3 天重构），但通过 schema + env var 约定预留扩展点；下一个 OpenAI 兼容厂家（Moonshot / 通义 / xAI）来时直接加 seed 不动 internal。
- v1 ETA 增量 ≈ 2-3 天（不是 ADR 0003 那种 5-7 天）。

## Consequences

### 正面

1. **§零A 第 1 条真正闭环**：v1 真测可以演示"Ark + DeepSeek 跨 provider 多 key 池，单 provider rate limit 时自动 fallback"，符合行业中转站标准形态。
2. **测试成本骤降两个数量级**：DeepSeek `deepseek-v4-flash` 跑 30×30 ≈ ¥0.03，原 Ark 同规模 ≈ ¥5。后续 rerun / 性能测试不再受成本约束。
3. **v1 形态符合实际产品语义**：v1 上线后客户看到的是"我配置了 N 家 N 把 key，gateway 跨家路由 + 自动 fallback"，而不是"一家 N 把 key"（后者本来就不存在 in coding plan）。
4. **Anthropic 协议推后是对的**：v1 不接 Anthropic 意味着 T-020 不阻塞 v1 上线，Anthropic 客户的需求按 v1.1+ 路线接。

### 负面

1. **v1 ETA 增 2-3 天**: 从 ADR 0003 的"~2 周"延后到"~2.5 周"。
2. **新增依赖维度**: 每加一家 OpenAI 兼容 provider 都要补一份 env var 约定 + virtual_models 路由配置。运维心智成本提升。
3. **virtual_models 表需要 provider 维度**: 之前 chat-* 全部默认走 Ark，现在 chat-fast 可以改成 DeepSeek，需要 admin UI 或 seed SQL 配。v1 暂走 seed SQL，admin UI 留 v1.1。
4. **memory 修正**: `project_omnitoken_ark_coding_plan` 必须重写——"5 model 共用 1 key"的洞察作为历史保留，但**v1 性价比资源验收靠 multi-provider，不靠 Ark 多 key**。

### 影响范围

| 层 | 改动 |
|---|---|
| Schema | `upstream_credentials` 加 `base_url text NOT NULL DEFAULT ''` 列（migration 000013）；`virtual_models` 加 `provider text NOT NULL DEFAULT 'ark'` 列；现有 Ark 行回填默认值 |
| Seed | `cmd/upstream-credential-seed` 加 DeepSeek key 解析 (`OMNITOKEN_DEEPSEEK_KEYS_*` + `OMNITOKEN_DEEPSEEK_BASE_URL`)，priority 跨 provider 顺序分配；audit snapshot 加 provider/base_url 字段 |
| Proxy | selector 增加 `Provider` 维度，按 credential.base_url 拼端点；OpenAI 兼容协议 (`/v1/chat/completions`) 共用现有 Ark non-stream 路径，**不需要新协议代码** |
| Virtual Models | seed SQL 加 provider 列，`chat-fast` 等映射先改成 DeepSeek（v1 测试期主路径走 DeepSeek，Ark 留作 fallback）|
| Usage | `usage_events` 已有 `upstream_credential_id`，跨 provider 归因自动生效（无需 schema 改动）|
| Tests | T-016 现有 e2e 复用，加一份 multi-provider e2e；T-CONC-RERUN 改为跨 provider 30×30 |
| Docs | ADR 0004（本文档）；`docs/operations/master-key-rotation.md` 加一句"adding a provider = adding env vars + seed re-run, no key file change" |

### 与既有任务的关系

- **T-016 / T-CONC-COST-ATTR**: ✅ 已完成，base URL 是 T-MP-DEEPSEEK 的增量改动，T-016 不重写。
- **T-CONC-RERUN H-6 二审**: 之前的"3 把 Ark key" rerun 数据保留作为历史证据；新的"Ark + DeepSeek 跨 provider"rerun 写进同一份 release 文档**新增段**（不覆盖旧段）。接受标准 #2 的 >80% 重新跑到达成才能签。
- **T-020 (Anthropic) / T-OAI-***: v1.1+ 范围不变。Anthropic 因协议差异不能借这条 ADR 进 v1；OpenAI 官方 API 协议兼容理论可以进，但 v1 测试以 DeepSeek 为准，不并入。

## Open Questions

- virtual_models 在 v1 是否对每个虚拟模型固定一个 provider，还是支持"chat-fast 跨 provider 轮询"（同 virtual model 在 Ark/DeepSeek 各有一份 real_model）？v1 简化为**固定单 provider**，跨 provider routing 是 v1.1 cost-aware routing 范围；T-MP-DEEPSEEK propose 时确认。
- DeepSeek key 在 selector 里是否与 Ark key 同 priority 序列（即 priority 1 = DeepSeek-1, priority 2 = DeepSeek-2, ...）还是按 provider 分段？v1 用**单一全局序列**，依赖 seed 顺序拍 priority。
