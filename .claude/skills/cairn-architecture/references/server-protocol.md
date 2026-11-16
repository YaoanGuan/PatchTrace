# Cairn Server 协议摘录

来源：`Cairn/docs/specs/server-protocol.md`（Cairn v1 协作探索协议）。
本文件是摘录与整理，不是全文；实现 `graph/` + `graphserver/` 时以本文件列出的字段、接口与语义为准。

Server 的定位：**图的一致性与真相源，不做推理、不做决策**。图是知识与因果的唯一表达；
项目级协调状态（仅 `Project.reason`）不是事实，不进图、不参与因果。

## 1. 核心概念

| 概念 | 说明 |
|---|---|
| Project | 有明确起点和终点的图实例；状态 `active` / `stopped` / `completed` |
| Fact | 已确认的客观事实，图节点；只增不改，永久保留；状态变化用新 Fact 表达 |
| Intent | 从 `from[]` 个 Fact 出发的探索过程（超边），`to` 为结论 Fact；未结论时 `to=null` |
| Hint | 图外输入（人工策略/态势）；不影响因果；`stopped`/`completed` 时仍可写 |

每个项目创建时写入两个特殊 Fact：`origin`、`goal`。普通 Fact 由系统生成 id（如 `f001`）。
`from` 不能包含 `goal`。

## 2. 数据结构

### Project

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 项目 id |
| `title` | string | 项目名称 |
| `status` | enum | `active` / `stopped` / `completed` |
| `bootstrap_enabled` | bool | 初始态是否允许 bootstrap，默认 `true` |
| `created_at` | timestamp | 创建时间 |
| `reason` | object \| null | 项目级 reason lease，null 表示无人执行 |

### Project.reason

| 字段 | 说明 |
|---|---|
| `worker` | 当前执行 reason 的消费者 |
| `trigger` | 本次 reason 的触发原因（消费者定义） |
| `started_at` | 本次 reason 开始时间 |
| `last_heartbeat_at` | 最近一次 reason 心跳时间 |

### Fact

| 字段 | 说明 |
|---|---|
| `id` | `origin` / `goal` / 系统生成（如 `f001`） |
| `description` | 客观事实描述（结论 + 大输出文件引用） |

### Intent

| 字段 | 说明 |
|---|---|
| `id` | 系统生成（如 `i001`） |
| `from` | 出发 Fact id 数组，至少一个；不能含 `goal` |
| `to` | 结论 Fact id；`null` = 尚无结论 |
| `description` | 意图描述 |
| `creator` | 声明者，创建时写入，不可变 |
| `worker` | 当前认领者；语义随状态变化（见 §3） |
| `last_heartbeat_at` | 最后一次心跳；仅未结论时有意义，永久保留 |
| `created_at` | 创建时间 |
| `concluded_at` | 结论时间；`null` = 尚无结论 |

创建时若同时认领，`worker` 必须等于 `creator`。保留 bootstrap intent 约定：
`from=["origin"]`、`description="bootstrap"`、`creator="dispatcher.bootstrap"`（消费者约定，非服务端约束）。

### Hint

| 字段 | 说明 |
|---|---|
| `id` | 系统生成 |
| `content` | 提示内容 |
| `creator` | 作者 |
| `created_at` | 创建时间 |

### Settings

| 字段 | 说明 |
|---|---|
| `intent_timeout` | 秒；超过此时间无心跳则未结论 Intent 的 worker 清空 |
| `reason_timeout` | 秒；超过此时间无心跳则 `Project.reason` 清空 |

## 3. Intent 的 worker 语义与租约

| 状态 | worker 含义 |
|---|---|
| 尚无结论，`worker=null` | 无人处理，等待认领 |
| 尚无结论，`worker` 有值 | 有人正在处理 |
| 尚无结论，worker 超过 `intent_timeout` | 清空为 null，等待重新认领 |
| 已结论（`to≠null`） | 产出结论者，永久保留；不参与超时 |

`release` 让未结论 Intent 回到无人认领；`last_heartbeat_at` 保留、不清空。
项目切到 `stopped` 时，Server 立即清空所有 open intent 的 `worker` 与 `project.reason`，claim 立刻失效。

## 4. 接口列表

### 全局设置

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/settings` | 返回 `intent_timeout`、`reason_timeout` |
| PUT | `/settings` | 更新上述两项 |

### Projects

| 方法 | 路径 | 用途 | 备注 |
|---|---|---|---|
| GET | `/projects` | 项目列表 + 元信息/统计 | 不含图数据 |
| POST | `/projects` | 创建项目 | 写入 `origin`/`goal`；`hints` 可选；`bootstrap_enabled` 默认 true |
| GET | `/projects/{id}` | 完整项目数据 | 含 `project` / `facts` / `intents` / `hints` |
| DELETE | `/projects/{id}` | 删除项目及全部数据 | Dispatcher 视不存在项目为 `deleted`，清理 orphan 容器 |
| PUT | `/projects/{id}/title` | 改标题 | 不改变图，任何状态均可 |
| PUT | `/projects/{id}/status` | `active` ↔ `stopped` | `completed` 不可经此接口变更 |
| POST | `/projects/{id}/reason/claim` | 认领项目级 reason lease | 单项目最多一个；同 worker 幂等 |
| POST | `/projects/{id}/reason/heartbeat` | reason 续约 | 仅当前持有者 |
| POST | `/projects/{id}/reason/release` | 释放 reason lease | 空则幂等 |
| POST | `/projects/{id}/intents` | 声明 Intent（可同时认领） | `from[]` 至少一个；`worker` 为 null 或等于 creator |
| POST | `/projects/{id}/intents/{iid}/heartbeat` | 认领/续约 Intent | `worker=null` 时可被任意消费者认领 |
| POST | `/projects/{id}/intents/{iid}/release` | 释放未结论 Intent | 仅当前 worker；空则幂等 |
| POST | `/projects/{id}/intents/{iid}/conclude` | 原子产出 Fact 并落定 Intent | 仅未结论 Intent |
| POST | `/projects/{id}/complete` | 原子建立 `from[] → goal` 完成边 | 项目变 `completed`，清空 reason |
| POST | `/projects/{id}/reopen` | 撤销完成态 | 仅 `completed` 项目 |
| POST | `/projects/{id}/hints` | 写 Hint | 任何状态均可 |

### 导出

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/projects/{id}/export?format=yaml` | 图 YAML 快照，供渲染 prompt |
| GET | `/projects/{id}/export?format=timeline` | 事件时间线，供审计/回放 |

## 5. 状态码语义

规范显式约束的只有以下三类；其余按 HTTP 常规（成功 200，对象不存在 404）实现。

| 码 | 语义 |
|---|---|
| 200 | 成功；返回当前对象或操作结果 |
| 403 | 探索写操作遇到项目 `stopped` / `completed`（硬停止） |
| 409 | claim/续约冲突：lease 被其他消费者占用且未超时；或续约时无人持有。写回时 403 表示项目已非 active，任务作废 |
| 422 | 文本字段（title/content/description/creator/worker/trigger）或 `from` 中的 Fact id 首尾裁剪后为空 |

补充：`reopen`、`PUT title`、`POST hints` 属于管理/图外写操作，不受 403 的 active 限制。

## 6. 关键原子操作

### conclude

输入 `worker` + `description`；原子写入新 Fact，并把 Intent 的 `to` 指向它、写 `concluded_at`。
claim 规则与 heartbeat 相同：未认领可直接 conclude；被其他人占用且未超时 → 409。

### complete

输入 `from[]` + `description` + `worker`；原子创建一条 `from → goal` 的已结论 Intent（`creator=worker=请求中的 worker`），
项目状态变 `completed`，立即清空 `project.reason`。

### reopen

仅 `completed` 项目可用。服务端：

1. 找到唯一一条 `to=goal` 的完成边，读取其 `from`；
2. 删除该完成边（不保留图内“曾完成”历史）；
3. 新建纠错 Fact（调用者提供的客观说明）；
4. 新建已结论 Intent，`description="external_feedback"`，`from` 继承原完成边的 `from`，`to` 指向纠错 Fact；
5. `creator`/`worker` 写为请求中的 `creator`；
6. 项目恢复 `active`，清空 `project.reason`。

### stopped 清理

`PUT status=stopped` 时立即清空所有 open intent 的 `worker` 与 `project.reason`。
`stopped` 可恢复为 `active`；`completed` 只能经 `reopen` 撤销。

## 7. 导出格式

### YAML

四段：`project`（title/origin/goal/bootstrap_enabled）、`hints`、`facts`、`intents`。
不含 `last_heartbeat_at`，不含 `project.reason`；`to=null` 的 Intent 保留，`worker` 反映当前处理者。
`created_at` / `concluded_at` 格式化为 `YYYY-MM-DD HH:mm:ss`。导出前服务端执行超时清理。

### timeline

纯文本，按事件发生时间排序。事件类型：`PROJECT CREATED`、`HINT`、`INTENT DECLARED`、
`INTENT CONCLUDED`、`PROJECT COMPLETED`。若完成后被 `reopen`，旧的 `PROJECT COMPLETED` 不保留。
YAML 展示拓扑，timeline 展示先后与因果链，二者互补。

## 8. 消费者典型顺序（Dispatcher 视角）

1. 读结构化项目数据（调度/判断） + `export?format=yaml`（渲染 prompt）。
2. 初始态（facts 仅 origin/goal，无普通 intent）：按 `bootstrap_enabled` 与 Worker 能力选择 bootstrap 或 reason。
3. 有未认领 intent：claim（heartbeat）→ explore → conclude（失败则 release）。
4. 无未认领 intent：claim reason lease → reason → complete / 新 intent / 不写图 → release。
5. 项目 `completed` 但有误：`reopen`（写纠错 Fact 与 `external_feedback` 边）→ 回到 active 继续。
