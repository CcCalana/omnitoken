# PROPOSAL: T-017a Virtual Model Resolver

## 1. Schema 弹性
v1 先用单列 `real_model text`，满足当前一对一映射。未来 T-017b 引入 fallback 时，增加 `fallback_models text[]` 列或者独立的级联表，对现有逻辑向后兼容。

## 2. 解析失败语义
未在表中查到的模型名直接**透传**（相当于 fallback 为真实模型）。这保证了后向兼容性，现有的直接使用方舟模型名（如 `ep-xxx`）的客户端可以继续工作。只有命中表且 `status != 'active'` 才会返回 400 `virtual_model_disabled`。

## 3. 请求 body 重写位置
重写发生在 `cmd/gateway` 的 `virtualModelMiddleware` 中（位于 `enforceMonthlyBudget` 之后、反代转发之前）。我们通过读取解析 Request Body 的 `model` 字段并执行替换，将虚拟原名注入到 Request Context (`VirtualModelKey`) 以供 `usage` 记录 `model_requested`，然后重新构造 `Body` 交给 `internal/proxy` 纯反代层。

## 4. 测试矩阵
我们至少覆盖以下 6 种场景，确保不同环节的行为符合预期：
- **Resolver 命中**: 请求被成功解析并返回真实模型名，标识 `IsVirtual=true`。
- **Resolver 未命中**: 请求模型不在映射表内，原样透传，标识 `IsVirtual=false`。
- **Resolver Disabled**: 命中表但 `status != 'active'`，阻断请求并返回特定 400 错误。
- **Resolver 数据库错误**: 模拟数据库读取失败，降级返回 500 避免静默错误。
- **Gateway 集成 (改写成功)**: HTTP Body 中的 model 被重写为真实模型名，且 Context 成功注入虚拟原名。
- **真实模型透传**: 不在映射表的模型走完整反向代理链路，确保不破坏现有的端到端流程。
