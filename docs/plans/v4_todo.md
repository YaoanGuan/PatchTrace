# PatchTrace v4 执行跟踪（v4_todo）

当前轮次：v4 第 0 轮（规划完成，未启动）· 2026-11-20
规格：`v4_re.md`（V4-R1…R16）· 计划：`v4_task.md`（T0…T16）· 战略：`v4_方案.md`

## 0. 下一轮唯一动作

**不启动 v4。v4 的下一步是 T0 Step 1：核对 v3 闸门 G0–G5 是否全部有证据。**

当前实际动作仍在 v3：`v3_todo.md` §0 的 T1 Step 1（`decision/decider_test.go` 的失败测试）。v3 未过，v4 一行代码都不写。

> 执行纪律：本文件状态只认命令输出。没有“本轮跑过的命令 + 原始输出”，一律记 `未开始`。

## 1. 执行协议

- 一轮只做一个 Task；做完即回写状态与证据。
- 每个 Task 走 `红 → 验红 → 实现 → 验绿 → 提交`；提交只在你明确要求时执行。
- 状态词：`未开始` / `进行中` / `阻塞` / `已验收`。
- `已验收` 门槛：Task 内全部测试 PASS，证据块贴在本文件对应小节。
- 闸门 G0–G7 任一未过，对应能力不得写“已落地”。

## 2. 总览

| Task | 阶段 | 交付 | 对应需求 | 闸门 | 状态 |
|---|---|---|---|---|---|
| T0 | V4-0 | 冻结 v3 基线 | R15 前置 | G0 | 阻塞（v3 未完成） |
| T1 | V4-1 | 契约 v1 + FakeServer | R2 | G1 | 未开始 |
| T2 | V4-1 | import 边界断言 | R1 | G1 | 未开始 |
| T3 | V4-2 | 沙箱抽取 | R3 | G2 | 未开始 |
| T4 | V4-2 | Agentic Tool（scan/intel_lookup） | R3 | G2 | 未开始 |
| T5 | V4-2 | 补丁情报抽取 | R4 | G2 | 未开始 |
| T6 | V4-2 | 证据库 | R4 | G2 | 未开始 |
| T7 | V4-2 | `pt-oi` 进程/CLI | R5 | G2 | 未开始 |
| T8 | V4-3 | Harness H1 + 黑板 | R6 | G3 | 未开始 |
| T9 | V4-3 | MTP | R7 | G3 | 未开始 |
| T10 | V4-4 | GraphRAG 邻域 | R8 | G4 | 未开始 |
| T11 | V4-4 | HCI 审批/Hint/暂停 | R9 | G4 | 未开始 |
| T12 | V4-5 | H2 Cybernetics | R10 | G5 | 未开始 |
| T13 | V4-5 | IoA 通讯 | R11 | G6 | 未开始 |
| T14 | V4-6 | 端侧 Agent 原型 | R12 | G7（可选） | 未开始 |
| T15 | V4-6 | 旧路径收口 + flag | R13 | G7 | 未开始 |
| T16 | V4-6 | 指标 + 真实验收 | R14/R15 | G7 | 未开始 |

## 3. 闸门

| 闸门 | 条件 | 过闸证据 |
|---|---|---|
| G0 | v3 G0–G5 全过 + 基线 JSON | T0 runbook + `v3_todo` 证据 |
| G1 | 契约 + import 断言全绿 | `go test ./internal/offensive/contract/ ./internal/archtest/` |
| G2 | OI 独立运行 + parity | `pt-oi` 日志 + 对照测试 |
| G3 | H1 + 黑板 + MTP 单测 | `go test ./internal/agent/...` |
| G4 | GraphRAG + HCI | `go test ./internal/agent/blackboard/ ./internal/hci/` |
| G5 | H1/H2 parity=100% | eval JSON |
| G6 | 双 Agent IoA 实验 | bus 记录 + 计划产物 |
| G7 | 旧路径收口 + 真实验收 | T15/T16 runbook |

## 4. 任务清单与证据

### T0 冻结基线（阻塞）

- [ ] 核对 `v3_todo.md` G0–G5 均有证据
- [ ] v3 相关包 `go test -race` 全绿
- [ ] 导出 eval JSON + graph/rules 计数
- [ ] 写 `docs/runbooks/v4-baseline-2026-11-20.md`

阻塞原因：v3 尚未验收（T0 环境阻塞、T1 未开始）。
证据：_待填_

### T1 契约 v1

- [ ] `TestResponseRequiresEvidenceForExec`、`TestContractVersionRejected`、`TestFakeServerRoundTrip`、`TestUnknownMethodStructuredError`
- [ ] 验红
- [ ] `contract.go`/`client.go`/`fake.go`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T2 依赖边界

- [ ] `TestOffensiveDoesNotImportAgent`、`TestAgentOnlyImportsContract`、`TestNoWebFromPlanes`
- [ ] 验红
- [ ] `internal/archtest/imports_test.go` + 空包占位
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T3 沙箱

- [ ] 四个测试（证据/超时/别名/幂等）
- [ ] 验红
- [ ] `offensive/sandbox`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T4 Agentic Tool

- [ ] 四个测试（聚合/不返回原始/分级/未知拒绝）
- [ ] 验红
- [ ] `scan`、`intel_lookup`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T5 情报

- [ ] 四个测试（真实 diff/无命中/规则回流/不伪造）
- [ ] 验红
- [ ] `offensive/intel`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T6 证据库

- [ ] 四个测试（round-trip/索引/缺字段拒绝/内容寻址）
- [ ] 验红
- [ ] `offensive/evidence` + `data/offensive.db`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T7 pt-oi

- [ ] 四个测试（JSON-RPC/版本/危险命令/纯 JSON 输出）
- [ ] 验红
- [ ] `cmd/pt-oi` + server + stdio client
- [ ] 验绿
- [ ] （待用户要求）提交
- [ ] 独立进程跑一次 scan 并留日志（G2）

证据：_待填_

### T8 Harness H1 + 黑板

- [ ] 六个测试（Step/短路/defer/completion/lease/证据闸门）
- [ ] 验红
- [ ] `agent/harness/react.go` + `blackboard`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T9 MTP

- [ ] 四个测试（排序/摘要/预算/未知 ID）
- [ ] 验红
- [ ] `agent/mtp`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T10 GraphRAG

- [ ] 三个测试（深度/过滤/无向量依赖）
- [ ] 验红
- [ ] `blackboard/neighborhood.go`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T11 HCI

- [ ] 五个测试（拒绝/覆盖审计/Hint 优先/暂停/审批超时）
- [ ] 验红
- [ ] `internal/hci`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T12 H2

- [ ] 四个测试（状态机/plan/overlay/parity）
- [ ] 验红
- [ ] `agent/harness/cybernetics.go` + prompts
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T13 IoA

- [ ] 三个测试（send/read/link、DAG、双 Agent 实验）
- [ ] 验红
- [ ] `agent/ioa`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T14 端侧 Agent

- [ ] 三个测试（四工具/无调度/授权绑定）
- [ ] 验红
- [ ] `cmd/aiagent`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T15 旧路径收口

- [ ] 四个测试（默认关/矩阵/无双写/回退）
- [ ] 验红
- [ ] `plane_mode` + 唯一写者
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T16 指标与真实验收

- [ ] 四个测试（指标/parity/经济学/基线对照）
- [ ] 验红
- [ ] `eval/offensive_metrics.go`
- [ ] 验绿
- [ ] 真实链路 H1/H2 各一次（环境允许）
- [ ] 写 `docs/runbooks/cve-2025-3248-v4-run-2026-11-20.md`
- [ ] （待用户要求）提交

证据：_待填_

## 5. 证据模板

```text
### <Task> 证据 · <日期>
命令：`<完整命令>`
退出码：<0/N>
输出（粘贴原始，含 PASS/FAIL 行）：
<...>
结论：<已验收 | 未达标 + 原因>
```

## 6. 阻塞与风险

| 项 | 状态 | 影响 | 解除条件 |
|---|---|---|---|
| v3 未验收 | 阻塞 T0 | v4 全部 | v3 G0–G5 有证据 |
| Docker/LLM | 阻塞 T16 真实链路 | 真实验收 | 环境可用 |
| 旧路径删除 | 依赖 T15 parity | 收口 | 全量测试绿 |
| 端侧 Agent | 可选闸门 | G7 可跳过 | 用户决定 |

## 7. 决策记录

| 日期 | 决策 | 理由 |
|---|---|---|
| 2026-11-20 | v4 启动条件 = v3 G0–G5 | 不在未验证 Harness 上拆平面 |
| 2026-11-20 | 默认同进程接口，`pt-oi` 为可选独立进程 | 避免 IPC 拖慢端到端 |
| 2026-11-20 | 契约只含能力与证据 | 不把编排层塞进契约 |
| 2026-11-20 | 旧路径默认关、可回退 | 迁移期必须能回滚 |
| 2026-11-20 | H2 作为同接口第二实现 | 换 Harness 不改 OI/黑板 |

## 8. 验收口径

- **OI 独立**：不启动 Agent，`pt-oi` 完成一次工具调用并产出可解析证据。
- **Harness parity**：同项目 H1/H2 的 fact/verdict/evidence hash 完全一致。
- **verified finding**：沿用 v3 口径（observation + verdict + 可复现命令），代理口径不计。
- **安全事件**：任何未审批执行或越权 project 访问，本轮直接不通过。
