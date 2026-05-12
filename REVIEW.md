# REVIEW.md — OmniToken Review Log

> **归档说明**: R-001 ~ R-007 已归档至 `docs/reviews/archive.md`（46KB）。本文件只保留最近 3 次 review + 未解决项摘要。

## 未解决项摘要（从所有 review 累积）

| ID | 来源 | 严重度 | 描述 | 状态 |
|----|------|--------|------|------|
| M-13 | R-006a | MEDIUM | gateway 硬依赖 DB，未来考虑 `--auth=stub` | Informational, 留 T-005c |
| M-14 | R-006a | MEDIUM | CreateVirtualKey 不开事务 | Informational, 留 T-005b |
| M-16 | R-008 | MEDIUM | body double-read (1MiB) | Informational, Phase 2 优化 |
| M-17 | R-008 | MEDIUM | SQL fallback 双重防御 | Informational, 设计正确 |
| M-18 | R-006b | MEDIUM | overview 3 条 query 非事务 | Informational, Phase 2 |

---

## R-008 (T-008, commits `4761671` + `e7f8036`)

**结论: `[+] Approved`** — 0C/0H/2M(info)/4N。覆盖率 93.7%。

**核心**: usage middleware 完全解耦 proxy，`context.Background()` deferred goroutine，三表事务原子写入，DB 端 numeric 精确计算，pricing CTE 三级 fallback。PROPOSAL 6 节全部精确落地。

**M-16**: body double-read (Phase 2 优化) | **M-17**: SQL fallback 双重防御 (设计正确)
**N-25~N-28**: README 补充 / SQL 注释 / capture WriteHeader / e2e cleanup — 均不阻塞

> 完整 review 见 R-008 归档前原文，关键表格保留：

| PROPOSAL 节 | 落地 | 接受标准 | 状态 |
|------------|------|---------|------|
| §1 middleware | ✅ | 解析点 non-stream/stream | ✅ |
| §2 包结构 | ✅ | 入账字段 events+breakdown | ✅ |
| §3 deferred goroutine | ✅ | cost_ledger pricing | ✅ |
| §4 numeric SQL | ✅ | 缺 pricing → cost=0+failed | ✅ |
| §5 failed zero-cost | ✅ | 同步入账不阻塞 | ✅ |
| §6 seed 价格 | ✅ | 失败不影响客户端 | ✅ |

---

## R-006b-prop (T-006b PROPOSAL, commit `c9cd5ff`)

**结论: `[+] Approved`** — 8 条正面信号，0 问题，2 open questions (Q-7 trend 30天 / Q-8 share 除零)。

**核心决策**: 拆小 query / `overviewStore` 接口 / 复用 `*sql.DB` / 降级分两层（启动全零200 / 请求500） / period UTC。

---

## R-006b (T-006b, commits `290a5bb` + `6379ccc`)

**结论: `[+] Approved`** — 0C/0H/1M(info)/2N。覆盖率 51.9%。9 个测试，530 行测试代码。

**正面信号 (10条)**: 旧 mock 完全删除 / `overviewStore` 接口精确落地 / DB 连接统一 / 降级双层 / summary 一条 SQL 三值 / trend 30d 只返回有数据天 / share 除零防御 / SQL 参数断言 / CORS 测试 / model COALESCE 三级

**Q-7 resolved**: `now.AddDate(0,0,-30)` 到 `now`，空天不出现。
**Q-8 resolved**: `if totalTokens > 0 { item.Share = ... }`，测试覆盖 totalTokens=0。

**M-18**: 3 条 query 非事务 (Phase 2) | **N-29**: fake driver 重复模式 | **N-30**: buffer String() 歧义

### 接受标准（全部 ✅）

| 标准 | 状态 |
|------|------|
| JSON 兼容 `overviewResponse` | ✅ |
| 当月聚合 tokens/cost/users | ✅ |
| trend 30天按日 / model share | ✅ |
| DB 复用 + 降级 | ✅ |
| 安全聚合 / 测试 ≥2 case | ✅ |
| 不改 usage / 不改前端 | ✅ |

**Demo-Ready 75% (6/8)**。下一拍: T-006c → T-006d → push。

---

## R-006c (T-006c, commits `51ba90c` + `3f594b1`)

**结论: `[+] Approved`** — 0C/0H/0M/2N。

**正面信号 (8条)**:
1. ✅ mock 数据完全替换为 `fetch('/api/admin/overview')`，KPI/trend/pie 三处全部接真 API
2. ✅ admin URL 解析链：`?admin=` → `localStorage` → 同 hostname `:8081` → `localhost:8081`，覆盖 file:// 和远程场景
3. ✅ `normalizeOverview()` 防御性极强：null/undefined/非 object/字段缺失全部 fallback 到安全零值
4. ✅ `AbortController` 8s 超时，不会因 admin 不可达而永远挂起
5. ✅ 三态 UI：loading（indigo）→ 成功渲染 / empty（slate）/ error（rose），不是空白也不是 JS 报错
6. ✅ 错误提示精确提到 CORS 配置，降低 Demo 调试门槛
7. ✅ `formatTokens()` 自动选单位（K/M/B），`formatUSD()` 小金额自动增精度位
8. ✅ Chart.js tooltip 定制：pie 显示百分比 + token 数，trend 显示格式化数字

**N-31**: `visibleModelUsage` 是模块级变量绑 tooltip callback 用，可以但略显隐式——如果 Chart.js 3.x 支持 `dataset.metadata` 会更清晰。不阻塞。
**N-32**: 用户/模型页面仍保留硬编码 mock 数据。接受标准明确"不在范围"，符合预期。

### 接受标准（全部 ✅）

| 标准 | 状态 |
|------|------|
| KPI/trend/pie 从 `GET /api/admin/overview` 拿实时数据 | ✅ |
| fetch 失败友好 error state | ✅ |
| CORS 配置提示 | ✅ |
| 不改后端 API / 不新增依赖 | ✅ |
| 手动打开可看到真实数据 | ✅ (fallback QA) |

**Demo-Ready 进度 87.5% (7/8)**。下一拍：T-006d 端到端验收 → push。

