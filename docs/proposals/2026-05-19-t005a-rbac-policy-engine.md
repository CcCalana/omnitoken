# T-005a RBAC 三角色策略引擎 PROPOSAL

本提案只覆盖 `internal/rbac` 策略引擎与测试。`adminAuthMiddleware` 替换、JWT/session、真实 actor 来源、以及把 RBAC 挂到 `cmd/admin` 路由上，全部留给 T-005b。

## 1. 矩阵编码形态

推荐用硬编码 Go policy map，不新增 `role_permissions` 表，也不追加 migration。

实现形态建议：

```go
type Role string

const (
    RoleAdmin  Role = "admin"
    RoleMember Role = "member"
    RoleViewer Role = "viewer"
)

var policy = map[Role]map[string]bool{
    RoleAdmin:  {...},
    RoleViewer: {...},
    RoleMember: {...},
}
```

理由：

- T-005a 已知 action 少于 20 条，三角色也是 `000004_rbac_schema` 里写死的系统 invariant，用 DB 表承载权限矩阵的热改收益很低。
- 本任务明确不改 schema；硬编码矩阵能保持 migration 零变更，测试也能直接覆盖完整真值表。
- 权限矩阵一旦走 DB 热改，就需要 admin CRUD、审计、缓存失效、回滚策略和误改恢复。这些能力本身依赖 T-005b/T-013 之后的控制面，不适合作为 Phase 2-B 起点。

后续如果 Phase 3 需要自定义角色或租户级权限配置，再新增 `role_permissions` 或 `custom_roles` migration，并把当前硬编码矩阵作为默认 seed。`internal/rbac` 可以保留 `Engine.Authorize` 对外形状，只替换内部 policy source。

## 2. Action vocabulary

推荐使用单一 `action` 字符串作为授权 key，不使用 `(action, resource_type)` 复合 key。`resource_type` 继续作为 audit 事实字段存在，但不进入 T-005a 的授权矩阵。

理由是 T-005a 接口已经定为 `Authorize(ctx, actor, action)`；当前每个 action 都能表达清楚资源语义。把 `resource_type` 塞进 key 会让调用方重复维护两套事实，且容易出现 `action` 与 `audit_logs.resource_type` 不一致。需要资源实例级授权时，再在 Phase 3 增加 `AuthorizeResource(ctx, actor, action, resource)`，不要提前扩接口。

Phase 2-A/2-B v1 action 全集建议如下：

| action | audit resource_type 对齐 | admin | viewer | member | 说明 |
| --- | --- | --- | --- | --- | --- |
| `view_overview` | `admin_overview` | yes | yes | no | 组织级 dashboard，只读 |
| `view_users` | `user_usage` | yes | yes | no | 组织内用户用量列表，只读 |
| `view_models` | `model_usage` | yes | yes | no | 组织级模型用量，只读 |
| `view_audit_logs` | `audit_log` | yes | yes | no | T-014 审计页只读；payload 已脱敏，viewer 可看 |
| `view_own_usage` | `usage_event` | yes | no | yes | member 的自助用量视角；当前不接路由，给后续自助页预留 |
| `create_virtual_key` | `virtual_key` | yes | no | no | 已落库 audit action，当前 dev key 创建参考 |
| `disable_virtual_key` | `virtual_key` | yes | no | no | Phase 2-B key lifecycle 写动作 |
| `update_quota` | `user_quota` | yes | no | no | T-015 用户月度 budget/quota 写动作 |

`unknown_admin_write` 不进入 RBAC vocabulary。它是 T-013 audit middleware 的失败兜底 action；如果调用方误把它传给 RBAC，按未知 action 处理，返回 `allowed=false, reason="action_not_permitted"`。

T-016 的 provider credential CRUD 属于 Phase 2-C；等任务卡落地时再追加 `create_provider_credential` / `update_provider_credential` / `disable_provider_credential` 等 action，避免 T-005a 先替未来 CRUD 猜完整命名。

## 3. 角色查询与缓存策略

推荐每次 `Authorize` 都查询 DB，不做内存 TTL cache。

查询边界：

- 从 `users` 按 `(organization_id, id)` 找 actor。
- `users.status='disabled'` 直接拒绝，`reason="user_disabled"`。
- JOIN `role_assignments` 与 `roles` 取所有 `canonical_name`。
- 用户不存在或没有任何角色，统一拒绝为 `reason="role_not_found"`。
- 多角色用户按 `admin > viewer > member` 优先级评估；最高权限角色能允许该 action 时返回对应 allowed reason。

不缓存的理由：

- Phase 2-B admin/control-plane QPS 很低，一次 JOIN 的成本可以接受。
- 角色撤销、用户禁用、quota 修改等都是安全敏感操作。每次查 DB 可以做到立即生效，不需要设计 60s TTL 里的权限残留窗口。
- 现在还没有角色管理写接口，也就没有可靠的 cache invalidation hook。引入 TTL cache 会把 T-005b 的接线复杂度提前到 T-005a。

如果未来需要 cache，建议在 T-005b 之后加 `RoleCache` 包装 `Store`，TTL 默认 30-60s，并在 role/user 写接口成功后按 `(org_id,user_id)` 主动失效。当前 proposal 不建议把 cache 放进第一版。

## 4. reason 字段

推荐保留并强制返回低基数、稳定的 `reason`。它是 T-005b 把 RBAC 判定写入 audit `after` 的解释字段，不应只返回 bool。

建议 reason 常量：

| reason | err | 说明 |
| --- | --- | --- |
| `allowed_by_admin` | nil | admin 角色命中 |
| `allowed_by_viewer` | nil | viewer 角色命中 |
| `allowed_by_member` | nil | member 角色命中 |
| `role_not_found` | nil | 用户不存在或无角色 |
| `user_disabled` | nil | 用户存在但被禁用 |
| `action_not_permitted` | nil | action 未知，或角色存在但矩阵不允许 |
| `invalid_actor` | nil | `org_id` / `user_id` 为空 UUID |
| `role_lookup_failed` | non-nil | DB 查询失败，错误用 `%w` 包装 |

未知 action 不单独返回 `unknown_action`，统一归入 `action_not_permitted`，这样不会通过错误原因泄漏完整 action surface。允许路径返回 `allowed_by_<role>` 而不是单纯 `allowed`，是为了让 audit 中能看出多角色用户最终由哪一层权限放行；三种 allowed reason 仍然是低基数、可检索的字符串。

DB 未配置时，`NewPostgresStore(nil)` / `NewEngine(nil)` 返回 nil。调用方在 T-005b 负责 fail-closed 降级；引擎本身不在无 DB 状态下擅自 allow。

## 实施备注

- 不引第三方依赖，不使用 Casbin / OPA。
- 不改 `migrations/000004_rbac_schema.*.sql`。
- 新包 `internal/rbac` 提供 `README.md`，说明职责为 admin/control-plane RBAC policy engine。
- 默认测试用 fake store 覆盖三角色 × 全 action 真值表、unknown action、unknown user、disabled user、multi-role 最高权限、DB error、nil engine 构造。Postgres store 用现有 fake SQL driver 模式断言 JOIN 查询与参数，不引 testcontainers。
