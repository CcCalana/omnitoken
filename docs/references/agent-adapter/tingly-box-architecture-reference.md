# Tingly Box 全方位架构设计参考

> 基于 [tingly-box](https://github.com/tingly-dev/tingly-box) 完整源码分析  
> 本文档与 `agent-adapter-pattern.md` 互补，聚焦除 Agent 适配外的全部核心架构

---

## 一、项目全景

Tingly Box 是一个 **Agent-First 的 AI 模型网关**，核心能力包括：
- 多协议接入（OpenAI、Anthropic、Google Gemini 等）
- 智能路由与负载均衡
- 熔断与健康监控
- 安全护栏（Guardrails）
- 多租户隔离
- IM Bot 远程控制
- MCP 协议支持
- 可观测性与审计

技术栈：Go 1.25+ / Gin / SQLite / GORM / fsnotify / OpenTelemetry

---

## 二、负载均衡与熔断 — `internal/loadbalance/`

### 2.1 服务模型设计

```go
type Service struct {
    Provider      string           // 提供商 UUID
    Model         string           // 模型名
    Weight        int              // 权重
    Active        bool             // 是否激活
    TimeWindow    int              // 统计窗口（秒）
    ModelCapacity *int             // 模型容量上限（覆盖提供商默认值）
    Priority      int              // 优先级（高优先级先尝试）
    Stats         ServiceStats     // 使用统计（存 SQLite，不序列化到配置）
}
```

**启示**：不要把统计状态放到配置文件里。配置只存静态定义，动态统计走数据库/内存，通过 `yaml:"-"` 排除序列化。

### 2.2 七种负载均衡策略

| 策略 | 说明 | 适用场景 |
|---|---|---|
| `token_based` | 按 token 消耗轮询 | 公平分配成本 |
| `random` | 加权随机 | 简单均衡 |
| `latency_based` | 按延迟选最优 | 追求速度 |
| `speed_based` | 按 token/s 选最快 | 追求吞吐 |
| `adaptive` | 多维度综合评分 | 默认推荐 |
| `capacity_based` | 按容量利用率 | 防止过载 |
| `priority` | 高优先级优先，失败降级 | 主备切换 |

**关键代码**：`TacticType` 自定义枚举 + JSON Marshal/Unmarshal 支持字符串形式（`"adaptive"`），向后兼容整数形式。

### 2.3 服务统计 — ServiceStats

跟踪每个服务的完整运行时指标：
- **Token 维度**：input/output/total，窗口内累计
- **延迟维度**：avg / P50 / P95 / P99（滚动窗口采样，排序计算分位数）
- **TTFT**（Time To First Token）：avg / P50 / P95 / P99
- **Token 速度**：tokens/second 滚动平均
- **缓存**：hit/miss 率
- **成本代理**：窗口内总 token 数作为成本代理

**启示**：做网关一定要把 **P99 延迟** 和 **TTFT** 纳入监控，这是用户体验的核心指标。

### 2.4 熔断器 — Breaker

三态设计：
```
Closed  ──失败阈值达成──▶  Open  ──超时到期──▶  HalfOpen
  ▲                                              │
  └───成功恢复────────────────────────────────────┘（失败则回到 Open）
```

关键特性：
- **共享状态**：同一 `provider:model` 的所有规则共享熔断器（进程级单例 `defaultStore`）
- **HalfOpen 仲裁**：只允许一个探针请求通过，防止雪崩
- **懒状态转换**：`Allow()` 时才检查是否需要 Open→HalfOpen

### 2.5 健康监控 — HealthMonitor

区分三类错误，差异化处理：

| 错误类型 | 行为 | 恢复方式 |
|---|---|---|
| 429 RateLimit | 立即标记不健康，设置 `RateLimitedUntil` 时间 | 超时自动恢复 |
| 401/403 Auth | 立即标记不健康 | 手动修复后恢复 |
| 一般错误 | 连续阈值（默认 3 次）后才标记 | 超时 + 探针恢复 |

**探针恢复**：配置 `ProbeEnabled` + `HealthProbeFunc`，在超时后先发一个探针请求确认服务可用，再恢复。探针失败则延长超时（指数退避可扩展）。

---

## 三、智能路由 — `internal/smart_routing/`

### 3.1 基于请求内容的路由规则

传统路由按模型名匹配，智能路由按**请求内容特征**匹配：

| 匹配维度 | 操作 | 示例 |
|---|---|---|
| `model` | contains / equals / glob | `"*claude*"` |
| `thinking` | enabled / disabled | thinking 模式开关 |
| `context.system` | contains / regex | 系统提示词包含特定关键字 |
| `context.user` | contains / regex | 历史用户消息内容 |
| `latest_user` | contains / request_type | 最新用户消息 |
| `tool_use` | equals | 使用了特定工具 |
| `token` | ge / gt / le / lt | 预估 token 数阈值 |
| `service.ttft` | avg_le / p99_le | 服务首 token 延迟 |
| `service.capacity` | util_le / util_ge | 服务容量利用率 |
| `agent.claude_code` | equals | main / subagent / compact |
| `proxy.vision` | enabled | 请求含图片 |

**启示**：中转站不是只做"模型名→提供商"的映射，还可以按**请求特征**做精细化路由，比如：
- 长上下文（token > 8k）→ 走大上下文模型
- 含图片 → 走 vision 模型
- subagent 请求 → 走便宜快速的模型
- system prompt 含敏感词 → 走本地模型

### 3.2 Claude Code 请求类型检测

通过请求内容指纹识别三种 Claude Code 请求：

```go
const (
    ClaudeCodeKindMain     = "main"      // "You are Claude Code"
    ClaudeCodeKindSubagent = "subagent"  // "You are an agent" (不含 main preamble)
    ClaudeCodeKindCompact  = "compact"   // "summary of the conversation"
)
```

**启示**：通过**系统提示词的特征字符串**来检测 Agent 的内部行为，无需修改 Agent 源码。

### 3.3 规则评估与 Trace

每条规则的每个 op 都有完整的评估结果：
```go
type RuleEvalResult struct {
    RuleIndex      int
    Description    string
    Matched        bool
    OpsTotal       int
    OpsEvaluated   int
    Ops            []OpEvalResult
}

type OpEvalResult struct {
    UUID      string
    Position  string
    Operation string
    Value     string
    Matched   bool
    Actual    string    // 实际值
    Reason    string    // 匹配/不匹配原因
}
```

**启示**：路由决策必须**可观测**。用户需要知道"为什么走了这条路由"，调试时 invaluable。

---

## 四、配置管理 — `internal/server/config/`

### 4.1 配置热加载 — Watcher

```go
type Watcher struct {
    config       *Config
    watcher      *fsnotify.Watcher
    callbacks    []func(*Config)
    paused       bool              // 防止 reload 循环
    fileModTimes map[string]time.Time
}
```

关键设计：
- **防抖（Debounce）**：1 秒防抖，处理编辑器多文件写入
- **ModTime 校验**：fsnotify 触发后对比文件修改时间，无实际变化则跳过
- **Pause 机制**：reload 期间暂停 watcher，防止迁移代码调用 `Save()` 触发无限循环
- **回调注册**：支持多个组件订阅配置变更

### 4.2 配置迁移 — Migration

版本化迁移，每个迁移函数有唯一日期标识：

```go
func Migrate(c *Config) error {
    migrate20251220(c) // UUID 和 LBTactic 初始化
    migrate20251221(c) // Provider V1→V2
    migrate20260103(c) // 规则 Scenario 补全
    migrate20260416(c) // 默认开启多租户
    // ...
    return nil
}
```

防重复执行机制：
```go
func (c *Config) hasMigrationCompleted(id string) bool
func (c *Config) markMigrationCompleted(id string)
```

**关键启示**：
- 迁移函数必须是**幂等**的（可安全重复执行）
- 对于会破坏幂等性的迁移（如自动填充默认值），用 `MigrationsCompleted` 列表标记只执行一次
- 迁移失败不阻断启动（`_ = c.Save()` 忽略错误），保证系统可用性

### 4.3 统一 Config 结构体

所有配置集中在一个结构体：
- 路由规则 `Rules`
- 提供商 `Providers`
- 场景配置 `Scenarios`
- 健康监控 `HealthMonitor`
- 多租户 `MultiTenantConfig`
- HTTP 传输 `HTTPTransport`
- 特性开关 `GenericMCP`
- 数据库管理器 `storeManager`
- 生命周期钩子 `providerUpdateHooks` / `providerDeleteHooks`

---

## 五、安全护栏 — `internal/guardrails/`

### 5.1 架构设计

```
Guardrails (运行时入口)
  ├── PolicyRunner (策略引擎接口)
  │     └── Evaluate(ctx, Input) → Result
  ├── CredentialCache (受保护凭证)
  │     ├── ByScenario: scenario → []ProtectedCredential
  │     └── ByID: id → ProtectedCredential
  ├── History (审计历史)
  │     └── Store (线程安全环形缓冲区)
  └── Config (激活状态配置)
```

### 5.2 凭证掩码设计

```go
type ProtectedCredential struct {
    ID      string
    Name    string
    Secret  string  // 实际敏感值
    Enabled bool
}
```

按场景缓存并排序（长的 secret 在前，避免短 secret 误匹配）：
```go
sort.Slice(enabledCredentials, func(i, j int) bool {
    return len(enabledCredentials[i].Secret) > len(enabledCredentials[j].Secret)
})
```

**启示**：在安全护栏中扫描敏感信息时，**先匹配长的 secret** 可以避免短 secret 被长 secret 的子串误触。

### 5.3 历史审计

每次 Evaluate 生成一条 Entry：
- 时间、场景、模型、提供商
- 方向（request/response）
- 阶段（phase）
- 裁决结果（verdict）
- 阻断消息（block_message）
- 内容预览（截断 160 字符）
- 触发的凭证引用

---

## 六、可观测性 — `internal/obs/`

### 6.1 BatchProcessor — 非阻塞批处理

```go
type BatchProcessor struct {
    queue    chan *Record      // 有界队列（默认 1024）
    exporter RecordExporter
    ticker   *time.Ticker      // 定时刷新（默认 5s）
    dropped  atomic.Int64      // 队列满丢弃计数
    maxBatch int               // 每批最大 256 条
}
```

关键设计：
- **非阻塞 Emit**：`select` + `default`，队列满则丢弃并计数，绝不阻塞请求 goroutine
- **后台 worker**：定时 + 定量双触发 flush
- **ForceFlush**：同步 flush，带 context deadline
- **Shutdown**：优雅退出，先 drain 队列再关闭

### 6.2 导出器设计

```go
type RecordExporter interface {
    Export(ctx context.Context, records []*Record) error
    Shutdown(ctx context.Context) error
}
```

实现包括：
- **MultiExporter**：同时写入多个后端
- **GzipExporter**：压缩后写入文件
- **CASExporter**：内容寻址存储（Content-Addressed Storage）

### 6.3 记录瘦身 — Slim

记录入队列前做**有损压缩**：
- 截断过长的消息内容
- 移除二进制数据
- 保留关键元数据

**启示**：可观测数据量极大，必须在**采集端做有损压缩**，否则存储和传输成本爆炸。

---

## 七、协议适配与端点路由 — `internal/server/`, `internal/protocol/`

### 7.1 自适应端点选择

OpenAI 生态现在有 Chat Completions 和 Responses 两个 API。Tingly Box 实现了**自适应端点路由**：

```go
func (s *Server) SelectOpenAIEndpoint(
    ctx context.Context,
    provider *typ.Provider,
    modelID string,
    opts OpenAIEndpointOptions,
) (*EndpointSelection, error)
```

决策逻辑：
1. **Codex 提供商** → 强制 Responses（OAuth 认证方式决定）
2. **规则覆盖** → 用户可强制指定 chat 或 responses
3. **能力探测缓存** → 查询该模型支持哪些端点
4. **按需降级** → Chat 请求可用 Responses 端点兜底；Responses 请求在不需要特殊特性（`previous_response_id`/`include`/`background`/`truncation`/`reasoning`）时可降级到 Chat

**启示**：做网关必须处理**同一提供商的多端点共存**问题，不能简单假设所有模型都支持同一套 API。

### 7.2 能力探测 — Probe

```go
type ModelCapability struct {
    SupportsChat            bool
    ChatSupportsStream      bool
    SupportsResponses       bool
    ResponsesSupportsStream bool
}
```

异步探测模型能力并缓存：
```go
go func() {
    probeCtx, cancel := context.WithTimeout(context.Background(), DefaultProbeTimeout)
    defer cancel()
    _, _ = NewAdaptiveProbe(s).ProbeModelEndpoints(probeCtx, ...)
}()
```

**启示**：未知模型先放行，后台异步探测，下次请求用缓存结果。避免首次请求被阻塞。

---

## 八、协议验证体系 — `internal/protocol_validate/`

### 8.1 验证矩阵

`matrix.go` 定义了完整的兼容性矩阵，验证不同协议之间的转换正确性。

### 8.2 场景测试

`scenarios.go` 定义了多种测试场景，覆盖：
- 不同 Agent 的请求模式
- 不同模型的响应格式
- 流式 vs 非流式
- 工具调用
- 多模态（图片）

### 8.3 真实模型测试

`real_model.go` 支持连接真实模型提供商进行端到端验证。

### 8.4 重放测试

`replay.go` 支持录制真实流量并重放，用于回归测试。

### 8.5 Agent 环境验证

`agent_env.go` 验证 Agent 环境变量配置是否正确。

**启示**：网关项目必须有**自动化协议验证体系**，因为协议转换是核心且易错的。矩阵覆盖 + 场景测试 + 重放测试三层保障。

---

## 九、服务端验证 — `internal/server_validate/`

### 9.1 客户端验证器

`client.go`：模拟客户端发送请求，验证网关行为是否符合预期。

### 9.2 服务端验证器

`server.go`：在网关内部验证请求处理流程。

**启示**：把验证代码独立成包，不耦合在主逻辑里，方便 CI 和本地测试分别运行。

---

## 十、特性开关 — `internal/feature/`

极简实现：
```go
var KnownFeatures = map[string]bool{
    FeatureCompact: true,
}

func ParseFeatures(expr string) map[string]bool {
    // "compact,other" → {"compact": true, "other": true}
}

func IsEnabled(features map[string]bool, feature string) bool
```

**启示**：特性开关不需要复杂系统，一个简单的 comma-separated 字符串 + map 即可满足大多数需求。

---

## 十一、多租户 — `MultiTenantConfig`

```go
type MultiTenantConfig struct {
    Enabled           bool
    APITokenSecret    string   // JWT 签名密钥
    APITokenAlgorithm string   // "HS256"
    APITokenIssuer    string   // "tingly-box"
}
```

设计要点：
- 默认开启（通过 migration 自动配置）
- 同时保留全局 token 向后兼容
- 每个租户独立的使用统计和提供商访问

---

## 十二、远程控制 — `internal/remote_control/`

### 12.1 IM Bot 架构

支持平台：Telegram、钉钉、飞书、Lark、微信、企业微信、Slack、Discord

目录结构：
```
internal/remote_control/
  bot/          # 各平台 Bot 实现
  config/       # Bot 配置
  smart_guide/  # 智能引导
```

### 12.2 智能引导 — SmartGuide

通过 IM 消息远程控制 Agent，支持场景化规则配置。

---

## 十三、MCP 协议支持 — `internal/mcp/`

### 13.1 架构

```
internal/mcp/
  builtin_server/  # 内置 MCP 服务器（web_search / web_fetch）
  local/           # 本地 stdio MCP 进程管理
  runtime/         # MCP 运行时生命周期管理
  tools/           # 工具注册与发现
```

### 13.2 本地 MCP 进程管理

启动本地 MCP 服务器作为子进程，通过 stdio 通信，支持：
- 进程生命周期管理（启动、停止、重启）
- 工具列表动态发现
- 调用超时控制

---

## 十四、数据层 — `internal/data/`, `internal/dataio/`

### 14.1 存储架构

- **SQLite**：配置、统计、使用记录、规则状态、提供商状态、IM Bot 设置
- **GORM**：ORM 层
- **StoreManager**：统一数据库管理

### 14.2 模型列表管理

`ModelListManager`：动态拉取和缓存各提供商的模型列表，支持模板化配置。

### 14.3 模板管理

`TemplateManager`：提供商配置模板，支持从本地文件或远程 URL 加载。

---

## 十五、HTTP 传输优化 — `HTTPTransportConfig`

```go
type HTTPTransportConfig struct {
    MaxIdleConns        *int  // 全局最大空闲连接
    MaxIdleConnsPerHost *int  // 每主机最大空闲连接
    MaxConnsPerHost     *int  // 每主机最大连接数（含活跃）
    DisableKeepAlives   *bool // 是否禁用长连接
    RespectEnvProxy     *bool // 是否使用系统代理
}
```

**启示**：网关是高并发场景，必须暴露 HTTP 传输层参数给用户调优。用指针类型实现"省略则用 Go 默认值"。

---

## 十六、关键工程实践总结

### 16.1 错误处理哲学

- 配置迁移失败 → 忽略错误，保证启动（`_ = c.Save()`）
- 路由规则缺失 → 返回 Warning，不阻断 Agent 配置
- 队列满 → 丢弃记录，计数告警，不阻塞主流程
- 探测失败 → 异步重试，先用默认值放行

### 16.2 并发设计

- `sync.RWMutex` 保护配置和运行时状态
- `atomic.Int64` 用于无锁计数（如 dropped 记录数）
- channel + goroutine 实现后台批处理
- BreakerStore 进程级单例共享熔断状态

### 16.3 向后兼容

- 枚举值支持字符串和整数两种 JSON 格式
- 废弃策略映射到新策略（`round_robin` → `token_based`）
- 配置字段保留旧版本（`ProvidersV1` + `Providers`）
- 迁移标记 `MigrationsCompleted` 防止重复执行

### 16.4 测试策略

| 测试类型 | 所在包 | 说明 |
|---|---|---|
| 单元测试 | `*_test.go` | 各模块独立测试 |
| 协议矩阵 | `protocol_validate/matrix.go` | 兼容性矩阵 |
| 场景测试 | `protocol_validate/scenarios.go` | 端到端场景 |
| 真实模型 | `protocol_validate/real_model.go` | 连接真实 API |
| 重放测试 | `protocol_validate/replay.go` | 流量录制回放 |
| 服务端验证 | `server_validate/` | 网关内部流程验证 |

---

## 十七、如果要自建类似系统，建议模块优先级

### P0 — 核心必须
1. **负载均衡**（random + adaptive + 统计追踪）
2. **熔断器**（三态 + 共享状态）
3. **健康监控**（429/401/一般错误区分）
4. **配置热加载**（fsnotify + debounce）
5. **配置迁移**（版本化 + 防重执行）

### P1 — 高级路由
6. **智能路由**（按 token/内容/延迟/容量匹配）
7. **Agent 检测**（请求内容指纹识别）
8. **自适应端点**（Chat vs Responses 自动选择）
9. **能力探测**（异步探测 + 缓存）

### P2 — 安全与观测
10. **安全护栏**（策略引擎 + 凭证掩码 + 审计）
11. **可观测性**（BatchProcessor + 多导出器）
12. **多租户**（JWT 隔离 + 独立统计）

### P3 — 生态扩展
13. **MCP 支持**（本地进程 + 内置服务器）
14. **IM Bot 远程控制**
15. **协议验证体系**（矩阵 + 场景 + 重放）

---

## 十八、一句话总结

> **"网关不是简单的转发层，而是集协议翻译、智能路由、弹性容错、安全审计于一体的控制平面。每一层都应有独立的抽象、完整的可观测性和优雅的降级策略。"**
