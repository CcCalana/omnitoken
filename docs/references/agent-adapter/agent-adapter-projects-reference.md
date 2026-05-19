# Agent 适配项目全景参考 — 博采众长

> 基于 GitHub 开源项目全面搜索与源码分析  
> 与 `agent-adapter-pattern.md`（tingly-box 专项）和 `tingly-box-architecture-reference.md` 互补

---

## 一、项目分类总览

| 类别 | 数量 | 核心价值 |
|---|---|---|
| **通用 AI Gateway** | 8+ | 架构设计、多协议转换、负载均衡 |
| **Claude Code 专用代理** | 15+ | Anthropic→OpenAI 协议转换、模型映射 |
| **Codex CLI 专用代理** | 5+ | Codex 配额管理、OAuth 轮换 |
| **多 Agent 统一网关** | 4+ | 把多个 Agent CLI 统一暴露为 OpenAI API |
| **成本优化路由器** | 3+ | 按请求复杂度自动选择模型层级 |
| **安全/护栏网关** | 2+ | Prompt 注入检测、PII 脱敏 |
| **MCP 网关** | 10+ | Model Context Protocol 桥接与管理 |

---

## 二、重点推荐项目（按可借鉴价值排序）

### 2.1 通用 AI Gateway（架构参考）

#### 1. LiteLLM ★47k
- **链接**：https://github.com/BerriAI/litellm
- **技术栈**：Python
- **核心能力**：100+ LLM 的统一调用接口，支持 Proxy Server 模式
- **Agent 适配相关**：支持虚拟 key、多租户、成本追踪、guardrails
- **可借鉴**：provider 注册表设计、虚拟 key 体系、spend tracking、load balancing 策略
- **与 tingly-box 对比**：LiteLLM 更重 Python 生态，tingly-box 更聚焦 Agent 场景和本地部署

#### 2. coaidev/coai (Chat Nio) ★9.1k
- **链接**：https://github.com/coaidev/coai
- **技术栈**：Go + React
- **核心能力**：多租户、计费、渠道管理、200+ 模型、35+ 提供商
- **可借鉴**：渠道分发系统、计费体系、管理后台设计
- **注意**：国内项目，侧重渠道分发而非 Agent 原生适配

#### 3. ENTERPILOT/GoModel ★879
- **链接**：https://github.com/ENTERPILOT/GoModel
- **技术栈**：Go
- **核心能力**：LiteLLM 的 Go 替代品，统一 OpenAI-compatible API
- **可借鉴**：Go 语言实现轻量网关的架构设计

---

### 2.2 多 Agent 统一网关（反向思维：Agent CLI → OpenAI API）

#### 4. agent-cli-to-api ★42
- **链接**：https://github.com/leeguooooo/agent-cli-to-api
- **技术栈**：Python (FastAPI/uvicorn)
- **核心能力**：把 **Codex / Cursor Agent / Claude Code / Gemini** 等 Agent CLI 暴露为统一的 OpenAI-compatible HTTP API
- **设计哲学**：不是让 Agent 走网关，而是让网关调用 Agent CLI
- **关键代码模式**：
  - `claude_oauth.py`：自动读取 `~/.claude/settings.json` 的 `env` 字段获取 `ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN`
  - `codex_responses.py`：封装 Codex CLI 的 `/responses` 和 `codex exec` 调用
  - `openai_compat.py`：OpenAI ChatCompletion 请求 ↔ Agent CLI 调用的转换层
- **可借鉴**：
  - **反向代理思路**：如果你的中转站同时想做"把本地 Agent 能力暴露为 API"，参考这个模式
  - **OAuth 凭证管理**：Claude OAuth token 的缓存、刷新、过期检测
  - **CLI 配置探测**：自动读取 `.claude/settings.json` 获取已有配置

```python
# agent-cli-to-api 的 Claude CLI 配置探测
@dataclass(frozen=True)
class ClaudeCliConfig:
    base_url: str | None
    auth_token: str | None
    default_model: str | None

def _load_claude_cli_settings() -> ClaudeCliConfig:
    settings_path = Path.home() / ".claude" / "settings.json"
    raw = json.loads(settings_path.read_text())
    env = raw.get("env") or {}
    return ClaudeCliConfig(
        base_url=env.get("ANTHROPIC_BASE_URL"),
        auth_token=env.get("ANTHROPIC_AUTH_TOKEN"),
        default_model=env.get("ANTHROPIC_DEFAULT_SONNET_MODEL"),
    )
```

---

### 2.3 Claude Code 专用代理/桥接（协议转换）

#### 5. claude-code-nexus ★250
- **链接**：https://github.com/KroMiose/claude-code-nexus
- **技术栈**：TypeScript / Hono / Cloudflare Workers / D1 / Drizzle ORM
- **核心能力**：Claude Code → 任意 OpenAI-compatible API 的代理平台
- **Agent 适配方式**：
  - 环境变量注入：`ANTHROPIC_API_KEY=ak-your-nexus-key` + `ANTHROPIC_BASE_URL=https://claude.nekro.ai`
  - **模型映射服务**：用户级模型映射配置（haiku/sonnet/opus → 任意目标模型）
  - Web UI 管理映射规则
- **关键代码模式**：
```typescript
// 模型映射服务 - 用户级自定义映射
class ModelMappingService {
  async findTargetModel(userId: string, modelName: string): Promise<string> {
    const config = await this.getEffectiveMappingConfig(userId);
    return findTargetModelFromConfig(modelName, config);
  }
}
```
- **可借鉴**：
  - **SaaS 化 Agent 代理**：用户通过 Web UI 配置自己的映射规则，多租户隔离
  - **Cloudflare Worker 部署**：边缘计算部署，全球低延迟
  - **D1 数据库存储用户配置**：serverless SQL，零运维

#### 6. anthropic-proxy-rs ★53
- **链接**：https://github.com/m0n0x41d/anthropic-proxy-rs
- **技术栈**：Rust / async
- **核心能力**：Anthropic API → OpenAI-compatible 格式的高性能代理（~3MB 二进制）
- **Agent 适配方式**：
  - Drop-in replacement：Claude Code / Claude Desktop 直接指向代理
  - **Extended Thinking 自动路由**：检测到 `thinking` 参数时自动切换 `REASONING_MODEL`
  - **模型映射**：`ANTHROPIC_PROXY_MODEL_MAP=source=target;other=target`
  - **System prompt 过滤**：可移除指定术语再转发上游
- **可借鉴**：
  - **Rust 高性能代理**：单二进制、低内存占用
  - **特性感知路由**：根据请求参数（thinking）自动选择不同上游模型
  - **环境变量简洁配置**：仅 `UPSTREAM_BASE_URL` + `UPSTREAM_API_KEY`

#### 7. ziozzang/claude2openai-proxy ★27
- **链接**：https://github.com/ziozzang/claude2openai-proxy
- **核心能力**：Claude API → OpenAI API 兼容网关
- **可借鉴**：协议转换的具体实现细节

#### 8. meaning-systems/claude-code-proxy ★13
- **链接**：https://github.com/meaning-systems/claude-code-proxy
- **核心能力**：把 Claude Code Max 订阅转为本地 OpenAI-compatible 端点
- **可借鉴**：OAuth 订阅凭证的本地代理利用

---

### 2.4 Codex CLI 专用代理

#### 9. codex-account-orchestrator ★2
- **链接**：https://github.com/DAWNCR0W/codex-account-orchestrator
- **核心能力**：Codex OAuth 账号轮换与无缝网关模式
- **可借鉴**：多账号配额管理与自动切换

#### 10. codex-turn-desktop ★2
- **链接**：https://github.com/kam74515-boop/codex-turn-desktop
- **核心能力**：Codex CLI 本地中转桌面应用
- **可借鉴**：桌面 GUI + 本地代理的结合方式

---

### 2.5 成本优化路由器（智能分层）

#### 11. NadirClaw ★497
- **链接**：https://github.com/NadirRouter/NadirClaw
- **技术栈**：Python
- **核心能力**：自动按请求复杂度路由到不同成本层级的模型，节省 40-70% 成本
- **Agent 适配方式**：
  - **Full onboard 模式**：检测 `~/.claude/settings.json` 中的模型配置，映射到 NadirClaw 的 simple/mid/complex/reasoning 四层
  - **Lightweight shim 模式**：`~/.nadirclaw/bin/claude` wrapper，懒启动代理 + 设置 env
  - **模型层级探测**：从 Claude Code 配置中探测已有模型，自动分类到对应 tier
  - **Agentic 任务检测**：自动识别 tool use、多步循环、agent system prompt，强制使用复杂模型
  - **Reasoning 检测**：识别 chain-of-thought 需求
  - **Vision 路由**：检测图片内容自动路由到 vision-capable 模型
  - **Session persistence**：多轮对话中固定模型，避免来回切换
- **关键代码模式**：
```python
# NadirClaw 的 Claude Code 集成 - 两种 onboard 模式
"""
1. Full onboard: 写入 ~/.claude/settings.json 的 env 字段
   - ANTHROPIC_BASE_URL + ANTHROPIC_API_KEY
   - 安装 launchd/systemd 自启动

2. Lightweight shim: ~/.nadirclaw/bin/claude wrapper
   - 懒启动代理
   - exec 真正的 claude 二进制并带 env
"""

# 模型层级分类
_TIER_DEFAULTS = {
    "simple":   {"google": "gemini-2.5-flash", "openai": "gpt-4.1-mini"},
    "complex":  {"anthropic": "claude-sonnet-4-5", "openai": "gpt-4.1"},
    "reasoning":{"openai": "o3", "deepseek": "deepseek-reasoner"},
}

def _candidate_models(claude_settings, claude_json):
    """从 Claude Code 配置中提取所有模型 ID"""
    candidates = []
    for cfg in (claude_settings, claude_json):
        candidates.append(cfg.get("model"))
        env = cfg.get("env") or {}
        candidates.append(env.get("ANTHROPIC_MODEL"))
        candidates.append(env.get("ANTHROPIC_SMALL_FAST_MODEL"))
        candidates.append(cfg.get("lastSelectedModel"))
        candidates.append(cfg.get("defaultModel"))
    return dedup(candidates)
```
- **可借鉴**：
  - **四层路由模型**：simple / mid / complex / reasoning 的成本分层策略
  - **Agent 配置探测**：自动读取已有 Agent 配置，而非让用户重新配置
  - **Shim wrapper 模式**：不改 Agent 配置，用 wrapper 脚本拦截调用
  - **Agentic 检测**：通过 system prompt 和 tool use 模式识别 Agent 行为

#### 12. token_proxy ★65
- **链接**：https://github.com/mxyhi/token_proxy
- **技术栈**：Rust (Tauri + CLI) + TypeScript 前端
- **核心能力**：本地 AI API 网关，一键配置 Claude Code / Codex / OpenCode
- **Agent 适配方式**（这是目前看到**最完整、最系统**的 Agent 配置实现之一）：

**Claude Code**：
```rust
// 写入 ~/.claude/settings.json 的 env 字段
let mut root = read_json_object_or_default(&settings_path).await?;
let env = ensure_json_object_field(&mut root, "env")?;
env.insert("ANTHROPIC_BASE_URL".to_string(), proxy_http_base_url);
env.insert("ANTHROPIC_AUTH_TOKEN".to_string(), local_api_key);
write_json_with_backup(&settings_path, &Value::Object(root)).await?;
```

**Codex**：
```rust
// 写入 ~/.codex/config.toml + ~/.codex/auth.json
// 使用 toml_edit 精确编辑 TOML，保留用户其他配置
doc["model_provider"] = toml_edit::value("token_proxy");
doc["model_providers"]["token_proxy"]["base_url"] = toml_edit::value(base_url);
doc["model_providers"]["token_proxy"]["requires_openai_auth"] = toml_edit::value(true);
doc["model_providers"]["token_proxy"]["wire_api"] = toml_edit::value("responses");
// 移除 experimental_bearer_token 避免冲突
auth_root.insert("OPENAI_API_KEY", serde_json::Value::String(token));
```

**OpenCode**：
```rust
// 写入 ~/.config/opencode/opencode.json
root.insert("$schema", OPENCODE_SCHEMA_URL);
providers.insert("token_proxy", build_opencode_provider_config(base_url, models));
auth_root.insert("token_proxy", { "type": "api", "key": token });
```

**关键设计细节**：
- **路径解析**：支持 `CODEX_HOME`、`OPENCODE_CONFIG`、`XDG_CONFIG_HOME`、`XDG_DATA_HOME` 等环境变量覆盖
- **备份机制**：每次写入前自动备份原文件到 `.bak`
- **无损编辑**：Codex 用 `toml_edit`（保留注释和格式），JSON 用 `serde_json::to_string_pretty`
- **条件写入**：本地鉴权关闭时，主动删除之前写入的 token，防止误传上游
- **IPv6 兼容**：URL host 自动加方括号
- **模型收集**：从上游配置中提取精确模型名（排除通配符 `*`）用于 OpenCode provider 的 `models` 字段

- **可借鉴**：
  - **三种 Agent 的统一配置 API**：一个函数分别处理 Claude Code / Codex / OpenCode
  - **Tauri 桌面应用 + CLI 双模式**：桌面端一键配置，CLI 端脚本化
  - **toml_edit 无损编辑 TOML**：比直接重写更能保留用户自定义注释
  - **环境变量感知的路径解析**：跨平台、支持 XDG 规范

---

### 2.6 安全/护栏网关

#### 13. AegisGate ★37
- **链接**：https://github.com/ax128/AegisGate
- **核心能力**：LLM API 安全网关 — Prompt 注入检测、PII 脱敏、危险响应清理、审计日志
- **Agent 支持**：Cursor、Claude Code、Codex 的 Drop-in proxy
- **可借鉴**：安全层与网关层的分离架构、MCP & Agent SKILL 支持

---

### 2.7 MCP 网关（生态扩展参考）

#### 14. acehoss/mcp-gateway ★133
- **链接**：https://github.com/acehoss/mcp-gateway
- **核心能力**：MCP STDIO → HTTP+SSE 桥接
- **可借鉴**：MCP 协议的多传输层桥接

#### 15. mcp-oauth-gateway ★56
- **链接**：https://github.com/atrawog/mcp-oauth-gateway
- **核心能力**：为任意 MCP 服务器添加 OAuth 2.1 认证
- **可借鉴**：MCP 服务的认证层设计

#### 16. locomotive-agency/mcp-anywhere ★41
- **链接**：https://github.com/locomotive-agency/mcp-anywhere
- **核心能力**：统一 MCP 工具发现与访问网关
- **可借鉴**：MCP 工具的动态发现与注册

---

## 三、Agent 适配的六种实现模式对比

| 模式 | 代表项目 | 实现方式 | 优点 | 缺点 |
|---|---|---|---|---|
| **A. 环境变量注入** | tingly-box, token_proxy, claude-code-nexus | 写入 `~/.claude/settings.json` 的 `env` 字段 | 标准、持久、跨 shell | 需要处理 JSON 合并 |
| **B. Wrapper 脚本** | NadirClaw (shim 模式) | `~/.nadirclaw/bin/claude` wrapper 设置 env 后 exec | 不改动现有配置、可懒启动 | 需要 PATH 管理 |
| **C. 系统服务自启动** | NadirClaw (onboard 模式) | launchd/systemd 守护进程 | 开机自启、后台常驻 | 需要系统权限 |
| **D. TOML/JSON 直接配置** | token_proxy (Codex/OpenCode) | 精确编辑 `config.toml` / `opencode.json` | 保留格式、支持 provider 自定义 | 需要了解各 Agent 配置格式 |
| **E. 协议转换代理** | anthropic-proxy-rs, claude-code-nexus | Anthropic API ↔ OpenAI API 转换 | 对 Agent 透明、无需改配置 | 需要完整协议兼容性 |
| **F. 反向 API 封装** | agent-cli-to-api | Agent CLI → OpenAI HTTP API | 把 Agent 能力暴露给其他工具 | 需要处理 CLI 的 stdin/stdout |

---

## 四、关键代码模式速查

### 4.1 Claude Code 配置写入（标准模式）

```rust
// token_proxy 的实现 — 最完整的参考
let settings_path = home_dir.join(".claude").join("settings.json");
let mut root = read_json_object_or_default(&settings_path).await?;
let env = ensure_json_object_field(&mut root, "env")?;
env.insert("ANTHROPIC_BASE_URL".to_string(), proxy_url);
env.insert("ANTHROPIC_AUTH_TOKEN".to_string(), token);
write_json_with_backup(&settings_path, &Value::Object(root)).await?;
```

### 4.2 Codex 配置写入（TOML 无损编辑）

```rust
let mut doc = toml_edit::DocumentMut::from_str(&input)?;
doc["model_provider"] = toml_edit::value("my_gateway");
doc["model_providers"]["my_gateway"]["base_url"] = toml_edit::value(base_url);
doc["model_providers"]["my_gateway"]["requires_openai_auth"] = toml_edit::value(true);
doc["model_providers"]["my_gateway"]["wire_api"] = toml_edit::value("responses");
// 清理冲突字段
if let Some(table) = doc["model_providers"]["my_gateway"].as_table_mut() {
    table.remove("experimental_bearer_token");
}
write_text_with_backup(&config_path, doc.to_string()).await?;
```

### 4.3 OpenCode 配置写入

```rust
let mut root = read_json_object_or_default(&opencode_config_path).await?;
root.insert("$schema".to_string(), serde_json::Value::String("https://opencode.ai/config.json".to_string()));
let providers = ensure_json_object_field(&mut root, "provider")?;
providers.insert("my_gateway".to_string(), build_opencode_provider_config(base_url, models));
write_json_with_backup(&opencode_config_path, &Value::Object(root)).await?;
```

### 4.4 路径解析（跨平台 + XDG）

```rust
fn resolve_home_dir() -> Result<PathBuf, String> {
    if let Ok(dir) = std::env::var("HOME") { return Ok(PathBuf::from(dir)); }
    if cfg!(target_os = "windows") {
        if let Ok(dir) = std::env::var("USERPROFILE") { return Ok(PathBuf::from(dir)); }
    }
    Err("Failed to resolve home directory.".to_string())
}

fn resolve_opencode_config_dir() -> PathBuf {
    if let Some(dir) = std::env::var_os("OPENCODE_CONFIG_DIR") { return PathBuf::from(dir); }
    if let Some(dir) = std::env::var_os("XDG_CONFIG_HOME") { return PathBuf::from(dir).join("opencode"); }
    resolve_home_dir().unwrap().join(".config").join("opencode")
}

fn resolve_codex_home_dir() -> PathBuf {
    std::env::var_os("CODEX_HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|| home_dir.join(".codex"))
}
```

### 4.5 配置探测（读取已有 Agent 配置）

```python
# NadirClaw / agent-cli-to-api 的共同模式
def _load_claude_cli_settings():
    settings_path = Path.home() / ".claude" / "settings.json"
    raw = json.loads(settings_path.read_text())
    env = raw.get("env") or {}
    return {
        "base_url": env.get("ANTHROPIC_BASE_URL"),
        "auth_token": env.get("ANTHROPIC_AUTH_TOKEN"),
        "model": env.get("ANTHROPIC_MODEL"),
    }
```

### 4.6 模型映射服务

```typescript
// claude-code-nexus 的模式
class ModelMappingService {
  async findTargetModel(userId: string, modelName: string): Promise<string> {
    const config = await this.getEffectiveMappingConfig(userId);
    // 系统默认映射 + 用户自定义映射
    return findTargetModelFromConfig(modelName, config);
  }
}
```

---

## 五、如果要自建 Agent 适配层，建议研究顺序

### Phase 1：基础配置注入（必看）
1. **tingly-box** → 最系统的分层架构（已有专项文档）
2. **token_proxy** → 三种 Agent 的完整配置实现（Rust + 无损编辑 + 备份）
3. **claude-code-nexus** → Web UI 管理 + SaaS 化 + 用户级模型映射

### Phase 2：协议转换（进阶）
4. **anthropic-proxy-rs** → Rust 高性能 Anthropic↔OpenAI 转换
5. **agent-cli-to-api** → 反向封装（Agent CLI → HTTP API）

### Phase 3：智能路由（高级）
6. **NadirClaw** → 成本优化四层路由 + Agent 行为检测 + Shim 模式
7. **LiteLLM** → 通用 Gateway 的完整能力矩阵

### Phase 4：安全与生态
8. **AegisGate** → 安全护栏层设计
9. **mcp-gateway 系列** → MCP 协议扩展

---

## 六、一句话总结每个项目的独特价值

| 项目 | 独特价值 |
|---|---|
| tingly-box | **最完整的 Agent 适配 + 路由联动 + 本地部署** |
| token_proxy | **三种 Agent 配置的最优雅实现（Rust + toml_edit + XDG）** |
| claude-code-nexus | **SaaS 化 Claude Code 代理 + 用户级模型映射** |
| anthropic-proxy-rs | **Rust 单二进制高性能协议转换** |
| NadirClaw | **四层成本路由 + Agent 行为检测 + Shim 模式** |
| agent-cli-to-api | **反向思维：Agent CLI → OpenAI API** |
| LiteLLM | **通用 Gateway 的能力天花板参考** |
| AegisGate | **安全层与网关层的分离架构** |
