# PROPOSAL: T-005b Admin Auth Login

## 1. Session 方案
推荐使用 **server-side in-memory session, token 是不透明随机字符串，前端用 localStorage 存**。每次登录时在后端生成 32-byte 随机 hex，存入内存，完全复用现在的 `adminAuthMiddleware` 提取 Bearer Token 逻辑。这避免了新增会话持久化状态表的复杂度。

## 2. 密码存储
使用 `golang.org/x/crypto/bcrypt`，cost 取 10（对目前的系统规模足以兼顾延迟与安全）。

## 3. Session 存哪及 Trade-offs
推荐保存在 in-memory map，带简单的过期机制（例如 24 小时自然过期）。
- **Trade-off 1 (重启清空)**: admin 进程重启会导致所有人重新登录（session 丢失）。这在 v1 单实例部署下是可接受的，但需要在部署文档中明确点名"admin 重启 = 用户被踢"。
- **Trade-off 2 (多实例状态不同步)**: 如果未来多实例水平扩容，in-memory session 无法跨实例共享。v1 单实例无此问题，但后续 vNext（T-016 后期）如果扩容则需要引入 Redis 共享 session 机制。

## 4. 首批 Admin 密码怎么入库
推荐 **a) seed SQL 内嵌 bcrypt 预生成的 hash（如密码硬编码为 admin-dev-password）**。
- **理由**: 这是针对 v1 `002_seed.sql` 和本地联调的最快路径，开箱即用。真正的生产环境应当使用专门的 CLI 工具 (`cmd/admin-cli reset-password`) 或环境变量等方式安全初始化，但这对于 v1 的 seed 测试数据已经足够。

## 5. 前端 Token 管理形态及安全性
**localStorage** 保存 token，发送 API 请求前注入 Header，收到 401 直接 `localStorage.removeItem` 并跳转到登录界面。
- **关于 localStorage XSS 风险**: localStorage 存 token 比 HTTP-only cookie 多一层 XSS 风险。v1 接受这个取舍（admin 控制台不渲染用户输入内容），但显式声明：**"v2 接收用户输入内容时改 HTTP-only cookie"**。

## 6. 登录端点是否自审计
推荐落库。使用 `actor_type = "anonymous"`，并将尝试登录的 Email / 登录失败状态记入 `audit_logs` 的 `after={email}` 或 `reason` 中，有助于事后分析撞库等异常行为。

## 7. 测试矩阵
与 T-005a / T-015 保持同档标准，至少覆盖 6 个 case：
- **login success**: 正确凭据返回 session token。
- **wrong password/email**: 错误凭据返回 401。
- **disabled user**: 用户被禁用，拒绝登录。
- **expired token**: 过期的 token 请求返回 401。
- **401 with revoked token**: 登出后的旧 token 请求返回 401。
- **audit on login attempt**: 登录的成功或失败动作都在 `audit_logs` 中正确记录（包含 `actor_type="anonymous"` 及 email 等痕迹）。
