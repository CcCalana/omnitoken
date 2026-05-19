# OmniToken 智能路由、性能指标与 Elastic/OpenSearch 远景评估草案

> 日期: 2026-05-14
> 用途: 给 Claude 评估是否纳入后续 Phase 2/Phase 3 规划、任务拆分或 ADR。
> 状态: proposal input。本文不替代 `规划.md`，也不直接改变当前 pre-push gate。

## 0. 结论摘要

本轮讨论确认了一个重要产品方向: OmniToken 不应只做 OpenAI-compatible API 转发，而应逐步升级为企业级 LLM 资源控制面，覆盖多上游 API Key、虚拟模型、配额感知路由、成本核算、性能观测、审计追踪和缓存优化。

与 New API、Sub2API 相比，OmniToken 当前功能完整度还不占优，但可以在企业级可信治理方向形成差异化: 精确账本、可解释路由、官方 BYOK / 多供应商 Key 池、性能 SLO 指标、合规密钥管理、审计日志，以及未来基于 Elasticsearch/OpenSearch 的请求级搜索、异常分析和语义缓存召回。

需要特别强调: Elasticsearch/OpenSearch 不建议近期作为核心强依赖引入，也不应替代 PostgreSQL 的权威账本。更合适的定位是可选的 observability/search/cache-index 二级数据平面，用于请求检索、模型性能矩阵、路由决策审计、异常分析，以及远期的语义缓存候选召回和离线缓存预热。

## 1. 当前技术栈与已落地能力

当前 OmniToken 以 Go 后端为主，使用 Go 标准库 HTTP server 实现 gateway/admin 服务，PostgreSQL 作为主数据库，`golang-migrate` 管理 up/down 迁移，`lib/pq` 连接 PostgreSQL。Docker Compose 已提供本地 Postgres、Redis、NATS、gateway、admin、migrate 环境；Redis 与 NATS 仍主要是架构预留，核心路径尚未强依赖。

前端位于 `web/`，采用 vanilla HTML/CSS/JS，无 React/Vue/Next，无 npm 构建流程，Chart.js CDN 用于图表。Overview / Users / Models 三个视图已经从真实 admin API 拉取数据。

数据面已有 `/v1/chat/completions` OpenAI-compatible 代理，支持非流式和 SSE 流式。当前上游主路径是火山方舟 Ark OpenAI-compatible endpoint，配置通过 `OMNITOKEN_` 前缀环境变量注入。日志使用 `log/slog`，安全约束是不输出 Authorization、API Key、Prompt 全文。

当前 Demo-Ready 已验证真实方舟调用、usage 入账和 admin overview 聚合。已落地的核心数据结构包括:

- `model_catalog`
- `model_pricing`
- `model_pricing_current`
- `usage_events`
- `usage_token_breakdown`
- `cost_ledger`

这说明成本与用量账本的底座方向是正确的，但还没有形成完整商业指标体系。

## 2. 当前价格、缓存、并发、TTFT、吞吐状态

### 2.1 价格与成本

已有较好的数据库底座，并能按请求写入 token breakdown 和 cost ledger。当前成本计算大致基于:

```text
prompt_tokens * input_rate
+ completion_tokens * output_rate
+ reasoning_tokens * reasoning_rate
+ cached_tokens * cached_input_rate
```

需要注意，当前 Ark seed 价格是 demo placeholder，不能用于真实商业报价。商业版应调整为更严谨的公式:

```text
cost =
  uncached_input_tokens / 1M * input_rate
+ cache_read_tokens / 1M * cached_input_rate
+ cache_creation_tokens / 1M * cache_creation_rate
+ output_tokens / 1M * output_rate
+ reasoning_tokens / 1M * reasoning_rate
```

关键风险是 cached tokens 可能是 prompt tokens 的子集，如果不拆分 uncached / cache read / cache write，可能发生双算。

### 2.2 缓存

数据库已预留缓存相关字段，例如 `supports_prompt_cache`、`cached_input_rate_usd`、`cache_creation_rate_usd`、`cached_tokens`、`cache_creation_tokens`、`cache_read_tokens`、`cache_status`。当前 parser 只初步解析 OpenAI usage 中的 `prompt_tokens_details.cached_tokens`。

结论: 缓存计费字段已有，缓存产品能力未完整落地。尚未实现:

- cache read / cache creation 的完整区分。
- gateway 级 exact response cache。
- provider prompt cache 策略优化。
- semantic cache。
- cache hit ratio、saved cost、saved latency 指标。

### 2.3 并发

Go HTTP server、goroutine、HTTP transport keep-alive、SSE 边读边 flush、usage 异步入账，都为高并发代理打好了基础。当前 proxy transport 默认有连接池配置，例如 `MaxIdleConns=100`、`MaxIdleConnsPerHost=10`。

但并发治理尚未完整落实。当前还没有:

- per-key / per-user / per-model 并发上限。
- RPM / TPM token bucket。
- 请求队列。
- 熔断 / backpressure。
- provider/key 健康度驱动调度。
- 大规模压测基线。

任务板已有 T-012，并发验证目标是 10 并发 x 10 请求，共 100 次。这是 Phase 1 sanity check，不等于商业容量验收。Phase 2 建议提升到 100 并发、10,000 次短请求回放压测，并覆盖 data race、panic、入账一致性和上游故障注入。

### 2.4 首字延时 TTFT

`usage_events.ttft_ms` 已预留，但当前代码未实际采集。proxy 中的 `ResponseHeaderTimeout` 是超时控制，不是产品指标。

建议未来在流式路径中记录从请求进入 gateway 到第一次成功写出上游 chunk 的时间。非流式也可以记录上游响应头到达时间。Admin 指标应展示 avg / p50 / p95 / p99 TTFT。

### 2.5 吞吐

当前没有完整 tokens/sec 指标。商业竞品截图展示的是每个模型 24 小时请求数、平均延迟和吞吐 t/s。截图量级约 31,069 requests / 24h，覆盖 8 个模型，平均不到 1 QPS。

工程判断: Go gateway 达到该日请求量问题不大。真正风险不在 HTTP 转发，而在:

- 上游模型延迟。
- 上游 429 / 5xx。
- fallback 策略。
- 预算一致性。
- 密钥安全。
- 审计可解释性。
- 多 key 真实压测。

未来应补充:

- `output_tokens_per_second`
- `total_tokens_per_second`
- avg / p50 / p95 / p99 latency
- TTFT
- 错误率
- fallback 率
- cache hit ratio
- cost per successful request
- provider health score

## 3. New API 与 Sub2API 对标

### 3.1 New API

New API 的优势在功能广度: 多模型聚合、协议转换、渠道权重、失败重试、用户/Token 管理、计费、缓存计费、模型性能排行、渠道优先级和权重等。它更像通用模型分发/商业中转平台。

短板或风险:

- AGPL-3.0，不适合复制代码进入闭源商业项目。
- 更偏中转平台和配额分发，企业审计账本、策略版本化、route decision 可解释性不是其最核心定位。

### 3.2 Sub2API

Sub2API 的优势在账号池调度: 多账号、OAuth/API Key、粘性会话、用户/账号并发控制、请求/token 限流、内置支付、PostgreSQL + Redis。它更贴近订阅额度池和账号调度。

短板或风险:

- 核心场景偏订阅共享/账号池调度，可能存在上游 ToS 风险。
- LGPL-3.0-or-later，不适合直接复制代码进入闭源商业项目。
- 企业级审计、合规 BYOK、策略治理不是其最主要卖点。

### 3.3 OmniToken 可超越方向

OmniToken 不应做一个更像 New API 的 New API，也不应复制 Sub2API 的订阅拼车路线。更适合的差异化定位是:

```text
企业级 AI API 资源治理:
精确账本 + 可解释路由 + 官方 BYOK + 性能观测 + 合规密钥管理 + 审计追踪
```

可超越点:

- 请求级 cost ledger，而不是单纯余额扣减。
- 每次路由有 route decision，说明为什么选中某 key/model/provider。
- 管理员可配置策略版本、灰度、回滚和审计。
- 支持 BYOK 与官方 API，降低订阅共享/账号池灰色风险。
- 更专业的模型性能矩阵: TTFT、P95/P99、tokens/s、fallback 率、cache hit ratio、cost/request。
- 许可证与交付可控，避免 AGPL/LGPL 代码并入风险。

## 4. 智能 Key 池与配额感知路由

建议新增 Phase 2 Epic: 智能 Key 池与配额感知模型路由。

核心产品定义:

管理员配置多个 upstream credentials，包括 provider、base_url、encrypted key、priority、weight、status、budget、RPM、TPM。用户只请求虚拟模型，例如 `chat-fast`、`chat-balanced`、`chat-premium`。系统根据健康度、额度、成本、延迟、吞吐、管理员策略自动选择真实 provider/key/model，并记录 route decision。

建议 MVP 路由策略:

- `priority_fallback`: 主 key/model 失败后切备用。
- `weighted_balance`: 按权重分流。
- `quota_aware`: 排除额度、RPM、TPM 已满的 key。
- `budget_downgrade`: 组织/用户预算接近阈值后降级到更便宜模型，或按策略拒绝。
- `admin_override`: 管理员禁用 provider / model / key，或临时强制走指定目标。

每次请求建议记录:

- selected provider / key alias / model
- fallback_from
- policy_id / policy_version
- route_reason
- latency_ms / ttft_ms
- tokens / cost
- error class

## 5. Elasticsearch/OpenSearch 的远景定位

Elasticsearch/OpenSearch 可以成为 OmniToken 远期超越 New API/Sub2API 的关键能力，但不建议近期作为核心强依赖引入。

正确定位:

```text
PostgreSQL = 权威账本与配置库
Redis = 实时限流、短 TTL exact cache、热数据
NATS / outbox worker = 异步事件管道
Elasticsearch/OpenSearch = 搜索、观测、分析、审计、语义缓存索引
```

推荐数据流:

```text
gateway/admin
  -> PostgreSQL 写入权威账本
  -> outbox / NATS / worker
  -> Elasticsearch or OpenSearch 批量索引
  -> Dashboard / Search / Alert / Analytics / Semantic Cache Candidate
```

要求:

- gateway 不同步写 Elasticsearch/OpenSearch。
- Elasticsearch/OpenSearch 挂了不影响请求转发和计费入账。
- 所有写入异步化，避免拖慢 SSE 和 TTFT。
- Prompt 全文默认不进入 Elasticsearch/OpenSearch。

建议索引:

- `omnitoken-request-events-*`
- `omnitoken-route-decisions-*`
- `omnitoken-cost-ledger-*`
- `omnitoken-provider-health-*`
- `omnitoken-audit-logs-*`
- `omnitoken-security-events-*`

许可证注意:

- Elasticsearch/Kibana 默认发行版涉及 Elastic License 2.0，源码许可也涉及 SSPL / AGPLv3 / Elastic License 选项。
- 按本项目依赖政策，Elastic License、SSPL、AGPL 均需要 propose / review，不应直接作为核心依赖引入。
- OpenSearch 是 Apache-2.0 fork，更适合作为私有化默认选项。
- 推荐写法: 支持 Elasticsearch/OpenSearch 作为可选观测与缓存索引后端；商业私有化默认优先 OpenSearch 或客户已有 Elastic 集群。

## 6. Elasticsearch/OpenSearch 作为提升缓存命中的手段

Elasticsearch/OpenSearch 的价值不只是日志搜索，也可以作为语义缓存召回层。但它提升的不是传统完全相同 key 命中，而是:

```text
相似请求发现 + 语义缓存候选召回 + 热点模式分析 + 离线缓存预热
```

建议缓存分层:

```text
L0: Redis exact cache
    完全相同请求 hash 命中，最快、最安全。

L1: Provider prompt cache optimization
    利用 OpenAI / Anthropic / Ark 等上游 prompt cache，降低输入 token 成本。

L2: Elasticsearch/OpenSearch semantic cache index
    对相似问题召回历史回答，经过严格过滤和安全校验后复用。
```

语义缓存文档可以包含:

- org_id / project_id
- virtual_model / model_actual
- system_prompt_hash
- tool_schema_hash
- policy_version
- input_hash
- input_embedding
- normalized_user_intent
- response_hash
- response_summary
- cache_ttl
- context_version / knowledge_base_version
- cost_usd
- latency_ms
- quality_score

语义缓存必须有严格边界:

- same org / project
- same policy_version
- same model class
- same system prompt hash
- same tool schema hash
- same knowledge base version
- TTL not expired
- similarity above threshold
- optional verifier passed

## 7. 同步语义缓存链路的延迟风险

完整在线语义缓存流程如果同步执行，可能显著增加 TTFT:

```text
Redis exact cache: 1-5ms
请求规范化/hash: <1ms 到数 ms
embedding: 50-300ms 或更高
Elasticsearch/OpenSearch kNN: 10-100ms，数据大时更高
候选过滤: 1-10ms
verifier 小模型校验: 100-800ms
上游 LLM: 1s-15s+
```

因此不建议每次请求都同步执行 embedding + kNN + verifier。推荐策略:

1. 主路径只做快速 exact cache 或已预热高置信缓存。
2. 未命中应立刻走上游模型，避免拖慢 TTFT。
3. 请求完成后异步生成 embedding，写入 Elasticsearch/OpenSearch。
4. 后台离线聚类高频相似请求，计算可缓存候选。
5. 将高置信候选预热进 Redis。
6. 在线 semantic cache 必须配置 `max_lookup_budget_ms`，例如 50ms，超时即 bypass。

建议默认策略:

- 精确缓存可默认开启。
- 在线语义缓存默认关闭。
- 语义缓存只对 FAQ、分类、摘要模板、固定知识库问答等低风险场景开启。
- 每条策略必须记录 `cache_status=exact_hit/semantic_hit/bypass/expired/unsafe`。

## 8. 建议 Phase 拆分

### Phase 1: 保持当前 pre-push gate

- 完成 T-010 admin bootstrap token 全路由鉴权。
- 完成 T-012 10 并发 x 10 请求 sanity 压测。
- 不在 Phase 1 引入 Elasticsearch/OpenSearch。

### Phase 2: 指标与路由底座

- 真实采集 `ttft_ms`。
- 增加 latency p50/p95/p99、tokens/s、错误率、fallback 率。
- 建立 `route_decisions` 表。
- 实现 per-key / per-user RPM、TPM、并发限制。
- 实现智能 Key 池 MVP: priority fallback、weighted balance、quota aware、budget downgrade。
- 修正 cache token 计费拆分，避免 cached token 双算。

### Phase 3: 可选观测后端

- 增加 outbox / NATS worker。
- 支持 OpenSearch/Elasticsearch 批量索引请求事件、路由决策、provider health、audit logs。
- 增加日志搜索、性能矩阵、异常告警。
- 保持 PostgreSQL 为权威账本。

### Phase 4: 语义缓存与缓存优化

- L0 Redis exact cache。
- L1 provider prompt cache optimization。
- L2 OpenSearch/Elasticsearch semantic cache index。
- 后台聚类与 Redis 预热。
- 在线语义缓存仅限严格策略、低风险场景和短 lookup budget。

## 9. 需要 Claude 决策的问题

1. 是否把“智能 Key 池与配额感知模型路由”提升为 Phase 2 Epic。
2. 是否新增 `route_decisions` 作为一等数据模型。
3. 是否将 TTFT、tokens/s、P95/P99、cache hit ratio、fallback rate 纳入 Phase 2 验收指标。
4. 是否将 Elasticsearch/OpenSearch 写入远景规划，但明确为 optional backend，不进入当前核心依赖。
5. 私有化默认后端是否优先 OpenSearch，以降低 Elastic License / SSPL / AGPL 许可复杂度。
6. 语义缓存是否只作为 Phase 4 低风险策略能力，而不是 Phase 2 主路径能力。

## 10. 建议给外部汇报的一句话

New API 和 Sub2API 解决的是模型分发、渠道管理和账号池调度；OmniToken 未来应通过精确账本、可解释路由、性能观测、合规密钥管理，以及 Elasticsearch/OpenSearch 驱动的请求搜索和语义缓存索引，升级为企业级 LLM FinOps + AI Gateway + Resource Control Plane。
