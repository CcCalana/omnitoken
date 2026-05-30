# T-014 异常 key 阈值告警 PROPOSAL

T-014 的 audit 查询 API 与前端 Audit tab 按既有 `users/models` 模式直接实施；本提案只覆盖需要先决策的异常 key 阈值告警段。

## 1. 告警信号源

推荐采用定时 goroutine 扫 `usage_events`，每 5 分钟查询过去 5 分钟内按 `api_key_id` 聚合的调用次数，并与阈值比较后打 WARN log。

不推荐在 usage middleware 入账时直接计数。原因是 middleware 计数会把告警状态放到请求热路径上，且必须处理并发、进程内状态、失败入账回滚和多实例分片问题；定时扫描以已经落库的 `usage_events` 为事实源，和现有 usage 入账边界解耦，也不会增加 gateway 请求延迟。

5 分钟滞后在 Phase 2-A 可接受：本任务目标是“异常 key 用量告警雏形”，不是实时熔断或风控拦截。后续若 T-015 budget/RPM 需要硬拦截，应在请求路径做 quota/rate-limit，而不是复用这个 WARN-only 机制。

## 2. 去抖与状态存储

推荐使用 admin 进程内存 map 去抖，key 为 `api_key_id + window_start`。同一 key 在同一个 5 分钟窗口只 WARN 一次；窗口推进后可再次告警。map 由 mutex 保护，并在每轮扫描后清理早于当前窗口的历史项。

这条方案的明确假设：Phase 2-A 默认单 admin 实例，重启后去抖状态丢失可接受；多实例部署时可能重复 WARN，留到 Phase 3 用 Redis/DB lease 或专门告警管道解决。当前不引 Redis，不新增依赖，也不把状态写进 `audit_logs`。

## 3. 阈值配置形态

推荐默认阈值硬编码为 `100`，并允许通过 `OMNITOKEN_ADMIN_KEY_ANOMALY_RPM_5M` 覆盖。配置解析遵守现有 `OMNITOKEN_` 前缀规则：未配置使用默认值，配置为非正数或非法整数时记录一条 WARN 并回退默认值。

Phase 2-A 不做 admin 可改阈值、按组织/项目/用户分层阈值、动态热更新或数据库配置表。这些都需要 RBAC、审计和前端配置页配套，放到 Phase 3 或 T-015/T-005b 之后再设计。

## 4. 告警 payload 字段白名单

WARN log payload 只允许包含：

- `api_key_prefix`
- `window_start`
- `count`
- `threshold`

明确不进 log 的字段包括完整 virtual key、Authorization header、prompt/response 全文、`api_key_id`、`user_id`、`organization_id`、请求体、上游 key、模型响应内容。`api_key_prefix` 只用于定位是哪把 key 出问题，与现有 dev-vkey 返回值和 T-013 audit snapshot 一致；其余身份归因等更细维度留给后续审计页/权限系统，不在 WARN payload 中扩散。
