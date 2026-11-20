# PatchTrace v4 需求规格：Agent Infra × Offensive Infra 正交重构（v4_re）

> 状态：**规划中，不实施**。启动条件：v3 G0–G5 全过、`go test` 全绿、真实 CVE 验收留档。
> 依据：`v4_方案.md`（战略与架构）+ `jev+reflex/` 图片 + 文章《AI智能化渗透的过去与未来》。
> 定位：v4 只改**归属与边界**，不改 v3 的 Decision/Reflex/Evidence 语义。

## 0. 战略输入（P10）

### 方向

把 PatchTrace 拆成三个可独立替换的平面：

1. **Offensive Infra（OI）**：沙箱、工具链、补丁情报、证据、POC、目标与授权范围。
2. **Agent Infra（AI）**：模型网关、Harness、Decision/Reflex、黑板、MTP、IoA。
3. **Human Infra（HCI）**：审批、Hint、暂停/恢复、隔离复核、报告。

### 成功标准

| 指标 | 目标 | 证明 |
|---|---|---|
| `cross_plane_imports` | 0 | `go test ./internal/archtest/` |
| `oi_standalone` | 不启 Agent 完成一次工具调用并落证据 | `pt-oi` CLI 记录 |
| `harness_swap_parity` | H1/H2 同项目事实/证据一致率 100% | eval 对照 |
| `evidence_parity` | 旧路径 vs OI 客户端 100% | 对照测试 |
| `tokens_per_verified_finding` | 比 v3 基线降 ≥15% | eval JSON |
| `time_to_verified_finding` | 比 v3 基线降 ≥20% | eval JSON |
| `tool_onboarding_days` | ≤1 人日 | 接入记录 |
| `safety_incidents` | 0 | 审计日志 |

### 约束

- 默认单二进制 + SQLite；OI 独立进程是可选部署形态（stdio/unix socket）。
- `graph.db` 协议不变；v4 只加只读投影。
- v3 所有 flag 保留，默认值不变。
- 每个 OI 能力必须有 CLI + 稳定 JSON schema。
- 不引入新存储引擎；新表进 `data/offensive.db`（modernc sqlite）。

### 不做什么

- 不做 L4 自主渗透、Agent Botnet、Agentic RL 训练。
- 不做多租户、不替换 Docker、不重写 graph 协议。
- 不为了“像文章”引入向量库、MCP 市场、知识库注入。

## 1. 背景与现状

### 1.1 文章结论（与 v4 直接相关）

- 渗透是开放性问题，不存在完整 Harness；不强调全自动，强调智能化程度与人机交互形态。
- 与模型正交的投入才可持续：数据、经验、基础设施（D×E×I）。
- 框架在模型与工具之间插入编排层，替模型思考；多 Agent 退化为分工与调度。
- L2 机制：Agentic Tool、MTP、GraphRAG、IoA；L3：Cairn 黑板、Cybernetics 意图状态机；L4：IoA 协调、端侧 Agent。

### 1.2 当前仓库的耦合点（2026-11-20 审计）

| 位置 | 问题 |
|---|---|
| `LabManager` | 总脑选组/派活；多 Agent 退化为调度 |
| `ProjectManager` / `DecisionBrain` | 任务状态、模型循环、记忆压缩与工具执行混在一起 |
| `agents/**` + 数字人池 | 能力与调度耦合；换工具要改 Agent 目录 |
| `toolCalling/**` | 工具实现、参数校验、审批、MCP 绑定混在一起 |
| `dockerManager` / `taskManager` | 沙箱与进程注册表被 Agent 代码直接 import |
| `graphserver` | 黑板协议可保留，但读写者不区分平面 |

### 1.3 为什么现在做

v3 把 Decision/Reflex/Evidence 做成 Harness 内核后，剩余风险集中在“换模型/换 Harness 要动工具代码、换工具要动 prompt”。v4 解耦后，两边可独立演进，符合文章的正交原则。

## 2. 目标 / 非目标 / 反目标

### 2.1 目标

- OI 可独立运行并被任意 Agent 调用。
- AI 平面可替换 Harness（H1 ReAct+JEV / H2 Cybernetics）而不改 OI。
- 工具与数据不进模型上下文，经 MTP 按需取。
- 人类只注入判断：审批、Hint、暂停、复核。
- 旧路径可回退，迁移期 parity 可证。

### 2.2 非目标

- 不改 v3 的 Decision/Reflex/Evidence 语义。
- 不重写 Web UI；只改数据来源与交互语义。
- 不引入消息队列、向量数据库、外部服务依赖。

### 2.3 反目标（即使技术可行也算失败）

- 解耦后端到端变慢、需要人工搬运数据。
- 为了“平面”而复制两套业务逻辑。
- OI 暴露到非本地网络且无授权校验。
- Harness 切换后事实/证据不一致。
- 人类重新回到输入命令的操作员角色。

## 3. 用户与场景

| 用户 | 关注 |
|---|---|
| 渗透工程师 operator | 授权范围内快速得到可复现证据 |
| 安全负责人 reviewer | 证据链、审批记录、可回放 |
| 平台工程师 | 换模型/换工具/换 Harness 不动另一边 |
| 外部 Agent 开发者 | 用 CLI/JSON 调用 OI 能力 |

必测场景：

| 场景 | 输入 | 期望 | 证据 |
|---|---|---|---|
| S1 OI 独立 | `pt-oi tool call scan --target ...` | 返回聚合结果 + 证据 hash | CLI 日志 + evidence |
| S2 契约 parity | 同一命令走旧路径与 OI 客户端 | 证据 hash/退出码一致 | 对照测试 |
| S3 Harness 切换 | 同项目 H1 → H2 | 事实/证据一致 | eval JSON |
| S4 MTP 预算 | scan 大量输出 | 原始输出不进 prompt，token 下降 | prompt 记录 |
| S5 审批 | 危险命令 | 默认拒绝；人工覆盖留痕 | approvals 表 |
| S6 真实链路 | CVE-2025-3248 | bootstrap → verdict → rule | runbook |

## 4. 概念契约

### 4.1 OI 契约（`internal/offensive/contract`）

```go
const Version = "v1"

type Request struct {
    Version   string          `json:"version"`
    Method    string          `json:"method"`
    ProjectID string          `json:"project_id"`
    Params    json.RawMessage `json:"params"`
}

type Response struct {
    Version  string            `json:"version"`
    OK       bool              `json:"ok"`
    Data     json.RawMessage   `json:"data,omitempty"`
    Evidence []evidence.Record `json:"evidence,omitempty"`
    Error    *Error            `json:"error,omitempty"`
}

type Client interface {
    SandboxCreate(ctx context.Context, projectID, image string) (sandboxID string, err error)
    SandboxExec(ctx context.Context, projectID, command, workdir string, timeout time.Duration) (evidence.Record, error)
    ToolCall(ctx context.Context, projectID, name string, args map[string]any) (ToolResult, error)
    IntelProfile(ctx context.Context, diff string) ([]Fact, error)
    IntelSearch(ctx context.Context, query string) ([]Candidate, error)
    EvidencePut(ctx context.Context, rec evidence.Record) (hash string, err error)
    EvidenceGet(ctx context.Context, hash string) (evidence.Record, error)
    POCClassify(ctx context.Context, path string) (POCClass, error)
    TargetGet(ctx context.Context, projectID string) (Target, error)
}
```

规则：

- 执行类方法（`SandboxExec`、`ToolCall`）成功响应必须带至少一条 `evidence.Record`；无证据的 `ok` 不被 AI 平面接受。
- 版本不匹配 → `ErrVersionMismatch`，不静默降级。
- 契约只含能力与证据，不含流程、prompt、模型参数。

### 4.2 Import 法

```text
internal/offensive/**  不得 import internal/agent/**、llm、prompts、Web
internal/agent/**      只可 import contract 与 evidence 类型，不得 import sandbox/toolchain/intel 内部包
internal/hci/**        可依赖两边
cmd/**                 负责组装
```

用 `go list -deps` 断言，违反即测试失败。

### 4.3 Harness 接口（`internal/agent/harness`）

```go
type Harness interface {
    Step(ctx context.Context, in Observation) (Outcome, error)
}

type Observation struct {
    ProjectID string
    IntentID  string
    Facts     []blackboard.Fact
    Hint      *blackboard.Hint
    Last      Outcome
}

type Outcome struct {
    Decision decision.Result
    Action   *Action
    ReflexID string
    Evidence []evidence.Record
}

type Action struct {
    Tool string
    Args map[string]any
}
```

### 4.4 黑板接口（`internal/agent/blackboard`）

```go
type Board interface {
    Facts(ctx context.Context, projectID string) ([]Fact, error)
    Claim(ctx context.Context, projectID, intentID, owner string, ttl time.Duration) (Lease, error)
    Complete(ctx context.Context, projectID, intentID, summary string, evidence []evidence.Record) error
    Neighborhood(ctx context.Context, projectID, intentID string, depth int) (Subgraph, error)
}
```

### 4.5 MTP（`internal/agent/mtp`）

```go
type Entry struct {
    ID      string          `json:"id"`
    Kind    string          `json:"kind"` // tool | dataset | doc
    Summary string          `json:"summary"`
    Schema  json.RawMessage `json:"schema,omitempty"`
    Cost    int             `json:"cost"` // 预估 token
}

type Catalog interface {
    Search(ctx context.Context, query string, k int) ([]Entry, error)
    Fetch(ctx context.Context, id string) (json.RawMessage, error)
}
```

### 4.6 IoA（`internal/agent/ioa`）

```go
type Message struct {
    ID        string    `json:"id"`
    From      string    `json:"from"`
    To        string    `json:"to,omitempty"`
    ThreadID  string    `json:"thread_id,omitempty"`
    ReplyTo   string    `json:"reply_to,omitempty"`
    Content   string    `json:"content"`
    Refs      []string  `json:"refs,omitempty"`
    CreatedAt time.Time `json:"created_at"`
}

type Bus interface {
    Send(ctx context.Context, m Message) (id string, err error)
    Read(ctx context.Context, agentID string, since time.Time) ([]Message, error)
    Link(ctx context.Context, fromID, toID string) error
}
```

### 4.7 HCI

```go
type Approval struct {
    ID, ProjectID, Kind, Subject, Reason string
    Status                               string // pending | approved | rejected
    Actor, Note                          string
}
type HumanAction struct {
    Actor, Action, Subject, Note string
    CreatedAt                    time.Time
}
```

## 5. 需求与验收

### V4-R1 正交边界

- `internal/offensive/**` 不 import AI/Web；`internal/agent/**` 只依赖 contract/evidence 类型。
- 验收：`TestOffensiveDoesNotImportAgent`、`TestAgentOnlyImportsContract`、`TestNoWebFromPlanes` 全绿。

### V4-R2 OI 契约 v1

- 版本、请求/响应、错误、证据要求固定；FakeServer 可供离线测试。
- 验收：契约 round-trip；无证据的执行响应被拒；未知方法返回结构化错误。

### V4-R3 OI 沙箱与工具

- 沙箱执行、超时、项目别名；Agentic Tool `scan`/`intel_lookup` 封装多步并返回聚合结果。
- 验收：执行返回证据；超时终止；`scan` 不返回原始 nmap 全文；未知工具拒绝。

### V4-R4 情报与证据

- 补丁情报（diff → patch_profile → rules）与证据库归 OI；证据可按 hash 存取、按项目索引。
- 验收：真实 diff 产出 profile；无命中不伪造；证据 round-trip；缺字段记录被拒。

### V4-R5 OI 独立进程与 CLI

- `pt-oi` 支持 stdio JSON-RPC 与子命令；`--json` 稳定输出。
- 验收：不启 Agent 完成一次工具调用；版本不匹配拒绝；危险命令默认拒绝。

### V4-R6 Harness H1 与黑板

- `Step` 实现观察 → Reason → Decision → Reflex|Action → 工具；写图仍由 runner；claim/lease 语义保持。
- 验收：Step 单测；reflex 命中短路；binding defer；completion 未通过不写 fact。

### V4-R7 MTP

- 工具/数据目录、search/fetch、上下文预算；原始输出留在 OI。
- 验收：search 排序稳定；fetch 只返回摘要/按需数据；预算裁剪生效；未知 ID 报错。

### V4-R8 GraphRAG

- 邻域检索按 Fact/Intent 类型与边深；不引入向量库。
- 验收：depth 正确；类型过滤正确；无 embedding 依赖。

### V4-R9 HCI 决策式交互

- 审批、Hint、暂停/恢复、quarantine 复核；人类动作全审计。
- 验收：危险命令不可绕过；覆盖留痕；Hint 优先于 reflex；暂停立即生效。

### V4-R10 H2 Cybernetics Harness

- `explorer → execute`；`plan` 节点请求人类；node prompt/task.md/overlay 为文本资产。
- 验收：状态机单测；plan 暂停；overlay 生效；同项目 H1/H2 parity=100%。

### V4-R11 IoA

- 消息 CLI（send/read/link），消息可组 thread/tree/DAG；应用层为自然语言。
- 验收：send/read/link 往返；DAG 结构可查；双 Agent 实验完成一次计划。

### V4-R12 端侧 Agent 原型（可选闸门）

- 单文件客户端，只有 read/write/glob/bash 四工具 + OI 客户端；无 MCP/task/调度器。
- 验收：四工具之外不可用；不启调度器；仅授权靶标。

### V4-R13 兼容与迁移

- 旧路径默认关闭、可回退；迁移期 parity；删除旧路径前全量测试绿。
- 验收：flag 矩阵；无双写；旧路径回退可用。

### V4-R14 指标与经济学

- `cross_plane_imports`、`oi_standalone`、`harness_swap_parity`、`tokens_per_verified_finding`、`time_to_verified_finding`、`tool_onboarding_days`、`safety_incidents`。
- 验收：JSON 输出；对照口径可复现；成本计入。

### V4-R15 真实验收

- CVE-2025-3248：拆分后全链路跑通，evidence parity 100%，≥1 verified finding，≥1 规则。
- 验收：runbook 记录命令与原始输出；失败点也记录。

### V4-R16 安全与审计

- OI 只监听本地；project/engagement 绑定；审批不绕过；所有人类动作与工具调用可审计。
- 验收：越权 project 拒绝；审批缺失拒绝；审计可按 ID 串联。

## 6. 数据模型（`data/offensive.db`）

```sql
CREATE TABLE evidence_index (
  hash TEXT PRIMARY KEY, project_id TEXT, kind TEXT, path TEXT, created_at TEXT
);
CREATE TABLE tool_runs (
  id TEXT PRIMARY KEY, project_id TEXT, tool TEXT, args_json TEXT,
  exit_code INTEGER, evidence_hash TEXT, started_at TEXT, ended_at TEXT
);
CREATE TABLE targets (
  project_id TEXT PRIMARY KEY, engagement TEXT, scope_json TEXT,
  workdir TEXT, created_at TEXT
);
CREATE TABLE approvals (
  id TEXT PRIMARY KEY, project_id TEXT, kind TEXT, subject TEXT, status TEXT,
  actor TEXT, note TEXT, created_at TEXT
);
```

`graph.db` 不变；`reflex.db` 不变。删除 `offensive.db` 即回滚 OI 审计数据。

## 7. 安全与合规边界

- 仅授权靶标；project/engagement 由 OI 强制校验。
- `pt-oi` 默认只监听本机 stdio/unix socket；TCP 需显式开启并带 token。
- 危险命令默认 `forbidden`；未知默认 `approval`；分类失败 fail-closed。
- 证据落库前脱敏；日志不打印凭据。
- 人类覆盖必须写 `approvals`，含 actor/note/时间。

## 8. 闸门

| 闸门 | 条件 | 对应 |
|---|---|---|
| G0 基线 | v3 G0–G5 全过 + 基线 JSON | R15 前置 |
| G1 契约 | 契约 + import 断言全绿 | R1, R2 |
| G2 OI | OI 独立运行 + parity | R3, R4, R5 |
| G3 Harness | H1 + 黑板 + MTP 单测 | R6, R7 |
| G4 检索与交互 | GraphRAG + HCI | R8, R9 |
| G5 H2 | 双 harness parity | R10 |
| G6 IoA | 双 Agent 实验 | R11 |
| G7 收口 | 旧路径删除 + flag 矩阵 + 真实验收 | R13, R14, R15 |

任一闸门未过，不得把对应能力标为“已落地”。

## 9. 失败模式与降级

| 场景 | 行为 |
|---|---|
| OI 进程不可用 | 同进程接口回退；再失败则阻塞并报错，不伪造证据 |
| 契约版本不匹配 | 拒绝调用，提示升级 |
| 证据缺失 | 拒绝写入事实，记 `evidence_missing` |
| MTP 目录过期 | 回退直接 OI 查询 |
| Harness 崩溃 | 保留黑板最后状态，可恢复/重开 |
| 审批超时 | 挂起，不自动放行 |
| 迁移 parity 失败 | 回滚旧路径 |

## 10. 交付物

1. 四份文档：`v4_方案.md` / `v4_re.md` / `v4_task.md` / `v4_todo.md`。
2. 代码：`internal/offensive/**`、`internal/agent/**`、`internal/hci/**`、`cmd/pt-oi`。
3. 契约 v1 + conformance + FakeServer。
4. 真实运行记录 `docs/runbooks/cve-2025-3248-v4-run-<date>.md`。
5. eval 对照 JSON 与经济学报告。

## 11. 变更记录

| 日期 | 变更 | 原因 |
|---|---|---|
| 2026-11-20 | 创建 v4 需求 | 拆解 `v4_方案.md`，与 v3 文档 ID 对齐 |
| 2026-11-20 | 启动条件固定为 v3 G0–G5 | 避免在未验证 Harness 上做平面拆分 |
