# PatchTrace v4 实施计划：Agent Infra × Offensive Infra 正交重构

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不改变 v3 Decision/Reflex/Evidence 语义的前提下，把 PatchTrace 拆成 Offensive Infra、Agent Infra、HCI 三个可独立替换的平面。

**Architecture:** `internal/offensive/` 拥有沙箱、工具链、情报、证据、POC、目标；`internal/agent/` 拥有模型网关、Harness、Decision/Reflex、黑板、MTP、IoA；`internal/hci/` 承审批、Hint、暂停与复核。两边只通过 `offensive/contract` v1 交互。

**Tech Stack:** Go 1.24、`modernc.org/sqlite`、现有 `graph`/`graphserver`/`dispatch`/`eval`、v3 的 `decision`/`reflex`/`evidence`、goja。

**Spec:** `docs/plans/v4_re.md`（本计划逐条实现 V4-R1…R16）；战略见 `docs/plans/v4_方案.md`。

## Global Constraints

- 启动前 v3 G0–G5 全过，`go test ./...` 相关包全绿；未过不动 v4。
- 默认单二进制 + SQLite；不引入消息队列/向量库/外部服务。
- `graph.db` 协议不变；`reflex.db` 结构不变。
- `internal/offensive/**` 不得 import `internal/agent/**`、`llm`、`prompts`、`Web`；`internal/agent/**` 只依赖 `offensive/contract` 与 `offensive/evidence` 类型。
- 执行类 OI 响应必须携带 `evidence.Record`；缺失即拒绝。
- 危险命令默认 `forbidden`，未知默认 `approval`，分类失败 fail-closed。
- v3 flag 全部保留，默认值不变；旧路径默认关闭且可回退。
- 提交只在你明确要求时执行；每个 Task 的提交步骤以用户指令为准。

## Review Focus

1. **契约漂移**：OI 升级到 v2 而 Agent 旧客户端继续调用 → 必须返回 `ErrVersionMismatch`，不得静默执行。测试：`TestContractVersionRejected`（T7）。
2. **证据丢失**：OI 返回 `ok` 但没有 evidence，Agent 直接写 fact → 必须拒绝并记 `evidence_missing`。测试：`TestResponseRequiresEvidenceForExec`（T1）、`TestExecReturnsEvidence`（T3）。
3. **审批绕过**：独立 `pt-oi` 进程绕过 HCI 直接执行危险命令 → 必须默认拒绝并留审计。测试：`TestApprovalRequiredForbidden`（T7）、`TestForbiddenBlocked`（T11）。
4. **迁移双写**：旧路径与新 Harness 同时写图 → 必须单写者，flag 切换原子。测试：`TestNoDoubleWrite`（T15）。
5. **Harness 切换语义漂移**：同项目 H1/H2 产出不同事实/证据 → parity 必须 100%，否则回滚。测试：`TestSwapParity`（T12）。

---

## 文件结构

```text
cmd/pathtrace/                  # 主程序组装（HCI + 两平面）
cmd/pt-oi/                      # OI 独立进程 CLI
cmd/aiagent/                    # 端侧 Agent 原型（T14，可选）
internal/offensive/
  contract/                     # v1 类型、Client、FakeServer、stdio client
  sandbox/                      # 沙箱运行时
  toolchain/                    # 工具注册 + Agentic Tool
  intel/                        # 补丁情报
  evidence/                     # 证据库
  poc/                          # POC 分级
  target/                       # 项目/授权/工作区
  server/                       # JSON-RPC 服务端
internal/agent/
  model/                        # llm 网关（T8 起薄适配）
  harness/                      # H1 react.go；H2 cybernetics.go
  decision/                     # v3 decision 迁入
  reflex/                       # v3 reflex 迁入
  blackboard/                   # graph 适配 + 邻域检索
  mtp/                          # 目录/搜索/预算
  ioa/                          # 消息总线/CLI
internal/hci/                   # 审批/Hint/暂停/复核
internal/archtest/              # import 断言
internal/eval/                  # 指标
```

---

## 阶段 V4-0：冻结基线

### Task 0：冻结 v3 基线（V4-R15 前置）

**Files:**
- Create: `docs/runbooks/v4-baseline-2026-11-20.md`
- Read: `docs/plans/v3_todo.md`、`docs/plans/v4_方案.md`

**Interfaces:**
- Consumes: v3 的 eval JSON、`data/graph.db`、`data/reflex.db`、`data/patchintel/rules.db`。
- Produces: 基线 JSON + 基线 runbook，供 T16 对照。

- [ ] **Step 1: 核对 v3 闸门**

Run: `grep -n "G0\|G1\|G2\|G3\|G4\|G5" docs/plans/v3_todo.md`
Expected: 六个闸门均有证据链接；任一缺失即停，记录阻塞。

- [ ] **Step 2: 跑 v3 全量相关测试**

Run: `CGO_ENABLED=1 go test ./graph/ ./graphserver/ ./dispatch/... ./eval/ ./decision/ ./evidence/ ./reflex/... ./safety/ ./fingerprint/ ./reflexopt/ -race -count=1`
Expected: 全部 `ok`。

- [ ] **Step 3: 导出基线**

把 eval JSON、节点/边计数、rules 行数、token 与 time-to-verdict 写入 runbook。

- [ ] **Step 4: 固定基线文件**

runbook 末尾写明“v4 对照基线 = <文件 hash>”；后续 T16 只对这份。

---

## 阶段 V4-1：契约与边界（G1）

### Task 1：OI 契约 v1 + FakeServer（V4-R2）

**Files:**
- Create: `internal/offensive/contract/contract.go`、`client.go`、`fake.go`、`contract_test.go`

**Interfaces:**
- Produces:
```go
const Version = "v1"
var ErrVersionMismatch = errors.New("contract: version mismatch")
var ErrEvidenceRequired = errors.New("contract: evidence required")
type Request struct { Version, Method, ProjectID string; Params json.RawMessage }
type Response struct { Version string; OK bool; Data json.RawMessage; Evidence []evidence.Record; Error *Error }
type ToolResult struct { Summary string; Data json.RawMessage; Evidence []evidence.Record }
type Client interface { /* 见 v4_re §4.1 */ }
func NewFakeServer() *FakeServer
```

- [ ] **Step 1: 写失败测试**

```go
func TestResponseRequiresEvidenceForExec(t *testing.T) {
    r := Response{Version: Version, OK: true} // method=sandbox.exec
    if err := r.Validate("sandbox.exec"); !errors.Is(err, ErrEvidenceRequired) { t.Fatal(err) }
}
func TestContractVersionRejected(t *testing.T) {
    _, err := DecodeResponse([]byte(`{"version":"v2","ok":true}`))
    if !errors.Is(err, ErrVersionMismatch) { t.Fatal(err) }
}
```
另：`TestFakeServerRoundTrip`、`TestUnknownMethodStructuredError`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/offensive/contract/ -v` → FAIL（包不存在）。

- [ ] **Step 3: 实现**

`Response.Validate(method)`：执行类方法无 evidence → `ErrEvidenceRequired`；版本 ≠ v1 → `ErrVersionMismatch`。`FakeServer` 用 map 注册 handler，便于离线测试。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/offensive/contract/ -race` → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`feat(contract): add offensive infra contract v1 with fake server`

---

### Task 2：依赖边界断言（V4-R1）

**Files:**
- Create: `internal/archtest/imports_test.go`

**Interfaces:**
- Consumes: `go list -deps` 输出。
- Produces: 构建级红线，后续 Task 不得违反。

- [ ] **Step 1: 写失败测试**

```go
func TestOffensiveDoesNotImportAgent(t *testing.T) {
    deps := depsOf(t, "AIxVuln/internal/offensive/...")
    for _, d := range deps {
        if strings.Contains(d, "AIxVuln/internal/agent") { t.Fatalf("offensive imports %s", d) }
    }
}
```
另：`TestAgentOnlyImportsContract`（允许 `contract`、`evidence`，禁止 `sandbox/toolchain/intel`）、`TestNoWebFromPlanes`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/archtest/ -v` → FAIL（包不存在）。

- [ ] **Step 3: 实现**

用 `exec.Command("go","list","-deps","./internal/offensive/...")`；先建空包 `internal/offensive/contract`、`internal/agent` 让命令可跑。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/archtest/ -race` → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`test(arch): enforce offensive/agent import boundaries`

---

## 阶段 V4-2：OI 抽取（G2）

### Task 3：沙箱抽取（V4-R3）

**Files:**
- Create: `internal/offensive/sandbox/sandbox.go`、`runtime.go`、`sandbox_test.go`
- Modify: `dockerManager/` 旧实现改为薄适配（保留到 T15）

**Interfaces:**
- Consumes: Docker SDK、project 别名解析（`<labID>_binary_security` → `<labID>`）。
- Produces:
```go
type Runtime struct { Exec contract.ExecFunc }
func (r *Runtime) Create(ctx context.Context, projectID, image string) (string, error)
func (r *Runtime) Exec(ctx context.Context, projectID, command, workdir string, timeout time.Duration) (evidence.Record, error)
func (r *Runtime) Destroy(ctx context.Context, projectID string) error
```

- [ ] **Step 1: 写失败测试**

`TestExecReturnsEvidence`（退出码/stdout/stderr/sandbox id 齐全）、`TestTimeoutKills`、`TestProjectAliasResolves`、`TestCreateDestroyIdempotent`。

- [ ] **Step 2: 跑测试确认失败**

`go test ./internal/offensive/sandbox/ -v` → FAIL。

- [ ] **Step 3: 实现**

包装现有 Docker 调用；`Exec` 用 `evidence.Capture` 生成 Record；超时返回 `context.DeadlineExceeded` 且 Record 带 `Truncated=true`；别名解析复用 taskManager 的 accessor。

- [ ] **Step 4: 跑测试确认通过**

`go test ./internal/offensive/sandbox/ -race` → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`feat(oi): extract sandbox runtime with evidence`

---

### Task 4：Agentic Tool（V4-R3）

**Files:**
- Create: `internal/offensive/toolchain/registry.go`、`scan.go`、`intel_lookup.go`、`toolchain_test.go`

**Interfaces:**
- Consumes: `sandbox.Runtime`、`poc.Classify`。
- Produces:
```go
type Tool interface { Name() string; Schema() json.RawMessage; Run(ctx context.Context, projectID string, args map[string]any) (contract.ToolResult, error) }
func Register(t Tool)
func Call(ctx context.Context, projectID, name string, args map[string]any) (contract.ToolResult, error)
```

- [ ] **Step 1: 写失败测试**

`TestScanAggregatesSteps`（fake exec 返回三段输出，scan 只返回聚合摘要 + 证据）、`TestScanReturnsEvidenceIDsNotRaw`、`TestIntelLookupClassifies`、`TestUnknownToolRejected`。

- [ ] **Step 2: 跑测试确认失败**

`go test ./internal/offensive/toolchain/ -v` → FAIL。

- [ ] **Step 3: 实现**

`scan`：主机发现 → 端口 → 服务 → 聚合；内部每步经 sandbox，原始输出只存 evidence，Summary 只给结构化结果。`intel_lookup`：查询 intel，POC 带 `validation_only|exploit_capable|unknown`。

- [ ] **Step 4: 跑测试确认通过**

`go test ./internal/offensive/toolchain/ -race` → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`feat(oi): add agentic scan and intel lookup tools`

---

### Task 5：补丁情报抽取（V4-R4）

**Files:**
- Create: `internal/offensive/intel/profile.go`、`search.go`、`intel_test.go`
- Modify: `patchintel/` 旧包改为调用新包

**Interfaces:**
```go
type Fact struct { Type, Content, EvidenceHash string }
func Profile(ctx context.Context, diff string) ([]Fact, error)
func Search(ctx context.Context, query string) ([]Candidate, error)
type Candidate struct { Kind, ID, Summary string; Confidence float64; POCClass POCClass }
```

- [ ] **Step 1: 写失败测试**

`TestProfileFromRealDiff`（Langflow diff → ≥1 patch_profile）、`TestSearchNoHitReturnsEmpty`、`TestRulesBackflowIncrements`、`TestNoFabricatedCandidate`。

- [ ] **Step 2: 跑测试确认失败** → FAIL。

- [ ] **Step 3: 实现**

把 `patchintel` 的解析/规则读写移入；`Search` 无命中返回空切片而非编造；规则回流带 evidence hash。

- [ ] **Step 4: 跑测试确认通过** → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`feat(oi): extract patch intel with evidence-linked rules`

---

### Task 6：证据库（V4-R4）

**Files:**
- Create: `internal/offensive/evidence/store.go`、`index.go`、`store_test.go`
- Modify: `evidence/` 旧包改为类型别名（保留到 T15）

**Interfaces:**
```go
type Store interface { Put(ctx context.Context, rec Record) (hash string, err error); Get(ctx context.Context, hash string) (Record, error); ListByProject(ctx context.Context, projectID string) ([]Record, error) }
func NewStore(dbPath string) (Store, error)
```

- [ ] **Step 1: 写失败测试**

`TestPutGetRoundTrip`、`TestIndexByProject`、`TestRejectRecordWithoutSandboxID`、`TestContentAddressableHash`。

- [ ] **Step 2: 跑测试确认失败** → FAIL。

- [ ] **Step 3: 实现**

SQLite 表 `evidence_index` + 内容文件 `data/evidence/<hash>.json`；hash = sha256(canonical JSON)；落库前脱敏。

- [ ] **Step 4: 跑测试确认通过** → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`feat(oi): add content-addressed evidence store`

---

### Task 7：`pt-oi` 进程与 CLI（V4-R5）

**Files:**
- Create: `cmd/pt-oi/main.go`、`internal/offensive/server/server.go`、`server_test.go`、`internal/offensive/contract/stdio_client.go`

**Interfaces:**
- Consumes: Task 1 契约、Task 3–6 实现。
- Produces: `pt-oi serve --stdio`、`pt-oi tool call <name> --json`、`pt-oi exec --json`；`contract.NewStdioClient(cmd *exec.Cmd)`。

- [ ] **Step 1: 写失败测试**

`TestJSONRPCExecReturnsEvidence`、`TestContractVersionRejected`、`TestApprovalRequiredForbidden`（`rm -rf /` 默认拒绝）、`TestCLIJSONOnly`（stdout 只有一行 JSON）。

- [ ] **Step 2: 跑测试确认失败** → FAIL。

- [ ] **Step 3: 实现**

JSON-RPC 2.0 over stdio；方法表见 `v4_re §4.1`；危险命令先过 guardrail；日志走 stderr，stdout 只输出协议 JSON。

- [ ] **Step 4: 跑测试确认通过** → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`feat(oi): add standalone pt-oi JSON-RPC server and CLI`

---

## 阶段 V4-3：AI 重组（G3）

### Task 8：Harness H1 与黑板（V4-R6）

**Files:**
- Create: `internal/agent/harness/harness.go`、`react.go`、`harness_test.go`
- Create: `internal/agent/blackboard/board.go`、`graph_adapter.go`、`board_test.go`

**Interfaces:**
- Consumes: `offensive/contract.Client`、`decision.Decider`、`reflex.Runtime`。
- Produces: `Harness`/`Observation`/`Outcome`（见 `v4_re §4.3`）；`Board`（§4.4）。

- [ ] **Step 1: 写失败测试**

`TestStepObserveReasonDecide`、`TestReflexShortCircuit`、`TestBindingDeferFallsBack`、`TestCompletionGatesWrite`、`TestBoardClaimTTL`、`TestBoardCompleteRequiresEvidence`。

- [ ] **Step 2: 跑测试确认失败** → FAIL。

- [ ] **Step 3: 实现**

`react.go`：读黑板 → Reason（LLM，可 fake）→ Decision(binding) → Reflex 或 `client.ToolCall` → Decision(completion) → 返回 Outcome；写图只经 runner/Board.Complete，且要求至少一条 evidence。

- [ ] **Step 4: 跑测试确认通过** → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`feat(agent): add H1 harness and blackboard adapter`

---

### Task 9：MTP（V4-R7）

**Files:**
- Create: `internal/agent/mtp/catalog.go`、`search.go`、`catalog_test.go`

**Interfaces:**
```go
func NewCatalog(entries []Entry) *Catalog
func (c *Catalog) Search(ctx context.Context, query string, k int) ([]Entry, error)
func (c *Catalog) Fetch(ctx context.Context, id string) (json.RawMessage, error)
func (c *Catalog) TrimForBudget(entries []Entry, budget int) []Entry
```

- [ ] **Step 1: 写失败测试**

`TestSearchRanksStable`、`TestFetchSummaryOnly`、`TestBudgetTrims`、`TestUnknownID`。

- [ ] **Step 2: 跑测试确认失败** → FAIL。

- [ ] **Step 3: 实现**

目录项来自 OI 工具 schema 与数据集清单；`Fetch` 只返回摘要与访问方式，不返回原始大输出；预算按 `Cost` 降序裁剪。

- [ ] **Step 4: 跑测试确认通过** → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`feat(agent): add MTP catalog with token budget`

---

## 阶段 V4-4：检索与交互（G4）

### Task 10：GraphRAG 邻域（V4-R8）

**Files:**
- Create: `internal/agent/blackboard/neighborhood.go`、`neighborhood_test.go`

**Interfaces:**
```go
type Subgraph struct { Nodes []Fact; Edges []Edge }
func (b *GraphBoard) Neighborhood(ctx context.Context, projectID, intentID string, depth int) (Subgraph, error)
```

- [ ] **Step 1: 写失败测试**

`TestNeighborhoodDepth`、`TestNeighborhoodIntentFilter`、`TestNoVectorStoreDependency`（`go list -deps` 无 embedding 包）。

- [ ] **Step 2: 跑测试确认失败** → FAIL。

- [ ] **Step 3: 实现**

从 graph server 读 intent 邻域，按 kind/type 过滤，深度上限 3；不建向量索引。

- [ ] **Step 4: 跑测试确认通过** → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`feat(blackboard): add graph neighborhood retrieval`

---

### Task 11：HCI 决策式交互（V4-R9）

**Files:**
- Create: `internal/hci/approval.go`、`hint.go`、`pause.go`、`hci_test.go`
- Modify: `Web/Server.go`（路由接新 handler；不改 URL）

**Interfaces:**
```go
type Approver interface { Request(ctx context.Context, a Approval) (Approval, error); Decide(ctx context.Context, id, actor, note, status string) error }
type Controller interface { Pause(ctx context.Context, projectID string) error; Resume(ctx context.Context, projectID string) error; Paused(ctx context.Context, projectID string) bool }
type Hints interface { Inject(ctx context.Context, projectID, content, actor string) (string, error) }
```

- [ ] **Step 1: 写失败测试**

`TestForbiddenBlocked`、`TestOverrideAudited`、`TestHintPrecedence`、`TestPauseStopsStep`、`TestApprovalTimeoutDoesNotPass`。

- [ ] **Step 2: 跑测试确认失败** → FAIL。

- [ ] **Step 3: 实现**

审批落 `approvals` 表；Hint 写黑板 human_intent；Pause 由 Harness 每 Step 检查；所有人类动作写 `HumanAction`。

- [ ] **Step 4: 跑测试确认通过** → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`feat(hci): add approval, hint and pause controls`

---

## 阶段 V4-5：H2 与 IoA（G5/G6）

### Task 12：Cybernetics Harness（V4-R10）

**Files:**
- Create: `internal/agent/harness/cybernetics.go`、`prompts/harness/explorer.md`、`execute.md`、`plan.md`、`cybernetics_test.go`

**Interfaces:**
- Consumes: 同一 `Harness` 接口、同一 `Board`、同一 `contract.Client`。
- Produces: `NewCybernetics(cfg) Harness`；`misc.harness_mode=react|cybernetics`。

- [ ] **Step 1: 写失败测试**

`TestExplorerToExecute`、`TestPlanRequestsHuman`、`TestOverlayApplied`、`TestSwapParity`。

- [ ] **Step 2: 跑测试确认失败** → FAIL。

- [ ] **Step 3: 实现**

状态机：`explorer → execute`，需要人类时 `explorer → plan → execute`；node prompt 定目的论，task.md 定方法论，overlay 叠加；流程分支在代码，prompt 只写语义。

- [ ] **Step 4: 跑测试确认通过** → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`feat(agent): add Cybernetics harness with intent state machine`

---

### Task 13：IoA 通讯（V4-R11）

**Files:**
- Create: `internal/agent/ioa/bus.go`、`cli.go`、`bus_test.go`

**Interfaces:**
```go
func NewBus(dbPath string) (Bus, error)
func SendCLI(ctx context.Context, bus Bus, m Message) error
func ReadCLI(ctx context.Context, bus Bus, agentID string) ([]Message, error)
```

- [ ] **Step 1: 写失败测试**

`TestSendReadLink`、`TestThreadDAG`、`TestTwoAgentDebate`（两个 fake agent 通过 bus 完成一次计划）。

- [ ] **Step 2: 跑测试确认失败** → FAIL。

- [ ] **Step 3: 实现**

SQLite 消息表；`Refs`/`ReplyTo` 支持 thread/tree/DAG；CLI 输出 JSON；不实现调度器。

- [ ] **Step 4: 跑测试确认通过** → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`feat(ioa): add message bus and CLI for agent coordination`

---

## 阶段 V4-6：端侧与收口（G7）

### Task 14：端侧 Agent 原型（V4-R12，可选闸门）

**Files:**
- Create: `cmd/aiagent/main.go`、`tools.go`、`agent_test.go`

**Interfaces:**
- Consumes: `contract.Client`（OI）、`ioa.Bus`（可选）。
- Produces: 单文件客户端，仅 `read`/`write`/`glob`/`bash` 四工具。

- [ ] **Step 1: 写失败测试**

`TestFourToolsOnly`、`TestNoScheduler`（无 dispatch/tick 依赖）、`TestOIClientBound`（非授权 project 拒绝）。

- [ ] **Step 2: 跑测试确认失败** → FAIL。

- [ ] **Step 3: 实现**

四工具 + OI 客户端；无 MCP、无 task、无调度器；默认只读授权工作区。

- [ ] **Step 4: 跑测试确认通过** → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`feat(aiagent): add single-binary endpoint agent prototype`

---

### Task 15：旧路径收口（V4-R13）

**Files:**
- Modify: `dispatch/lane.go`、`Web/Server.go`、`misc/ConfigDB.go`
- Create: `internal/archtest/legacy_test.go`

**Interfaces:**
- Consumes: 全部新平面。
- Produces: `misc.plane_mode=legacy|split`；`split` 为默认仅在 T16 真实验收通过后。

- [ ] **Step 1: 写失败测试**

`TestLegacyPathOffByDefault`、`TestFlagMatrix`（legacy/split × harness_mode）、`TestNoDoubleWrite`、`TestRollbackToLegacy`。

- [ ] **Step 2: 跑测试确认失败** → FAIL。

- [ ] **Step 3: 实现**

旧 `LabManager`/`agents` 路径在 `split` 下不启动；图写者唯一；回退只改 flag；删除前置条件写进 `v4_todo`。

- [ ] **Step 4: 跑测试确认通过** → PASS。

- [ ] **Step 5: 提交（待用户要求）**

`refactor: switch default path to split planes behind flag`

---

### Task 16：指标与真实验收（V4-R14/R15）

**Files:**
- Create: `internal/eval/offensive_metrics.go`、`offensive_metrics_test.go`
- Create: `docs/runbooks/cve-2025-3248-v4-run-2026-11-20.md`

**Interfaces:**
```go
type V4Metrics struct {
    CrossPlaneImports int     `json:"cross_plane_imports"`
    OIStandalone      bool    `json:"oi_standalone"`
    HarnessSwapParity float64 `json:"harness_swap_parity"`
    TokensPerFinding  int64   `json:"tokens_per_verified_finding"`
    TimeToFindingMS   int64   `json:"time_to_verified_finding_ms"`
    ToolOnboardingDays float64 `json:"tool_onboarding_days"`
    SafetyIncidents   int     `json:"safety_incidents"`
}
```

- [ ] **Step 1: 写失败测试**

`TestMetricsFromRuns`、`TestHarnessSwapParity`、`TestEconomicsJSON`、`TestBaselineComparison`。

- [ ] **Step 2: 跑测试确认失败** → FAIL。

- [ ] **Step 3: 实现指标与对照**

读 T0 基线 JSON 与本次运行记录；输出 JSON；成本含决策/生成/优化 token。

- [ ] **Step 4: 真实运行（环境允许时）**

`pt-oi` + H1 跑 CVE-2025-3248；H1/H2 各一次；记录命令与原始输出。环境不允许则记录阻塞，不标完成。

- [ ] **Step 5: 提交（待用户要求）**

`feat(eval): add v4 orthogonal metrics and real-run record`

---

## Self-Review

**1. Spec coverage:** V4-R1→T2；R2→T1；R3→T3/T4；R4→T5/T6；R5→T7；R6→T8；R7→T9；R8→T10；R9→T11；R10→T12；R11→T13；R12→T14；R13→T15；R14→T16；R15→T0/T16；R16→T1/T3/T7/T11。无遗漏。

**2. Step scan:** 每 Task 五步（红、验红、实现、验绿、提交）；测试名与关键断言已给出，无 TBD。

**3. Type consistency:** `contract.Client` 在 T1 定义并贯穿 T3–T14；`evidence.Record` 在 T6 定稿、T3/T8 复用；`Harness`/`Board` 在 T8 定义，T12 复用；`V4Metrics` 只在 T16。

**4. Review Focus:** 五条风险各有测试：契约漂移 T7、证据丢失 T1/T3、审批绕过 T7/T11、迁移双写 T15、切换漂移 T12。

**5. Proportion:** 计划以接口签名、测试名、验收命令为主；只为证据/版本/边界三处给出实现细节，因为这三处决定正确性。

## Execution Handoff

- 启动前提：`v3_todo.md` 的 G0–G5 全部有证据；否则本计划保持 `未开始`。
- 先做 T0，再做 T1–T2；T3–T7 可并行；T8 起依赖 OI 契约。
- 每个 Task 完成即回写 `v4_todo.md`，附命令与原始输出。
