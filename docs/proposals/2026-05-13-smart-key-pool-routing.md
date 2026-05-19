# OmniToken 智能 Key 池与配额感知模型路由（提案草案）

> 日期: 2026-05-13
> 状态: proposal — Phase 2 Epic 候选，不阻塞当前 Pre-push gate
> 出处: 从 TASKS.md 抽离，方便随时引用
> 关联文档: [`2026-05-14-smart-routing-elastic-cache.md`](./2026-05-14-smart-routing-elastic-cache.md)（次日扩展版，含性能指标与 Elastic 远景）

## 背景

用户明确提出一个关键产品能力：管理员可以配置多个上游 API Key，组织用户调用时系统根据使用情况、额度、健康度、成本和管理员策略，自动选择真实 key / provider / model；管理员可以调整该策略。

## 竞品调研摘要

- LiteLLM Proxy: virtual keys、team/user budget、RPM/TPM、model access、router fallback / load balancing / cost-aware routing。
- Portkey AI Gateway: 多 provider/key load balancing、fallback、conditional routing、budget / rate limit。
- OpenRouter: provider routing，可按 price / throughput / latency / fallback 选择 provider。
- Cloudflare AI Gateway: BYOK secret store、dynamic routing、rate/cost 控制节点、fallback / retry。
- Vercel AI Gateway / Helicone / TrueFoundry / Kong AI Gateway: 均覆盖多 provider 凭据、fallback、限流、预算、可观测路由决策等能力。
- AWS Bedrock Intelligent Prompt Routing / Azure Model Router: 云厂商已开始做 prompt-aware / model-family 内的自动模型选择。

## 建议产品定义

OmniToken 不只做 API 转发，而是组织级 LLM 资源调度器。对内暴露虚拟模型（如 `chat-fast` / `chat-balanced` / `chat-premium`），对外隐藏真实 provider/key/model；每次请求由策略引擎决定真实落点，并记录可审计的 route decision。

## 建议 MVP 范围

- Key 池: 管理员配置多个 upstream credentials（provider、base_url、model、encrypted key、priority、weight、status、budget、RPM/TPM）。
- 虚拟模型: `virtual_models` 映射到多个真实 model targets。
- 路由策略:
  - `priority_fallback`: 主 key/model 失败后切备用。
  - `weighted_balance`: 按权重分流。
  - `quota_aware`: 排除额度、RPM、TPM 已满的 key。
  - `budget_downgrade`: 组织/用户预算接近阈值后降级到更便宜模型，或按策略拒绝。
  - `admin_override`: 管理员禁用 provider / model / key，或临时强制走指定目标。
- 决策日志: 记录 selected provider/key/model、fallback_from、policy_id、reason、latency、tokens、cost、error class。

## 商业竞品 benchmark（用户截图，2026-05-13）

- 截图中单日约 31,069 requests / 24h，覆盖 8 个模型。
- top model `claude-opus-4-7`: 18,461 requests / 24h，平均延迟 12.96s，吞吐 94.8 t/s。
- 其他模型平均延迟约 1.99s ~ 15.46s，吞吐约 41.3 ~ 121.7 t/s。
- 该规模按平均值折算不到 1 QPS，瓶颈主要在上游模型延迟、上游限流、连接稳定性和账本写入异步化，而不是 Go gateway 的纯 HTTP 转发能力。

## Codex 初步判断

- 达到截图所示量级在工程上可行；现有 Go + SSE proxy + deferred usage 入账路线是正确方向。
- 真正风险不在"能不能扛住 3 万次/天"，而在多租户隔离、上游 429/5xx fallback、预算一致性、策略可解释性、密钥安全、以及真实多 key 下的压测与审计。
- 建议 Claude 将该能力拆成 Phase 2 Epic，不要塞入当前 Pre-push gate；Phase 1 继续先完成 admin 鉴权与并发压测。

## 建议验收口径（Phase 2）

- 100 并发、至少 10,000 次短请求回放压测无 panic / data race。
- key/provider 故障注入时 fallback 可控，route decision 100% 可追踪。
- budget / RPM / TPM 超限时行为可配置为 downgrade 或 reject，且测试覆盖边界条件。
- 管理端可查看每个虚拟模型、真实模型、key alias 的请求数、平均延迟、吞吐、错误率、成本与降级次数。
