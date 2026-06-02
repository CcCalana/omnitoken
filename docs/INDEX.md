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

## 评审归档 (docs/reviews/)

| 文件 | 内容 | 何时查阅 |
|---|---|---|
| `docs/reviews/archive.md` | R-001 ~ R-007 (Phase 0/1) | 最早期的 review |
| `docs/reviews/archive-2026-05-12.md` | R-006b-prop ~ R-006d (Demo-Ready) | Phase 1 收尾 |
| `docs/reviews/archive-2026-05-19.md` | R-008 ~ R-005b-fix (Phase 2-A + 2-B) | Phase 2 底座三角 |
| `docs/reviews/archive-2026-05-20.md` | R-INT ~ R-040 (v1 联调 + Phase 3-A Adapter) | v1 release + Agent 适配 |
| `docs/reviews/archive-2026-06-02.md` | R-045-prop ~ R-CONC-CHECK (Phase 3-A + v1 关键实施 + 并发摸底) | Phase 3-A + v1 并发/多 provider/credential |
| `REVIEW.md` (根目录) | vNext 路线 review (R-100 起) + 未解决项摘要 | 当前活跃 review |

## 任务归档 (docs/tasks/)

| 文件 | 内容 |
|---|---|
| `docs/tasks/archive-2026-06-02.md` | Phase 3-A / v1 release / v1 关键实施 / vNext 已完成任务体 |
| `TASKS.md` (根目录) | CHANGELOG + 速查表 + vNext 活跃任务体 |

## 发布报告 (docs/release/ + docs/archive/releases/)

| 文件 | 内容 |
|---|---|
| `docs/release/v1-quota-baseline-2026-06.md` | T-QUOTA-CACHE-PROBE PG 额度查询性能基线 |
| `docs/archive/releases/` | v1 release 报告 + 并发摸底报告 |

> 归档文档均为冻结历史记录，日常不读。git log 可追溯，archive 只是就近备查。
