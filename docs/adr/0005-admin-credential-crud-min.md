# ADR 0005: Admin Credential CRUD UI (Minimal) + Hot Reload via Polling

Status: Accepted, 2026-05-23

## Context

R-MP-DEEPSEEK 之后用户提出 v1 上线 UX gap：所有 upstream credential 必须改 `.env` → docker restart → re-seed。"前端配置 key 的便捷性" 是 v1 上线必备而非 v1.1+ 项。

ADR 0003/0004 原本把 admin CRUD UI 推到 v1.1+，理由是 master key trust boundary + 热加载复杂度 + scope 控制。重新评估：

- **trust boundary**: admin 进程共享 `OMNITOKEN_MASTER_KEY` env 是 v1 简化的合理 trade-off（v1 已有 ops 文档；KMS 仍 v1.1+）。用户明确认可。
- **热加载**: 用 PG 轮询（30s tick + SELECT WHERE updated_at > last_seen + 原子 swap selector pool）规避 LISTEN/NOTIFY 复杂度；polling 在已有 PG 连接上做，无新依赖。
- **scope**: 切掉 UPDATE（disable + new add 等价覆盖）、admin 端 master key 管理、key 健康检查 worker、自动 rotation。只做 ADD + DISABLE + LIST + 30s reload。

## Decision

把 **admin credential CRUD UI 的最小子集** 拉进 v1，配套 **PG 轮询热加载**。任务标识 T-016b-MIN。

### v1 范围

- ✅ 已有：`GET /admin/credentials` 只读列表（RBAC 保护，返回 `PublicCredential` 不漏密文）
- ✅ 新增 backend：`POST /admin/credentials` (create) + `PATCH /admin/credentials/:id/disable` (status:disabled)。两个端点都走 admin role RBAC（T-005a）+ 写 audit_logs（T-013）。
- ✅ 新增 frontend：admin web 加 "Upstream Credentials" tab，含列表 + 新增表单 modal + 禁用按钮。Mutation 后顶部 banner："已写入数据库，gateway 将在 30s 内自动加载新池；如需即时生效，手动 restart gateway。"
- ✅ 新增 gateway：credentials 池后台轮询（30s tick），按 `updated_at` 增量识别，原子 swap selector 池；新增 env `OMNITOKEN_CREDENTIAL_POLL_INTERVAL`（默认 `30s`，可 0 关闭）。
- ✅ admin 进程加 `OMNITOKEN_MASTER_KEY` / `OMNITOKEN_MASTER_KEY_FILE` env 注入（与 gateway 同一把），用于 POST 时 encrypt 入库。
- ⏭ v1.1+: UPDATE credential / DELETE / master key 自动 rotation / KMS / 健康检查 worker / 多 admin 角色细分 / cross-provider cost-aware routing UI

### 为什么这次能进 v1

- B 的写路径很薄：T-016 的 schema + envelope.Encrypt + audit_logs middleware 全部现成，admin handler 只需 ~50 行新增 + validation。
- B2 polling 是单 goroutine + 已有 selector mutex 之上的 atomic swap，gateway 增量 < 80 行。
- frontend 已有 audit/users/models 三 tab 的模板，credentials tab 复用。
- 总 ETA 增量 ~2 天，v1 ETA 在 R-MP-DEEPSEEK 收官的当下接受。

## Consequences

### 正面

1. **v1 上线 ops UX 闭环**：加 key / 禁用 key 全部走 admin web，不再要求 SSH + vi .env。
2. **热加载默认开启**：30s 内新池自动生效，运维改完不必手动 restart gateway。
3. **不引入新依赖**：polling 在 PG 现有连接上做，不引 LISTEN/NOTIFY / 消息总线。
4. **trust boundary 显式**：admin 共享 master key 在 ops 文档中明确，v1.1 接 KMS 时一并迁移。
5. **scope 严格**：只做 ADD + DISABLE，不做 UPDATE，避开 master key 重加密的复杂分支。

### 负面

1. **v1 ETA +2 天**：从 R-MP-DEEPSEEK 当下到 v1 上线 ready 时刻延后 2 天。
2. **admin 进程多一个敏感 env**：`OMNITOKEN_MASTER_KEY` 现在 gateway 和 admin 两处都要注入。运维心智成本提升；docker-compose 要明确两处。
3. **polling 有 30s 延迟**：用户加完 key 可能等最长 30s 才生效。可调整 interval 但越短 DB 压力越大。30s 是 v1 默认折中。
4. **UPDATE 缺失**：用户改 priority / base_url 必须先 disable 再 add new。少数场景体验差。

### 影响范围

| 层 | 改动 |
|---|---|
| Schema | `upstream_credentials.updated_at` 已存在（000001 起）；migration 000014 可选加 trigger 自动维护，或在 UPDATE/INSERT 语句里显式 set |
| Backend (admin) | `cmd/admin`: POST/PATCH 两个 handler + validation；`OMNITOKEN_MASTER_KEY` env 加载；envelope.Encrypt 复用 |
| Backend (gateway) | `internal/credentials`: selector.Replace 或同等 atomic swap；新增 polling goroutine（在 cmd/gateway main 启动）；env `OMNITOKEN_CREDENTIAL_POLL_INTERVAL` |
| Frontend | `web/src/`: 加 Upstream Credentials tab；新增 form modal；调用现有 `/admin/credentials` GET + 新 POST/PATCH