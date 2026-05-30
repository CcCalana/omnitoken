# OmniToken 文档索引

> 入口线索：需要定位某个主题的文档？在本文件中搜索关键词 → 按路径打开目标文件。本文件 ~60 行，廉价可读。

## 核心工作文件（根目录）

| 文件 | 用途 |
|---|---|
| `CLAUDE.md` | Claude 工作守则，含角色边界、任务格式、评审标准 |
| `AGENTS.md` | Codex 工作守则，实施铁律、测试义务、commit 规范 |
| `TASKS.md` | 任务看板，唯一任务入口，含状态/接受标准/依赖 |
| `REVIEW.md` | 评审记录，每条 review 含等级/位置/修复要求 |
| `规划.md` | 产品与技术方案，唯一事实来源，含 §零A 设计理念 |
| `README.md` / `README.zh-CN.md` | 公开仓库首页（中英双版） |
| `THIRD_PARTY_LICENSES.md` | 第三方依赖许可证台账 |
| `Makefile` | 开发命令入口（up / test / lint / docker-build 等） |

## 架构决策记录 (docs/adr/)

| 编号 | 主题 | 状态 |
|---|---|---|
| ADR 0001 | 技术栈定版 Go 1.23 / PG 16 / Redis 7 / NATS | 活跃 |
| ADR 0002 | 单仓库布局 (monorepo) | 活跃 |
| ADR 0003 | 多 key 池优先级与 v1 拉回决定 | 活跃（T-016 来源） |
| ADR 0004 | Multi-provider 池 v1（DeepSeek 接入） | 活跃（T-MP-DEEPSEEK 来源） |
| ADR 0005 | Admin 凭据 CRUD (Min) + 热加载 | 活跃（T-016b-MIN 来源） |

## 运维与操作 (docs/operations/ + docs/runbooks/)

| 文件 | 用途 |
|---|---|
| `docs/operations/master-key-rotation.md` | Master key 轮换流程与 trust boundary 说明 |
| `docs/runbooks/local-dev.md` | 本地开发环境搭建（Docker Compose 起服务） |

## API 规范

| 文件 | 用途 |
|---|---|
| `docs/api/openapi.yaml` | OpenAPI 3.0 规范文档 |

## 参考文档 (docs/references/)

| 文件 | 用途 |
|---|---|
| `docs/references.md` | 外部参考链接索引 |
| `docs/references/agent-adapter/agent-adapter-pattern.md` | Claude Code / Codex / OpenCode 适配器实现模式 |
| `docs/references/agent-adapter/agent-adapter-projects-reference.md` | tingly-box 等参考项目分析 |
| `docs/references/agent-adapter/tingly-box-architecture-reference.md` | tingly-box 架构速查 |

## 历史归档 (docs/archive/)

| 目录 | 内容 | 何时查阅 |
|---|---|---|
| `docs/archive/proposals/` | 16 个已完成任务的设计提案 | 回溯某任务的原始设计讨论 |
| `docs/archive/reviews/` | 4 个历史 review 归档 | 查旧 review 结论 |
| `docs/archive/releases/` | 5 个 v1 release 报告 + 并发摸底报告 | 查 v1 压测数据 |

> 归档文档均为冻结历史记录，日常不读。git log 可追溯，archive 只是就近备查。
