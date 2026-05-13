# OmniToken

语言：[English](README.md) | 简体中文

OmniToken 是面向企业内部平台团队的 AI 访问网关。它负责签发虚拟 API Key、把 OpenAI-compatible 请求转发到上游模型厂商、记录 token 与成本账本，并为管理员提供用户、Key、模型与预算治理的控制面。

长期定位要刻意收窄：OmniToken 不打算做“模型最多的聚合市场”。它要做的是企业内部 AI 访问控制与成本账本层，让公司能清楚知道谁、用哪个 Key、在什么策略下、调用了哪个模型、产生了多少成本，同时不泄露真实厂商 Key 或默认记录 Prompt 全文。

> Phase 1 状态：Demo-Ready。本地端到端流程已经可跑通，但 dev virtual-key endpoint 不是生产注册系统。完整 admin 鉴权、RBAC、配额执行、生产 Key 生命周期等仍在后续任务中。

## 为什么是 OmniToken

现在市面上的 AI Gateway 产品大多收敛在几个常见形态：

| 市场形态 | 代表产品 | 强项 | OmniToken 的差异化 |
| --- | --- | --- | --- |
| 多模型代理 | [LiteLLM](https://docs.litellm.ai/docs/proxy_server), [New API](https://github.com/QuantumNous/new-api) | 多厂商、OpenAI-compatible 路由、虚拟 Key、预算、重试 | 初期不追求最广模型覆盖，而是优先做企业账本：用户/项目/Key 归因、provider-specific token 拆分、可审计成本记录。 |
| 托管开发者网关 | [Vercel AI Gateway](https://vercel.com/docs/ai-gateway), [Cloudflare AI Gateway](https://developers.cloudflare.com/ai-gateway/) | 上手快、托管式观测、缓存、迁移 base URL 简单 | 默认自托管，更适合企业内部安全边界、私有成本中心与可控数据保留策略。 |
| API Gateway 插件套件 | [Kong AI Gateway](https://docs.konghq.com/gateway/latest/get-started/ai-gateway/), [Envoy AI Gateway](https://aigateway.envoyproxy.io/) | 成熟流量治理、插件生态、Kubernetes 原生 | 不从“通用网关”切入，而从 AI 治理切入：虚拟 Key 策略、成本核算、管理流程是一等能力。 |
| 开发者门户 / API 产品平台 | [APIPark](https://github.com/APIParkLab/APIPark), 企业 API Portal | API 申请、订阅、审批、开发者 onboarding | 未来门户围绕内部 AI 使用：申请访问、签发 scoped key、执行模型/预算策略、提供分摊和审计证据。 |
| LLM 可观测与实验平台 | [Helicone](https://docs.helicone.ai/getting-started/integration-method/gateway), [TensorZero](https://www.tensorzero.com/docs/gateway) | 请求日志、trace、prompt、实验、反馈闭环 | 可观测先服务成本与安全。默认不采集 Prompt 全文；优先做账本、脱敏、审计，再考虑实验优化。 |

换句话说，OmniToken 应该把五件事做深：

1. 企业虚拟 Key：组织、项目、用户、Key 前缀、状态、过期时间、模型白名单、预算、RPM/TPM、轮换。
2. 精确 AI 成本账本：prompt、completion、cached、reasoning、多模态、provider、请求模型、实际上游模型、延迟、状态、结算状态。
3. 安全默认网关：不暴露厂商 Key、不记录完整 Authorization 头、不默认记录 Prompt 全文、错误 envelope 统一。
4. 内部管理流程：注册或邀请、用户设置、Key 签发、用量查看、预算审查，未来补审批与审计。
5. 自托管控制面：Go data plane、Postgres 账本、Docker/Kubernetes 部署路径，以及可替换的静态 admin console。

## 最终产品流程

一个企业管理员最终应该能这样使用 OmniToken：

1. 注册或创建组织。
2. 邀请用户，或从企业身份系统同步用户。
3. 添加上游模型厂商凭证，并安全存储。
4. 创建项目与模型访问策略。
5. 给用户、应用或服务账号签发虚拟 Key。
6. 让团队统一调用一个 OpenAI-compatible gateway endpoint。
7. 执行配额、模型白名单、预算限制与速率限制。
8. 按组织、项目、用户、Key、模型、厂商、时间窗口查看用量。
9. 导出审计与成本数据，用于 FinOps、安全审查和内部成本分摊。

当前 demo 已实现这条路径中的最小可用闭环：

1. 使用 seed 中的组织与用户。
2. 填入上游 Ark API Key。
3. 创建一个虚拟 Key。
4. 通过网关发起一次 chat completion。
5. 在 admin API 与 web console 中看到真实用量。

## 架构

```mermaid
flowchart LR
  User["带虚拟 Key 的用户或应用"] --> Gateway["Gateway :8080"]
  Gateway --> Auth["虚拟 Key 鉴权"]
  Gateway --> Ark["火山方舟 OpenAI-compatible API"]
  Gateway --> Usage["usage_events + token breakdown + cost_ledger"]
  Admin["Admin API :8081"] --> Usage
  Web["web/ 静态控制台"] --> Admin
  Postgres[(Postgres)] --> Auth
  Postgres --> Usage
```

## 当前已支持

- OpenAI-compatible `GET /v1/models`
- OpenAI-compatible `POST /v1/chat/completions`
- 通过 admin 服务创建 demo virtual key
- Docker Compose 执行 Postgres migration 与 seed
- Chat completion 后写入 usage 与 cost ledger
- Admin overview/users/models API
- `web/` 静态 admin console

## 环境要求

- Docker Desktop 或兼容 Docker engine
- Go 1.23+，用于本地测试与工具
- Python 3，用于启动静态 web console
- `curl`
- `make` 可选；Windows 用户可使用 `scripts/dev.ps1`

## 快速开始

### 1. 创建 `.env`

PowerShell：

```powershell
Copy-Item .env.example .env
notepad .env
```

Bash：

```bash
cp .env.example .env
${EDITOR:-vi} .env
```

至少填写这些值：

```dotenv
OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN=dev-bootstrap-token-change-me
OMNITOKEN_ARK_API_KEY=<your-volcano-ark-dev-key>
OMNITOKEN_ARK_DEFAULT_MODEL=ark-code-latest
OMNITOKEN_ADMIN_CORS_ORIGINS=http://localhost:3000
```

不要提交 `.env`。它已经被 git ignore。

### 2. 启动本地栈

```powershell
make up
```

Windows fallback：

```powershell
.\scripts\dev.ps1 up
```

该命令会构建 gateway/admin/migrate 镜像，启动 Postgres/Redis/NATS，执行数据库迁移，应用 `deploy/postgres/002_seed.sql`，并启动：

| 服务 | 地址 |
| --- | --- |
| Gateway | `http://localhost:8080` |
| Admin API | `http://localhost:8081` |
| Postgres | `localhost:15432` |
| Redis | `localhost:16379` |
| NATS | `localhost:14222` |

健康检查：

```powershell
curl.exe http://localhost:8080/healthz
curl.exe http://localhost:8081/healthz
```

### 3. 使用 seed 中的 demo 租户

Seed 文件会创建一个组织、一个项目和 11 个 demo 用户。第一次试用建议使用 demo admin：

| 字段 | 值 |
| --- | --- |
| Organization | `Demo Organization` |
| Organization ID | `00000000-0000-0000-0000-000000000001` |
| Project | `Default Project` |
| Project ID | `00000000-0000-0000-0000-000000000101` |
| Demo Admin | `admin@democorp.local` |
| Demo Admin User ID | `00000000-0000-0000-0000-000000000201` |

当前还没有公开注册接口。本地 demo 里的“注册”指使用 seed 租户，或手动向 Postgres 插入新用户。

### 4. 为用户创建虚拟 Key

PowerShell：

```powershell
$AdminToken = "dev-bootstrap-token-change-me"
$Body = @{
  organization_id = "00000000-0000-0000-0000-000000000001"
  user_id = "00000000-0000-0000-0000-000000000201"
} | ConvertTo-Json

$KeyResponse = Invoke-RestMethod `
  -Method Post `
  -Uri "http://localhost:8081/api/admin/dev/virtual-keys" `
  -Headers @{ Authorization = "Bearer $AdminToken" } `
  -ContentType "application/json" `
  -Body $Body

$VirtualKey = $KeyResponse.virtual_key
$KeyResponse | ConvertTo-Json
```

Bash：

```bash
curl -sS -X POST http://localhost:8081/api/admin/dev/virtual-keys \
  -H "Authorization: Bearer dev-bootstrap-token-change-me" \
  -H "Content-Type: application/json" \
  -d '{"organization_id":"00000000-0000-0000-0000-000000000001","user_id":"00000000-0000-0000-0000-000000000201"}'
```

响应里包含 `virtual_key`。请立刻保存；明文 secret 只会在这次响应中返回。它以 `omt_` 开头。

### 5. 用户试用：调用 gateway

查看模型列表：

```powershell
curl.exe http://localhost:8080/v1/models `
  -H "Authorization: Bearer $VirtualKey"
```

发送非流式 chat completion：

```powershell
curl.exe -X POST http://localhost:8080/v1/chat/completions `
  -H "Authorization: Bearer $VirtualKey" `
  -H "Content-Type: application/json" `
  -d '{"model":"ark-code-latest","messages":[{"role":"user","content":"Output exactly: pong"}],"stream":false,"max_tokens":32}'
```

流式 SSE 示例：

```powershell
curl.exe --no-buffer -X POST http://localhost:8080/v1/chat/completions `
  -H "Authorization: Bearer $VirtualKey" `
  -H "Content-Type: application/json" `
  -d '{"model":"ark-code-latest","messages":[{"role":"user","content":"Count from 1 to 5."}],"stream":true,"max_tokens":64}'
```

Gateway 会把虚拟 Key 留在 OmniToken 内部，只向上游注入真实 Ark Key，并在响应后写入用量账本。

### 6. 验证用量

等待 deferred ledger write 完成，再查询 admin API：

```powershell
Start-Sleep -Seconds 2
curl.exe http://localhost:8081/api/admin/overview
curl.exe http://localhost:8081/api/admin/users
curl.exe http://localhost:8081/api/admin/models
```

预期信号：

- `total_tokens > 0`
- `active_users >= 1`
- `model_usage` 包含 Ark-backed 模型
- `users` 中 demo admin 用户出现非零用量

Phase 1 当前说明：今天只有 dev key creation endpoint 要求 bootstrap token。Admin read endpoints 会在 T-010 中补全鉴权。

### 7. 打开 web console

在 `localhost:3000` serve `web/`，这与默认 admin CORS allow-list 一致：

```powershell
cd web
python -m http.server 3000
```

打开：

```text
http://localhost:3000/?admin=http://localhost:8081
```

控制台包含三个视图：

- Overview：月度 tokens、预估成本、活跃用户、趋势、模型占比
- Users：按用户展示 token 用量与配额占位
- Models：prompt/completion token 拆分、成本与调用次数

## 添加另一个本地 demo 用户

打开 psql：

```powershell
docker compose --env-file .env -f deploy/docker-compose.yml exec postgres `
  psql -U omnitoken -d omnitoken
```

插入用户并授予 seed 中的 member 角色：

```sql
INSERT INTO users (organization_id, email, display_name)
VALUES (
  '00000000-0000-0000-0000-000000000001',
  'new.user@democorp.local',
  'New Demo User'
)
RETURNING id;

INSERT INTO role_assignments (organization_id, user_id, role_id)
SELECT
  '00000000-0000-0000-0000-000000000001',
  users.id,
  roles.id
FROM users
JOIN roles ON roles.canonical_name = 'member'
WHERE users.email = 'new.user@democorp.local'
ON CONFLICT (organization_id, user_id, role_id) DO NOTHING;
```

把返回的 `users.id` 用在 virtual-key 创建请求里。

## 常用命令

| 目标 | 命令 |
| --- | --- |
| 启动本地栈 | `make up` |
| 停止本地栈 | `make down` |
| 查看日志 | `make logs` |
| Windows 启动 | `.\scripts\dev.ps1 up` |
| 重置本地数据卷 | `docker compose --env-file .env -f deploy/docker-compose.yml down -v` |
| 运行 Go 测试 | `go test -count=1 ./...` |
| 运行 vet | `go vet ./...` |
| Race 测试 | `make test` |

## 常见问题

### Gateway 返回 `401 invalid_api_key`

请使用 `/api/admin/dev/virtual-keys` 返回的完整 `virtual_key`，不要使用 `key_prefix`。Key 应该以 `omt_` 开头。

### Gateway 无法访问上游模型厂商

确认 `.env` 中填写了 `OMNITOKEN_ARK_API_KEY`，然后重建 gateway：

```powershell
make up
```

### Web console 出现 CORS 错误

从 `http://localhost:3000` serve console，或把你的 origin 加入 `.env` 中的 `OMNITOKEN_ADMIN_CORS_ORIGINS`，然后重启 admin：

```dotenv
OMNITOKEN_ADMIN_CORS_ORIGINS=http://localhost:3000,http://127.0.0.1:3000
```

### Admin 图表为空

先跑通至少一次 `/v1/chat/completions`，等待一到两秒让 usage recording 完成，再刷新 web console。

### 从干净数据库重新开始

```powershell
docker compose --env-file .env -f deploy/docker-compose.yml down -v
make up
```

## 仓库结构

| 路径 | 职责 |
| --- | --- |
| `cmd/gateway` | OpenAI-compatible gateway |
| `cmd/admin` | Admin API 与 dev virtual-key endpoint |
| `cmd/migrate` | golang-migrate wrapper |
| `internal/auth` | Virtual-key 生成与鉴权 middleware |
| `internal/proxy` | Ark chat-completions proxy |
| `internal/usage` | Usage 解析、记录与 cost ledger 写入 |
| `migrations` | 数据库 schema migrations |
| `deploy` | Dockerfiles、Compose 与 seed SQL |
| `web` | 静态 admin console |
| `docs/runbooks/local-dev.md` | 本地开发 runbook |

## 安全说明

- 不要提交 `.env`、厂商 Key、虚拟 Key 或完整 Authorization 头。
- `deploy/postgres/002_seed.sql` 里的本地 demo 定价是占位数据，不能用于商业报价。
- Dev virtual-key endpoint 不是生产 admin API。
- 日志设计上不打印厂商 Key、虚拟 Key 或 Prompt 正文。
