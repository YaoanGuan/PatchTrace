# 数据模型：Cairn 字段与 PatchTrace 扩展

来源：`Cairn/docs/specs/server-protocol.md` §数据结构、`docs/plans/requirement.md` §5.1。
左半是 Cairn 的原样字段，右半是 PatchTrace 在图存储中的落地形态。代码标识英文，说明中文。

## 1. Cairn 字段表

### Project

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 项目 id |
| `title` | string | 名称 |
| `status` | enum | `active` / `stopped` / `completed` |
| `bootstrap_enabled` | bool | 默认 true；false 时跳过 bootstrap 直接 reason |
| `created_at` | timestamp | 创建时间 |
| `reason` | object \| null | 项目级 reason lease：`{worker, trigger, started_at, last_heartbeat_at}` |

### Fact

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | `origin` / `goal` / `fNNN` |
| `description` | string | 客观事实；只增不改 |

### Intent

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | `iNNN` |
| `from` | string[] | 出发 Fact id，≥1；不得含 `goal` |
| `to` | string \| null | 结论 Fact；null = 未结论 |
| `description` | string | 意图描述 |
| `creator` | string | 声明者，不可变 |
| `worker` | string \| null | 当前认领者；已结论时永久保留 |
| `last_heartbeat_at` | timestamp | 仅未结论时有意义；永久保留 |
| `created_at` | timestamp | 创建时间 |
| `concluded_at` | timestamp \| null | 结论时间 |

### Hint

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | `hNNN` |
| `content` | string | ~100 tokens 的判断或策略，不注入方法 |
| `creator` | string | 作者 |
| `created_at` | timestamp | 创建时间 |

## 2. PatchTrace 图模型

### 节点 `Node`

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string | 唯一 id |
| `project_id` | string | 所属项目（内部 lab id） |
| `kind` | enum | `fact` / `intent` / `hint` |
| `type` | enum | Fact 细分类型（见 §3）；intent/hint 使用固定类型 |
| `content` | string | 客观描述，不含状态机文本 |
| `state` | enum | `open` / `claimed` / `resolved` / `refuted`；仅 intent 有意义 |
| `from` | string[] | 因果上游（intent 的出发 Fact） |
| `to` | string \| null | 结论 Fact id |
| `worker` | string \| null | 认领者 |
| `lease_until` | timestamp \| null | 租约到期；等价于 Cairn 的 `last_heartbeat_at + intent_timeout` |
| `created_at` | timestamp | 创建时间 |
| `concluded_at` | timestamp \| null | 结论时间 |

### 边 `Edge`

| `kind` | 语义 | 典型用途 |
|---|---|---|
| `derive` | 由该 Fact 推导出 | `patch_profile` → `constraint` |
| `support` | 支持该 Intent / 结论 | `observation` → `verdict` |
| `refute` | 反驳该 Intent / 结论 | 反证、排除方向 |
| `block` | 依赖阻塞（强制无环） | 排序、前置条件 |
| `produce` | Intent 结论产出的 Fact | 溯源主链 |

节点/边的取舍：Cairn 的 `Intent.from[] → to` 超边在 PatchTrace 中落为 `Node(kind=intent)` + `produce` 边；
`derive`/`support`/`refute` 是 PatchTrace 新增的语义边，用于表达 Cairn 图中没有显式编码的推导与证据关系。

## 3. PatchTrace Fact 类型扩展

| `type` | 含义 | 主要生产者 | 典型 `from` / 边 |
|---|---|---|---|
| `patch_profile` | 补丁摄入结果：文件、函数、修复语义摘要 | `bootstrap`（`patchintel.Ingest*`） | `origin`；`derive` → constraint |
| `exploit_idea` | 语义反演出的可利用方向 | `bootstrap`（`patchintel.Invert`） | `patch_profile` |
| `constraint` | 漏洞约束三元组：condition / harm / evidence | `bootstrap` / `reason` | `derive` 自 patch_profile |
| `observation` | 沙箱中的客观观测（含失败、超时、环境错误） | `explore` | `support` / `refute` 目标 Intent 或 verdict |
| `verdict` | 对约束/漏洞成立的判决；必须带证据边 | `explore` 终态 / 判决 | 必须有 `support` 或 `refute` 边 |
| `rule_hit` | RuleBase 命中记录（pattern、版本、置信度） | `explore` / CE4 回流 | `support` 当前推理 |
| `finding` | 最终漏洞结论；必须可溯源到 origin | `reason` / `complete` 前 | 必须存在 `origin → … → finding` 的 produce/derive/support 路径 |

Intent / Hint 的类型：`gap`（信息缺口）、`human_intent`（人工意图）、`scene_template`（场景模板注入）。

## 4. 不变量（必须有测试）

1. **原子 claim**：一个 Intent 同时只能被一个 worker 认领（CAS）；并发第二个得到 409。
2. **block 无环**：写入 `block` 边前做环检测，成环即拒绝，图中不得出现环。
3. **refuted 永久保留**：已 `refuted` 的节点不删除、不改写。
4. **finding 可溯源**：任何 `finding` 必须存在一条 `origin → … → finding` 的 `produce`/`derive`/`support` 路径，
   否则不得 `complete`。
5. **evidence 强约束**：`verdict` 必须带 `support` 或 `refute` 边，且证据来自沙箱观测。
6. **lease 回收**：`lease_until` 过期且未结论的 Intent 回到 `open`，可被重新认领；已结论不复用。

## 5. Cairn → PatchTrace 字段映射

| Cairn | PatchTrace | 说明 |
|---|---|---|
| Fact | `Node{kind=fact}` | `description` → `content`；新增 `type` |
| Intent | `Node{kind=intent}` | `from[]`/`to`/`worker`/`concluded_at` 同名；新增 `state`、`lease_until` |
| Intent 的 `last_heartbeat_at` | `lease_until` | 存到期时刻，避免每次读取都做超时推算 |
| Hint | `Node{kind=hint}` | `content` 直接映射 |
| Project.reason | Dispatch 本地租约 + 图库 `projects` 表 | 仍保证单项目单 reason |
| Fact `origin` / `goal` | 项目创建时写入的两个 Fact 节点 | `goal` 仅由 complete 边指向 |
| `export?format=yaml` | `graph/export.go` 的 YAML 快照 | 四段结构；不含租约内部字段 |
| `export?format=timeline` | `graph/export.go` 的 timeline | 按 `created_at`/`concluded_at` 排序 |

存储约定：`modernc.org/sqlite`，WAL，`MaxOpenConns(1)`；库文件 `data/patchtrace_graph.db`。
