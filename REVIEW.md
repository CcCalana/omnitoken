# REVIEW.md — OmniToken Review Log

> **归档说明**:
> - R-001 ~ R-007 → `docs/reviews/archive.md`
> - R-006b-prop / R-006b / R-006c / R-006d → `docs/reviews/archive-2026-05-12.md`
> - R-008 ~ R-005b-fix (Phase 2-A 收官 + Phase 2-B 全程) → `docs/reviews/archive-2026-05-19.md`
>
> 本文件保留最近 review；老的归档到 docs/reviews/。

## 未解决项摘要（从所有 review 累积）

| ID | 来源 | 严重度 | 描述 | 状态 |
|----|------|--------|------|------|
| M-13 | R-006a | MEDIUM | gateway 硬依赖 DB，未来考虑 `--auth=stub` | Informational, 留 T-005c |
| M-14 | R-006a | MEDIUM | CreateVirtualKey 不开事务 | Informational, 留 T-005b |
| M-16 | R-008 | MEDIUM | body double-read (1MiB) | Informational, Phase 2 优化 |
| M-17 | R-008 | MEDIUM | SQL fallback 双重防御 | Informational, 设计正确 |
| M-18 | R-006b | MEDIUM | overview 3 条 query 非事务 | Informational, Phase 2 |
| M-19 | R-010 | MEDIUM | 503 admin_auth_not_configured 在生产部署中可能成隐患（默认不放行正确，但运行期需 alerting） | Informational, Phase 2 alerting |

---

## R-INT (T-INT v1 联调收官, commit `4db5057`)

**结论: `[+] Approved — v1 release candidate ready`** — 17/17 真方舟 + 14/14 control-plane smoke + 前端 4/4 + `git check-ignore` 验证 .env 不入库。

**正面信号**:
1. ✅ 密钥纪律到位：`OMNITOKEN_ARK_API_KEY` 仅落本地 `.env`（gitignored），release report 显式贴 `git check-ignore -v .env` 证据；commit diff grep 无 `sk-` / `ARK_API_KEY=<value>` 泄漏。
2. ✅ `scripts/v1_integration.py` 186 行可重复跑：control-plane smoke 默认跑、`OMNITOKEN_RUN_REAL_UPSTREAM=1` 才打 Ark——成本可控。
3. ✅ Dockerfile gateway/admin/migrate 统一升 Go 1.25，与 `go.mod` 对齐，避免 compose 内 build mismatch。
4. ✅ viewer seed 登录 + session role propagation 修补到位（admin/viewer 真实可登录 + RBAC 403 真实触发）；前端 Users tab 按 `/api/admin/me` 读到的 role 显隐编辑按钮——T-015 N-3 留的"viewer 模式没法点"自然 resolved。
5. ✅ 部署文档闭环：`README.md` 加 v1 部署章节、`.env.example` 补齐 v1 新增 env、`docs/release/v1-integration-2026-05-19.md` 含截图证据 + 17/17 报告。

**N-1**: T-005b R-005b-fix 那条"503→401 transition"注释仍未落到 `cmd/admin/main.go` `adminAuthMiddleware`；不阻塞 v1，下次涉及该函数顺手加。

**Phase 2-B: 5/5 ✅ — v1 release candidate 就绪**。

---

## R-041-prop (T-041 PROPOSAL, commit `3187fb2`)

**结论: `[+] Approved`** — 3 决策全采纳推荐方向 + 4 条 implementation notes 都到位。1 个 Q 实施时必须明确，2 个 N 不阻塞。

**正面信号**:
1. ✅ 决策 1/2/3 全部对齐 R-prop 推荐：独立 `cmd/omnitoken-adopt` / 显式 `--token` / 备份全部保留。CLI 子命令签名（`adopt claude-code` / `restore claude-code`）清晰。
2. ✅ Windows 文件名安全是真问题——RFC3339 的 `:` 在 NTFS 是 ADS 分隔符，建议格式 `2026-05-19T10-02-00.000000000Z` 保 lex 排序 = 时序，`restore` 取最新备份无歧义。
3. ✅ "CLI 输出禁回显完整 token" 这条 operational guardrail 自动落到 §11.6 安全基线一致——secret 不进 stdout/stderr/log。
4. ✅ 防御性细节：root JSON / `env` 字段不是 object 时 fail-without-write、首次写无备份（无原文件可保）、严格不碰 `.claude.json`/状态栏/Codex/OpenCode。
5. ✅ `ANTHROPIC_BASE_URL` 直接透传 `--gateway-url`，把 Anthropic-format endpoint 行为完全留给 T-045，不在 T-041 偷跑协议层。

**Q-1（实施前必须明确）**: PROPOSAL 说"preserve existing unrelated env keys"，但 tingly-box `agent-adapter-pattern.md` §3.3 的 env 字段集除了 `ANTHROPIC_BASE_URL`/`ANTHROPIC_AUTH_TOKEN`/`ANTHROPIC_MODEL`/`ANTHROPIC_DEFAULT_*_MODEL`/`CLAUDE_CODE_SUBAGENT_MODEL`，还有 `API_TIMEOUT_MS=3000000` / `DISABLE_TELEMETRY=1` / `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` / `CLAUDE_CODE_MAX_OUTPUT_TOKENS=32000` 这些"OmniToken 推荐配置"。请明确"OmniToken 管理 (overwrite) 的 env key 白名单"——建议在代码里写成常量 `omniTokenManagedKeys = []string{...}`，OmniToken 接管的 key 每次都覆盖，其他 key 一律保留。实施 Result 里贴一下这个清单。

**N-1**: 时间戳格式建议直接用 Go `time.Now().UTC().Format("20060102T150405.000000000Z")`（compact ISO 8601 basic format，无 `:`，22 字符），比 PROPOSAL 中 `2026-05-19T10-02-00.000000000Z` 带 `-` 分隔符更短且 lex 排序仍正确。或更简：`time.Now().UTC().UnixNano()` 纯数字，跨 OS 100% 安全，缺点是不易人眼读。任选其一在实施 commit 里说明。

**N-2**: "fail without writing if root/env 不是 object" — CLI 退出码请用非零（推荐 `2` for invalid existing config），便于 shell 脚本检测。同时 stderr 一行清晰错误，不要 panic stacktrace。

**Codex 可开 T-041 实施**。N-1 / N-2 落实施 Result 即可。

---

## R-041 (T-041 Claude Code 适配 实施, commit `a6d1d09`)

**结论: `[+] Approved with MEDIUM follow-ups`** — 接受标准 7/7 ✓；R-041-prop 的 Q-1 / N-1 / N-2 全部落地；2 条 MEDIUM 不阻塞 v1 演示，建议并入 T-042 一起修；2 条 NIT 可忽略。

**正面信号**:
1. ✅ Q-1 答得到位 + 加分：`omniTokenManagedKeys` 12-key 常量列表 + 导出 `ManagedClaudeCodeEnvKeys()` 返回 copy 防外部 mutation + CLI 把列表打到 stdout (`managed_env ANTHROPIC_BASE_URL,...`)，员工运行后能直接看见"OmniToken 接管了哪些 key"。`TestManagedClaudeCodeEnvKeysReturnsCopy` 显式守护 immutability。
2. ✅ N-1 / N-2 都落到位：时间戳就是 `20060102T150405.000000000Z`（compact ISO 8601 basic, lex 排序 = 时序），无效 config 走 `ErrInvalidExistingConfig` sentinel → CLI `errors.Is` 后返回 exit 2 + `TestRunCLIInvalidExistingConfigExitsTwo` 断言 stderr 单行无 panic stacktrace。
3. ✅ 安全断言写进测试而非靠 review 抓：`TestRunCLIAdoptClaudeCodeUsesHomeOverride` 显式 `strings.Contains(stdout/stderr, "omt_secret")` 验证 token 不外泄 —— 比 review comment "请小心别打印 token" 强一万倍，未来重构也跑得住。
4. ✅ 4 种无效 config 全覆盖（root 非 object / env 非 object / env=null / malformed JSON），每条都验证"原文件未改 + 备份未建"。`uniqueBackupPath` 同纳秒 `.001` 后缀防碰撞是 defense-in-depth，`TestLatestBackupPathSortsSuffixes` 验证排序仍正确。
5. ✅ `TestRestoreClaudeCodeSettingsUsesDefaultHome` **故意不 `t.Parallel()`**（因 `t.Setenv` HOME/USERPROFILE 会污染并行测试），其他 case 全 `t.Parallel()` —— Go 测试小肌肉记忆到位。

**M-20 (MEDIUM)** `internal/agent_adapter/claude_code.go:89-93`: `root["env"] = encodedEnv`（compact JSON bytes）写回，然后 `writeJSONFile` 用 `json.MarshalIndent` 对 root indent — 但 `json.RawMessage` 字节会**原样嵌入不重新 indent**，结果是用户原本多行格式化的 `settings.json`，被改完后 `env` 块变成单行紧凑 JSON、其他字段照常 indent，视觉很丑，git diff 也难看。修法：把 `env` 也用 `map[string]any` decode/encode 走正常 marshal 路径，或在嵌入前对 env 单独 `MarshalIndent` 后做缩进对齐。**不阻塞 v1，但 T-042/T-043 触类旁通，建议一并修**。

**M-21 (MEDIUM)** `internal/agent_adapter/claude_code.go:112` `writeJSONFile` 直接 `os.WriteFile` 覆盖 settings.json，不是原子写。崩溃 / 断电时会留下半写入文件，导致 Claude Code 下次启动 parse 失败。修法：write to `<path>.tmp` + `os.Rename`（POSIX rename 原子）。**v1 演示场景下概率极低不阻塞**；员工大规模部署前必修。

**N-3 (NIT)** `copyFile:247` 备份与 restore 都 hard-code `0o600`，丢用户原文件 perm。Claude Code 默认 `0o644`，restore 后变 `0o600` —— 更严更好，但 silent 改 perm 是 surprise。可用 `os.Stat(src).Mode()` 透传原 perm。

**N-4 (NIT)** CLI `--gateway-url` 不校验是否 https。员工误传 `http://...` → token 明文 over wire。建议 plain HTTP 时 stderr 打 WARN（不 fail，本地 localhost 调试要允许）。

**📌 提醒（非缺陷，与代码质量无关）**: Codex 在 chat Result 里写了"race blocked by missing gcc"。`a6d1d09` 是 19:44 提交，**早于 AGENTS.md §3.3a 第 5 条新增（禁汇报 missing gcc / -race blocked，race 统一 Docker/CI 兜底）**，所以这次不算违规。**下一轮拉新任务前请重读 §3.3a + §3.3 + §7 一次**，T-042 / T-MK-RACE 实施 Result 里不要再出现这类措辞。`-race` 红时才汇报，绿与 N/A 都不必提。

**M-20 / M-21 处理建议**: 不开独立任务（避免任务卡爆），并入 **T-042 Codex 适配** 时一起改 —— 那时也要写 `~/.codex/config.toml`，文件写入与备份的 helper 大概率会抽出来，原子写 + indent 保形可以在抽 helper 时一并修。Codex 在 T-042 commit message 里 `refs T-041` + 简述 fix 即可，**不要**单独开一条 fix-only commit。

---

## R-MK-RACE (T-MK-RACE Makefile race 移入 Docker, commit `a44f27a`)

**结论: `[+] Approved with 1 NIT`** — 5/5 接受标准满足；新政策的"chat 里不再提 missing gcc"也立刻生效（commit message 干净，TASKS Result 段没出现禁言措辞）。

**正面信号**:
1. ✅ `golang:1.25` 而不是我说的 `1.23` —— Codex catch 了我的 stale memory（CI / Dockerfile.* / go.mod 现都在 1.25，详见 R-INT 第 3 条）；这是 Codex 主动核对 ground truth 的正面信号，不照搬 spec。
2. ✅ named volume `omnitoken-go-mod` + `omnitoken-go-build` 挂 `/go/pkg/mod` + `/root/.cache/go-build`，**不污染工作树** —— AGENTS.md §3.3 末条目要求落到位。
3. ✅ `.PHONY` 列表、`make help` 文案、`make test` / `make test-race` 双 target 一次性同步，没留半成品。
4. ✅ CI workflow 没动 —— 符合"不在范围"清单。

**N-5 (NIT)** `cmd/migrate/main.go:237-249` 引入 `slashPath` 替 `filepath.ToSlash` —— 解释是"让同一 suite 在 Linux 容器跑通"，但 `filepath.ToSlash` 在 Linux 上对纯 Linux 路径是 no-op，只有混入 `\\` 字符串时新增的 `strings.ReplaceAll` 才生效。可能是 testdata fixture 或 env 注入路径携带反斜杠。下次 commit message 或测试注释里点一句"哪条 migrate 测试在 Linux 容器吃了 `\\`"，便于以后维护。**不阻塞**，逻辑正确。

**T-MK-RACE 验证**: 我没有 Docker daemon 没真跑 `make test-race`，信 Codex 的 "all green"；下次 CI 触发时若 race job 红则反向追查。

---

## R-042-prop (T-042 PROPOSAL, commit `34ea18f`)

**结论: `[+] Approved`** — 3 决策全部对齐我在 T-042 任务体里给的推荐方向。1 Q 实施时必须验证，1 N 落实施 Result 即可。**Codex 可开 T-042 实施**。

**正面信号**:
1. ✅ Decision 1 选 line-based 最小 patcher 而非引 `pelletier/go-toml/v2` —— 完全对齐"管已知白名单、保其他字节"的 T-041 哲学，省一条依赖 propose、避免 license ledger 改动。
2. ✅ Decision 1 给了 4 条 fail-fast scanner 规则（unterminated string / malformed header / duplicate managed key / 非 string-bool 值）+ "替换整个 `[model_providers.omnitoken]` table body 但不动其他 provider table"的清晰边界 —— 是真做过 design 不是糊弄。
3. ✅ Decision 2 把 secret 与 routing 分两文件落（`auth.json` 只放 `OPENAI_API_KEY`，`config.toml` 放路由/auth mode）—— 与 Codex 官方文档以及 §十一 第 6 条"凭据不落代码"一致。
4. ✅ Decision 3 显式拒绝现在抽 `Registry` / `AgentConfig`，只抽 `writeAtomic` + JSON merge + 备份 helper —— AGENTS.md §3.1 "三处重复再抽象" 信达雅。`writeAtomic` 用 sibling temp + `fsync` best-effort + rename，正是 R-041 M-21 的标准修法。
5. ✅ `/v1` 后缀解释主动写出来 —— "OmniToken 现在暴露 OpenAI-compat `/v1/models` 和 `/v1/chat/completions`"，给 T-045 时换 Anthropic-format 留好接口。

**Q-1（实施前必须明确）**: Decision 1 "line-based scanner + duplicate managed key 检测" 的精确边界。具体三个 case 请在 implementation Result 里说明处理：① `[foo]\nmodel = "x"`（managed key 出现在非 OmniToken 子 table），scanner 是否会误判为 top-level duplicate？② multi-line string 值 `key = """\n...\n"""` 是否被 "unterminated string" 误报？③ inline-table 写法 `model_providers = { omnitoken = { ... } }` —— 拒绝 / 重写 / 保留？建议 ③ 直接 fail-fast 报 "unsupported config style, run restore first"，员工层不会写这种。

**N-6 (NIT)**: Decision 2 的 `cli_auth_credentials_store = "file"` 覆盖政策。如果员工原来手工设了 `system`（OS keychain），OmniToken 强写为 `file` 是合理但**突然**的安全行为变更。建议 CLI stdout 在写之前一行 WARN："cli_auth_credentials_store: <old> → file (OmniToken-managed)"。同理 `requires_openai_auth = true` 如果不是 Codex 默认值也值得提一句。

**Codex 可开 T-042 实施**。Q-1 答在实施 commit Result 里，N-6 落 stdout WARN 即可。

---

## R-042 (T-042 Codex 适配 实施, commit `ceb123c`)

**结论: `[+] Approved`** — 8/8 接受标准过；R-042-prop Q-1 三个 edge case 全覆盖；N-6 不仅做了 `cli_auth_credentials_store` WARN 还顺带做了 `requires_openai_auth`；R-041 留的 M-20 / M-21 / N-3 一次性全修。可直接进 T-043 OpenCode（依赖 T-042 已 satisfy）。

**正面信号**:
1. ✅ Q-1 三个 edge case 在 `scanCodexConfig` 都验过 + 测试守护：① 非 OmniToken 子 table 同名 key 不误判为 top-level duplicate（`section == ""` 守门 line 333）；② 多行字符串 `"""` 走 `startsTripleString` 旁路、绕过 `hasUnterminatedString`；③ inline-table `model_providers = { ... }` 显式 `inlineProvider = true` → 返回我建议的 `unsupported config style; run restore first` 错误（line 248-249）。
2. ✅ R-041 历史项**一次性全修**到 `fileio.go`：M-20 用 `map[string]any` 递归 marshal、env 块不再压成单行 + `claude_code_test.go:95` 加 `strings.Contains(settings, "  \"env\": {\n    \"ANTHROPIC_AUTH_TOKEN\"")` literal 断言守护回潮；M-21 `writeAtomic` 走完整 MkdirAll → CreateTemp → Write → Chmod → Sync → Close → Rename → **parent dir fsync (`syncDir`)**，教科书级原子写；N-3 `copyFile` 改读 `os.Stat(src).Mode().Perm()` 透传原 perm 不再硬编码 0o600。
3. ✅ N-6 + 加分：`codexManagedWarnings` 同时检测 `cli_auth_credentials_store` 非 `"file"`（员工原走 keychain 的会被警示）和 `requires_openai_auth` 非 `true` —— 比我建议的多覆盖一项；warning 通过 `Result.Warnings` 数组冒泡到 CLI stdout `WARN ...` 行，不混进 stderr。
4. ✅ helper 抽法克制：只到 `fileio.go` 文件级（writeAtomic / readJSONObject / uniqueBackupPath / latestNamedBackupPath / copyFile / resolveHome / nowUTC），**没**抽 `Registry` / `AgentConfig` interface —— 严格守 AGENTS.md §3.1 "三处重复再抽象"。Result struct 拓展为兼容字段（`SettingsPath` 保留 + 新 `ConfigPath`/`AuthPath`/`BackupPaths`/`Warnings`），不破坏 T-041 现有调用。
5. ✅ URGENT 处理纪律到位：T-042 commit message 干净无"missing gcc"语，AGENTS.md §3.3a 第 5 条 + §9.5 (smoke 方法学) 都遵守。`internal/agent_adapter` 覆盖率 82.6%（达标 ≥80%），`cmd/omnitoken-adopt` 69.2%（cmd/* 不要求）。

**N-7 (NIT)**: `fileio.go:571 firstString` 用 `sort.Strings` 取字母序最小路径作为 `Result.BackupPath` 兼容字段 —— config backup 字母序在 auth backup 之前，所以 `BackupPath` 显示 config 备份名。语义略 confusing（"first" 通常理解为"先建的"），但实际调用者应该消费 `BackupPaths []string`，`BackupPath` 是单值 backward-compat。T-043 OpenCode 实现时若需要类似的"主备份"概念可以借机重命名为 `PrimaryBackupPath` 或干脆删除。**不阻塞**。

**T-040 触发判断**: T-042 完成后 `internal/agent_adapter` 已有两个具象 adapter（Claude Code / Codex）+ 共享 fileio helper。T-043 OpenCode 落地后即满足"三处重复"，**届时**抽 `Registry` / `AgentConfig` interface（T-040）即可。当前不动。

---

## R-043-prop (T-043 PROPOSAL, commit `d3088d3`)

**结论: `[+] Approved`** — 3 决策全部对齐我的推荐方向，且 Decision 1 主动纠正了我任务体里的 spec 错误。1 Q 实施时拍板，1 N 落实施即可。**Codex 可开 T-043 实施**。

**正面信号**:
1. ✅ Decision 1 **纠 spec**：OpenCode 官方 schema 是 **singular `provider`**（不是我在任务体里写的复数 `providers`）。Codex 对照 https://opencode.ai/config.json + tingly-box + token_proxy 三方交叉验证后定singular，并明确"如果用户既有 `providers` 复数 key 存在，**保留不动**当用户数据，只写正确的 singular `provider`"——这是第二次 Codex 主动核对 ground truth 纠我（前一次是 golang:1.25 vs 我的 1.23 stale memory）。这种独立性正是我们想要的。
2. ✅ Decision 1 字段集与 OpenCode `@ai-sdk/openai-compatible` provider 形状对齐：`npm` / `options.baseURL` / `options.apiKey` / `models.<model>.name`，含 `$schema` root 字段。Token 落 `options.apiKey` 明文但 CLI stdout 只打 key path 不打 value——明确写出"never echo to stdout/stderr, cover leakage with the same CLI-output assertions used in T-041/T-042"。
3. ✅ Decision 2 XDG 路径三档顺序清晰（`--home` → `XDG_CONFIG_HOME` → `<home>/.config/opencode/`），**Windows 不走 `%APPDATA%`** —— 与 token_proxy 实现 + tingly-box 参考一致，避免再加平台分支。测试明确 "explicit tests for XDG_CONFIG_HOME set, unset fallback via HOME/USERPROFILE, and --home overriding XDG_CONFIG_HOME"——三档覆盖完整。
4. ✅ Decision 3 拒绝 `--config-home`，理由"T-041/T-042 都只暴露 `--home`，加第二个 path 旗标只为 OpenCode 是 CLI 表面复杂化"——架构一致性优先。
5. ✅ 末尾点了"T-043 should not extract Registry / AgentConfig; after R-043 approval, T-040 is the correct place"——纪律到位，没顺手抽抽象。

**Q-1（实施时拍板）**: `provider.omnitoken.models` 子对象**写几个 model**？proposal 示意只写 `--model` 传入的那一个（默认 `chat-balanced`），但 OmniToken 已有 T-017a 注册的 5 个虚拟模型（chat-fast / chat-balanced / chat-quality / ...）+ Ark 直连模型名。员工在 OpenCode 模型选单里只看见 `chat-balanced`、看不见 `chat-fast` —— 是不是 UX 缺一截？两个方向都可接受：
   - (a) **保持单一**：只写 `--model` 一项，员工改其他要重跑 adopt 或手编 opencode.json。简单、与 T-041/T-042 行为一致。
   - (b) **全量写**：把 OmniToken 当前已知虚拟模型列表全写进 `models` 子对象，员工 OpenCode 选单直接可见。但需要从某处枚举模型清单（写死常量 vs 查 admin API）。
   
   **默认我倾向 (a)**——与 T-041/T-042 形状一致，多模型是 T-044（路由规则联动）的范围；如 Codex 觉得 (b) 实施代价低（写一份与 T-017a seed 一致的硬编码常量列表）也可。Result 里说明选择 + 理由即可。

**N-8**: Token 明文落 `opencode.json` 是 OpenCode 设计限制（无外置 secret store），proposal 已直面这点。建议在 `internal/agent_adapter/opencode.go` 文件级 comment 里写一行"OpenCode does not expose a separate secret store; apiKey lives in opencode.json by design. CLI output MUST never echo this field's value." —— 给后来读代码的人一个明确锚点，避免未来误以为是 bug。

**Codex 可开 T-043 实施**。Q-1 答在实施 commit Result（建议方向 (a)），N-8 落文件注释即可。

---

## R-043 (T-043 OpenCode 适配 实施, commit `5254c48`)

**结论: `[+] Approved`** — 8/8 接受标准过；Q-1 取 (a) 单 model（与 T-041/T-042 一致）；N-8 文件 comment 落 opencode.go 顶部；XDG 三档全测；plural `providers` 用户数据保留有专测。**T-040 抽象层提取可以启动**。1 NIT 不阻塞。

**正面信号**:
1. ✅ N-8 + Decision 1 双重落地：`opencode.go:1-2` 文件级 comment 写明"OpenCode does not expose a separate secret store; options.apiKey lives in opencode.json by design. CLI output must never echo this field's value."；写入用 singular `provider`（line 76-77 + `providerObject:150`），且 `TestWriteOpenCodeSettingsPreservesSchemaOtherProviderAndPluralProviders` 显式断言 `root["providers"]`（旧复数用户数据）和 `provider.other`（其他 provider）双双保留——proposal 承诺兑现。
2. ✅ XDG 三档完整覆盖、各有专测：① `TestWriteOpenCodeSettingsHomeOverrideBeatsXDGConfigHome` 验 `--home` 强于 XDG（且 line 247 反向断言 XDG 路径未被建出）；② `TestWriteOpenCodeSettingsUsesXDGConfigHome` 验 XDG_CONFIG_HOME 走 XDG 路径；③ `TestWriteOpenCodeSettingsFallsBackToHomeConfig` 验未设 XDG 时走 `<home>/.config/opencode/`。proposal Decision 2 的三档全部有专属断言。
3. ✅ CLI 安全断言移植：`TestRunCLIAdoptOpenCodeUsesHomeOverride:241` 显式 `strings.Contains(stdout/stderr, "omt_secret")` —— 即使 OpenCode 设计上 token **必须**落 opencode.json 明文，CLI 输出层依然有 grep-proof assertion 守护，未来重构改 stdout 也跑得住。T-041/T-042 模式一致。
4. ✅ 双路径 invalid config 全覆盖：`TestWriteOpenCodeSettingsRejectsInvalidRootWithoutBackupOrWrite` + `TestWriteOpenCodeSettingsRejectsInvalidProviderWithoutBackupOrWrite`，sentinel error `ErrInvalidExistingOpenCodeConfig` 经 CLI `errors.Is` → exit 2（`TestRunCLIAdoptOpenCodeInvalidConfigExitsTwo`）。原文件不改、备份不建——失败语义与 T-041/T-042 完全对齐。
5. ✅ **T-040 trigger 主动点出**：commit message 末尾"leaves Registry/AgentConfig extraction to T-040 now that the three adapters are present"——纪律到位，没顺手抽抽象但显式提示下一拍。agent_adapter 包覆盖率 82.2%（达标 ≥80%）；cmd/omnitoken-adopt 70.0%（cmd/* 不要求）。

**N-9 (NIT)**: `nonEmptyStrings` 助手定义在 `internal/agent_adapter/claude_code.go:193`，但实际是跨 adapter 的通用工具（OpenCode 也用了，line 101）。逻辑上应该在 `fileio.go` 或 T-040 抽象层提取时归并。**不阻塞**，T-040 抽 Registry 时顺手挪即可。

**T-040 启动信号**: 三个具象 adapter（Claude Code / Codex / OpenCode）+ 共享 fileio helper 凑齐"三处重复"，AGENTS.md §3.1 触发条件达成。**建议下一步直接 T-040**——`Registry` + `AgentConfig` interface 抽象 + `nonEmptyStrings` 归并 + Result struct 收敛（当前 `SettingsPath`/`ConfigPath`/`AuthPath`/`BackupPath`/`BackupPaths`/`ManagedKeys`/`ManagedEnvKeys`/`ManagedTomlKeys`/`Warnings` 字段冗余可借机重构）。无 v1 阻塞，可与 T-045 协议转换并行。

---

## R-040-prop (T-040 PROPOSAL, commit `fd8310a`)

**结论: `[+] Approved`** — 3 决策全采纳推荐方向，且 Decision 2 主动**抓出我任务体里的内在矛盾**（"Result 弃单值字段" × "CLI 零修改"互斥）并给了分阶段解法。1 N 实施时一句注释解决。**Codex 可开 T-040 实施**。

**正面信号**:
1. ✅ Decision 1 走 base-options + 同构 interface 路线 (route c)：interface 三方法 `Type() / Write(BaseOptions) / Restore(BaseRestoreOptions)` 干净；type-safe + Registry 可同构。backward-compat 通过 type alias 或 thin struct 保 `ClaudeCodeOptions`/`CodexOptions`/`OpenCodeOptions` 不破现有 caller，`WriteXxxSettings` wrapper delegate 到 `(&XxxConfig{}).Write(BaseOptions(opts))`。三个 RestoreXxxOptions 当前都只含 `Home` 字段，collapse 成 `BaseRestoreOptions` 是水到渠成的简化。
2. ✅ Decision 2 **接住了我的 spec 内在矛盾**：任务体说"Result 删 `BackupPath` 单值字段" + "CLI 零改"在 CLI 直读 `result.BackupPath` 的现状下逻辑互斥。proposal 显式点出 + 给方案 (2) **分阶段移除**：T-040 加 canonical 字段（`Paths map[string]string` + `BackupPaths`/`RestoredFromPaths`/`ManagedKeys`/`Warnings`），同时**保留 legacy 单值字段作为 compat shim** populated from canonical；物理删除留 T-046 CLI rewire 时做。这是 Codex 第三次主动纠 Claude spec（前两次：golang:1.25 / provider singular），独立性继续保持。
3. ✅ Decision 3 init() 默认 registry + `NewRegistry()` 测试隔离 + 4 方法（Register / MustRegister / Get / List）+ **List() 确定性排序**（默认 registry 列出 agent 顺序稳定，避免测试 flaky）—— `Register` 重名返回 error / `MustRegister` panic 的两层语义清晰。
4. ✅ Path roles 设计干净：`Paths["settings"]` (Claude Code) / `Paths["config"] + Paths["auth"]` (Codex) / `Paths["config"]` (OpenCode) —— 以 role 为 key 比固定 struct 字段更可扩展（未来加 IDE 插件 etc 不动 Result struct）。
5. ✅ 实施 note 主动列了 5 条边界：CLI 零改 / `nonEmptyStrings` 挪 fileio / registry.go 新文件 / 三 adapter minimally refactor / 新增 `registry_test.go` 含 duplicate + default registry parity 表驱动测试。

**N-10 (NIT)**: proposal Decision 3 示例代码 `MustRegisterDefault(&ClaudeCodeConfig{})` 这个函数名前文没定义。两种合理形态：(a) `DefaultRegistry.MustRegister(&ClaudeCodeConfig{})` 直接走 registry 方法；(b) 包级 helper `func MustRegisterDefault(c AgentConfig) { DefaultRegistry.MustRegister(c) }`。实施时任选其一在 `registry.go` 里**显式定义**即可，commit message 一句说清楚。

**Codex 可开 T-040 实施**。N-10 在 `registry.go` 落具体形态即可，无需 propose 二次。

---

## R-040 (T-040 实施, commit `147502da`)

**结论: `[+] Approved`** — 三 PROPOSAL 决策全落地，9 条接受标准逐条达成。`go vet` clean、`agent_adapter` 83.6%（+1.4pp）、`cmd/omnitoken-adopt` 测试全绿、CLI diff 空。无 CRITICAL / HIGH。**T-040 可关，T-044 / T-046 解锁**。

**正面信号**:
1. ✅ 三决策严格对齐 PROPOSAL：`AgentConfig{Type/Write/Restore}` + `BaseOptions`/`BaseRestoreOptions` 同构接口（registry.go:18-35）；`Paths map[string]string` + slice 字段做 canonical + legacy 字段做 compat shim（fileio.go:18-32，附 T-046 删除注释）；`init()` 注册三 adapter 到 `DefaultRegistry`（registry.go:40-46），`NewRegistry()` 暴露给隔离测试。
2. ✅ N-10 选 (a) 路线 `DefaultRegistry.MustRegister(...)`，未引入冗余包级 helper —— 比 (b) 更简，commit message 也点了。
3. ✅ CLI 零改实锤：`git diff 147502da~1 147502da -- cmd/omnitoken-adopt/` 输出为空，CLI 测试全绿；compat shim 三处都把 legacy 单值字段从 canonical 派生（`BackupPath = firstString(backupPaths)` / `RestoredFrom = firstString(restored)`），避免双向漂移。
4. ✅ 测试覆盖到位：`TestRegistryRegisterGetListAndDuplicate` / `TestRegistryRejectsInvalidConfig`（nil + 空 Type 两路）/ `TestRegistryMustRegisterPanicsOnDuplicate` / `TestDefaultRegistryContainsBuiltInAdapters` / `TestRegistryWriteMatchesExportedWrappers`（三 adapter 表驱动 wrapper vs registry parity）/ `TestCanonicalResultFieldsArePopulated`（canonical + legacy 双向验证）/ `TestNonEmptyStringsSharedHelper`（R-043 N-9 跟进）—— 各能力维度都有 1-2 例。
5. ✅ Codex propose 拍板 ManagedKeys 收敛细节做得稳：Codex 的 canonical `ManagedKeys = ManagedEnv + ManagedToml` 拼接（codex.go:139, 196），同时 legacy `ManagedEnvKeys`/`ManagedTomlKeys` 保留给 CLI；CLI 输出两行 `managed_env` / `managed_toml` 不破。

**N-11 (NIT)**: `fileio.go:194` 新增 `paths()` helper 过滤空值，但当前三 adapter 的 map 字面量值都非空（`configPath`/`authPath` 在 Codex 是必填，settings/config 在 Claude/OpenCode 也是必填）—— 过滤分支永不命中。可以下次裁掉，或把它换成直接 `map[string]string{...}` 字面量。不阻塞合并。

**N-12 (NIT)**: `init()` 用 `&ClaudeCodeConfig{}`（指针）注册，但三 Config 的方法用 value receiver（如 `func (ClaudeCodeConfig) Type() AgentType`）。两种风格混用不会出 bug（Go 自动派发），但风格上要么全 value-receiver + `ClaudeCodeConfig{}` 注册，要么 pointer-receiver + `&ClaudeCodeConfig{}` 注册。后续若 Config 要带状态再统一即可。

**[+] 决策遗产**: PROPOSAL Decision 2 的"两阶段移除"（T-040 加 canonical + 保 legacy，T-046 删 legacy）是这次能"CLI 零改 + Result 收敛"同时成立的关键。**已在 fileio.go:25-32 留下显式注释**指向 T-046，迁移路径清晰。

---

## R-CONC-CHECK (T-CONC-CHECK 实施, 报告 `04fff8a7`, status `c6c4262f`)

**结论: `[+] Approved with follow-ups`** — 任务形式上完成：报告交付 + V2 candidates 主动列 3 条 + 严格"不修代码"。但**baseline 数据不完整**且**抓到底座成本归因隐患**。T-CONC-CHECK 本身可关；需起 3 个 follow-up（M-23 / H-3 / H-4），其中 M-23 阻塞 Phase 3-A 商业化场景，建议先开。

**正面信号**:
1. ✅ Codex 严格守"只测量不修"边界：所有发现写进报告 §V2 Candidate Fixes，未单边改 DSN / 加索引 / 调连接池 —— 任务体接受标准末条达成。
2. ✅ Preflight 设计严谨：跑前先打 1 发 chat-fast 看实际下游模型，**抓出了 `virtual_models` 表 vs `usage.model_actual` 的不一致**（实质是 Ark backend 模型名披露，gateway 转发逻辑经 `cmd/gateway/main.go:235` + `internal/usage/parser.go:38,41` 证实正确）；同时跑前清零 demo admin 的 monthly budget，避免 quota 抑制 mask 上游真实行为，实验设计强。
3. ✅ Gateway 自身韧性: 50 并发 × 50 真 chat 跑下来 **0 panic / 0 5xx / 0 timeout / 0 client error**，428 成功请求 P95 1.798s / P99 2.415s（含 Ark 上游往返）—— `internal/proxy` SSE 反代 + budget/auth/audit 中间件全链路在压力下没出 race / 没崩。
4. ✅ 报告透明披露所有偏差：vegeta 不可用 → 临时 Go driver（已标出）/ DB sampling filter 失效（peak=0）→ 主动说明"not proof of zero connections"；healthz 实际 996.2 RPS vs 配置 1000 RPS 也照实记。无"成功率掩饰"的迹象。
5. ✅ V2 candidate fixes 三条都精准对症：upstream-aware load profile（解 H-4）/ DSN `application_name` 显式设置（解 H-3）/ loadtest summary 加 429 计数（提升可观测性）—— 不是泛泛的"以后再说"，是可直接立任务的清单。

**M-23 (MEDIUM, 升级建议)**: `model_actual = deepseek-v4-pro` ≠ gateway 重写后 `kimi-k2.6` —— 经查 `cmd/gateway/main.go:235` (`payload["model"] = res.RealModel`) + `internal/usage/parser.go:38` (`ModelActual = response.Model`)，**gateway 转发逻辑正确**，这是 Ark 上游响应里自报的 backend 模型名（与 memory `project_omnitoken_ark_coding_plan` 中"5 模型共用一把 key"的设计契合）。**但**：`cmd/admin/main.go:667/791/803` admin overview 全部按 `model_actual` 聚合成本/请求数 → 用户问"我用了多少 kimi-k2.6"会被答"deepseek-v4-pro"。**这是 OmniToken 底座"性价比资源"角的成本归因路径污染**。建议起 **T-CONC-COST-ATTR**：(a) 复现 Ark 行为并补 ADR 记录预期；(b) `usage_records` 加 `model_routed`（gateway 转发出的模型名，从 `httpx.WithVirtualModel` ctx 取）作为归因 ground truth；(c) admin overview 默认按 `model_routed` 聚合，`model_actual` 保留供审计。**建议 Phase 3-A 之前做**。

**H-3 (HIGH)**: 任务体接受标准第 3 项"DB 连接峰值"形式上未达成（filter `application_name LIKE 'omnitoken%'` peak=0），Codex 已透明说明。建议起 **T-CONC-DSN**：在 `cmd/gateway/main.go` / `cmd/admin/main.go` 的 DSN 构造处显式拼 `application_name=omnitoken-gateway` / `application_name=omnitoken-admin`，并在 `cmd/loadtest/README.md` 把采样 SQL 一并写好。**不阻塞 Phase 3-A**，但底座可观测性短板要补，建议穿插做。

**H-4 (HIGH)**: 2500 真跑 83% 是 Ark 429 → **v1 真实并发上限 baseline 实际上没拿到**。任务体目标"测 v1 真实并发上限"被 Ark rate limit 吃了。Codex 结论"primary bottleneck is Ark upstream rate limiting"诚实，但用户层面"v1 上线后能扛多少 RPS"这个问题没答案。建议起 **T-CONC-RERUN**：(a) mock upstream 跑 50 并发 / 100 并发 / 200 并发 各 1 分钟，量 gateway 真实承载；(b) 真 Ark 跑低 RPS 长时间（如 3 RPS × 600s = 1800 请求，成本约 9 RMB），定位 Ark 429 阈值。**Phase 3-A 启动不强依赖此数**（Agent 适配是离线配置写入，不打 Ark）—— 可推到 v1 真实流量进来前做。

**N-13 (NIT)**: 报告第 30-36 行解释 `model_actual = deepseek-v4-pro` 那一段语义偏弱（"routing target evidence is the virtual_models row"），未点透"这是 Ark backend 模型名披露"。可在后续报告版本加一句"Ark coding plan 5 模型 backend 推理可能共用 —— 这不是 gateway bug"。

**[+] 方法学遗产**: Preflight 校验"客户端→gateway→上游→usage"的端到端模型路径，是个**值得沉淀到 AGENTS.md §3.3 的 smoke 方法学**（在 `AGENTS.md §9.5` smoke 守则之后补一条"涉及虚拟模型路由的实测必须 preflight 比对 virtual_models 表 vs usage.model_actual"）。这次 Codex 主动做了，未来其他任务也该照搬。
