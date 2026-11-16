# PatchTrace 迭代实施计划（task.md）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 AIxVuln 仓库内新增一条类 Cairn 的图驱动后端（溯源图服务 + Dispatcher + Worker driver），把补丁反演、沙箱实证、证据判决、规则回流全部落成溯源图上的节点与边，旧车道保留为兼容路径。

**Architecture:** 单二进制内四层：`graphserver`（SQLite 溯源图协议真相源）←HTTP 回环→ `dispatch`（唯一协议写入者，三类任务 bootstrap/reason/explore）→ `dispatch/worker`（native driver 复用 `llm/` + `toolCalling/`）→ 沙箱复用 `dockerManager/`。补丁反演复用既有 `patchintel/`。

**Tech Stack:** Go 1.24 · gin v1.11 · modernc.org/sqlite（纯 Go）· goccy/go-yaml（间接依赖，本计划提升为直接依赖）· 既有 `llm/`、`toolCalling/`、`dockerManager/`、`misc/`。

**Spec:** `docs/plans/requirement.md`（需求编号 R1–R16；本计划的每个 Task 在标题后标注对应需求）

**输入材料:** `docs/claude/` 四份材料 + `Cairn/docs/specs/server-protocol.md`、`Cairn/docs/specs/dispatcher-design.md`

---

## Global Constraints

- 不推倒重来：42k 行存量，每次改动可单独验收、单独回滚。
- 单二进制 + SQLite：不引入新存储引擎；图库与规则库都用 `modernc.org/sqlite`。
- 工厂尾部 append：不动 `agents/agent_builder.go` 的 `factories[:7]` / `factories[:12]` 下标块。
- 分层不倒置：`graph/`、`graphserver/`、`dispatch/` 只能向下依赖 `patchintel/`、`llm/`、`misc/`、`dockerManager/`；不得 import `Web/`。
- 判定纪律：拒绝文本不算攻击成功；网络错误、证据不足一律 `error` / `inconclusive`，不得报 clean。
- 模型不可见：状态、进度、预算由基础设施承担；prompt 里不写流程分支。
- 中文界面与提示词，英文代码标识。
- 任何“完成”必须附命令与输出；不得改验收标准来通过。

## Review Focus

1. 两个 Worker 同时认领同一 Intent → 恰好一个成功，另一个拿到 409，不得双写。
2. 写入 `block` 边 A→B→C→A → 拒绝并返回错误，图中不得出现环。
3. Worker 挂掉、lease 过期 → Intent 回到 open，可被重新认领，不得永久卡死。
4. `complete` 时 Finding 没有 `origin → … → finding` 的推导路径 → 拒绝完成。
5. LLM 返回拒绝文本、非法 JSON、缺字段 → 任务作废且不写图，不得落 Fact、不得报成功。

---

## 迭代执行提示词（每个 Task 开工时复制）

```text
按 docs/plans/task.md 执行 Task N。
1. 只读本 Task 的 Files / Interfaces / Steps，以及 requirement.md 对应 R*。
2. 不知道的接口先问，不许猜；不改其他 Task 的文件。
3. TDD：先写失败测试 → 跑失败 → 最小实现 → 跑通过 → 提交。
4. 完成后回写 docs/plans/todo.md，贴命令与真实输出。
5. 若发现计划与现实不符，停下并在 todo.md 记阻塞，不改验收标准。
```

---

## 文件结构（本迭代新增，锁定边界）

```
graph/                    # 溯源图模型、存储、查询、导出（唯一图状态实现）
  model.go                # Node / Edge / Project / 错误哨兵
  store.go                # SQLite 落库、CRUD、claim/lease、conclude/complete/reopen
  migrate.go              # schema 版本化迁移
  query.go                # Provenance / Gaps / 路径排序
  export.go               # YAML 快照 + timeline
  *_test.go

graphserver/              # 图协议 HTTP 层（Cairn server 的 Go 版）
  server.go               # gin 路由注册与 handler
  client.go               # Dispatcher 使用的 Client 接口 + HTTP 实现
  local_client.go         # 测试用内存实现（直接调 graph.Store）
  *_test.go

dispatch/                 # 调度核心（Cairn dispatcher 的 Go 版）
  config.go               # dispatch.yaml 结构与校验
  dispatcher.go           # Run/Tick 主循环
  planner.go              # 任务选择（bootstrap / reason / explore）
  worker_select.go        # Worker 过滤与选择
  heartbeat.go            # 租约续约与超时
  tasks/
    common.go             # 任务上下文、JSON 契约解析
    bootstrap.go          # 补丁反演
    reason.go             # 溯源图推理
    explore.go            # 沙箱实证
    outcome.go            # CE4 终态回流
  runtime/
    sandbox.go            # SandboxRuntime 接口 + dockerManager 适配
  worker/
    driver.go             # Driver 接口与 Result 结构
    registry.go           # driver 注册与健康窗口
    native.go             # 复用 llm/ + toolCalling/
  *_test.go

prompts/graph/default/    # 任务提示词（数据，不是代码）
  bootstrap.md  bootstrap_conclude.md
  reason.md     explore.md  explore_conclude.md

eval/                     # patcheval 对照评测
  patcheval.go
  patcheval_test.go

docs/runbooks/
  cve-2025-3248.md        # 端到端演示与验收手册
```

---

### Task 1: 迭代地基与两个 Skill（R1, R2）

**Files:**
- Create: `.gitignore`
- Create: `.claude/skills/cairn-architecture/SKILL.md`
- Create: `.claude/skills/cairn-architecture/references/server-protocol.md`
- Create: `.claude/skills/cairn-architecture/references/dispatcher-design.md`
- Create: `.claude/skills/cairn-architecture/references/data-model.md`
- Create: `.claude/skills/patchtrace-iteration/SKILL.md`
- Create: `.claude/skills/patchtrace-iteration/references/workflow.md`

**Interfaces:**
- Consumes: 无
- Produces: 两个可被新会话加载的 Skill；Git 基线。后续所有 Task 用 `git commit` 收尾。

- [ ] **Step 1: 初始化 Git 基线**

```bash
cd /root/PatchTrace
printf 'data/\ndist/\n__MACOSX/\n*.log\nCairn/datas/\n' > .gitignore
git init
git add -A
git commit -m "chore: baseline AIxVuln/PatchTrace tree (build green)"
```

Expected: `git status` 输出 `nothing to commit, working tree clean`。

- [ ] **Step 2: 写 `cairn-architecture` Skill**

`.claude/skills/cairn-architecture/SKILL.md` 的 frontmatter：

```markdown
---
name: cairn-architecture
description: Use when implementing or reviewing PatchTrace's Cairn-like backend (graph server, dispatcher, leases, bootstrap/reason/explore tasks).
---
```

正文必须覆盖：四层组件（Server/Dispatcher/Container/Worker）、三个原语（Fact/Intent/Hint）、三类任务与输出契约、Intent 生命周期与 lease、Dispatcher 是唯一协议写入者、`export?format=yaml|timeline`。正文 ≤120 行，细节放 references：

- `references/server-protocol.md`：摘录 `Cairn/docs/specs/server-protocol.md` 的数据结构、接口列表、状态码语义；
- `references/dispatcher-design.md`：摘录任务触发条件、Worker 选择顺序、超时与 conclude fallback；
- `references/data-model.md`：Fact/Intent/Hint/Project 字段表 + PatchTrace 的 Fact 类型扩展。

- [ ] **Step 3: 写 `patchtrace-iteration` Skill**

`.claude/skills/patchtrace-iteration/SKILL.md` 的 frontmatter：

```markdown
---
name: patchtrace-iteration
description: Use when starting or resuming the PatchTrace iteration; routes to requirement.md, task.md, todo.md and enforces evidence-based progress.
---
```

`references/workflow.md` 写明：读三文档的顺序、TDD 步骤、todo 回写格式、验收证据要求、阻塞上报格式。

- [ ] **Step 4: 验证 Skill 可被读取**

Run: `grep -c '^---' .claude/skills/*/SKILL.md`

Expected: 每个文件输出 `2`（frontmatter 起止）。

- [ ] **Step 5: 提交**

```bash
git add .claude .gitignore
git commit -m "feat: add cairn-architecture and patchtrace-iteration skills"
```

---

### Task 2: 溯源图模型与存储（R3）

**Files:**
- Create: `graph/model.go`
- Create: `graph/migrate.go`
- Create: `graph/store.go`
- Test: `graph/store_test.go`

**Interfaces:**
- Consumes: `modernc.org/sqlite`
- Produces:
  - `type Store struct{}`；`func Open(path string) (*Store, error)`；`func (s *Store) Close() error`
  - `type Project struct { ID, Title, Origin, Goal, Status string; BootstrapEnabled bool; CreatedAt time.Time; Reason *ReasonLease }`
  - `type Node struct { ID, ProjectID, Kind, Type, Content, State string; From []string; To *string; Creator, Worker string; LeaseUntil, CreatedAt, ConcludedAt *time.Time }`
  - `type Edge struct { ProjectID, From, To, Kind string }`
  - 错误哨兵：`ErrNotFound`、`ErrConflict`、`ErrForbidden`、`ErrInvalid`、`ErrCycle`
  - 方法：`CreateProject`、`GetProject`、`ListProjects`、`SetProjectStatus`、`AddHint`、`DeclareIntent`、`ClaimIntent`、`HeartbeatIntent`、`ReleaseIntent`、`ConcludeIntent`、`ClaimReason`、`HeartbeatReason`、`ReleaseReason`、`Complete`、`Reopen`、`AddEdge`

- [ ] **Step 1: 写失败测试（生命周期与原子性）**

```go
func TestClaimIntentIsAtomic(t *testing.T) {
    s := newTestStore(t)
    p := mustProject(t, s)          // origin/goal 已写入
    it := mustIntent(t, s, p.ID)    // state=open, worker=nil
    var ok int32
    var wg sync.WaitGroup
    for i := 0; i < 8; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            if _, err := s.ClaimIntent(ctx, p.ID, it.ID, fmt.Sprintf("w%d", i), time.Minute); err == nil {
                atomic.AddInt32(&ok, 1)
            }
        }(i)
    }
    wg.Wait()
    if ok != 1 { t.Fatalf("want exactly 1 claim, got %d", ok) }
}

func TestLeaseExpiryReopensIntent(t *testing.T) { /* claim with -1m lease, read, expect state=open */ }
func TestBlockEdgeCycleRejected(t *testing.T)   { /* A→B→C then C→A expect ErrCycle */ }
func TestConcludeCreatesFactAndProduceEdge(t *testing.T) { /* expect to!=nil, node kind=fact, edge kind=produce */ }
func TestCompleteRejectsUnknownFact(t *testing.T)        { /* expect ErrInvalid */ }
func TestReopenCreatesCorrectionFact(t *testing.T)       { /* expect status=active, external_feedback intent */ }
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./graph/ -run 'TestClaimIntentIsAtomic|TestLeaseExpiry' -v`
Expected: FAIL，编译错误 `undefined: Open`。

- [ ] **Step 3: 实现 schema 与存储**

`graph/migrate.go` 建表（`projects`、`nodes`、`edges`、`counters`），开启 `PRAGMA journal_mode=WAL`、`PRAGMA busy_timeout=5000`，`db.SetMaxOpenConns(1)`。

原子认领用单条 UPDATE：

```sql
UPDATE nodes SET worker=?, state='claimed', lease_until=?
WHERE id=? AND project_id=? AND kind='intent' AND state='open'
  AND (worker IS NULL OR worker='' OR lease_until < ?)
```

`RowsAffected()==0` → `ErrConflict`。所有读方法先执行 lease 清理（`state='claimed' AND lease_until < now` → `worker=NULL, state='open'`）。

`AddEdge` 对 `kind='block'` 做 DFS 环检测；成环返回 `ErrCycle`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./graph/ -race -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add graph/
git commit -m "feat(graph): add provenance graph model, sqlite store, atomic claim"
```

---

### Task 3: 溯源图查询与导出（R8 支撑）

**Files:**
- Create: `graph/query.go`
- Create: `graph/export.go`
- Test: `graph/query_test.go`

**Interfaces:**
- Consumes: Task 2 的 `Store`、`Node`、`Edge`
- Produces:
  - `func (s *Store) Provenance(ctx context.Context, projectID, nodeID string) ([]Node, error)`
  - `func (s *Store) Gaps(ctx context.Context, projectID string) ([]Node, error)`（按“未满足 block 依赖数升序、created_at 升序”排序）
  - `func (s *Store) ExportYAML(ctx context.Context, projectID string) (string, error)`
  - `func (s *Store) ExportTimeline(ctx context.Context, projectID string) (string, error)`

- [ ] **Step 1: 写失败测试**

```go
func TestProvenanceReturnsPatchToFindingPath(t *testing.T) {
    // origin → f001(patch_profile) → f002(exploit_idea) → f003(verdict)
    got, err := s.Provenance(ctx, p.ID, "f003")
    // expect ids == [origin f001 f002 f003]
}

func TestGapsExcludesBlockedIntents(t *testing.T) { /* i002 blocked-by i001(open) 不出现 */ }
func TestExportYAMLMatchesProtocolShape(t *testing.T) { /* 含 project/facts/intents/hints 四段，不含 reason */ }
func TestTimelineIsChronological(t *testing.T) { /* PROJECT CREATED < HINT < INTENT DECLARED < INTENT CONCLUDED */ }
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./graph/ -run 'TestProvenance|TestGaps' -v`
Expected: FAIL，`undefined: Provenance`。

- [ ] **Step 3: 实现查询与导出**

`Provenance`：沿 `produce`、`derive`、`support` 边从目标节点反向 DFS 到 `origin`，返回去环后的有序路径；无路径返回 `ErrNotFound`。
`Gaps`：取 `kind=intent AND state=open` 的节点，过滤仍有 open blocker 的项，排序返回。
`ExportYAML`：字段顺序固定为 `project / hints / facts / intents`，时间格式 `YYYY-MM-DD HH:mm:ss`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./graph/ -race -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add graph/
git commit -m "feat(graph): add provenance query, gap ranking and protocol export"
```

---

### Task 4: 图协议 API（R4）

**Files:**
- Create: `graphserver/server.go`
- Create: `graphserver/client.go`
- Create: `graphserver/local_client.go`
- Test: `graphserver/server_test.go`

**Interfaces:**
- Consumes: `graph.Store`（Task 2/3）
- Produces:
  - `type Client interface { CreateProject(...); GetProject(...); DeclareIntent(...); ClaimIntent(...); HeartbeatIntent(...); ReleaseIntent(...); ConcludeIntent(...); Complete(...); ClaimReason(...); HeartbeatReason(...); ReleaseReason(...); AddHint(...); ExportYAML(...) }`
  - `func New(store *graph.Store) *Server`
  - `func (s *Server) Register(r gin.IRouter)`——挂在 `/api/graph`
  - `func NewHTTPClient(baseURL, token string) Client`
  - `func NewLocalClient(store *graph.Store) Client`

- [ ] **Step 1: 写失败测试（生命周期 + 状态码）**

```go
func TestRoutesLifecycle(t *testing.T) {
    // POST /api/graph/projects → 201
    // POST /api/graph/projects/:id/intents → 201
    // POST .../intents/:iid/heartbeat → 200
    // POST .../intents/:iid/conclude → 200（返回 fact + intent）
    // POST .../complete → 200；GET .../export?format=yaml → 200
}

func TestStoppedProjectRejectsWrites(t *testing.T) { /* PUT status=stopped 后 POST intent 期望 403 */ }
func TestConflictStatusCodes(t *testing.T)        { /* 他人持有的 intent 被 claim 期望 409 */ }
func TestInvalidBodyReturns422(t *testing.T)      { /* description 空白期望 422 */ }
func TestProvenanceEndpoint(t *testing.T)         { /* GET .../provenance?node=f003 → 200 数组 */ }
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./graphserver/ -run TestRoutesLifecycle -v`
Expected: FAIL，`undefined: New`。

- [ ] **Step 3: 实现路由与错误映射**

端点：`POST/GET /projects`、`GET /projects/:id`、`PUT /projects/:id/status`、`POST /projects/:id/reopen`、`POST /projects/:id/hints`、`POST /projects/:id/intents`、`POST .../intents/:iid/{heartbeat,release,conclude}`、`POST /projects/:id/reason/{claim,heartbeat,release}`、`POST /projects/:id/complete`、`GET /projects/:id/export?format=yaml|timeline`、`GET /projects/:id/provenance?node=`。

错误映射：`ErrNotFound→404`、`ErrConflict→409`、`ErrForbidden→403`、`ErrInvalid→422`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./graphserver/ -race -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add graphserver/
git commit -m "feat(graphserver): add cairn-style graph protocol api"
```

---

### Task 5: 调度核心（R5）

**Files:**
- Create: `dispatch/config.go`
- Create: `dispatch/dispatcher.go`
- Create: `dispatch/planner.go`
- Create: `dispatch/worker_select.go`
- Test: `dispatch/planner_test.go`
- Test: `dispatch/config_test.go`

**Interfaces:**
- Consumes: `graphserver.Client`（Task 4）、`worker.Driver`（Task 6 定义，本 Task 先用最小接口）
- Produces:
  - `type Config struct { ServerURL string; MaxWorkers, MaxRunningProjects, MaxProjectWorkers int; Interval, HealthcheckTimeout time.Duration; BootstrapTimeout, BootstrapConcludeTimeout, ReasonTimeout, ExploreTimeout, ExploreConcludeTimeout time.Duration; PromptGroup string }`
  - `func LoadConfig(path string) (Config, error)`（必填项缺失返回错误）
  - `type Decision struct { ProjectID string; TaskType worker.TaskType; IntentID, WorkerName, Trigger string }`
  - `func New(cfg Config, client graphserver.Client, drivers []worker.Driver) *Dispatcher`
  - `func (d *Dispatcher) PlanProject(ctx context.Context, p graph.Project) (Decision, bool, error)`
  - `func (d *Dispatcher) Run(ctx context.Context) error`

- [ ] **Step 1: 写失败测试（决策规则）**

```go
func TestPlanPrefersBootstrapOnInitialProject(t *testing.T) { /* facts 仅 origin/goal → TaskType=bootstrap */ }
func TestPlanReasonOnNewFacts(t *testing.T)                 { /* fact 数增加且无 open intent → reason */ }
func TestPlanExploreWhenOpenIntentExists(t *testing.T)      { /* 有 open intent → explore 且带 IntentID */ }
func TestReasonOnlyOncePerProject(t *testing.T)             { /* 已持有 reason lease → 不再派发 reason */ }
func TestConfigRequiresTimeouts(t *testing.T)               { /* 缺 explore.timeout → error */ }
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./dispatch/ -run 'TestPlan|TestConfig' -v`
Expected: FAIL，`undefined: Config`。

- [ ] **Step 3: 实现配置与调度规则**

调度顺序（单项目）：初始态 bootstrap（`bootstrap_enabled` 且无保留 intent）→ 新态势 reason → open intent explore → 无动作。

“新态势”定义：Fact 数增加、Hint 数增加、或从“有 open intent”变为“无 open intent”。Dispatcher 为每个项目保存 `lastReason{factCount,hintCount,hadOpen}`。

Worker 选择顺序：任务类型 → `max_running` → 健康冷却窗口 → `priority` 升序 → 运行数升序 → 随机。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./dispatch/ -race -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add dispatch/
git commit -m "feat(dispatch): add config and scheduler decision rules"
```

---

### Task 6: Worker driver 与 native 实现（R6）

**Files:**
- Create: `dispatch/worker/driver.go`
- Create: `dispatch/worker/registry.go`
- Create: `dispatch/worker/native.go`
- Create: `prompts/graph/default/bootstrap.md`
- Create: `prompts/graph/default/bootstrap_conclude.md`
- Create: `prompts/graph/default/reason.md`
- Create: `prompts/graph/default/explore.md`
- Create: `prompts/graph/default/explore_conclude.md`
- Test: `dispatch/worker/native_test.go`

**Interfaces:**
- Consumes: `llm` 客户端工厂；`prompts/graph/default/*.md`
- Produces:
  - `type TaskType string`（`TaskBootstrap`、`TaskReason`、`TaskExplore`）
  - `type Task struct { ProjectID, IntentID, Type string; Prompt string; Timeout, ConcludeTimeout time.Duration }`
  - `type Result struct { Accepted bool; Fact, Complete, Intent *string; Description string; Intents []IntentSpec; Raw string }`
  - `type IntentSpec struct { From []string; Description string }`
  - `type Driver interface { Name() string; Supports(TaskType) bool; HealthCheck(ctx context.Context) error; Execute(ctx context.Context, t Task) (Result, error); Conclude(ctx context.Context, t Task) (Result, error); SupportsConclude() bool }`
  - `func NewNative(cfg NativeConfig) Driver`
  - `func ParseResult(raw string, tt TaskType) (Result, error)`

- [ ] **Step 1: 写失败测试（契约解析）**

```go
func TestParseResultAcceptsExploreFact(t *testing.T) {
    r, err := ParseResult(`{"accepted":true,"data":{"description":"/api/v1/validate/code 存在未授权代码执行"}}`, TaskExplore)
    // expect r.Accepted, r.Description 非空
}
func TestParseResultRejectsRefusal(t *testing.T)      { /* accepted:false → error 或 Accepted=false, 不写图 */ }
func TestParseResultRejectsMissingDescription(t *testing.T) { /* accepted:true 但缺 data → error */ }
func TestParseResultAcceptsBootstrapIntents(t *testing.T)  { /* data.intents[{from,description}] 解析 */ }
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./dispatch/worker/ -run TestParseResult -v`
Expected: FAIL，`undefined: ParseResult`。

- [ ] **Step 3: 实现 driver 与 prompt 加载**

`ParseResult` 对 `stdout` 全文做 JSON 解析，剥离 ```json 代码围栏；按任务类型校验必备字段。`accepted:false` 视为任务失败，不返回可写结果。

`native` driver 的 `Execute`：`llm.GetResponsesClient(section, "main_setting")` 获取客户端，加载 prompt，注入 `{graph_yaml}` / `{fact_ids}` / `{open_intents}` / `{intent_id}` / `{intent_description}` 占位符，返回原文交给 `ParseResult`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./dispatch/worker/ -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add dispatch/worker prompts/graph
git commit -m "feat(dispatch): add worker driver interface and native driver"
```

---

### Task 7: bootstrap 补丁反演任务（R7）

**Files:**
- Create: `dispatch/tasks/common.go`
- Create: `dispatch/tasks/bootstrap.go`
- Test: `dispatch/tasks/bootstrap_test.go`

**Interfaces:**
- Consumes: `patchintel.ParseDiff/IngestLocal/IngestURL/Invert/BuildIdeaFromResponse/RuleBase`、`graphserver.Client`
- Produces:
  - `func RunBootstrap(ctx context.Context, c graphserver.Client, d worker.Driver, p graph.Project) error`
  - `func bootstrapPrompt(p graph.Project, profile *patchintel.PatchProfile, rules []patchintel.Rule, template string) string`

- [ ] **Step 1: 写失败测试**

```go
func TestBootstrapConcludesPatchProfileFact(t *testing.T) {
    // fake driver 返回 fact description；断言 client 收到一条 conclusion，且 fact.type=patch_profile
}
func TestBootstrapDeclaresInversionIntents(t *testing.T) {
    // driver 返回 2 个 intents；断言 POST /intents 两次，from 均为 patch_profile fact id
}
func TestBootstrapSkipsWhenNoPatchSource(t *testing.T) {
    // origin 不是 diff 路径/URL → 返回明确 error，不写图
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./dispatch/tasks/ -run TestBootstrap -v`
Expected: FAIL，`undefined: RunBootstrap`。

- [ ] **Step 3: 实现 bootstrap**

流程：保留 intent（`from=["origin"]`，`description="bootstrap"`，`creator="dispatcher.bootstrap"`）→ claim → 构建补丁档案（本地路径优先，URL 兜底）→ 取 top-k 规则 + 场景模板 → 渲染 prompt → driver 执行 → `conclude` 写 `patch_profile` Fact → 为每条 `data.intents` 调 `DeclareIntent` → release（若失败）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./dispatch/tasks/ -race -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add dispatch/tasks
git commit -m "feat(dispatch): add patch inversion bootstrap task"
```

---

### Task 8: reason 溯源图推理任务（R8, R12）

**Files:**
- Create: `dispatch/tasks/reason.go`
- Modify: `graph/query.go`（增加 `GapScore`）
- Test: `dispatch/tasks/reason_test.go`

**Interfaces:**
- Consumes: `graph.Store.Gaps/Provenance`、`graphserver.Client`、`worker.Driver`
- Produces:
  - `func RunReason(ctx context.Context, c graphserver.Client, d worker.Driver, p graph.Project) error`
  - `func reasonPrompt(p graph.Project, graphYAML string, factIDs, openIntentIDs, gaps, hints []string) string`
  - `func (s *Store) GapScore(ctx context.Context, projectID string) ([]Gap, error)`；`type Gap struct { IntentID, Description string; OpenBlockers, DepthFromOrigin int }`

- [ ] **Step 1: 写失败测试**

```go
func TestReasonCompletesWhenGoalCovered(t *testing.T)   { /* driver 返回 complete → client.Complete 调用 1 次 */ }
func TestReasonDeclaresIntentWithValidFrom(t *testing.T) { /* 返回 intent.from=[f001] 且 f001 存在 → DeclareIntent */ }
func TestReasonRejectsIntentWithUnknownFrom(t *testing.T) { /* from=[f999] → 不写图，返回 ErrInvalid */ }
func TestReasonPromptIncludesGapsAndHints(t *testing.T)  { /* prompt 含 gap 列表与 hint 文本 */ }
func TestReasonNoopWritesNothing(t *testing.T)           { /* data={} → 无写操作 */ }
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./dispatch/tasks/ -run TestReason -v`
Expected: FAIL，`undefined: RunReason`。

- [ ] **Step 3: 实现 reason**

先 claim 项目级 reason lease（拿不到则跳过），持续心跳；取 `ExportYAML`、`fact_ids`、`open_intents`、`GapScore` top-5、hints；渲染 prompt；解析结果：`complete` → `Complete`，`intent` → 校验 from 后 `DeclareIntent`，空 → 不写；最后 `ReleaseReason`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./dispatch/tasks/ -race -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add dispatch/tasks graph/query.go
git commit -m "feat(dispatch): add provenance-aware reason task"
```

---

### Task 9: explore 沙箱实证与证据判决（R9, R10）

**Files:**
- Create: `dispatch/tasks/explore.go`
- Create: `dispatch/runtime/sandbox.go`
- Create: `graph/evidence.go`
- Test: `dispatch/tasks/explore_test.go`
- Test: `graph/evidence_test.go`

**Interfaces:**
- Consumes: `graphserver.Client`、`worker.Driver`、`dockerManager`
- Produces:
  - `type SandboxRuntime interface { Exec(ctx context.Context, projectID string, req ExecRequest) (ExecResult, error) }`
  - `type ExecRequest struct { Command, Workdir string; Timeout time.Duration }`
  - `type ExecResult struct { Stdout, Stderr string; ExitCode int; Err error }`
  - `func RunExplore(ctx context.Context, c graphserver.Client, d worker.Driver, rt SandboxRuntime, p graph.Project, intentID string) error`
  - `func (s *Store) AddVerdict(ctx context.Context, projectID, targetID, verdict, description, evidenceRef string) (Node, error)`（自动建 `support` 或 `refute` 边）

- [ ] **Step 1: 写失败测试**

```go
func TestExploreConcludesObservationFact(t *testing.T)  { /* fake driver + fake runtime → conclude 1 次 */ }
func TestExploreTimeoutFallsBackToConclude(t *testing.T) { /* Execute 超时 → Conclude 被调用一次 */ }
func TestExploreFailureStillWritesObjectiveFact(t *testing.T) { /* 失败也必须落客观结论，不得空响应 */ }
func TestVerdictRequiresSupportOrRefuteEdge(t *testing.T) { /* AddVerdict 后边表存在 support/refute */ }
func TestCompleteRejectsFindingWithoutProvenance(t *testing.T) { /* 无路径的 finding → ErrInvalid */ }
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./dispatch/tasks/ -run TestExplore -v`
Expected: FAIL，`undefined: RunExplore`。

- [ ] **Step 3: 实现 explore 与 verdict**

claim intent → 渲染 prompt（含 graph_yaml、intent_id、intent_description）→ driver 执行；超时或解析失败且 driver 支持 conclude → 同一 session 收尾；成功 → `ConcludeIntent(description)`；若结论是判决，调用 `AddVerdict` 建 verdict 节点 + `support`/`refute` 边。

`Complete` 前置校验：每个 `from` 中的 finding 节点必须有 `Provenance` 结果，否则 `ErrInvalid`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./dispatch/tasks/ ./graph/ -race -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add dispatch/tasks dispatch/runtime graph/evidence.go
git commit -m "feat(dispatch): add sandbox explore task and evidence verdicts"
```

---

### Task 10: CE4 规则回流（R11）

**Files:**
- Create: `dispatch/tasks/outcome.go`
- Test: `dispatch/tasks/outcome_test.go`

**Interfaces:**
- Consumes: `patchintel.RuleBase`、`graph.Store`
- Produces:
  - `func ApplyOutcome(ctx context.Context, rb *patchintel.RuleBase, s *graph.Store, projectID, verdictID string, confirmed bool, note string) error`
  - 行为：确认 → `confirm_count+1`、置信度上调、新版本；失败且归因规则 → 置信度下调；新模式 → `Promote` 生成 version+1。

- [ ] **Step 1: 写失败测试**

```go
func TestConfirmedVerdictIncrementsConfirmCount(t *testing.T) {}
func TestFailedVerdictLowersConfidence(t *testing.T)         {}
func TestNewPatternPromotedAsNewVersion(t *testing.T)        { /* 同 pattern 第二次 Promote → version=2，旧版保留 */ }
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./dispatch/tasks/ -run TestConfirmedVerdict -v`
Expected: FAIL，`undefined: ApplyOutcome`。

- [ ] **Step 3: 实现回流**

按 verdict 关联的 `rule_id` 调用 `UpdateOutcome`；无关联且模式匹配失败时调用 `Promote`；把回流动作为新 Fact 写入图（type=`rule_hit`），保证可溯源。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./dispatch/tasks/ -race -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add dispatch/tasks/outcome.go
git commit -m "feat(dispatch): add CE4 rule feedback loop"
```

---

### Task 11: 评测、端到端与灰度（R13–R16）

**Files:**
- Create: `eval/patcheval.go`
- Create: `eval/patcheval_test.go`
- Create: `docs/runbooks/cve-2025-3248.md`
- Modify: `misc/ConfigDB.go`（新增 `graph_lane_enabled` 默认 `false`）
- Test: `eval/patcheval_test.go`

**Interfaces:**
- Consumes: `dispatch.Dispatcher`、`patchintel.RuleBase`、`graph.Store`
- Produces:
  - `type Report struct { Runs []RunResult; Metrics Metrics }`
  - `type Metrics struct { InversionSuccessRate float64; AvgTrialRounds float64; TokensPerFinding float64 }`
  - `func Run(ctx context.Context, opts Options) (Report, error)`
  - `func (r Report) JSON() ([]byte, error)`

- [ ] **Step 1: 写失败测试**

```go
func TestPatchevalRunsDualConditionAndReportsMetrics(t *testing.T) {
    // 冷启动 / 预置规则库两组各 N=3，fake runtime；断言 Metrics 三字段非零、Runs 长度 6
}
func TestPatchevalRequiresN(t *testing.T) { /* N<3 → error */ }
func TestGraphLaneFlagDefaultsOff(t *testing.T) { /* 默认 false；开启后才走 dispatch */ }
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./eval/ -run TestPatcheval -v`
Expected: FAIL，`undefined: Run`。

- [ ] **Step 3: 实现评测与灰度开关**

评测输出 JSON：`{"condition":"cold|seeded","run":1,"inversion_ok":true,"trial_rounds":2,"tokens":12345}` 逐条 + 汇总 Metrics。`graph_lane_enabled=false` 时 `main.go` 不启动 dispatcher，既有路径不变。

写 `docs/runbooks/cve-2025-3248.md`：靶标启动、diff 导入、验收命令、timeline 回放步骤、失败降级路径。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./eval/ ./... 2>&1 | tail -20`
Expected: 全部 `ok`；`go build ./...` exit 0。

- [ ] **Step 5: 提交**

```bash
git add eval docs/runbooks misc/ConfigDB.go
git commit -m "feat(eval): add patcheval dual-condition metrics and graph lane flag"
```

---

## Self-Review

**1. Spec coverage:** R1→T1，R2→T1，R3→T2，R4→T4，R5→T5，R6→T6，R7→T7，R8→T3/T8，R9→T9，R10→T9，R11→T10，R12→T8（Hint 读取）与 T4（Hint 写入），R13→T11 Step 3 后置面板，R14→T11，R15→T11 + runbook，R16→T11 Step 3。无遗漏。

**2. Step scan:** 每个 Step 只有一个动作与一个可检查结果；无 TBD、无“适当处理”。

**3. Type consistency:** `graph.Store` 方法签名在 T2 定义、T3/T8/T9 复用；`graphserver.Client` 在 T4 定义、T5/T7/T8/T9 复用；`worker.Driver` 在 T6 定义、T5 先按最小接口引用（`Supports` / `Name`），T7–T9 使用完整接口。

**4. Review Focus:** 五个输入类分别在 T2（并发 claim）、T2（环检测）、T2（lease 回收）、T9（无溯源路径拒绝 complete）、T6（非法输出不写图）有测试。

**5. Proportion:** 计划以签名、测试名与断言为主；仅原子认领 SQL 与错误映射给出实现细节，因为这两处决定正确性。
