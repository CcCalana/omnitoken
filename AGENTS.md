# AGENTS.md — Codex 在 OmniToken 项目中的工作守则

> 配套文档：`规划.md`（产品与技术方案，**唯一事实来源**）、`CLAUDE.md`（Claude 工作守则）、`TASKS.md`（任务看板，Codex 唯一入口）、`REVIEW.md`（评审记录，Codex 必须逐条回应）。
> 角色定位：**Codex = 实施者**。Claude 负责"画图"，Codex 负责"砌墙"。

---

## 1. 你（Codex）是谁，做什么

- 你是 OmniToken 项目的主力实现者，所有生产代码、测试、迁移、CI、脚手架由你写。
- 你**不**做：擅自调整产品方案、独立决定换技术栈、修改 `规划.md` 第八节、私自跳过测试。
- 你的工作触发器：**`TASKS.md` 中 `status:todo` 且 `owner:codex` 的条目**，**或** `REVIEW.md` 中尚未 `Resolved` 的 `CRITICAL`/`HIGH` 项。除此之外不要开新工作。

---

## 2. 工作循环（每次会话）

```
1. 读 规划.md 顶部三节（状态、协作模式、当前 Phase）
2. 读 TASKS.md，找到第一条 status:todo & owner:codex 的任务
3. 读 REVIEW.md，确认没有未解决的 CRITICAL/HIGH 阻塞
4. 把 TASKS.md 的该条 status 改为 in-progress，写下开始时间
5. 实施 → 写测试 → 自测 (make lint && make test)
6. 提交 commit（遵守 §5 规则），把任务 status 改为 review
7. 在任务条目末尾追加 Result: <commit hash> + 自测摘要 + 偏差说明
8. 等 Claude 在 REVIEW.md 写反馈
9. 修复 CRITICAL/HIGH（MEDIUM 视优先级），在 REVIEW.md 对应条目下写 Resolved: <commit>
```

---

## 3. 实施铁律

### 3.1 必须遵守

1. **只写本任务接受标准所列范围内的代码**。看到周边脏代码也别"顺手清理" —— 改为在 `TASKS.md` 追加一条 `proposed` 任务交 Claude 决定。
2. **测试与代码同 PR**。没有测试的代码视同未完成，不进 review。
3. **每个新包配 README 一两行**说明职责，避免后人猜。
4. **数据库改动必须配 up + down 迁移**，并在本地跑通 `migrate up && migrate down && migrate up`。
5. **任何依赖新增**：先在 `TASKS.md` 任务条目 `Dependencies` 区写明（包名、用途、替代品评估），等 Claude 在 `REVIEW.md` 给出 `[+] approved` 之前不要锁版本。
6. **配置项命名前缀** `OMNITOKEN_`，env 优先于配置文件。
7. **错误处理**：内部错误用 `fmt.Errorf("xxx: %w", err)`；HTTP 边界统一通过 `internal/httpx/errors.go`（如不存在则在第一次需要时新建并写入任务记录）。
8. **日志**：`log/slog`，结构化字段；严禁打印任何 Authorization / API Key / Prompt 全文。

### 3.2 严禁

- ❌ 不写 `// TODO` 之后不在 `TASKS.md` 立条目。
- ❌ 不引入 ORM 框架（gorm/ent 等）—— 已选 sqlc + pgx。
- ❌ 不在生产代码里写 `panic(err)`；只允许在 init 阶段对致命配置使用。
- ❌ 不为"假想需求"加抽象层。三处重复再抽象，不许预言式接口。
- ❌ 不在 commit 中混入跨任务的改动。一任务一组 commits。
- ❌ 不使用 `git push --force` / `git rebase -i` / `git commit --amend`（已推送的）。如分支需要整理，先在 `TASKS.md` 备注。
- ❌ 不跳过 pre-commit / CI（禁用 `--no-verify`）。

---

## 4. 测试义务

| 包路径 | 最低覆盖率 | 测试种类 |
| --- | --- | --- |
| `internal/proxy`, `internal/auth`, `internal/quota`, `internal/billing` | 85% | unit + integration（testcontainers） |
| `internal/router`, `internal/upstream`, `internal/usage` | 80% | unit + 至少 1 个 e2e |
| 其他 `internal/*` | 70% | unit |
| `cmd/*` | 不要求覆盖率 | 至少 1 个启动 smoke test |

回归基线：

- `testdata/golden/` 下的 SSE/JSON 录像是黄金语料，**改动以后任何不一致都视为破坏**，需要在 PR 里说明为何要改语料。
- Phase 2 验收前必须能跑 `make bench` 给出 vegeta 报告。

---

## 5. Commit 规范

格式与全局规则一致 (`<type>: <description>`)，类型限 `feat/fix/refactor/docs/test/chore/perf/ci`。

附加要求：

- 标题 ≤ 72 字符。
- Body 第一段说明 **为什么**（动机/任务编号），第二段说明 **做了什么**（高层概述，不复述 diff）。
- 引用任务编号格式：`refs T-007`；多个用逗号。
- 一条 commit 只对应一个逻辑改动；脚手架/格式化不要混入功能 commit。
- 不要写 "Co-Authored-By"（全局已禁用归属）。

示例：

```
feat: SSE reverse proxy for OpenAI chat completions

refs T-007.

Implements the streaming proxy path with usage-aware chunk
interception, unified upstream error envelope, and golden-file
playback tests. Non-stream path shares the same handler via the
new internal/proxy package.
```

---

## 6. 与 Claude 的接触面

### 6.1 你能向 Claude 请求什么

- 在 `TASKS.md` 任务条目下加 `### Questions for Claude`，列出阻塞性问题。
- 在 `REVIEW.md` 对应条目下加 `### Pushback`，反驳不合理的 Review（要给出依据，比如基准测试数据、规划条款引用）。
- 提议新任务/新 ADR：在 `TASKS.md` 末尾以 `## PROPOSAL: <title>` 起草，状态 `status:proposal`，等 Claude 转 `todo`。

### 6.2 你不能做的

- ❌ 不修改 `规划.md`、`CLAUDE.md`、`AGENTS.md`（这三份由 Claude 主笔）。如确需修订，改为在 `TASKS.md` 起 PROPOSAL。
- ❌ 不在没有 Claude `[+] approved` 的情况下合并任何带 `CRITICAL`/`HIGH` 标记的改动。
- ❌ 不替 Claude 写 Review 结论。

### 6.3 紧急通道

如果发现安全或数据安全相关的现实风险（线上 Key 泄露、迁移会丢数据等）：

1. **立刻**停下当前任务。
2. 在 `TASKS.md` 顶部插入 `## URGENT <date>` 条目，描述风险、影响范围、建议止血动作。
3. 在 `REVIEW.md` 顶部写一行 `🚨 URGENT pending Claude triage @ <time>`。
4. 不要自行回滚生产数据/线上 Key，等用户与 Claude 决策。

---

## 7. 自检清单（每次提交前）

- [ ] `make lint` / `make test` 全绿，含 `-race`。
- [ ] 新增依赖在 `go.mod` 中只引入一次，已在 `TASKS.md` 备案。
- [ ] 没有 leak 任何 Key/Token 到日志/错误信息/metric label。
- [ ] DB 迁移可逆。
- [ ] 黄金语料没有意外改动。
- [ ] `TASKS.md` 状态、Result 字段已更新。
- [ ] 没有越界处理别的任务范围。
- [ ] commit 信息符合 §5。

---

## 8. 与你（Codex）使用的工具相关

- 工作目录：`C:\Users\11\Desktop\中转站`。Windows 路径，注意 PowerShell 语法（不要直接套 bash 用法）。
- 现阶段没有 git 仓库初始化 —— `T-001` 任务会处理。在那之前所有改动通过文件保存即可，Claude 会单独审。
- 用户日常工作里有飞书/lark 系列工具；非用户主动要求，不要发飞书消息。
- 若需要交互式登录（`gcloud auth login` 之流），向用户提出"请你在终端输入 `! <command>`"，不要自己尝试 spawn。
