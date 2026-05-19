# 近期活跃参考项目与产品资料（从规划.md 第七节归档）

> 归档时间: 2026-05-12。本文件内容原位于 `规划.md` 第七节，为减少每次会话 token 消耗而独立归档。

## 1. 开源主参考项目

| 项目 | 活跃度与使用信号 | 适合参考的能力 | 对 OmniToken 的启发 |
| --- | --- | --- | --- |
| LiteLLM / BerriAI | GitHub 约 46k stars，2026.05 仍有稳定版本发布；定位为 Proxy Server / AI Gateway，支持 100+ LLM。来源: https://github.com/BerriAI/litellm ，https://docs.litellm.ai/ | OpenAI 兼容接口、多供应商协议转换、Virtual Keys、预算、RPM/TPM、团队/项目维度成本跟踪、路由、Fallback、Guardrails、Admin Dashboard。 | 可作为第一优先级产品能力参考。MVP 可先实现 LiteLLM 类似的"虚拟 Key + 模型白名单 + 预算 + 路由配置"。 |
| New API / QuantumNous | GitHub 约 31k stars、6k+ forks，2026.05.06 发布 v1.0.0-rc.4，2026.05.07 仍更新。来源: https://github.com/QuantumNous/new-api ，https://github.com/QuantumNous/new-api/releases | 中文生态常见的模型聚合与分发平台；支持 OpenAI / Claude / Gemini 协议转换、用户管理、Token 分组、模型限制、渠道权重、失败重试、计费、缓存计费、新版 v1 前端、模型可用性/性能排行。**含完整的管理员审计日志（log_t 表）、令牌/渠道/用户 CRUD 与 quota 修改流程、渠道健康检查与失败重试策略**。 | 很适合作为国内企业管理台、渠道页、模型定价页、额度页、日志页的 UX 参考。**Phase 2 难点专项参考**: ① audit_logs 表结构与字段设计；② Token 分组的多租户隔离；③ 渠道权重 + 失败重试的 fallback 链；④ 缓存计费的 cached_tokens 入账。但其 AGPL-3.0 协议不适合直接复制代码到闭源商业项目，只建议参考产品形态与字段命名。 |
| sub2api 类项目 | 把 Claude Pro / ChatGPT Plus / Gemini 等订阅式登录态转成 OpenAI-compatible API 的开源工具集合（包括 `meaning-systems/claude-code-proxy`、`codex-account-orchestrator` 等，已在 `references/agent-adapter/agent-adapter-projects-reference.md` 第 2.3/2.4 节展开）。 | OAuth 凭证缓存与刷新、订阅 quota 检测、多账号轮换、订阅态↔API 态的协议适配 | **Phase 3 难点专项参考**: ① Agent 适配层中"订阅模式 + API 模式混合上游"的处理；② upstream_credentials 表中 OAuth / Bearer / 订阅 cookie 三种凭据类型的字段统一；③ 多账号轮换避免单账号被风控。Phase 2 主线不涉及，但 §零A 性价比一角的"白嫖订阅配额"长尾能力可参考。 |
| Higress / Alibaba | GitHub 约 8k stars，2026.04 发布 v2.2.1，2026.03 加入 CNCF Sandbox。来源: https://github.com/alibaba/higress ，https://www.cncf.io/blog/2026/03/25/higress-joins-cncf-delivering-an-enterprise-grade-ai-gateway-and-a-seamless-path-from-nginx-ingress/ | 基于 Envoy/Istio 的 AI Native API Gateway，支持 OpenAI 兼容统一入口、模型负载均衡、Fallback、语义缓存、Token 级限流、内容安全、MCP Server 托管、Wasm 插件扩展。 | 如果企业客户偏云原生/Kubernetes，可参考 Higress 的插件化网关路线。我们的 Go 网关也应预留插件接口和流式处理扩展点。 |
| Kong AI Gateway | Kong 主仓约 43k stars，2026.04 发布 AI Gateway 3.14 / Agent Gateway 相关能力。来源: https://docs.konghq.com/gateway/latest/get-started/ai-gateway/ ，https://developer.konghq.com/plugins/ai-proxy/ ，https://www.prnewswire.com/news-releases/kong-ai-gateway-now-supports-agent-to-agent-traffic-becoming-the-most-comprehensive-ai-gateway-for-the-agentic-era-302741741.html | 成熟 API Gateway 的 AI 插件化做法：AI Proxy、协议标准化、模型上游代理、高级路由、重试、观测、A2A/MCP 代理。 | 适合参考企业级治理边界：鉴权、审计、插件、代理协议、可观测性应和普通 API Gateway 一样标准化。 |
| Portkey AI Gateway | GitHub 约 11k stars，2026.03 宣布统一 Gateway 开源并披露大规模生产使用。来源: https://github.com/Portkey-AI/gateway ，https://portkey.ai/docs/virtual_key_old/product/ai-gateway | Universal API、Vault/Virtual Keys、Budget Limits、Rate Limits、缓存、条件路由、负载均衡、Canary、Fallback、Guardrails、MCP Gateway。 | 适合参考"路由配置即策略"的设计。OmniToken 可把 route config 版本化，支持灰度、权重、Fallback 链、预算和安全策略组合。 |
| Envoy AI Gateway | GitHub 约 1.6k stars，2026.05.05 发布 v0.6.0；背靠 Envoy/CNCF 生态。来源: https://github.com/envoyproxy/ai-gateway | Kubernetes Gateway API 风格，两层网关：Tier One 做鉴权、顶层路由、全局限流；Tier Two 做自托管模型细粒度访问、Endpoint Picker、InferencePool。 | 适合参考生产架构分层。我们的系统可拆成 Control Plane（管理配置）与 Data Plane（高性能转发），后续支持多地域/多集群。 |
| APIPark | GitHub 约 1.7k stars，2026.05 仍在更新；定位为 AI & API Gateway + Developer Portal。来源: https://github.com/APIParkLab/APIPark | API 申请与审批、开发者门户、订阅管理、调用统计、API 与 AI 能力打包、日志导出、多租户。 | 适合参考企业内部"申请 API / 审批 / 订阅 / 应用管理"的管理流程，不只做 Key 列表。 |
| Helicone | GitHub 约 5k stars，2026.04 仍更新；AI Gateway + LLM Observability。来源: https://github.com/helicone/helicone ，https://docs.helicone.ai/getting-started/integration-method/gateway | 单入口日志、成本与延迟追踪、请求列表、Sessions、Prompts、缓存、Rate Limits、Fallback、BYOK。 | 适合参考日志/观测页面。OmniToken 需要尽早设计 request_id、trace_id、session_id，否则后续排障会很痛。 |
| Langfuse | GitHub 约 26k stars，2026.05.01 发布 v3.172.1，2026.05.07 仍更新。来源: https://github.com/langfuse/langfuse ，https://langfuse.com/integrations | LLM Observability、OpenTelemetry、Prompt Management、Playground、Datasets、Evals，与 LiteLLM / Helicone / Kong / Portkey 等网关集成。 | 不一定要在 OmniToken 内部重做全部 LLMOps。企业版可先提供 Langfuse/OpenTelemetry 导出，把深度评测和 Prompt 管理交给专业工具。 |
| TensorZero | 约 11k stars，2026.03-04 仍有发布/资料更新。来源: https://www.tensorzero.com/docs/gateway ，https://github.com/tensorzero/tensorzero | Rust 高性能 LLM Gateway、结构化 inference、观测、实验、A/B Test、Fallback、反馈闭环。 | 可参考其"网关 + 实验 + 反馈"的长线方向。OmniToken 初期不用做优化闭环，但数据表要能保留 experiment_id / variant_id。 |

## 2. 云产品与商业产品功能对标

| 产品 | 近期资料 | 值得参考的点 |
| --- | --- | --- |
| Cloudflare AI Gateway | 文档 2026.04 仍更新。来源: https://developers.cloudflare.com/ai-gateway/observability/logging/ ，https://developers.cloudflare.com/ai-gateway/features/caching/ ，https://developers.cloudflare.com/ai-gateway/reference/limits/ | 控制台日志字段非常完整：prompt、response、provider、status、token、cost、duration、DLP action；缓存支持按请求头设置 TTL、跳过缓存和自定义 cache key；日志保留与 Logpush 是企业客户会问的能力。 |
| Vercel AI Gateway | 文档 2026.01-02 更新。来源: https://vercel.com/docs/ai-gateway/ ，https://vercel.com/docs/ai-gateway/openai-compat ，https://vercel.com/docs/ai-gateway/capabilities/observability | OpenAI-compatible baseURL 迁移体验很好；Dashboard 按 Team / Project / API Key 聚合请求数、tokens、TTFT、成本；模型发现接口返回模型能力、上下文窗口、价格。 |
| OpenAI / Anthropic Prompt Caching | OpenAI 使用 `usage.prompt_tokens_details.cached_tokens`；Anthropic 使用 `cache_creation_input_tokens` 与 `cache_read_input_tokens`。来源: https://platform.openai.com/docs/guides/prompt-caching ，https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching | 成本核算表不能只存 prompt_tokens/completion_tokens，要拆分 cached、cache write/read、reasoning、tool、image、audio 等 token 类型。否则账单与厂商后台会对不上。 |

## 2.5 Agent 适配参考（Phase 3 Epic 落地素材，2026-05-18 归档）

> 用户提供的跨项目总结，落地到 `docs/references/agent-adapter/` 三份文件。Phase 3 Agent 适配 Epic 实施前必读，规划参见 `规划.md` §十四。

| 文件 | 内容 | Phase 3 用法 |
| --- | --- | --- |
| [`agent-adapter/agent-adapter-pattern.md`](references/agent-adapter/agent-adapter-pattern.md) | tingly-box 完整源码级分析: `ai/agent/` 核心适配层 (Registry+Interface) + `internal/agent/` 业务编排层 + Claude Code / OpenCode / Codex 三个 Agent 的完整 Apply/Restore 实现 + 备份恢复 | 直接对照实现 `internal/agent_adapter/` 包结构与接口；T-040 ~ T-044 实施模板 |
| [`agent-adapter/agent-adapter-projects-reference.md`](references/agent-adapter/agent-adapter-projects-reference.md) | 跨 16 个开源项目横向对比；提炼 6 种 Agent 适配模式 (env 注入 / wrapper / 系统服务 / TOML+JSON 编辑 / 协议转换 / 反向 API) + 关键代码片段 | T-040 设计抉择参考；token_proxy 的 toml_edit 无损编辑模式可直接借鉴 |
| [`agent-adapter/tingly-box-architecture-reference.md`](references/agent-adapter/tingly-box-architecture-reference.md) | tingly-box 除 Agent 适配外的全部架构: 负载均衡 (7 种策略) + 熔断器三态 + 健康监控 (429/401/一般错误差异化) + 智能路由特征匹配 + 配置热加载 + 多端点自适应 (Chat vs Responses) + 协议验证矩阵 | **Phase 2-C 提前借鉴**: T-017 智能路由的特征匹配 / T-018 熔断器三态设计；Phase 3 协议转换的能力探测与降级 |

**关键路径选择**: Phase 3 Agent 适配 Epic 的实施顺序建议按 `agent-adapter-pattern.md` 的"Phase 1 → Phase 2 → Phase 3"分阶段，先 Claude Code（最复杂，覆盖度最高），再 Codex（TOML 无损编辑），最后 OpenCode（JSON 最简单）。

## 3. 前端控制台页面建议

| 页面 | 核心字段/控件 | 参考来源 |
| --- | --- | --- |
| API Keys / Tokens | Key 名称、所属组织/项目/用户、Key 前缀、创建时间、过期时间、最后调用、状态、模型白名单、RPM/TPM、月预算、已花费、备注、复制一次、轮换、禁用、删除。 | LiteLLM Virtual Keys、Portkey Virtual Keys、Vercel API Key 维度观测。 |
| Providers / Channels | Provider 类型、Base URL、上游 Key 池、权重、优先级、健康状态、余额/额度、错误率、P95 延迟、最后失败原因、测试连接、启停。 | New API 渠道、Portkey 路由、Kong AI Proxy、Higress 模型多活。 |
| Routing Policies | 请求模型、实际上游模型、团队/项目条件、权重、Fallback 链、重试次数、超时、Canary 比例、地域/标签、策略版本、发布/回滚。 | Portkey Config、LiteLLM Router、Envoy AIGatewayRoute、Vercel providerOptions。 |
| Usage & Cost | 按组织/项目/用户/Key/模型/渠道聚合；input/output/reasoning/cached/image/audio token；成本、同比环比、预算消耗、导出 CSV。 | LiteLLM Spend Tracking、Cloudflare Logs、Vercel Observability、Helicone Analytics。 |
| Logs / Traces | request_id、trace_id、时间、状态码、模型、渠道、Key、用户、延迟、TTFT、Token 明细、成本、错误类型、是否命中缓存、脱敏后的请求摘要、原始日志查看权限。 | Cloudflare Logging、Helicone Requests、Langfuse Traces。 |
| Models & Pricing | 模型 ID、厂商、上下文窗口、最大输出、输入价、输出价、缓存读写价、reasoning 价、多模态价、启用状态、别名、自动同步时间。 | New API 价格页、Vercel Models API、Helicone Cost API。 |
| Guardrails / Safety | PII 检测、DLP 规则、敏感词、Prompt Injection 检测、响应拦截、命中策略、放行/阻断/仅记录。 | Portkey Guardrails、Cloudflare DLP、Higress 内容安全。 |
| Alerts / Webhooks | 预算阈值、错误率阈值、延迟阈值、Key 泄露/异常调用、渠道不可用、通知方式（邮件/飞书/企业微信/Webhook）。 | LiteLLM、Helicone、Cloudflare Logpush。 |
| Audit Logs | 管理员操作、Key 创建/轮换/删除、预算修改、渠道配置修改、策略发布、登录记录、导出记录。 | LiteLLM Enterprise、Kong/Konnect、企业合规通用要求。 |
