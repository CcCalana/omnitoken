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
1. 读 规划.md **仅**零节「当前项目状态」（约 5 行）— 确认当前 Phase。不要读全文。
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
5. **依赖与许可证政策**（**2026-05-11 R-003-license 更新为分级规则**）：

   依赖新增的流程：先在 `TASKS.md` 任务条目 `Dependencies` 或 `PROPOSAL` 区写明（包名、用途、替代品评估、license），等 Claude 在 `REVIEW.md` 给 `[+] approved` 后再锁版本。**直接 + 间接**依赖都要写。

   **许可证分级**（按是否需要 propose 决定）：

   **5.a Approved without propose（permissive OSI，可直接用）**
   - MIT / BSD-2-Clause / BSD-3-Clause / ISC
   - Apache-2.0
   - 0BSD / Unlicense
   - BSL-1.0 (Boost Software License，**不是** Business Source License)

   **5.b Approved as transitive only, MUST NOT fork/modify（weak copyleft，间接依赖可接受）**
   - MPL-2.0
   - EPL-2.0
   - CDDL-1.0 / 1.1

   出现在 `go.mod` 间接依赖里时不需 propose；但必须更新 `THIRD_PARTY_LICENSES.md` 台账，标注"未 fork、未修改源文件"。**禁止** 改这些包的源代码——一改性质就变成"直接受 license 约束"。

   **5.c Propose required（必须停下来让 Claude 决策）**
   - 把 5.b 中的 weak copyleft 作为**直接依赖**引入。
   - LGPL-2.1 / LGPL-3.0（即使间接也要 propose）。
   - 任何 GPL 系：GPL-2.0 / GPL-3.0 / AGPL-3.0。
   - 商业受限或"源可见但商用受限"协议：SSPL、BSL（Business Source License）、Elastic License、Confluent Community License、Redis Source Available License 等。
   - license 标注缺失、不清晰、或自定义协议。

   **5.d Hard ban（直接拒绝，不接受 propose）**
   - Proprietary 闭源依赖。
   - 任何明确禁止商业使用的协议。
   - 修改 AGPL 源码并合并到本仓库。

   **5.e 自动化门**: CI 必跑 `github.com/google/go-licenses check ./...`，允许列表 = 5.a + 5.b；命中 5.c 即阻断 CI 直至 propose 通过并加进 allow-list。allow-list 文件 `.licenses/allowed.txt`，每次新增条目都附 `REVIEW.md` 引用号。
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

---

## 9. 本地 Secrets 与上游联调（Phase 1 用）

**仓库根目录 `.env`** 是本地唯一的 secret 落点，**已加入 `.gitignore`**（`.gitignore:7`）。Phase 1 的所有联调与 L2 e2e 测试都从这里读取。

### 9.1 当前已授权的 dev secrets

- `OMNITOKEN_ARK_API_KEY` —— 火山方舟 dev key，用户 2026-05-11 授权用于 Phase 1 全量测试（含 L2 e2e）。`.env` 已填好。
- 上游 URL / 默认模型 / DisableThinking 默认值同步落在 `.env`。

### 9.2 调用规则（**强约束**）

1. **不要把 `.env` 的真 key 写入任何源文件、测试 fixture、commit 消息、日志、Issue/PR 描述、注释**。`testdata/golden/ark/` 里已经有脱敏样本作为回放基线，做单测时优先用这些。
2. **不要把 key 通过 HTTP / 任何外发请求泄漏**。`internal/proxy` 转发上游时也只走配置注入路径，不允许打印请求头。
3. **运行成本意识**：单次 e2e 跑预计消耗 1–5 元 RMB token（详见 T-100 的成本上限保护）。L2 套件**默认不在每次 PR 上跑**，只 nightly + 手动；本地调试时也要给出预算上限（建议 `MAX_REQUESTS` 环境变量）。
4. **如果发现 key 在任何输出里出现**：立刻按 `AGENTS.md §6.3 紧急通道` 处理（顶部 URGENT + REVIEW.md 红旗），等用户决定是否轮换。
5. **CI/GitHub Actions 注入**：T-100 nightly job 必须从 `secrets.OMNITOKEN_ARK_API_KEY` 读，本仓库 `.env` 文件**只**在本地使用。

### 9.3 加载 `.env` 的两种姿势（任选其一，**不要**写 .env loader 进生产代码）

- PowerShell：`Get-Content .env | Where-Object { $_ -and -not $_.StartsWith('#') } | ForEach-Object { $kv = $_ -split '=', 2; [System.Environment]::SetEnvironmentVariable($kv[0], $kv[1]) }`
- Bash：`set -a && source .env && set +a`

或者使用一个**只用于 e2e / smoke**的 dev 辅助二进制（如 `cmd/e2e-runner` 自带 godotenv），但生产 `cmd/gateway` / `cmd/admin` 必须只读环境变量，不读 `.env` 文件。

### 9.4 实测过的最快配方（写进未来代码默认值或 demo 文档）

OpenAI-compat 端点 + 请求体加 `thinking: {"type": "disabled"}` + `stream_options: {"include_usage": true}` → 稳定 1.7–2.0s 完成含 usage 的完整 SSE。Anthropic-compat 端点延迟显著更高且 thinking 默认开启，**Phase 1 内不推荐做主路径**。
