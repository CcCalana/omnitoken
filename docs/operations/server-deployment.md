# OmniToken 服务器部署手册

目标：在一台 Linux 服务器上部署 OmniToken（DeepSeek-only），通过 nginx 反向代理对外暴露 HTTPS，供 5-10 人使用 Claude Code / Codex 测试。

## 1. 前提条件

- **服务器**: Linux（Ubuntu 22.04+ / Debian 12+），≥ 4 GB 内存，≥ 20 GB 磁盘
- **软件**: Docker 24+ + docker-compose 2+，git，openssl
- **上游**: 3 把 DeepSeek API key
- **防火墙**: 开放 443/tcp；**关闭** 8080、8081、5432、6379、4222 的公网访问

## 2. 服务器初始化

```bash
# 安装 Docker（如果未安装）
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# 重新登录使 docker 权限生效

# 克隆仓库
git clone https://github.com/CcCalana/omnitoken.git
cd omnitoken
```

## 3. 配置

### 3.1 生成 master key

```bash
openssl rand -hex 32 > .omnitoken-master-key
chmod 600 .omnitoken-master-key
```

### 3.2 生成自签 SSL 证书

```bash
mkdir -p deploy/ssl
# 把 <SERVER-IP> 替换为服务器公网 IP
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout deploy/ssl/server.key \
  -out deploy/ssl/server.crt \
  -subj "/CN=<SERVER-IP>"
chmod 600 deploy/ssl/server.key
```

### 3.3 创建 .env

```bash
cp .env.example .env
```

编辑 `.env`，填入以下内容（注释掉 Ark 相关行）：

```bash
# ===== 必填（DB/Redis/NATS/addr 已在 docker-compose.server.yml 中硬编码，.env 无需重复）=====
OMNITOKEN_MASTER_KEY_FILE=.omnitoken-master-key
OMNITOKEN_LOG_BODY_MODE=off

# ===== DeepSeek（至少 3 把 key）=====
OMNITOKEN_DEEPSEEK_KEYS_1=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
OMNITOKEN_DEEPSEEK_KEYS_2=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
OMNITOKEN_DEEPSEEK_KEYS_3=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
OMNITOKEN_DEEPSEEK_BASE_URL=https://api.deepseek.com/v1

# ===== 安全 =====
OMNITOKEN_LOG_BODY_MODE=off

# ===== Gateway =====
OMNITOKEN_CREDENTIAL_POLL_INTERVAL=30s

# ===== Admin =====
OMNITOKEN_ADMIN_SESSION_TTL=24h
# 把 <SERVER-IP> 替换为服务器公网 IP
OMNITOKEN_ADMIN_CORS_ORIGINS=https://<SERVER-IP>

# ===== Ark（DeepSeek-only 部署不用，注释掉）=====
# OMNITOKEN_ARK_KEYS_1=
# OMNITOKEN_ARK_KEYS_2=
# OMNITOKEN_ARK_KEYS_3=
# OMNITOKEN_ARK_OPENAI_BASE_URL=
# OMNITOKEN_ARK_DEFAULT_MODEL=
# OMNITOKEN_ARK_DISABLE_THINKING=true
```

## 4. 构建与启动

> **注意**: `nginx -t` 语法检查需要 SSL 证书文件存在且上游 hostname 可解析。如果 compose 未启动，`nginx -t` 会因为 `deploy/ssl/` 下无证书文件而失败。正确做法是先 `docker compose up -d nginx` 再验证，或单独准备证书文件后检查。

```bash
# 构建镜像（gateway / admin / migrate）
make docker-build

# 构建 nginx 镜像 + 启动全部服务
docker compose -f deploy/docker-compose.server.yml --env-file .env up -d --build

# 查看日志确认无报错
docker compose -f deploy/docker-compose.server.yml logs -f gateway admin
```

等 gateway 和 admin 输出 `listening on :8080` 和 `listening on :8081`。

## 5. 部署后配置

### 5.1 创建管理员账号

```bash
# 通过 admin API 获取 bootstrap 登录（dev 模式）
# 先查 admin 日志拿到 bootstrap token
docker compose -f deploy/docker-compose.server.yml logs admin | grep -i bootstrap
```

浏览器打开 `https://<SERVER-IP>` → 用 bootstrap token 登录 → 进入 session。

### 5.2 调整 virtual_models（DeepSeek-only）

seed SQL 中只有 `chat-fast` 默认走 DeepSeek，其他四个走 Ark。DeepSeek-only 部署需要全部改到 DeepSeek。

在 admin web → 虚拟模型映射 tab → 逐个编辑（或通过 API）：

| name | real_model | provider |
|---|---|---|
| chat-fast | deepseek-v4-flash | deepseek |
| chat-balanced | deepseek-v4-pro | deepseek |
| chat-quality | deepseek-v4-pro | deepseek |
| chat-code | deepseek-v4-pro | deepseek |
| chat-reasoning | deepseek-v4-pro | deepseek |

> **注意**: DeepSeek 的实际模型名可能变化。`deepseek-v4-flash` / `deepseek-v4-pro` 是示例，请根据 DeepSeek 官方文档确认当前模型名。

### 5.3 创建虚拟 Key

admin web → API Keys tab → 为每个测试用户创建一个 virtual key。
记下生成的 key（只显示一次）。

## 6. 客户端配置（测试人员）

### 6.1 信任自签证书

测试人员需要先信任服务器的自签证书，否则 Claude Code / Codex 会 TLS 报错。

**方案 A：系统级信任（推荐，一劳永逸）**

```bash
# macOS
sudo security add-trusted-cert -d -r trustRoot \
  -k /Library/Keychains/System.keychain deploy/ssl/server.crt

# Linux (Ubuntu/Debian)
sudo cp deploy/ssl/server.crt /usr/local/share/ca-certificates/omnitoken.crt
sudo update-ca-certificates

# Windows
certutil -addstore -f "ROOT" server.crt
```

**方案 B：环境变量（不修改系统）**

```bash
export NODE_EXTRA_CA_CERTS=/path/to/server.crt
```

> 注意：方案 B 仅对 Node.js 应用（Claude Code）生效。Codex 是 Go 二进制，需方案 A 或用 `SSL_CERT_FILE`。

### 6.2 配置 Agent

```bash
# Claude Code
omnitoken-adopt adopt claude-code \
  --gateway-url https://<SERVER-IP> \
  --token <your-virtual-key> \
  --model chat-fast \
  --admin-url https://<SERVER-IP> \
  --real-model deepseek-v4-flash \
  --provider deepseek

# Codex
omnitoken-adopt adopt codex \
  --gateway-url https://<SERVER-IP> \
  --token <your-virtual-key> \
  --model chat-fast
```

### 6.3 验证

```bash
# 非流式
curl -s https://<SERVER-IP>/v1/messages \
  -H "x-api-key: <your-virtual-key>" \
  -H "Content-Type: application/json" \
  -d '{"model":"chat-fast","max_tokens":16,"messages":[{"role":"user","content":"say hi"}]}' \
  --cacert deploy/ssl/server.crt | jq .

# 应该返回: {"type":"message","role":"assistant","content":[...],"usage":{...}}
```

## 7. 运维

### 日常命令

```bash
# 查看日志
docker compose -f deploy/docker-compose.server.yml logs -f --tail=100

# 重启 gateway（改 credential 后）
docker compose -f deploy/docker-compose.server.yml restart gateway

# 更新代码
git pull
make docker-build
docker compose -f deploy/docker-compose.server.yml up -d gateway admin

# 查看 PG 连接数
docker compose -f deploy/docker-compose.server.yml exec postgres \
  psql -U postgres -d omnitoken -c \
  "SELECT application_name, state, count(*) FROM pg_stat_activity GROUP BY 1,2"
```

### 性能参考（基于 T-CONC-RERUN 实测）

| 并发量 | DeepSeek 成功率 | 说明 |
|---|---|---|
| 5-10 用户（~10-20 req/s） | 预期 100% | 5-10 人各自跑 agent（agent 的 think 时间稀释实际并发） |
| 30 并发（~25 req/s） | 100.0% | T-MP-DEEPSEEK 实测 |
| 50+ 并发（~210 req/s mock） | PG 开始饱和 | 需关注 `pg_stat_activity`，必要时调大 `max_connections` |

### 故障排查

| 现象 | 可能原因 | 检查方法 |
|---|---|---|
| 客户端 TLS 报错 | 自签证书未信任 | `curl --cacert deploy/ssl/server.crt ...` 先排除 |
| `x-api-key` 401 | virtual key 无效或过期 | 查 admin → API Keys → 状态是否 active |
| `model not found` 404 | virtual_models 里没这个 name | admin → 虚拟模型映射 → 确认 name 存在 |
| SSE 流式卡住 | nginx 缓冲未关闭 | 确认 `proxy_buffering off` |
| gateway 5xx | 上游 DeepSeek rate limit | `docker compose logs gateway \| grep "upstream credential retry"` |
