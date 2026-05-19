# T-015 用户月度 budget + quota 编辑 PROPOSAL

本提案覆盖用户级月度 budget、gateway 402 硬拦截、admin quota 编辑 API 与 Users tab 编辑形态。T-005b 负责真实登录态、RBAC 强制执行和 `/api/admin/me`；T-015 只按 bootstrap 阶段的 admin 假设把全栈 budget 能力先落地。

## 1. Budget 字段单位

推荐在 `users` 表新增 `monthly_budget_cents bigint`，`NULL` 表示无限制，非空必须 `>= 0`。不复用 `api_keys.monthly_budget_usd numeric(18,6)`，因为那是 key 级旧预留字段；T-015 的产品语义是管理员给员工划线，应该落在 user 维度。

选择 USD cents，而不是 token 数：

- Admin 页面当前已经展示 `estimated_cost_usd`，`cost_ledger.cost_usd` 也是最终结算事实；企业 budget 的自然语言是钱，不是 token。
- token 数不适合多模型定价：同样 100k tokens 在不同模型、input/output、reasoning/cache 组合下成本不同，未来 provider 扩展后会更不准。
- `bigint cents` 比 `numeric usd` 更适合作为人工配置字段：表单输入、JSON、比较、测试都稳定，避免浮点与 decimal 展示误差。检查时不要把已用成本先向下取整；用 `COALESCE(SUM(cl.cost_usd), 0) >= (monthly_budget_cents::numeric / 100)` 做 exact numeric 比较。

建议 migration `000008_user_monthly_budget.up/down.sql`：

- `ALTER TABLE users ADD COLUMN monthly_budget_cents bigint CHECK (monthly_budget_cents IS NULL OR monthly_budget_cents >= 0);`
- down 删除该列。

Admin API 建议返回低歧义字段；展示用的 `used_budget_cents` 用 `CEIL(SUM(cost_usd) * 100)::bigint`，避免 sub-cent 用量在 UI 上被低估：

```json
{
  "users": [{
    "user_id": "...",
    "used_tokens": 12345,
    "used_budget_cents": 37,
    "budget_cents": 500,
    "status": "active"
  }]
}
```

`used_tokens` 保留，避免破坏现有 Users tab；`quota` 可在 v1 前端里作为兼容别名读取，但新字段以 `*_cents` 为准。

## 2. 检查时机与一致性

推荐新增 `internal/quota` 的 DB-backed checker，由 gateway 在虚拟 key 鉴权成功后、调用上游前执行：

1. 从 `auth.SubjectFromContext` 取 `org_id + user_id`。
2. 查用户 `monthly_budget_cents`。
3. 若 budget 为 `NULL`，放行。
4. 聚合当前自然月 `usage_events` + `cost_ledger` 的 `SUM(cost_usd) * 100`。
5. 若 `used_cents >= monthly_budget_cents`，返回 402；否则放行。

不建议把检查放在 usage recorder 入账前的 goroutine 中，因为那时上游请求已经完成，无法阻止本次消费；也不建议 gateway 入口做 token 预估，当前请求体没有可靠 max-cost，模型实际与 usage 也要等上游响应。v1 接受一个明确 tradeoff：当前请求可能把用户从 99.9% 推到 100% 以上，下一次请求会被 402 拦住。

不引 Redis Lua 原子扣减。规划里 Redis 适合 RPM/TPM 或高并发预扣，但 T-015 的 v1 目标是发布候选里的员工月度预算，admin/gateway 单实例和低 QPS 是默认假设。用 DB SUM 的好处是没有新依赖、没有双写账本、和 `cost_ledger` 事实源保持一致；缺点是并发下可能有小额 overshoot，T-INT 记录为 v1 已知限制，vNext 再做 Redis pre-deduct 或 DB reservation。

## 3. 402 envelope 形状

推荐复用 gateway 401 的 envelope 形状：

```json
{
  "error": {
    "message": "monthly budget exhausted",
    "type": "quota_exceeded",
    "code": "monthly_budget_exhausted"
  }
}
```

HTTP status 用 `402 Payment Required`。不在错误 body 中返回 budget、used、user_id、org_id 或 key prefix，避免把成本线索扩散到 data-plane 调用方；定位问题通过 admin Users tab 和日志。日志可记录低敏字段 `request_id`、`user_id`、`organization_id`、`used_cents`、`budget_cents`，但仍不能记录 Authorization、virtual key 明文或 prompt。

Quota checker DB 异常建议 fail-closed，返回 500 `quota_check_failed`，不默认放行。原因是 budget 是 v1 上线口径的一条硬控制线；数据库不可用时继续放行会悄悄烧钱。无 budget 配置时才是显式无限制。

## 4. 前端编辑形态与角色判断

推荐 Users tab 使用 inline cell edit，而不是弹窗。当前页面已经是表格型操作台，单字段金额编辑用 inline input 更快，且不需要新增复杂 modal 状态。交互形态：

- `budget_cents == null` 显示“无限制”。
- used/budget 用金额和进度条展示，`>=85%` warning，`>=100%` danger。
- admin 看到编辑按钮，点击后把该行切成金额输入；保存调用 `PATCH /api/admin/users/:id/quota`，成功后刷新 Users 数据。
- viewer 不显示编辑按钮。

角色来源推荐前端读 `/api/admin/me`，由 T-005b 提供真实 role/session。T-015 实施期先用一个可注入的 role provider：bootstrap/dev 模式默认 `admin`，前端测试显式传入 `admin` / `viewer` 锁住渲染差异。不要让后端按角色返回不同字段集；GET `/api/admin/users` 保持同一数据形状，权限只影响“能否执行写动作”。这样 T-005b 接上真实 `/me` 后，只替换 role provider，不重写 Users tab。

`PATCH /api/admin/users/:id/quota` 请求体建议：

```json
{ "budget_cents": 500 }
```

`budget_cents: null` 表示清除预算、恢复无限制。该写动作设置 audit：`action=update_quota`、`resource_type=user_quota`、`resource_id=<user_id>`、`before/after` 只包含 `budget_cents`，不包含密钥或 prompt。T-015 阶段仍走 bootstrap token；T-005b 会把 RBAC `update_quota` 和真实 actor 接上。

## 实施备注

- 不新增第三方依赖。
- migration 只新增 `000008`，不回改历史 migration。
- `internal/quota` 由 README 中的“Redis-backed”预期收敛为 v1 DB checker；README 需更新为“DB-backed monthly budget now, Redis RPM/TPM later”。
- 测试覆盖：budget 充足放行、budget 不足 402、`NULL` budget 放行、quota update SQL 与 audit snapshot、前端 admin/viewer 渲染差异、checker DB error fail-closed。
