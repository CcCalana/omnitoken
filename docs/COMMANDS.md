# OmniToken 命令与操作索引

> 要跑什么？在本文件中搜索关键词 → 复制命令。本文件 ~100 行，廉价可读。

## 开发环境

```bash
# 启动全部服务（PG + Redis + NATS + gateway:8080 + admin:8081）
make up

# 停止全部服务
make down

# 查看日志
make logs

# 重新构建镜像后启动
make docker-build && make up
```

## 测试

```bash
# 全部单元测试（不依赖外部服务）
make test

# 仅 vet 检查
make lint

# 带 race detector（Docker 内跑，无需本地 gcc）
make test-race

# 单元测试 + race（Docker 容器内）
docker compose --env-file .env -f deploy/docker-compose.yml run --rm test go test ./...

# 单包测试 + 覆盖率
go test -cover ./internal/proxy/

# e2e 集成测试（需先 make up）
go test -tags=e2e -count=1 ./internal/proxy/
```

## Docker Compose 服务

| 服务 | 命令 | 说明 |
|---|---|---|
| 起 PG/Redis/NATS | `docker compose up -d postgres redis nats` | 仅基础设施 |
| 跑迁移 | `docker compose run --rm migrate` | 执行所有 up migration |
| 种子数据 | `docker compose run --rm seed` | 插入 demo 组织/用户/角色 |
| 凭据加密入库 | `docker compose run --rm credential-seed` | 从 .env 读 key，AES 加密写入 |
| 起 Gateway | `docker compose up -d gateway` | :8080 |
| 起 Admin | `docker compose up -d admin` | :8081 |
| Mock 上游 | `docker compose up -d mock-ark` | :18090，用于压测 |
| 全栈测试 | `docker compose run --rm test go test ./...` | golang:1.25 容器内跑 |

Docker Compose 配置文件：`deploy/docker-compose.yml`（项目名 `omnitoken`）

## CLI 入口（cmd/）

| 入口 | 用途 | 关键参数 |
|---|---|---|
| `cmd/gateway/main.go` | API 代理网关，SSE 透传 + token 统计 | 配置文件 / env |
| `cmd/admin/main.go` | 管理 API，RBAC + audit + 凭据 CRUD | 配置文件 / env |
| `cmd/migrate/main.go` | 数据库迁移，支持 up/down/version | `migrate up` |
| `cmd/loadtest/main.go` | 并发压测工具 | `-concurrency 30 -duration 30s` |
| `cmd/loadtest/mockark/main.go` | Mock 上游（<1ms 响应） | 用于压测 gateway |
| `cmd/e2e-runner/main.go` | T-100 L2 多租户正确性套件 | `--gateway-url --admin-url --admin-token --database-url` |
| `cmd/upstream-credential-seed/main.go` | 读取 env 中 key → 加密写入 PG | `credential-seed` |
| `cmd/omnitoken-adopt/main.go` | Agent 适配器（Claude/Codex/OpenCode） | `omnitoken-adopt adopt <agent>` |

## 脚本（scripts/）

| 脚本 | 用途 | 运行方式 |
|---|---|---|
| `scripts/dev.ps1` | Windows 开发命令（镜像 make） | `.\scripts\dev.ps1 up` |
| `scripts/e2e_verify.py` | E2E 全链路验证 | `python scripts/e2e_verify.py` |
| `scripts/run_e2e.py` | 运行 e2e 测试套件 | `python scripts/run_e2e.py` |
| `scripts/v1_integration.py` | v1 控制面联调 smoke | `python scripts/v1_integration.py` |

## 前端管理控制台

```bash
cd web
python -m http.server 3000
# 浏览器打开 http://localhost:3000/?admin=http://localhost:8081
# 登录: admin@democorp.local / password
```

## 常用操作速查

```bash
# 全新环境一键起
cp .env.example .env      # 编辑填入 key
make up                    # 起全部服务

# 改了 schema 后重新迁移
docker compose -f deploy/docker-compose.yml run --rm migrate up

# 新增上游 key 后
# 1. 编辑 .env 填入新 key
# 2. docker compose run --rm credential-seed
# 3. docker compose restart gateway   （或等 30s 热加载）

# 只重跑 gateway（不改代码）
docker compose up -d --force-recreate gateway
```

## 成本估算

| 操作 | 估算开销 |
|---|---|
| 单元测试 `go test ./...` | 0 元（无上游调用） |
| E2E 测试 (L2) | < ¥5 / 次 |
| loadtest 30 并发 × 30s | < ¥1 |
