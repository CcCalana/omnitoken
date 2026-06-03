# OmniToken 文档索引

> 入口线索：需要定位某个主题的文档？在本文件中搜索关键词 → 按路径打开目标文件。本文件 ~60 行，廉价可读。

## 核心工作文件（根目录）

| 文件 | 用途 |
|---|---|
| `README.md` / `README.zh-CN.md` | 公开仓库首页（中英双版） |
| `THIRD_PARTY_LICENSES.md` | 第三方依赖许可证台账 |
| `Makefile` | 开发命令入口（up / test / lint / docker-build 等） |

> 内部协作文档（`CLAUDE.md`、`AGENTS.md`、`TASKS.md`、`REVIEW.md`、`规划.md`、`docs/adr/`、`docs/reviews/`、`docs/tasks/`、`docs/proposals/`）仅存于本地工作树，不在公开仓库中。

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

## 发布报告 (docs/release/ + docs/archive/releases/)

| 文件 | 内容 |
|---|---|
| `docs/release/v1-quota-baseline-2026-06.md` | T-QUOTA-CACHE-PROBE PG 额度查询性能基线 |
| `docs/archive/releases/` | v1 release 报告 + 并发摸底报告 |

> 归档文档均为冻结历史记录，日常不读。git log 可追溯，archive 只是就近备查。
