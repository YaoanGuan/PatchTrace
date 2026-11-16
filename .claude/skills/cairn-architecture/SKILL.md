---
name: cairn-architecture
description: Use when implementing or reviewing PatchTrace's Cairn-like backend (graph server, dispatcher, leases, bootstrap/reason/explore tasks).
---

# Cairn 架构（PatchTrace 后端模板）

来源：`Cairn/docs/specs/server-protocol.md`、`Cairn/docs/specs/dispatcher-design.md`。只取架构，不搬业务。
细节见 `references/`；本页给判断所需的最小完整模型。

## 何时加载

- 实现或评审 `graphserver/`、`dispatch/`、`dispatch/worker/`
- 判断一份状态该进图（Fact/Intent）还是进项目级协调状态
- 排查 claim 竞争、lease 过期、任务超时、conclude fallback

## 四层组件

| 层 | 职责 | PatchTrace 落点 |
|---|---|---|
| **Cairn Server** | 协议真相源：保存 Project / Fact / Intent / Hint；维护认领、心跳、结论；不推理、不决策 | `graph/` 存储 + `graphserver/` HTTP |
| **Dispatcher** | 读图 → 选任务 → 选 Worker → 管容器/session/超时 → 写回；唯一协议写入者 | `dispatch/` |
| **Project Container** | 每个项目一个隔离执行环境：工具链、网络、Worker 进程 | 复用 `dockerManager/` |
| **Worker / Agent CLI** | 只收 prompt、执行、返回结构化 JSON；不直接调用协议 | `dispatch/worker/`（native 复用 `llm/` + `toolCalling/`） |

依赖方向：Server ← Dispatcher → Container → Worker。Worker 不得绕过 Dispatcher 写 Server。

## 三个原语

| 原语 | 是 | 语义 |
|---|---|---|
| **Fact** | 图节点 | 已确认的客观事实；只增不改、永久保留；状态变化用新 Fact 表达。每个项目固定有 `origin`、`goal` |
| **Intent** | 图边（可为超边） | 从一个或多个 Fact 出发的探索过程；字段 `from[]`、`to`（结论 Fact）、`description`、`creator`、`worker` |
| **Hint** | 图外输入 | 人工策略或态势判断；不参与因果，读图时吸收；项目 `stopped` / `completed` 时仍可写 |

Fact 描述写结论而非原始数据；大输出写文件引用，保持图轻量。

## 三类任务与输出契约

任务类型固定三类；领域差异只进 prompt、Fact 类型与边谓词，不新增任务类型。

| 任务 | 触发（摘要） | 输出契约 |
|---|---|---|
| `bootstrap` | 项目初始态：facts 仅 `origin` + `goal`，无普通 intent；`bootstrap_enabled=true` 且有支持 Worker | 主阶段成功必须同时返回 `fact` + `complete`；`bootstrap_conclude` 只返回 `fact` |
| `reason` | 无未认领 intent、无其他 reason；首次或出现“新态势”（Fact/Hint 增加、open intent 从有到无） | `complete{from,description}` 或 `intent{from,description}` 或空 `data{}`；三者互斥 |
| `explore` | 存在一个当前可认领的未结论 intent | `description`：客观结论；未打出漏洞也必须给客观结论，禁止拒绝文本 |

统一信封：`{"accepted": true, "data": {...}}`。`accepted:false`、非法 JSON、缺字段 = 任务作废，不写图。
`bootstrap` / `explore` 双阶段：超时或解析失败时用同一 session 进入 `*_conclude`，只产出 Fact。

## Intent 生命周期与 lease

```
open(worker=null) --claim/heartbeat--> claimed(worker≠null) --conclude--> concluded(to≠null)
        ^                                    |
        +------- lease 超时 / release --------+
```

- 认领是原子 CAS：同一 Intent 同时只能有一个 worker；第二个认领得到 409。
- `last_heartbeat_at` 超过 `intent_timeout` → 未结论 Intent 的 `worker` 清空为 null，等待重新认领；已结论 Intent 不参与超时，`worker` 永久保留。
- `release` 主动回到 open；`last_heartbeat_at` 保留不清空。
- `conclude` 原子产出 Fact 并落定 `to`；`complete` 原子建立 `from[] → goal` 完成边。
- 项目级 `reason` lease：单项目同一时刻最多一个 `reason`，`reason_timeout` 到期清空；跨项目可并行。
- 项目 `stopped` 是硬停止：立即清空所有 open intent 的 worker 和 `project.reason`，拒绝探索写操作（403）；`completed` 只能经 `reopen` 撤销。
- `bootstrap` 用保留 Intent 表达：`from=["origin"]`、`description="bootstrap"`、`creator="dispatcher.bootstrap"`，不新增接口。

## Dispatcher 是唯一协议写入者

- Worker 不 claim、不 heartbeat、不直接调用 Server；只接收 prompt 并返回 JSON。
- `bootstrap` / `explore`：Dispatcher 先 claim（heartbeat）成功，才启动 Worker；结束后 `conclude` 或 `release`。
- `reason`：先 claim 项目级 lease；成功 → `complete`，新 intent → `POST /intents`（`worker=null`），否则不写图；收尾 `release`。
- 写回失败（尤其 403）→ 本次任务作废，不立即重试；释放本地 claim 并记日志。
- 单 Dispatcher 实例；多实例共享同一 Server 不在支持范围。

## 导出与读取

- `GET /projects/{id}/export?format=yaml`：图快照，仅供渲染 prompt；不含 `last_heartbeat_at` 与 `project.reason`，`to=null` 的 Intent 仍在。
- `GET /projects/{id}/export?format=timeline`：事件时间线（`PROJECT CREATED` / `HINT` / `INTENT DECLARED` / `INTENT CONCLUDED` / `PROJECT COMPLETED`），供审计与回放；与 YAML 的拓扑视图互补。
- 结构化接口用于调度与状态判断；YAML 只用于喂给 Worker，不能替代协议写入。

## PatchTrace 映射与红线

- 四层映射：`graph/` + `graphserver/`、`dispatch/`、`dockerManager/`、`dispatch/worker/`。
- Fact 类型扩展：`patch_profile`、`exploit_idea`、`constraint`、`observation`、`verdict`、`rule_hit`、`finding`；Intent 增加 `block` 依赖边。
- 不变量：claim 原子；`block` 成环拒绝；`refuted` 永久保留；`finding` 必须有 `origin → … → finding` 的推导路径才可 `complete`。
- 判定必须可证伪；prompt 不写流程分支；不改验收标准。

## References

- `references/server-protocol.md` — 数据结构、接口列表、状态码语义、导出格式
- `references/dispatcher-design.md` — 触发条件、Worker 选择顺序、超时与 conclude fallback
- `references/data-model.md` — Cairn 字段表、PatchTrace 节点/边模型与 Fact 类型扩展
