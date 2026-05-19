# T-015 联调手册 — 纯点击版

> 用途：你坐在电脑前，**不写命令、不开 curl、不改代码**，只打开浏览器点。
> 范围：T-015（月度 budget + 在 Users 页改预算 + 审计落库）。
> 角色假设：你扮演 admin。

---

## 0. 别人帮你起好服务（一次）

让 Codex 跑一遍：

```
make up
```

这一条命令会起好 Postgres / migrate / seed / gateway / admin。起完之后告诉你：admin 端口（默认 `http://localhost:8081`）+ 静态前端端口（默认 `http://localhost:3000`）都通了。

如果中途要重启：`make down && make up`，桌面 demo 5 分钟内完成。

---

## 1. 打开页面

浏览器粘贴下面这个**完整 URL**（已经把临时 token 拼进去了，T-005b 接登录页后就不需要了）：

```
http://localhost:3000/?token=dev-bootstrap-token-change-me
```

**期望**：左侧导航 4 项（Overview / Users / Models / Audit），右侧默认进 Overview。**没有红色 error 横幅**。

如果还是红色 `invalid_api_key`：刷新一次；还不行就喊 Codex 检查 `make up` 的 admin container 日志。

---

## 2. Overview 巡检（30 秒）

- KPI 三张卡（总 tokens / 活跃用户 / 估算成本）有数字（即使是 0，不是 `--`）
- 趋势折线图 + 模型占比环形图渲染出来（没数据时显示 "暂无数据"，不是空白）
- **不应该看到**：红色 error / loading 卡死 / JS 控制台报错

---

## 3. Users 巡检（核心场景）

点左侧 **Users**。

**期望看到**：表格至少 11 行（demo seed 的 admin@democorp.local + user01~user10），每行有：

| 列 | 应该看到 |
|---|---|
| 邮箱 | `userNN@democorp.local` |
| 已用 tokens | 数字（首次跑是 0） |
| 已用 / 预算 | 进度条 + 文字。**预算为"无限制"是正常的**——seed 没设 budget |
| 状态 | `active` 绿色徽章 |
| 操作 | ✏️ 编辑按钮 |

### 3.1 改一个用户的预算

1. 找到 `user01@democorp.local` 那一行，点 ✏️
2. 出现金额输入框（单位是 USD 分；填 `100` 代表 1.00 USD 预算）
3. 输 `100`，按"保存"或回车
4. **期望**：表格自动刷新，那一行的"预算"列从"无限制"变成 `$1.00`，进度条出现（已用 $0 / 预算 $1.00）

### 3.2 把预算改成 0（演示拦截会生效）

1. 再点 user01 的 ✏️
2. 输 `0`，保存
3. **期望**：该行预算列显示 `$0.00`，进度条变红/danger（"已超额"标记）

> 这一步**只在前端**视觉验证 budget=0 的展示。"真发 chat 是否会被 402 拦"属于 gateway 行为，不在 admin 控制台里能点出来——交给 Codex 在 T-INT 阶段用脚本验证。

### 3.3 清掉预算

1. 第三次点 ✏️
2. 清空输入框（或输 `null`，看你的 UI 接不接），保存
3. **期望**：该行回到"无限制"

---

## 4. Audit 巡检（核对刚才的操作有留痕）

点左侧 **Audit**。

**期望看到**：最近 3 行就是你刚才改的 3 次预算。每行：

| 列 | 应该看到 |
|---|---|
| 时间 | 几秒/几分钟前 |
| Actor | `bootstrap`（T-005b 之前固定，T-005b 后会是你的 user UUID） |
| Action | `update_quota` |
| Resource | `user_quota / 00000000-...-0202` |
| Status | `200` |

点开任一行（展开 before/after JSON）：
- `before` 是上一次预算值（如 `{"budget_cents": 100}`）
- `after` 是新预算值（如 `{"budget_cents": 0}`）
- **不能出现**任何"secret"、"token"、"prompt"、"Authorization" 字样——这是 §11.6 安全基线

---

## 5. Models 巡检（10 秒）

点 **Models**。柱状图渲染、按模型分组的成本表能加载。**没数据是正常的**（demo seed 没真打过方舟）。

---

## 6. 不需要你点但要知道的事

| 项 | 现状 |
|---|---|
| 登录页 | 还没做（T-005b），现在 URL 带 `?token=...` 暂用 |
| viewer 角色 | 当前页面默认 admin，没法点出 viewer 模式（要 T-005b 接 `/api/admin/me` 才能切） |
| 真聊天 → 402 | 不在控制台里能点，Codex 用脚本验 |
| 异常 key 告警 | 每 5min 后台扫一次，超阈值打 WARN log，**不在前端 UI 里显示**（T-014 接受标准） |

---

## 7. 你点完得到的结论

如果第 2-5 节全部"期望看到"都能看到：**T-015 前端 + admin API 这层联调通过**。

如果哪一步卡住：截图 + 说哪一步、看到了什么。我或 Codex 接着排。

---

## 8. 收尾

不需要你做。Codex 自己 `make down`。
