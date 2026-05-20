# ADR 0003: Multi-Key Pool 拉回 v1 (Phase 2-C)

Status: Accepted, 2026-05-20

## Context

`规划.md` §零A 第 1 条把"性价比高的资源"列为 OmniToken 底座三角的第一角，明确手段包括"**多 upstream key 池**、provider/model fallback、按预算降级、cost-aware routing"。`规划.md` §四十六（行 468）也早就预见："Phase 2-C 多 key 池 + fallback —— 适配后流量会显著放大，单 key 直连撑不住"。

2026-05-19 用户基于 Ark coding plan 单 key 5 模型的成本算账，决定"v1 上线优先"，T-016（`upstream_credentials` 表 + AES-256-GCM envelope encryption + admin CRUD）推到 vNext。判断依据见 memory `project_omnitoken_ark_coding_plan`。

2026-05-20 T-CONC-CHECK 实测数据迫使这个决策被重新审视：

- 50 并发 × 50 = 2500 次真 chat：428 个 2xx（17.1%），2072 个上游 429。
- Gateway 自身 0 panic / 0 5xx / 0 timeout / 0 client error；瓶颈完全在 Ark 单 key rate limit。
- 估算 Ark 单 coding plan key 的实际 rate limit ≈ 7 RPS（基于 428 个成功请求和测试时长）。

用户进一步指出：行业中转站的标准做法本就是上游多 key 池负载均衡，单 key 直连不符合中转站的实际生产形态。

memory `project_omnitoken_ark_coding_plan` 的判断有盲点 —— 它把两件事混在一起了：

1. **"5 模型共用 1 key"是计费侧的简化**：不用 admin 维护多个 provider credential 关系。
2. **"多 key 池负载均衡"是运行时侧的能力**：即便只有 Ark 一家、同一个 coding plan，买 N 把 key 做轮询也是突破单 key rate limit 的标准做法。

当时只覆盖了第 1 件，第 2 件被忽略。

## Decision

把 **T-016 (upstream_credentials + 多 key 池 + envelope encryption)** 从 vNext 拉回 Phase 2-C，作为 v1 上线前的必做项。v1 ETA 因此从原"~1 周"调整为"~2 周"。

**v1 阶段 T-016 的可裁剪范围**：

- ✅ **必须包含**: `upstream_credentials` 表 schema + 字段定义；2-3 把 Ark coding plan key 通过 seed SQL 加密入库；gateway 按 priority/weight 轮询；429/5xx 切到 pool 下一个 key 重试；envelope encryption (AES-256-GCM + 主密钥从 env 注入)；usage 流水记录 `upstream_credential_id`（哪把 key 实际处理了请求）。
- ⏭ **可推 v1.1**: admin CRUD UI（用户登录管理 key 的界面）；key 健康检查后台 worker；自动 rotation；多 provider（OpenAI/Anthropic）支持 —— 多 provider 该独立任务 T-016b 起。
- ⏭ **可推 vNext**: KMS 集成（v1 用 env 注入主密钥即可，KMS 替换是后续事）。

**为什么这个裁剪能成立**：

- Seed SQL 加密入库已经够让"运行时多 key 轮询"跑起来 —— v1 价值落地，不强依赖 UI。
- admin 端先用 audit_logs 看哪把 key 被路由命中，UI 是体验层不是能力层。
- envelope encryption 本身是 audit/compliance 角，不能省（用户拿 v1 给 IT 部门看时密钥不能明文）。

## Consequences

### 正面

1. **底座三角第一角真正落地**：v1 上线时可以演示"4 把 Ark key 轮询，单 key 限速时自动 fallback"，符合 §零A 第 1 条的产品定位。
2. **T-CONC-RERUN 拉上 v1**：多 key 池上线后，T-CONC-CHECK 才能跑出真实 v1 并发上限的 baseline。原计划挂 vNext 的 T-CONC-RERUN 可以拉回作为 T-016 验收的一部分。
3. **路线一致性恢复**：规划.md §零A 第 1 条 ↔ Phase 2-C 安排 ↔ §四十六风险表 ↔ v1 上线物 现在闭环了。
4. **OpenAI/Anthropic 接入更平滑**：v1 把 upstream_credentials 抽象做好，未来接第二家 provider 直接加 provider 维度，不需要重做底层。

### 负面

1. **v1 ETA 延后 5-7 天**：原"~1 周"变 "~2 周"。
2. **复杂度提升**：gateway 多了"按 credential 选择 → 轮询 → 重试切 key"这条新路径，proxy 层和 usage 层都要改。需要严格 e2e 测试覆盖。
3. **2-3 把 Ark key 的采购**：用户需要再买 2-3 把 Ark coding plan key 入库（成本可控，单价信息见 Ark 公开渠道）。
4. **memory 修正**：`project_omnitoken_ark_coding_plan` 那条要更新，明确"多 key 池 vs 计费简化"是两件不同事。

### 影响范围

| 层 | 改动 |
|---|---|
| Schema | 新增 `upstream_credentials` 表 + `usage_records.upstream_credential_id` 列 |
| Gateway | proxy 层加 credential resolver + retry 切 key 状态机；现有 SSE 反代要兼容 |
| Admin | overview 加"按 key 维度"视图（v1 看 audit_logs 也行，UI 留 v1.1） |
| Usage | 流水记录 credential id；M-23 model_routed 修复（T-CONC-COST-ATTR）同期并行 |
| Crypto | 新增 `internal/crypto/envelope.go` AES-256-GCM 封装 + 主密钥从 env 注入 |
| Tests | integration test 覆盖"key 池轮询 + 429 切换"，docker-compose 测试栈复用 |

### 与既有任务的关系

- **T-CONC-COST-ATTR (M-23)**: 仍然要做，与 T-016 同 phase 并行 —— `model_routed` 是路由意图归因，`upstream_credential_id` 是 key 池归因，两件事正交。
- **T-CONC-DSN / T-CONC-RERUN**: H-3 留 vNext；H-4 改在 T-016 验收时一起跑（多 key 池上线才有意义）。
- **Phase 3-A (Agent 适配)**: 不阻塞 —— Agent 适配是离线配置写入，不打 Ark 流量。但 v1 路线延后后，Phase 3-A 启动相应延后约 1 周。

## Open Questions

- v1 是否需要"按虚拟模型分配上游 credential"（chat-fast 走 key-pool-A，chat-quality 走 key-pool-B）？v1 简化为"所有虚拟模型共享一个全局 pool"，更细粒度的绑定推 v1.1。T-016 propose 时确认这条。
- envelope encryption 主密钥从 env 注入是否够安全？v1 用 env，部署文档说明"主密钥不入 git，运维手动注入"。v1.1 接 KMS 时再迁移。
