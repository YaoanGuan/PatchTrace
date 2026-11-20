# PatchTrace v3 实施计划：JEV 决策 × Reflex 自进化（v3_task）

> **For agentic workers:** 逐 Task 执行，先写失败测试再实现；每个 Task 独立可验收、可回滚。
> **Goal:** 在现有图调度车道外挂决策层与 reflex 短路层，让重复渗透动作复用程序化经验，降低 token 与到达验证的时间，同时保持证据质量。
> **Architecture:** `decision/` 产出判断与 claim；`evidence/` 统一证据信封；`reflex/` 执行程序化经验并返回 Outcome；`dispatch` 在 executor 前匹配、失败落回；`safety/`、`fingerprint/`、`reflexopt/` 提供护栏、路由与自进化；`eval/` 负责对照度量。
> **Tech Stack:** Go 1.24、`modernc.org/sqlite`、`github.com/dop251/goja`、现有 `graph`/`graphserver`/`dispatch`/`eval`。
> **Spec:** `docs/plans/v3_re.md`（本计划逐条实现其 V3-R1…R16）。

## Global Constraints

- `misc.reflex_enabled=false`（默认）时，现有车道行为逐字节不变。
- 单二进制 + SQLite；不引入新存储引擎；不 import `Web/`。
- 不改 graph 协议、任务状态机、审批流；不移动 `agents/agent_builder.go` 的 `factories[:7]` / `factories[:12]`。
- 判定纪律：没有命令与输出的 observation 不算成功；reflex 不得直接写图。
- 流程分支只在代码里；prompt 不写“命中 reflex 就……”。
- 新包只向下依赖；`reflex` 不 import `dispatch`（用本地 `ExecFunc` 接口），由 `dispatch/lane.go` 适配。
- 所有成本（决策、生成、优化）计入 token 账，不能只算收益。

## Review Focus

1. **签名碰撞**：新增 fact/hint 后签名不变，导致新态势被短路 → 必须有“签名对图变化敏感”的测试。
2. **部分执行后失败 / defer 重试**：reflex 已执行命令但写图前失败，落回 LLM 造成双写或脏状态；`defer` 后同任务内重试同一版本造成循环 → 必须有 no-double-write 与 defer-once 测试。
3. **护栏 fail-open**：分类器报错时放行危险命令 → 必须 fail-closed 测试。
4. **JS 逃逸**：通过 `constructor`/`eval`/`import` 拿到宿主对象 → 静态检查 + 运行时白名单双保险测试。
5. **Hint 被短路吞掉**：有 Hint 时 reflex 仍按旧签名命中 → Hint 纳入签名或强制 reason 的测试。

---

## 阶段 0：真实基线（阻塞项）

### Task 0：CVE-2025-3248 真实跑通（G0）

**Files:**
- Create: `docs/runbooks/cve-2025-3248-run-2026-11-20.md`
- Read: `docs/runbooks/cve-2025-3248.md`、`docs/plans/todo.md` §2.1

**Interfaces:**
- Consumes: 现有 `/api/graph`、`dispatch.yaml`、Docker、模型配置。
- Produces: 真实验收记录 + 可回放的 `graph.db`/`rules.db` 状态 + ≥3 组同签名 explore claim。

- [ ] **Step 1: 环境预检**

Run: `docker info`；`GET /api/graph/projects` 带 Bearer；确认 `main_setting.MODEL` 非空。
Expected: Docker 可用、token 有效、模型已配置。任一不满足即停，记录为阻塞。

- [ ] **Step 2: 建真实图项目**

`origin` 指向真实 diff；`bootstrap_enabled=true`；id 使用 lab id。

- [ ] **Step 3: 跑完整链路并记录**

等待 bootstrap → reason → explore；导出 `export?format=yaml` 与 `timeline`。

- [ ] **Step 4: SQL 验收**

```sql
SELECT type,count(*) FROM nodes WHERE project_id='<id>' GROUP BY type;
SELECT kind,count(*) FROM edges WHERE project_id='<id>' GROUP BY kind;
SELECT count(*) FROM rules; -- rules.db
```

Expected: `patch_profile ≥1`、`exploit_idea fact ≥1`、`verdict ≥1`、`rules ≥1`。

- [ ] **Step 5: 写运行记录**

命令、原始输出、失败点、时间戳全部落盘。没有输出不得进入 C2。

---

## 阶段 C1：决策与证据（G1/G2）

### Task 1：`decision/` 契约与启发式判断（V3-R1）

**Files:**
- Create: `decision/claim.go`、`decision/result.go`、`decision/decider.go`、`decision/heuristic.go`、`decision/decider_test.go`

**Interfaces:**
- Produces:
```go
type Judgment int
const (JudgeBool Judgment = iota; JudgeChoice; JudgeRank; JudgeScore)
type Point string
const (PointBinding Point = "binding"; PointCompletion Point = "completion")
type Claim struct {
    ID, ProjectID, TaskType, Signature, Content string
    Point Point
    Options []string          // 必须含 "defer"
    Features map[string]any
}
type Result struct {
    Bool bool; Choice string; Rank []string
    Probabilities map[string]float64
    Score, Confidence float64
}
type Decider interface { Judge(context.Context, Claim) (Result, error) }
var ErrInvalid = errors.New("decision: invalid claim")
var ErrUnavailable = errors.New("decision: unavailable")
```

- [ ] **Step 1: 写失败测试**

`TestJudgeBool`、`TestJudgeChoice`、`TestJudgeRank`、`TestJudgeScore`、`TestJudgeBindingReturnsAcceptOrDefer`、`TestJudgeCompletionStopsOnGrounded`、`TestJudgeProbabilitiesSortedAndBounded`、`TestHeuristicIsDeterministic`、`TestUnknownClaimReturnsInvalid`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./decision/ -run TestJudge -v`
Expected: FAIL（包不存在）。

- [ ] **Step 3: 实现**

启发式只读 `Features`：open intent 数、observation 数、intent class、历史失败率。未知 `TaskType` 返回 `ErrInvalid`；不得 panic。`binding` 默认 `defer`，只有命中已发布 reflex 且置信度达阈值才返回该 reflex ID；`completion` 只在报告含 grounded 证据时返回完成。`Probabilities` 按 `Rank` 对齐、单调不增、总和 ≤1。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./decision/ -race`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add decision && git commit -m "feat(decision): add JEV claim contract and heuristic decider"
```

---

### Task 2：Claim 落库与签名（V3-R2）

**Files:**
- Create: `decision/store.go`、`decision/signature.go`、`decision/store_test.go`
- Create: `data/reflex.db` schema（运行时创建，不入库）

**Interfaces:**
```go
type Features struct { FactCount, IntentCount, OpenIntents, ObservationCount int; TypeHistogram map[string]int; IntentClass, ToolClass, CommandClass, ResponseClass string }
func Signature(taskType string, f Features) string
type ClaimStore interface {
    Save(context.Context, Claim, Result, Meta) error
    ListBySignature(context.Context, string) ([]Record, error)
    CountBySignature(context.Context, string) (int, error)
}
type Meta struct { CostTokens int64; LatencyMS int64; Outcome string }
```

- [ ] **Step 1: 写失败测试**

`TestSignatureStableAcrossText`、`TestSignatureChangesWithNewFact`、`TestSignatureExcludesSecrets`、`TestClaimStoreConcurrentSave`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./decision/ -run 'TestSignature|TestClaimStore' -v` → FAIL。

- [ ] **Step 3: 实现**

签名 = `sha256(canonical_json(features))[:16]`；SQLite 表见 `v3_re.md` §6；WAL；并发写用事务。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./decision/ -race` → PASS。

- [ ] **Step 5: 提交**

`feat(decision): persist claims and derive stable signatures`

---

### Task 3：证据信封 `evidence/`（V3-R3）

**Files:**
- Create: `evidence/record.go`、`evidence/capture.go`、`evidence/redact.go`、`evidence/record_test.go`

**Interfaces:**
```go
type Record struct { Command string; ExitCode int; Stdout, Stderr, SandboxID string; StartedAt, EndedAt time.Time; OutputHash string; Truncated bool; Redacted []string }
type ExecFunc func(ctx context.Context, projectID, command, workdir string, timeout time.Duration) (stdout, stderr string, exitCode int, err error)
func Capture(ctx context.Context, exec ExecFunc, projectID, command, workdir string, timeout time.Duration) (Record, error)
func (r Record) Encode() string
func Parse(content string) (Record, bool, error)
func Redact(s string) (string, []string)
```

- [ ] **Step 1: 写失败测试**

`TestCaptureRecordsExitAndHash`、`TestCaptureRedactsSecrets`、`TestParseRoundTrips`、`TestParseRejectsMissingSandboxID`、`TestCaptureTruncatesLargeOutput`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./evidence/ -v` → FAIL。

- [ ] **Step 3: 实现**

命令为空/退出码缺失/sandbox id 为空 → `ErrInvalid`。脱敏 `Authorization`、`Cookie`、`Set-Cookie`、`password=`、`api_key=`。输出截断保留头尾并置 `Truncated=true`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./evidence/ -race` → PASS。

- [ ] **Step 5: 提交**

`feat(evidence): add redacted, hashable pentest evidence envelope`

---

### Task 4：接入 planner/executor 与 flag（V3-R1/R15）

**Files:**
- Modify: `misc/ConfigDB.go`、`dispatch/planner.go`、`dispatch/executor.go`、`dispatch/lane.go`
- Create: `dispatch/decision_integration_test.go`

**Interfaces:**
- Consumes: `decision.Decider`、`decision.ClaimStore`。
- Produces: `dispatch.Config.Decider` 可选注入；关闭时 nil。

- [ ] **Step 1: 写失败测试**

`TestPlanUnchangedWhenDecisionDisabled`、`TestPlanSkipsLowScoreIntent`、`TestPlanKeepsBootstrapPriority`、`TestDecisionClaimRecordedOnce`、`TestDecisionErrorFallsBackToLLM`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./dispatch/ -run 'TestPlan.*Decision|TestDecision' -v` → FAIL。

- [ ] **Step 3: 实现**

配置新增：`misc.reflex_enabled=false`、`misc.decision_mode=heuristic`、`misc.reflex_generate_threshold=3`、`misc.reflex_sample_rate=0.1`、`misc.guardrail_enabled=false`、`misc.poc_safety_filter=false`、`misc.fingerprint_router=false`、`misc.engagement_mode=validation`。
决策只做：低分 intent 延后（不是删除）、记录 claim；不得改变 bootstrap/reason/explore 的语义。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./dispatch/ ./decision/ -race` → PASS。

- [ ] **Step 5: 提交**

`feat(dispatch): gate low-value intents behind reflexive decision layer`

---

## 阶段 C2：Reflex 闭环（G3）

### Task 5：goja 运行时与静态检查（V3-R4）

**Files:**
- Create: `reflex/runtime.go`、`reflex/staticcheck.go`、`reflex/runtime_test.go`
- Modify: `go.mod`（`github.com/dop251/goja`）

**Interfaces:**
```go
type ExecFunc func(ctx context.Context, projectID, command, workdir string, timeout time.Duration) (stdout, stderr string, exitCode int, err error)
type BindingJudge func(ctx context.Context, call ToolCall) (bool, error) // true=accept, false=defer
type Input struct { ProjectID, Workdir string; Timeout time.Duration; Args map[string]any }
type Outcome struct { Evidence evidence.Record; Fact string; Stop bool; Defer bool; Verdict *VerdictInput; TokensUsed int64 }
type Runtime struct { Exec ExecFunc; Bind BindingJudge; MaxOutput int; DefaultTimeout time.Duration }
func (r *Runtime) Run(ctx context.Context, src string, in Input) (Outcome, error)
```
`Run` 调用程序入口 `function run(a, read, step, occurrence)`；`a` 即 `Input.Args`，`read/step/occurrence` 由调用方按当前任务传入。

- [ ] **Step 1: 写失败测试**

`TestRuntimeRejectsForbiddenIdentifiers`（`eval`/`Function`/`require`/`process`/`fetch`/`import`）、`TestRuntimeTimesOut`、`TestRuntimeCapsOutput`、`TestRuntimeHasNoHostObjects`、`TestRuntimeExecReturnsEvidence`、`TestRuntimeDeferSkipsExecute`、`TestRuntimeSameProgramDifferentArgs`、`TestStaticCheckRejectsEscapes`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./reflex/ -run TestRuntime -v` → FAIL。

- [ ] **Step 3: 实现**

goja VM 只注入 `execute(request)`（request=`{name, arguments}`，先过 `Bind`，`name="bash"` 时映射到 `ExecFunc`）、`readFile/log/stop/finish`；不注入 `require`/`process`/`globalThis` 扩展；用 `vm.Interrupt` 做超时；输出上限默认 256KB；静态检查用 goja parser 拒绝动态代码与逃逸标识符，并要求存在入口 `run`。`Bind` 返回 false 时 `Run` 立即以 `Defer=true` 返回，且不执行任何工具调用。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./reflex/ -race` → PASS。

- [ ] **Step 5: 提交**

`feat(reflex): add sandboxed goja runtime with whitelisted primitives`

---

### Task 6：executor 短路、落回与 quarantine（V3-R5/R16）

**Files:**
- Modify: `dispatch/executor.go`、`dispatch/lane.go`
- Create: `reflex/match.go`、`reflex/store.go`、`dispatch/reflex_integration_test.go`

**Interfaces:**
```go
type Matcher interface { Match(ctx context.Context, projectID, signature string) (*Reflex, error) }
type ApplyFunc func(ctx context.Context, p graph.Project, out Outcome) error
type CompletionFunc func(ctx context.Context, out Outcome) (done bool, err error)
```
`ApplyFunc`/`CompletionFunc` 由 `dispatch` 注入，复用 `tasks` 的写图helper；reflex 本身不 import `graphserver`。

- [ ] **Step 1: 写失败测试**

`TestReflexHitSkipsLLM`、`TestReflexFailureFallsBack`、`TestReflexQuarantinesAfterThreeFailures`、`TestReflexNoDoubleWrite`、`TestReflexParityMismatchQuarantines`、`TestReflexRespectsStop`、`TestBindingJudgeDefersBeforeAnyExec`、`TestCompletionJudgeGatesApply`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./dispatch/ -run TestReflex -v` → FAIL。

- [ ] **Step 3: 实现**

执行前：签名 → Match → 注入 `Bind`（binding 判断）→ `Run`（内部每次 `execute` 再问 binding）→ `CompletionFunc`（completion 判断）→ `ApplyFunc`。
`Run` 返回 `Defer=true`：不写图、不重试同版本、记 `reflex_runs(success=0,error=defer)`，落回 runner。
失败：记 `reflex_runs`、`fail_count+1`、落回 runner；连续 3 次失败 → `quarantined`。成功：`hit_count+1`、记 tokens saved。
Hint 存在时：签名必须包含 hint 摘要，或强制走 reason（二选一，测试固定行为）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./dispatch/ ./reflex/ -race` → PASS。

- [ ] **Step 5: 提交**

`feat(dispatch): short-circuit repeated tasks through validated reflexes`

---

### Task 7：内置渗透 Reflex（V3-R8，先交付确定性价值）

**Files:**
- Create: `reflex/builtin/http_probe.js`、`reflex/builtin/evidence_collect.js`、`reflex/builtin/stop_on_marker.js`、`reflex/builtin/embed.go`、`reflex/builtin/builtin_test.go`

**Interfaces:**
- Consumes: `Runtime`、`evidence.Record`。
- Produces: 三条内置 reflex，签名固定，`version=1`，`status=active`。

- [ ] **Step 1: 写失败测试**

`TestBuiltinHTTPProbeCapturesEvidence`、`TestBuiltinEvidenceCollectOnFailure`、`TestBuiltinStopOnMarkerRefutes`、`TestBuiltinRequiresExplicitCommand`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./reflex/builtin/ -v` → FAIL。

- [ ] **Step 3: 实现**

- `http_probe`：`curl -sS -D -`，记录状态/头/正文摘要；不跟随外域。
- `evidence_collect`：执行给定命令，失败也返回客观 Fact。
- `stop_on_marker`：标记不存在 → `Stop=true` + refute 输入。
全部走 `execute({name:'bash', arguments:{command:...}})`，不直连网络。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./reflex/builtin/ -race` → PASS。

- [ ] **Step 5: 提交**

`feat(reflex): ship built-in http/evidence/stop reflexes`

---

### Task 8：Reflex 生成管线（V3-R6）

**Files:**
- Create: `reflex/generate.go`、`prompts/reflex/generate.md`、`reflex/generate_test.go`

**Interfaces:**
```go
type Generator struct { Threshold int; Complete func(ctx, prompt string) (string, error) }
func (g *Generator) MaybeGenerate(ctx context.Context, sig string, claims []decision.Record) (*Candidate, error)
type Candidate struct { Name, Source, ArgsSchema string; Signature string; Claims []string }
```
`Source` 必须以 `function run(a, read, step, occurrence)` 开头并返回 `execute(...)` 或 `{defer:true}`；`ArgsSchema` 只描述参数结构，不含具体目标值。

- [ ] **Step 1: 写失败测试**

`TestGenerateRequiresThreshold`、`TestGenerateRunsFiveClaims`、`TestGenerateEmitsRunEntry`、`TestGenerateRejectsForbiddenSource`、`TestGenerateDeduplicatesCandidates`、`TestGenerateCountsTokens`。

- [ ] **Step 2: 跑测试确认失败**

`go test ./reflex/ -run TestGenerate -v` → FAIL。

- [ ] **Step 3: 实现**

同签名成功 ≥ `misc.reflex_generate_threshold` 才触发；5 个 claim 并行（`errgroup` 或 WaitGroup），各自超时；候选必须含 `run` 入口与 `stop`/`finish`/`{defer:true}` 之一；静态检查不过直接拒绝；prompt 里给出 `run(a, read, step, occurrence)` 骨架与一个真实 claim 的 `a` 示例（脱敏）；生成 token 计入 claim/reflex 成本。

- [ ] **Step 4: 跑测试确认通过**

`go test ./reflex/ -race` → PASS。

- [ ] **Step 5: 提交**

`feat(reflex): generate reflex candidates from parallel claims`

---

### Task 9：回放调试、入库与版本（V3-R7）

**Files:**
- Create: `reflex/debug.go`、`reflex/store.go`、`reflex/debug_test.go`、`reflex/store_test.go`

**Interfaces:**
```go
func Replay(ctx context.Context, rt *Runtime, src string, claims []decision.Record) (Parity, error)
type Parity struct { Matched, Mismatched int; Reasons []string }
func (s *Store) Promote(ctx context.Context, c Candidate, parity Parity) (Reflex, error)
func (s *Store) Quarantine(ctx context.Context, id, reason string) error
```

- [ ] **Step 1: 写失败测试**

`TestReplayMismatchRejected`、`TestPromoteIncrementsVersion`、`TestQuarantineStopsMatching`、`TestConfidenceUpdatesOnHit`、`TestRetireKeepsHistory`。

- [ ] **Step 2: 跑测试确认失败**

`go test ./reflex/ -run 'TestReplay|TestPromote|TestQuarantine' -v` → FAIL。

- [ ] **Step 3: 实现**

回放必须 100% parity 才可入库，且候选需通过“同程序换一组 `a` 仍可执行”的参数化检查（Mechanism validated）；升版保留旧版；置信度按 hit/confirm/fail 更新；退役不删除历史。

- [ ] **Step 4: 跑测试确认通过**

`go test ./reflex/ -race` → PASS。

- [ ] **Step 5: 提交**

`feat(reflex): replay-validate and version reflex promotion`

---

## 阶段 C3：安全路由与自进化（G4/G5）

### Task 10：命令护栏（V3-R9）

**Files:**
- Create: `safety/guardrail.go`、`safety/guardrail_test.go`
- Modify: `dispatch/lane.go`（注入 approver）、`reflex/runtime.go`（exec 前分类）

**Interfaces:**
```go
type CommandClass int
const (ClassSafe CommandClass = iota; ClassApproval; ClassForbidden)
type Guardrail interface { Classify(ctx context.Context, command string) (CommandClass, float64, error) }
```
分类失败或未知 → `ClassApproval`。

- [ ] **Step 1: 写失败测试**

`TestGuardrailFailsClosed`、`TestForbiddenCommandNeverExecuted`、`TestUnknownDefaultsApproval`、`TestOverrideIsAudited`。

- [ ] **Step 2: 跑测试确认失败** → FAIL。

- [ ] **Step 3: 实现**

reflex 的 `execute(request)` 与 explore 命令都先过 guardrail；`ClassForbidden` 直接拒绝并写审计；`ClassApproval` 走现有审批接口（由 dispatch 注入，不 import Web）。

- [ ] **Step 4: 跑测试确认通过** → PASS。

- [ ] **Step 5: 提交**

`feat(safety): fail-closed command guardrail for reflex and explore`

---

### Task 11：POC 安全分级（V3-R10）

**Files:**
- Create: `safety/poc.go`、`safety/poc_test.go`、`safety/poc_index.go`

**Interfaces:**
```go
type POCClass string
const (POCValidation POCClass = "validation_only"; POCExploit POCClass = "exploit_capable"; POCUnknown POCClass = "unknown")
func ClassifyPOC(ctx context.Context, path, content string) (POCClass, float64, error)
func Allowed(c POCClass, mode string) bool // mode: validation | full
```

- [ ] **Step 1: 写失败测试**

`TestValidationModeBlocksExploitPOC`、`TestUnknownDefaultsValidation`、`TestFullModeNeedsExplicitConfig`、`TestClassifierFailureFailsClosed`、`TestOverrideIsAudited`。

- [ ] **Step 2: 跑测试确认失败** → FAIL。
- [ ] **Step 3: 实现**：规则 + 分类器；unknown 默认仅验证；`engagement_mode` 来自配置；覆盖需留痕。
- [ ] **Step 4: 跑测试确认通过** → PASS。
- [ ] **Step 5: 提交**

`feat(safety): classify POCs by engagement safety`

---

### Task 12：指纹路由（V3-R11）

**Files:**
- Create: `fingerprint/router.go`、`fingerprint/router_test.go`、`reflex/builtin/fingerprint_lookup.js`

**Interfaces:**
```go
type Match struct { Product, Version, SourceKeyword string; Confidence float64 }
type Router interface { Match(ctx context.Context, response string) ([]Match, error) }
```
无命中 → 回退现有 `patchintel` 搜索；不得伪造。

- [ ] **Step 1: 写失败测试**

`TestRouterMatchesKeywordToProduct`、`TestRouterNoMatchFallsBack`、`TestRouterRejectsFabricatedFingerprint`、`TestFingerprintLookupBuiltin`。

- [ ] **Step 2-4: 红→实现→绿**（同前格式）。
- [ ] **Step 5: 提交**

`feat(fingerprint): route responses to product fingerprints before regex POC scan`

---

### Task 13：旁路优化器（V3-R12）

**Files:**
- Create: `reflexopt/optimizer.go`、`reflexopt/optimizer_test.go`、`eval/reflex_eval.go`

**Interfaces:**
```go
type Optimizer struct { SampleRate float64; Complete func(ctx, prompt string) (string, error) }
func (o *Optimizer) MaybeOptimize(ctx context.Context, r reflex.Reflex, samples []reflex.Run) (*reflex.Candidate, error)
```
采样率默认 0.1；A/B 由 eval 驱动；未胜出不升版。

- [ ] **Step 1: 写失败测试**

`TestOptimizerRequiresSampleSize`、`TestOptimizerPromotesOnlyOnABWin`、`TestOptimizerRollsBackOnRegression`、`TestOptimizerCostsCounted`。

- [ ] **Step 2-4: 红→实现→绿**。
- [ ] **Step 5: 提交**

`feat(reflexopt): sample and A/B promote reflex improvements`

---

### Task 14：评测指标（V3-R13）

**Files:**
- Modify: `eval/patcheval.go`、`eval/patcheval_test.go`

**Interfaces:**
```go
type V3Metrics struct {
    ShortCircuitRate float64 `json:"short_circuit_rate"`
    ReflexTokensSaved int64 `json:"reflex_tokens_saved"`
    ParityFailures int `json:"parity_failures"`
    TimeToVerifiedFindingMS int64 `json:"time_to_verified_finding_ms"`
    ReflexRegressionRate float64 `json:"reflex_regression_rate"`
    SafetyIncidents int `json:"safety_incidents"`
}
```

- [ ] **Step 1: 写失败测试**：`TestV3MetricsShortCircuitRate`、`TestV3MetricsTokensPerVerifiedFinding`、`TestV3MetricsParityFailures`、`TestVerifiedFindingDefinition`。
- [ ] **Step 2-4: 红→实现→绿**。finding 口径：observation + support verdict + 可复现命令；代理口径单独保留为 `legacy_proxy_finding`。
- [ ] **Step 5: 提交**

`feat(eval): add v3 short-circuit and verified finding metrics`

---

### Task 15：真实渗透验收（V3-R14）

**Files:**
- Create: `docs/runbooks/cve-2025-3248-v3-run-<date>.md`

- [ ] **Step 1: 跑 T0 同场景，开启 v3 flag**
- [ ] **Step 2: 核对指标**：`short_circuit_rate ≥20%`、`tokens_per_verified_finding` 下降 ≥25%、finding 不降、`safety_incidents=0`。
- [ ] **Step 3: 记录命令与原始输出**；失败也记录。
- [ ] **Step 4: 更新 `v3_todo.md` 状态；未达标不得打勾。**

---

### Task 16：兼容、灰度与可观测收口（V3-R15/R16）

**Files:**
- Modify: `dispatch/lane.go`、`misc/ConfigDB.go`、`eval/patcheval.go`
- Create: `dispatch/v3_flag_test.go`

- [ ] **Step 1: 写 flag 矩阵测试**：全关 → 现有测试全绿；只开 decision；只开 reflex；全开；`TestKillSwitchImmediateFallback`（运行中关闭 reflex 后下一次任务直接回 LLM，无缓存副作用）。
- [ ] **Step 2: 跑 `CGO_ENABLED=0 go build ./...`、`go test ./graph/ ./graphserver/ ./dispatch/... ./eval/ ./decision/ ./evidence/ ./reflex/... ./safety/ ./fingerprint/ ./reflexopt/ -race`。**
- [ ] **Step 3: 检查红线**：无 `import "AIxVuln/Web"`；工厂下标未动；无新存储引擎；prompt 无流程分支。
- [ ] **Step 4: 可观测**：日志含 claim id、signature、reflex id/version、quarantine 原因；错误不吞。
- [ ] **Step 5: 提交收口。**

---

## Self-Review

**1. Spec coverage:** V3-R1→T1/T4；R2→T2；R3→T3；R4→T5；R5→T6；R6→T8；R7→T9；R8→T7/T12；R9→T10；R10→T11；R11→T12；R12→T13；R13→T14；R14→T0/T15；R15→T4/T16；R16→T4/T6/T9/T10/T16。无遗漏。

图片校正（`jev+reflex/`，2026-11-20）已并入：两判断点（T1/T6）、`run(a, read, step, occurrence)` 入口（T5/T8）、`execute(request)` 工具边界（T5/T7）、概率排序与 `defer`（T1/T6）。

**2. Step scan:** 每个 Task 五个步骤（红、验红、实现、验绿、提交），无 TBD；签名、表结构、错误语义均已给出。

**3. Type consistency:** `decision.Claim/Result` 在 T1 定义，T2/T4/T8 复用；`evidence.Record` 在 T3 定义，T5/T6/T7 复用；`reflex.Outcome` 在 T5 定义，T6/T7 复用；`ApplyFunc` 只在 dispatch 注入。

**4. Review Focus:** 五个风险各有对应测试：签名敏感（T2）、双写（T6）、fail-closed（T10）、JS 逃逸（T5）、Hint 冲突（T6）。

**5. Proportion:** 计划以接口签名、测试名与验收命令为主；仅签名算法与白名单列出实现细节，因为这两处决定正确性。

---

## Execution Handoff

- 先做 Task 0；无 Docker/LLM 时记录阻塞，不得跳过。
- C1 可离线先行（T1–T4）；C2 依赖 C1；C3 依赖 C2。
- 每个 Task 完成即回写 `v3_todo.md`，附命令与输出；未附证据不算完成。
