# Cairn Dispatcher 设计摘录

来源：`Cairn/docs/specs/dispatcher-design.md`。
本文件抽取调度决策所需的触发条件、Worker 选择、超时与 conclude fallback；完整配置附录不在摘录范围。

## 1. 定位与主链路

Dispatcher 是 Cairn 的客户端执行器：协调项目容器、给 Agent 下发明确任务、代 Agent 调用 Cairn API 写回图。
**Dispatcher 是唯一协议写入者和控制面**；Agent 不 claim、不 heartbeat、不直接调用 Cairn API。

固定主链路：

1. 从 Server 读取项目图（结构化接口 + `export?format=yaml`）；
2. 依据调度规则选择任务类型与 Worker；
3. 渲染 prompt；
4. `explore` 先 heartbeat 认领 Intent；`reason` 先 claim 项目级 reason lease；
5. 在项目容器内启动 Worker；
6. Worker 输出结构化 JSON；
7. Dispatcher 解析并调用 `complete` / `intents`（新建）/ `conclude` / `release` 写回。

项目切到非 `active`：视为硬停止 —— 不再派发，取消本地任务；被取消的 bootstrap/explore **不再进入 conclude fallback**；
后续轮次停止容器。项目被删除 → 视为 `deleted`，取消任务并删除 orphan 容器。

## 2. 三类任务总表

| 任务 | 触发条件 | 输入占位符 | 输出 | 超时策略 |
|---|---|---|---|---|
| `bootstrap` | active；`bootstrap_enabled=true` 且存在支持 bootstrap 的 Worker 或已有保留 bootstrap intent；facts 仅 origin+goal；无普通 intent | `{origin}` `{goal}` `{hints}`（不读图 YAML） | 主阶段成功固定 `fact + complete`；收尾阶段只 `fact` | 双阶段：`timeout` 后 `conclude_timeout` 收尾；两阶段都失败则 release，不写图 |
| `reason` | active；无未认领 intent；项目内无其他 reason；首次或“新态势”重触发 | `{graph_yaml}` `{fact_ids}` `{open_intents}` | `complete` / `intent` / 空 `data{}` | 仅 `timeout`；超时或非法结果直接作废，不写图 |
| `explore` | active；存在一个可认领的未结论 intent | `{graph_yaml}` `{intent_id}` `{intent_description}` | 一个客观 Fact 结论 `description` | 单阶段超时作废；双阶段超时/解析失败可进入 conclude |

统一输出信封：`{"accepted": true, "data": {...}}`。Dispatcher 对 stdout 全文做 JSON 解析与任务级 schema 校验。

## 3. 各任务细节

### bootstrap

触发补充：

- 保留 intent 约定固定为 `description="bootstrap"`、`creator="dispatcher.bootstrap"`、`from=["origin"]`；
- 允许不存在 bootstrap intent，或只存在保留的 open bootstrap intent；
- 单项目同时最多一个 bootstrap 任务，且尽量只保留一个 open 保留 intent。

输出约束：

- 主阶段只有在已解决问题时才返回；必须同时包含 `data.fact.description` 与 `data.complete.description`；
- 主阶段不允许只返回 `fact`；
- `bootstrap_conclude` 只允许 `fact`，不允许 `complete`，prompt 必须要求“不继续推进，只总结当前最关键事实”。

接口映射：

| 时机 | 接口 |
|---|---|
| 首次进入初始态且无保留 intent | `POST /intents` 创建保留 intent |
| 派发前 / 执行中按 interval | `POST /intents/{iid}/heartbeat` |
| 主阶段或收尾返回合法 fact | `POST /intents/{iid}/conclude` |
| 主阶段 `fact + complete` 成功 | `POST /complete`（先 conclude 再 complete） |
| 两阶段都失败 / conclude 写回失败 | `POST /intents/{iid}/release` |

失败语义：complete 写回失败时已写入的 fact 仍保留，下一轮 reason 可再完成项目；两阶段都失败则 release、不写 Fact，
项目下轮仍按新项目处理。

### reason

触发补充（“新态势”的精确定义）：

- 首次：当前没有任何 open intent；
- 之后仅以下三种情况重触发：
  1. Fact 数量增加；
  2. Hint 数量增加；
  3. 项目从“存在 open intents”进入“没有 open intents”。
- 单次 explore 失败、掉心跳、临时 release 导致 intent 仍 open —— **不构成新态势**；
- “总 intent 数量增加”不是重触发条件（通常只是上一次 reason 刚建了新 intent）；
- 当前项目内无其他 reason lease、无未认领 intent，且未超过 `max_project_workers`。

输出约束（三者互斥）：

- `data.complete`：必须是对象，含 `from` 与 `description`；不得同时带 `intent`；
- `data.intent`：含 `from` 与 `description`；
- 空 `data{}`：不写图；
- 若 `{open_intents}` 为空且没有 complete，则**必须**返回 intent；`{open_intents}` 非空时允许不返回 intent。

写回映射：

| 输出 | 动作 |
|---|---|
| `complete` | `POST /complete`（worker = 当前 Worker 名） |
| `intent` | `POST /intents`（creator = 当前 Worker 名，worker 固定 null） |
| 空 `data` | 不写图 |
| 收尾（无论是否写图） | `POST /reason/release`；项目已 completed/stopped 则由服务端清空 |

失败：`accepted:false`、非法 JSON、缺字段、退出码非 0、超时 → 直接作废，不写图，不立即重试。
写回 403 通常表示项目已非 active，任务作废并记 info。

### explore

输出约束：

- `data.description` 必须存在，且是客观事实结论；
- 即使未打出漏洞，也要返回客观结论（例：“测试常见 SQL 注入 payload，未发现可利用迹象”）；
- 不允许“我拒绝帮助渗透”等文本作为 description。

派发顺序固定：

1. 选中一个可认领 intent；
2. 先 `POST /intents/{iid}/heartbeat` claim；
3. heartbeat 成功后才启动 Worker。

双阶段 fallback 只适用于支持 session/conclude 的 Worker（Cairn 的 claudecode、codex、mock 都支持）：

1. 杀掉当前进程，保留 session id；
2. 保持 heartbeat，用同一 session 进入 `explore_conclude`；
3. prompt 要求“不继续探索，只总结截至目前已完成的探索与结论”；
4. conclude 在 `conclude_timeout` 内返回合法 JSON 且 `accepted:true` → `POST /conclude`；
5. conclude 再次超时/非法/`accepted:false`/写回失败 → 整次作废、不写图、release。

可直接失败、不进入 conclude：第一阶段 `accepted:false`、退出码非 0、Worker 未产生可读结果。
单阶段 Worker：超时或解析/结构校验失败即失败，杀进程、作废、release、记 warn。

## 4. Worker 选择顺序

1. 按任务类型筛选（`task_types` 包含该任务）；
2. 过滤达到 `max_running` 的 Worker；
3. 过滤处于本地 `retry_after` 不可选窗口的不健康 Worker；
4. 剩余中优先 `priority` 更小的；
5. 同 `priority` 优先当前运行中任务数更少的；
6. 仍相同 → 随机；
7. `explore`：先 heartbeat claim 成功，再启动任务；
8. `reason`：claim 成功后由 `reason/heartbeat` 维持；若 `worker_healthcheck=startup_and_task`，启动前再做一次健康检查，失败则本次任务作废并让该 Worker 进入短暂不可选窗口。

## 5. 健康检查

- 由 driver `check_health` 在 Dispatcher 进程内发起（不在容器里 curl）；
- 目标是验证该 Worker 的 LLM 配置可用：base URL 可达、API key 有效、model 可调用；
- 每次启动 Worker 前检查（`startup_and_task` 时还包括真正启动前）；HTTP 2xx = 健康；
  非 2xx / 连接失败 / 超时（`healthcheck_timeout`）= 不健康；
- 不健康 → 本次任务作废，记录本地 `retry_after`，窗口内不再派发；
- `explore_conclude` / `bootstrap_conclude` fallback **不重复**健康检查，直接尝试收尾。

## 6. 调度顺序与并发约束

全局：

1. 已运行项目优先，但只优先可立即派发的任务；
2. 运行中项目处于初始态且可 bootstrap → 优先继续；
3. 否则有可执行 explore → 优先继续探索；
4. 所有运行中项目暂无可派发任务，且 `max_running_projects` 有余量 → 启动一个新项目。

单项目：初始态按 `bootstrap_enabled` + Worker 能力选 bootstrap/reason → 满足新态势优先 reason → 有未认领 intent 则 explore。
`runtime.interval` 同时是主循环节拍与带 claim 任务的 heartbeat 周期（明确设计，不是偶然耦合）。

并发约束：

- 单 Dispatcher 实例；多实例共享同一 Server 不在支持范围；
- 单项目同时最多一个 bootstrap、一个 reason；
- 跨项目 reason 可并行；
- `max_workers`：同时运行任务总数上限；
- `max_running_projects`：已接手且仍 active 的项目上限（无任务也占名额）；
- `max_project_workers`：单项目任务上限（bootstrap + reason + explore 统一计入）；
- `workers[].max_running`：单个 Worker 并发上限。

## 7. 超时与 fallback 速查

| 任务 | 第一阶段 | 收尾 | 收尾触发 | 最终失败动作 |
|---|---|---|---|---|
| bootstrap | `tasks.bootstrap.timeout` | `bootstrap_conclude` / `conclude_timeout` | 超时、解析失败、契约不满足 | release 保留 intent，不写 Fact |
| reason | `tasks.reason.timeout` | 无 | — | 作废，不写图 |
| explore（单阶段） | `tasks.explore.timeout` | 无 | — | 作废，release |
| explore（双阶段） | `tasks.explore.timeout` | `explore_conclude` / `conclude_timeout` | 超时、非法 JSON、缺字段、schema 不符 | 作废，不写图，release |

## 8. 加载时最小校验

以下缺一即拒绝启动（摘录，完整清单见原规范 §配置校验）：

- `dispatch.yaml` 可读；`server` 存在；
- `runtime.max_workers`、`max_running_projects`、`max_project_workers`、`interval`、`healthcheck_timeout`、`prompt_group` 存在；
- `worker_healthcheck` 只允许 `startup_and_task` / `startup_only` / `disabled`；
- `tasks.bootstrap.timeout`、`tasks.bootstrap.conclude_timeout`、`tasks.reason.timeout`、
  `tasks.explore.timeout`、`tasks.explore.conclude_timeout` 存在；
- 每个 Worker 必须有 `type` 与正整数 `max_running`；`task_types` 仅 bootstrap/reason/explore；
- driver `type` 仅 claudecode/codex/mock；prompt 组目录与必需占位符存在。

运行时校验：driver 存在且支持该任务类型；健康检查通过；未超 `max_running`；不在 retry 窗口；
bootstrap/explore 必须先 claim 成功；双阶段必须拿到 session id。
