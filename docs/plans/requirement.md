# PatchTrace 需求分析（requirement.md）

> 迭代名：AIxVuln → PatchTrace
> 定位：基于**补丁反演 + 溯源图推理**的自主渗透系统
> 版本：v1.0 · 2026-11-16
> 状态：待评审（评审通过后进入 task.md 执行）
> 上游输入：`docs/claude/` 四份材料 + 工作区根目录 `Cairn/` 源码
> 下游文档：`docs/plans/task.md`（迭代计划）、`docs/plans/todo.md`（待办与进度）

---

## 0. 给 Agent 的迭代管理提示词（每轮先读）

```text
你是 PatchTrace 迭代的 owner（P7 执行位）。
开始任何动作前：
1. 读 requirement.md，确认本次工作对应的需求编号（R*）。
2. 读 task.md，确认所属 Task 与 Interfaces 块（谁产出了什么、名字与签名）。
3. 读 todo.md，确认该 Task 的当前状态与下一步 checkbox。
4. 只做当前这一步；做完回写 todo.md，附验证命令与真实输出。
5. 没有验收证据，不得把任何项标记为完成；不得改验收标准来“通过”。
6. 发现需求缺口，先记到本文件 §12「需求变更记录」，再动代码。

输出格式：先给“下一步动作”，再给证据，最后给一个两分钟内可执行的下一步。
```

---

## 1. 命题与一句话定义

**PatchTrace** 是把 AIxVuln（多智能体漏洞挖掘实验室）迭代为**以补丁为起点、以溯源图为唯一知识与推理载体**的自主渗透系统。

一句话定义：

> 输入真实厂商补丁 diff，反演出可执行的漏洞约束；把约束、观测、判决全部落成一张可追溯的溯源图；由平等的 Worker 读图、认领信息缺口、在沙箱中实弹验证；每条 Finding 都必须有一条从补丁到证据的完整推导路径。

三个关键词对应三件事：

| 关键词 | 含义 | 载体 |
|---|---|---|
| 补丁反演 | diff → 修复语义 → 漏洞约束（condition / harm / evidence 三元组） | `patchintel/`（已存在） |
| 溯源图推理 | 事实、意图、证据以 DAG 生长；推理 = 在图上找缺口、找路径、做判决 | 新增 `graph/` + `graphserver/` |
| 自主渗透 | 无中心 planner；Worker 读图自调度，沙箱执行，证据回写 | 新增 `dispatch/` + 既有 `dockerManager/` |

---

## 2. 输入材料与结论摘要

| 文件 | 核心结论 | 对本迭代的作用 |
|---|---|---|
| `docs/claude/4.AIxBreak-改动方向解析+重构图解析.pdf` | AIxBreak 的 42,227 行基线诊断：状态字符串机、9 份主循环拷贝、双记忆、工具池膨胀；多 Agent 角色分工在评测中是负收益 | 提供“必须改”的结构性问题清单与 W1/W2/W3 拆解 |
| `docs/claude/AIxBreak-Native+图工程+反证约束-方案.pdf` | 三需求合一：能力外置、正交投资 D/E/I、条件边闭环（CE1–CE6） | 提供条件边、补丁反演、规则回流的机制定义 |
| `docs/claude/方案-图工程原生.md` | 三图一池一通道：证据黑板 / 条件边工作图 / 规则信息素图 + Worker 平等池 | 本迭代“图驱动协调”的直接蓝本 |
| `docs/claude/方案-补丁反推.md` | 前文 + PatchOracle 完整设计：PatchIngest / SemanticInverter / RuleBase、双回路、评测口径 | 提供数据模型、验收标准、CVE-2025-3248 演示剧本 |
| `Cairn/`（源码 + `docs/specs/*.md`） | Server（Fact/Intent/Hint 真相源）+ Dispatcher（唯一协议写入者）+ 项目容器 + Worker CLI；bootstrap/reason/explore 三类任务；lease/heartbeat；stigmergy | 本迭代**后端架构模板** |

**合流结论（本迭代的判据）**：

1. 方向做成数据（约束、规则、场景模板），不做进提示词；
2. 反馈做成回路（证据级内环 + 规则级外环），不做成一次性报告；
3. 协作做成可演化的图（溯源图 + 认领租约），不做成中心 planner；
4. 判断做成可证伪的闸门（严格 true/false + 沙箱证据），不做成文本判定。

---

## 3. 现状基线（2026-11-16 实测）

### 3.1 已验证事实

| 项 | 状态 | 证据 |
|---|---|---|
| 模块名 | `AIxVuln`（`go.mod`） | 根目录即 Go module 根 |
| 编译 | **通过** | `CGO_ENABLED=0 go build ./...` exit 0 |
| 单测 | **通过** | `go test ./patchintel/ ./toolCalling/` 均 ok |
| `skill/` 包 | 已存在 | `skill/skill.go`、`skill/files.go` |
| `resource/` 包 | 已存在 | `resource/resource.go` |
| `patchintel/` 三件套 | 已存在 | `diff.go`（PatchIngest）、`invert.go`（SemanticInverter）、`rules.go`（RuleBase） |
| 四个补丁工具 | 已实现并挂载 | `IngestPatch/InvertPatch/QueryRuleBase/PromoteRule`，已挂到 Analyze 与 Verifier（`agents/agent_builder.go:176-179,307-310`） |
| 场景模板 | 部分 | 仅 `data/templates/langflow/template.md` |
| `patchflow/`（CE 边表） | **缺失** | 目录不存在 |
| `blackboard/`（证据黑板） | **缺失** | 目录不存在 |
| `patcheval`（对照评测） | **缺失** | 无相关包或脚本 |
| 人类意图注入点 | **缺失** | 全库无 `HumanIntent` |
| Git 版本基线 | **缺失** | `git status` → not a git repository |

### 3.2 架构现状

```
Web (gin/ws) → LabManager 总脑 → ProjectManager / DecisionBrain 组脑
                                   → agents 数字人池 → toolCalling → llm / docker
```

问题（引自三源报告，代码勘察 2026-10-11，本次抽查仍成立）：

- 分派靠一次 LLM tool-call 枚举 group_ids（`LabManager/LabManager.go:1364` 附近）；
- exploit 状态是中文字符串（`DecisionBrain/Utils.go:272-300`）；
- 9 份 agent 主循环拷贝、双记忆实现；
- 工具注册依赖 `factories[:7]` / `factories[:12]` 下标拼接；
- 关键信息没有结构化载体——上下文里没有的东西，agent 就不知道。

**本迭代的判断**：不推倒 42k 行，而是新增一条**类 Cairn 的图驱动车道**，旧车道保留为兼容路径（默认行为不变），新车道用 feature flag 打开，逐项对照验收。

---

## 4. Cairn 模板拆解（只取架构，不搬业务）

### 4.1 四层组件

| 组件 | 职责 | 对本迭代的映射 |
|---|---|---|
| **Cairn Server** | 协议真相源：Project / Fact / Intent / Hint；认领、心跳、结论；不推理 | `graphserver/`：溯源图协议服务 |
| **Dispatcher** | 读图、选任务、选 Worker、管容器、唯一协议写入者 | `dispatch/`：调度核心 |
| **Project Container** | 每个项目一个执行环境 | 复用 `dockerManager/`：目标容器 + 攻击容器 |
| **Worker / Agent CLI** | 只收 prompt、返回结构化 JSON，不 direct claim | `dispatch/worker/`：driver 接口 + native driver（复用 `llm/` + `toolCalling/`） |

### 4.2 三个原语

| 原语 | 语义 | PatchTrace 扩展 |
|---|---|---|
| **Fact** | 已确认的客观事实，只增不改 | 细分类型：`patch_profile` / `exploit_idea` / `constraint` / `observation` / `verdict` / `rule_hit` / `finding` |
| **Intent** | 声明式探索方向（信息缺口），可认领 | 增加 `block` 依赖边与 lease 到期回收 |
| **Hint** | 图外人工判断，下一次读图被吸收 | 场景模板与 top-k 规则也以 hint 形态注入（~100 tokens） |

### 4.3 三类任务（本迭代保留三类型，不扩类型）

| 任务 | Cairn 语义 | PatchTrace 语义 |
|---|---|---|
| `bootstrap` | 项目初始态直接尝试解决问题 | 补丁摄入 + 语义反演 → 首批 Fact + Intent |
| `reason` | 读全图：是否达成 goal？下一步探索什么？ | **溯源图推理**：目标覆盖检查、证据缺口检测、生成 Intent、判定 complete |
| `explore` | 认领一个 Intent，执行，产出一个 Fact | 在沙箱中验证一条约束 / 构造一条证据，产出 observation 或 verdict Fact |

> 设计纪律：任务类型保持三类，领域差异全部放进 prompt、Fact 类型和边谓词里。新增任务类型 = 换引擎，违反“流程外置为数据”。

### 4.4 协议与租约（必须照抄的机制）

- 项目状态：`active` / `stopped` / `completed`；`completed` 只能通过 `reopen` 撤销；
- Intent 生命周期：`open（worker=null）→ claimed（worker≠null）→ concluded（to≠null）`；lease 超时自动清空 worker；
- 项目级 `reason` lease：单项目同一时刻最多一个 reason；
- `conclude` 原子产出 Fact 并落定 Intent；`complete` 原子建立 `from → goal` 的完成边；
- `export?format=yaml` 供 prompt 渲染；`export?format=timeline` 供审计与回放；
- Dispatcher 是唯一协议写入者，Worker 不直接 claim / heartbeat。

---

## 5. PatchTrace 目标架构（类 Cairn）

```
┌──────────────────────────────────────────────────────────────┐
│ Web / WS（既有 gin 服务，面板与报告）                          │
└──────────────────────────┬───────────────────────────────────┘
                           │ 挂载 /api/graph/*
┌──────────────────────────▼───────────────────────────────────┐
│ graphserver/  溯源图协议服务（唯一真相源）                      │
│  Project · Fact · Intent · Hint · lease · export · provenance │
│  SQLite: data/patchtrace_graph.db（WAL, MaxOpenConns(1)）      │
└──────────────────────────▲───────────────────────────────────┘
                  读图 / 写回（HTTP 回环，可注入 fake）
┌──────────────────────────┴───────────────────────────────────┐
│ dispatch/  调度核心（唯一协议写入者）                           │
│  scheduler · worker select · timeout · conclude fallback      │
│  tasks: bootstrap / reason / explore                          │
│  runtime: sandbox adapter（dockerManager）                     │
└──────────────────────────┬───────────────────────────────────┘
                           │ Driver 接口
┌──────────────────────────▼───────────────────────────────────┐
│ dispatch/worker/  native driver（llm + toolCalling）           │
│                    cli driver（claude / codex，后置）           │
└──────────────────────────────────────────────────────────────┘
```

### 5.1 溯源图数据模型

**节点**（`kind` 三选一）：

```
Node{
  id, project_id,
  kind:   fact | intent | hint,
  type:   patch_profile | exploit_idea | constraint | observation | verdict
        | rule_hit | finding | gap | human_intent | scene_template,
  content,                           # 客观描述，不含状态机文本
  state:  open | claimed | resolved | refuted,   # 仅 intent 有意义
  from []string, to *string,         # intent 的因果关系
  worker, lease_until, created_at, concluded_at
}
```

**边**（`kind` 五选一）：

| 边 | 语义 | 用途 |
|---|---|---|
| `derive` | 由该 Fact 推导出 | 补丁 → 约束 |
| `support` | 支持该 Intent / 结论 | 证据 → 判决 |
| `refute` | 反驳该 Intent / 结论 | 反证 |
| `block` | 阻塞（强制无环） | 依赖排序 |
| `produce` | Intent 结论产出的 Fact | 溯源主链 |

**不变量**（必须有测试）：

1. 一个 Intent 只能被一个 worker 认领（原子 CAS）；
2. `block` 边成环即拒绝写入；
3. 已 `refuted` 的节点永久保留，不删除、不改写；
4. 任何 `finding` 节点必须存在一条 `origin → … → finding` 的 `produce/derive/support` 路径，否则不得 `complete`。

### 5.2 溯源图推理（R8 的定义）

“推理”不是又一个 LLM 调用，而是对图做四类可证伪操作：

1. **覆盖检查**：goal 是否被某组 Fact 共同满足（对应 Cairn `reason` 的 complete 分支）；
2. **缺口检测**：找出 `state=open` 且依赖满足的 Intent，按“离 goal 的路径长度 / 信息增益”排序；
3. **路径检索**：给定节点，返回 `Provenance(id)` 的推导链，供报告、审计、回放；
4. **判决校验**：`verdict` Fact 必须带 `support` 或 `refute` 边，且证据来自沙箱观测，否则拒绝。

### 5.3 与既有代码的边界

| 新增 | 复用 | 不动 |
|---|---|---|
| `graph/`、`graphserver/`、`dispatch/`、`eval/` | `patchintel/`、`toolCalling/`、`llm/`、`dockerManager/`、`misc/`、`taskManager/` | `Web/Route.go` 主体、`factories[:7]`、`factories[:12]`、`llm/` 客户端层 |

---

## 6. 需求条目

优先级：P0 = 本迭代必须；P1 = 本迭代应做；P2 = 可后置。
验收标准一律写成**可执行命令或可对照产物**。

### 6.1 P0（地基与图）

| ID | 需求 | 验收标准 | 来源 |
|---|---|---|---|
| **R1** | 建立可回退基线 | `git init`；`git status` 干净；基线提交包含当前可编译状态；`go build ./...` 与 `go test ./patchintel/ ./toolCalling/` 通过 | 四份材料共同前置 |
| **R2** | Cairn 架构与开发计划沉淀为 Skills | `.claude/skills/cairn-architecture/SKILL.md` 与 `.claude/skills/patchtrace-iteration/SKILL.md` 存在，含 frontmatter 与 references；新会话可仅凭 skill 说清 Cairn 四层组件与三类任务 | 用户指令 |
| **R3** | 溯源图模型与存储 | `graph/` 包：Node/Edge CRUD、SQLite 落库、原子 claim、lease 超时回收、block 环检测；`go test ./graph/ -race` 通过 | Cairn protocol + 图工程 §2.1 |
| **R4** | 图协议 API | `graphserver/` 实现 Project/Fact/Intent/Hint/claim/heartbeat/release/conclude/complete/reopen/export；`httptest` 覆盖每个状态码（200/403/409/422） | Cairn server-protocol |
| **R5** | 调度核心 | `dispatch/`：三类任务、Worker 选择、租约续约、超时、conclude fallback、并发上限；fake driver + fake client 跑通一个完整项目 | Cairn dispatcher-design |
| **R6** | Worker driver | `dispatch/worker.Driver` 接口 + native driver（复用 `llm/` 与 `toolCalling/`）；JSON 输出契约校验；不健康 Worker 进入 retry 窗口 | Cairn driver 约定 |

### 6.2 P0（补丁反演闭环）

| ID | 需求 | 验收标准 | 来源 |
|---|---|---|---|
| **R7** | 补丁反演 bootstrap | `bootstrap` 任务调用 `patchintel.Ingest*` → `Invert` → 写入 `patch_profile` / `exploit_idea` Fact，并声明首批 Intent；单测覆盖「diff → 两类 Fact」 | 方案-补丁反推 §3.2 |
| **R8** | 溯源图推理 reason | `reason` 任务基于图状态输出 `complete` / `intent` / 空；缺口排序可复现；`intent.from` 必须是合法 Fact id；prompt 中不含可执行状态机逻辑 | 用户命题 + Cairn reason |
| **R9** | 沙箱实证 explore | `explore` 认领 Intent 后经 `dockerManager` 执行验证，产出 `observation` Fact；失败也必须落客观结论；超时/conclude fallback 可验收 | 方案-补丁反推 §3.4 |
| **R10** | 证据判定与反证 | `verdict` Fact 必须带 `support` 或 `refute` 边；无证据路径的 Finding 不得 complete；`reopen` 可撤销错误完成并写入纠错 Fact | 方案-补丁反推 §7 + 图工程 §3.1 |
| **R11** | 规则演化回流 | `explore` 终态经 CE4 回流 `RuleBase`：版本 +1、置信度变化、hit/confirm 计数；第二次反演 prompt 可见 top-k 注入 | 方案-补丁反推 §5 |
| **R12** | 人类意图注入 | Hint 接口可写入 ~100 tokens 人工判断；下一次 `reason`/`explore` 读图可见；不注入方法，只注入判断 | 方案-图工程原生 §2.4 |

### 6.3 P1（产品面与验收）

| ID | 需求 | 验收标准 | 来源 |
|---|---|---|---|
| **R13** | Web 面板与报告 | 图视图（Fact/Intent/Hint/边）、补丁面板、Finding 报告；均从图 API 取数 | AIxVuln Web 资产 |
| **R14** | 对照评测 | `eval/` 输出 JSON：反演成功率、到达 PoC 的试错轮次、单位 finding token 成本；双条件（规则库空 / 预置）N≥3 | 方案-补丁反推 §10 |
| **R15** | 端到端验收 | CVE-2025-3248：diff 导入 → 约束 → 沙箱验证 → Finding → 规则库新增 ≥1；timeline 可回放完整过程 | 方案-补丁反推 §1 |
| **R16** | 兼容与灰度 | 新车道 feature flag 默认关；关闭时 `go build`、既有闭环、`exploits.db` 行为与改造前可比对 | 图工程 §5 |

### 6.4 P2（后置，不在本迭代）

- 多 Dispatcher 实例协同（Cairn 明确不支持，本迭代不挑战）；
- CLI driver（claude/codex）接入；
- 向量检索规则库；
- 多租户与权限模型重构。

---

## 7. 全局约束（红线）

1. **不推倒重来**：42k 行存量，每次改动必须可单独验收、单独回滚。
2. **单二进制 + SQLite**：不引入新存储引擎；图库与规则库都用 `modernc.org/sqlite`。
3. **工厂尾部 append**：不动 `factories[:7]` / `factories[:12]` 的下标切片。
4. **分层不倒置**：新增包只能向下依赖 `patchintel/`、`llm/`、`misc/`、`dockerManager/`；不得 import `Web/`。
5. **判定纪律**：拒绝文本不算成功；网络错误、证据不足一律 `error` / `inconclusive`，不得报 clean。
6. **模型不可见**：状态、进度、预算由基础设施承担；prompt 里不写流程分支。
7. **中文界面与提示词，英文代码标识**。
8. **验收证据**：任何“完成”必须附命令与输出；不得改标准通过。

---

## 8. 非目标

- 不做 L4 自组网 / IoA；
- 不替换 `llm/` 客户端层；
- 不重写 `Web/Route.go` 与前端框架；
- 不在本迭代做多 Dispatcher 集群；
- 不把 `prompts/` 里的领域流程搬进新的硬编码引擎。

---

## 9. 验收闸门

| 闸门 | 条件 | 对应需求 |
|---|---|---|
| **G0 地基** | git 基线 + 两个 skill 可被新会话读取 + build/test 绿 | R1, R2 |
| **G1 图** | `go test ./graph/ ./graphserver/ -race` 全绿；claim 竞争与环检测有测试 | R3, R4 |
| **G2 调度** | fake 环境中一个项目从 bootstrap → reason → explore → complete 跑通 | R5, R6, R7 |
| **G3 闭环** | CVE-2025-3248 端到端；Finding 有完整溯源路径；规则库新增 | R8–R11, R15 |
| **G4 评测** | 双条件 N≥3 产出三指标 JSON；新旧车道对照数据可复现 | R12–R14, R16 |

---

## 10. 风险登记

| 风险 | 影响 | 应对 |
|---|---|---|
| 新旧两条调度车道双写 | 重复任务、状态冲突 | 车道互斥 flag，默认只开一条；图 API 单写者 |
| 图快照撑大上下文 | token 成本上升 | 摘要级注入 + md5 delta；只注入本人相关子图 |
| 引擎内 HTTP 回环引入时序问题 | 测试不稳定 | GraphClient 接口化，测试注入内存 fake |
| LLM 输出不稳定 | 任务失败 | JSON 契约校验 + conclude fallback + 不健康 Worker 冷却 |
| 证据质量不足 | 误报累积 | verdict 必须带 support/refute 边；无路径不得 complete |
| 范围膨胀 | 迭代失控 | P2 一律后置；每个 Task 独立可回滚 |

---

## 11. 术语表

| 术语 | 含义 |
|---|---|
| 溯源图 | Fact/Intent/Hint 构成的有向无环图；边记录因果与证据关系 |
| Intent | 信息缺口 / 工作单元，可被 Worker 原子认领 |
| lease | 认领租约；超时未心跳则自动回收 |
| bootstrap | 项目初始态任务：补丁摄入 + 反演 |
| reason | 读图推理任务：覆盖检查 + 缺口检测 + 生成 Intent |
| explore | 认领一个 Intent 并在沙箱执行的任务 |
| CE1–CE6 | 条件边：放行 / 整改 / 重定向 / 终态回流 / 规则注入 / 模板兜底 |
| patcheval | 双条件对照评测脚本与口径 |

---

## 12. 需求变更记录

| 日期 | 变更 | 原因 | 影响 |
|---|---|---|---|
| 2026-11-16 | v1.0 建立 | 用户指令：Cairn 模板 + 四份材料 | 初始化 task.md / todo.md |
