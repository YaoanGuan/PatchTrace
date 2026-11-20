# PatchTrace 待办与进度跟踪（todo.md）

> 配套：`docs/plans/requirement.md`（需求）、`docs/plans/task.md`（实施计划）
> 更新规则：每完成一步立即回写；没有命令输出就不打勾。
> 当前轮次：第 2 轮（复核审计）· 2026-11-20

---

## 0. 下一步动作（只做这一件）

> **第 2 轮结论（2026-11-20，只复核不改代码）**：P0 代码与测试基本落地，但“真实闭环”证据不足。按 `requirement.md` R1–R16 逐条核对，见 §2.1。
> **关键事实**：真实 `data/graph.db` 中 `patch_profile` = 0、`verdict` = 0、`support/refute` 边 = 0、`finding` = 0；两个项目 `bootstrap_enabled` 都为 `0`；`data/patchintel/rules.db` 的 `rules` 表为 0 行。前端能看到的那张图是 observation/gap 链，不是“补丁 → 约束 → 思路 → 观测 → 判决 → finding”的完整反演链。
> **本轮证据**：`CGO_ENABLED=0 go build ./...` → exit 0；`CGO_ENABLED=1 go test ./graph/ ./graphserver/ ./dispatch/... ./eval/ -race -count=1` → 7 个包全 `ok`（共 223 个 Test）；隔离数据目录起服务实测 flag=on 时 `GET projects` 200 / `POST projects` 201 / `POST hints` 201 / 空 hint 422 / 未知节点 404，flag=off 时 `/api/graph` 未挂载；只读查询 `data/graph.db` 与 `rules.db` 见 §2.1。
> **更正上轮表述**：GraphOnly 只关闭 driver 与 executor，**存储仍可写**（实测 `POST /api/graph/projects` 返回 201），不是“只读面板”。
> **下一步唯一动作**：用真实 Langflow diff 建图项目（`bootstrap_enabled=true`，origin 为 diff），让车道真实写出 `patch_profile` + `exploit_idea`；随后走 explore/verdict，并确认 `rules.db` 增长。完成前不得把 R7/R10/R11/R15/G3 计为通过。

---

## 1. 迭代管理提示词（每轮复制）

### 1.1 开启一轮

```text
你是 PatchTrace 第 N 轮的执行 owner。
先读 docs/plans/requirement.md 的 R* 与约束，再读 task.md 当前 Task，再读 todo.md 状态。
只做 todo.md「下一步动作」里的那一件事。
结束前：回写 todo.md（状态 + 命令 + 输出），并给出下一轮的唯一动作。
没有证据不得打勾；不得修改验收标准。
```

### 1.2 验收一轮

```text
验收 Task N。
1. 按 todo.md 列的验收命令逐条执行，原样贴输出。
2. 对照 task.md 的 Steps 与 Review Focus 检查是否有遗漏。
3. 检查是否有越界改动（新增依赖、动了 factories 下标块、import 了 Web/）。
4. 结论只允许：通过 / 不通过（附最小复现）。
```

### 1.3 阻塞上报

```text
阻塞格式（三行）：
- 现象：哪条命令、什么输出。
- 原因假设：最可能的一条，不列清单。
- 需要谁决策：问题一句话 + 两个选项。
```

### 1.4 状态回写模板

```text
- [x] Task N Step M
  证据：<命令> → <关键输出一行>
  变更：<文件列表>
  下一步：<两分钟内可执行的动作>
```

---

## 2. 进度总览

| # | Task | 需求 | 状态 | 闸门 | 证据 |
|---|---|---|---|---|---|
| 1 | 迭代地基与两个 Skill | R1, R2 | 完成 | G0 | `7ec3ca7`；评审通过；新会话实读通过 |
| 2 | 溯源图模型与存储 | R3 | 完成 | G1 | `6f2a6b3`；race 6/6；评审 Approved |
| 3 | 溯源图查询与导出 | R8 | 完成 | G1 | `a31275d`+`a111d4f`；race 16/16；评审 Approved |
| 4 | 图协议 API | R4 | 完成 | G1 | `fc3fc1e`；8/8；评审 Approved |
| 5 | 调度核心 | R5 | 完成 | G2 | `f6cfcf3`；31+4 PASS；评审 Approved |
| 6 | Worker driver 与 native 实现 | R6 | 完成 | G2 | `e63b212`；37/37；评审修复轮通过 |
| 7 | bootstrap 补丁反演任务 | R7 | 完成 | G2 | `96d1da1`；22+planner PASS；评审 2 轮后通过 |
| 8 | reason 溯源图推理任务 | R8, R12 | 完成 | G3 | `93bff65`；40+ PASS；评审 1 轮后通过 |
| 9 | explore 沙箱实证与判决 | R9, R10 | 完成 | G3 | `59b12d5`+`ffb5a4e`；评审 1 轮修复后 ADDRESSED |
| 9b | 调度接线（执行/并发/超时） | R5, R9 | 完成 | G3 | `87474a4`；评审 Approved（5 Minor 延后） |
| 10 | CE4 规则回流 | R11 | 完成 | G3 | `48c1ed3`+`e93148d`；评审 1 轮修复后 ADDRESSED |
| 11 | 评测、端到端与灰度 | R13–R16 | 完成（R13 面板后置） | G4 | `b794e85`+`2c40df2`+`6cc683c`；最终评审后单轮修复 `f118081`..`e83793f`；re-review ADDRESSED；R13 面板/报告见 requirement §12 |
| F | 前端图驱动首页与旧入口保留 | R3–R16 产品面 | 完成 | F-A1–F-A8 | `f94a941`..`e008d0f`；`tsc`/`npm build`/`go build` exit 0；9999 重启 `/` 200；首页断言 11/12、图面板 8/8；旧实验室四来源与 start/stop/enter/delete 保留 |

状态取值：`未开始` / `进行中` / `阻塞` / `完成`。

> 复核备注（2026-11-20）：上表“完成”指**该 Task 的单测与代码交付**完成，不等于需求真实闭环通过。真实数据侧结论见 §2.1；含“完成”的 R7/R10/R11/R15 需要实弹证据才能升级为验收通过。

### 2.1 R1–R16 复核表（2026-11-20，本轮新跑证据）

复核口径：不看 todo 勾选，按 `requirement.md` 的验收标准逐条对照代码、测试、实时 API 与真实数据。

| ID | 复核状态 | 本轮证据 | 缺口 / 说明 |
|---|---|---|---|
| R1 | **未达标** | `git status --porcelain` 非空：`dispatch/data/`、`logo.png`、`docs/claude/logo.png`、`docs/plans/marketplace/`、`docs/superpowers/plans/2026-11-17-game-theme-ui.md`、`webui-marketplace/` 未跟踪；`go build ./...` exit 0；`patchintel`/`toolCalling` 测试通过 | 验收要求“git status 干净”不成立 |
| R2 | 达标 | 两个 SKILL.md + 三份/一份 references 均在；正文含“四层组件”“三类任务”；`go test ./toolCalling -run TestReadSkill` 2/2 PASS | 无 |
| R3 | 达标 | `go test ./graph/ -race` ok；原子 claim、lease 回收、block 环检测均有测试 | 无 |
| R4 | 达标 | `go test ./graphserver/ -race` ok；实时 API：401 未授权 / 200 列表 / 201 创建与 Hint / 404 未知 / 422 非法 body | 无 |
| R5 | 达标（离线） | `TestLaneOfflineEndToEnd`、`TestPlan*`、`TestSelectWorker*`、`TestExecutor*`、`TestTick*` PASS | 真实模型/Docker 下未复跑 |
| R6 | 达标但缩水 | driver/native/ParseResult/registry 测试 PASS | native 只 import `llm`/`misc`，未接 `toolCalling` 循环（§12 已记录） |
| R7 | **仅单测** | `TestBootstrapWritesPatchProfileAndExploitIdeaFacts`、`TestBootstrapDeclaresInversionIntents` PASS | 真实库 `patch_profile` 事实 = 0；两个项目 `bootstrap_enabled=0`，bootstrap 从未真实执行 |
| R8 | 达标 | reason 三分支、from 校验、gap/hint prompt 测试 PASS | 无 |
| R9 | 部分 | 沙箱适配器、失败落客观结论、超时 fallback 测试 PASS | 真实项目只有 48 条 observation；没有 verdict，无法证明“实证→判决” |
| R10 | **仅单测** | `TestVerdictRequiresSupportOrRefuteEdge`、`TestCompleteRejectsFindingWithoutProvenance`、`TestReopenCreatesCorrectionFact` PASS | 真实库 verdict = 0、support/refute 边 = 0；生产代码无 `finding` 节点生产者；完成项目仅凭 observation 事实 `from` 到 goal |
| R11 | **仅单测** | `ApplyOutcome` 版本/置信度/hit/confirm 测试 PASS；`bootstrap.go:185` top-k 注入代码在 | 真实 `rules` 表 0 行，规则回流从未真实发生 |
| R12 | 达标（有缺口） | 实时 Hint 201 + export yaml 可见；reason prompt 含 hint 测试 PASS | 没有 ~100 token 的硬截断，仅靠提示词约束 |
| R13 | **未交付** | requirement §12 明示后置；现有 `GraphPanel`/`GraphOverview` 为既有前端（`6d918d2` 等），读 `/api/graph` | 无补丁面板、无 Finding 报告；上轮“面板已补齐”与 §12 口径冲突，以 §12 为准 |
| R14 | 部分 | `eval/` 三指标 JSON、双条件 N≥3 测试 PASS；finding 为代理口径 | 仓库无真实对照评测 JSON，真实数据无法产生 finding |
| R15 | **未验收** | 离线 `TestLaneOfflineEndToEnd`；`docs/runbooks/cve-2025-3248.md` 为操作步骤 | 无实机执行记录；真实规则库 0 行；G3 不成立 |
| R16 | 达标 | `graph_lane_enabled` 默认 false；`TestGraphLaneFlagDefaultsOff` PASS；实时 flag=off 时 `/api/graph` 落到 SPA、既有 `/projects` 正常 | 无 |

闸门：G0 部分（R1 未清）；G1 达标；G2 达标（离线）；G3 **未达标**；G4 部分（仅单测）。

真实图库快照（只读查询，2026-11-20）：

- `data/graph.db` 共 2 个项目、96 个节点、106 条边，**全部边为 `produce`**
- 节点构成：observation 48、gap intent 45、goal 2、origin 2、hint 2、exploit_idea intent 2、human_intent 1
- 缺失：`patch_profile`、`constraint`、`verdict`、`rule_hit`、`finding`；`derive`/`support`/`refute` 边为 0
- `data/patchintel/rules.db`：`rules` 表 0 行

### 2.2 复核后的产品面结论

前端只能看到一张溯源图，根因不是渲染，而是数据：图里没有补丁事实、没有判决、没有 finding。“按补丁往下推理”目前只以文字形式存在于 origin/observation 的 content 中，图结构无法自证。补齐 R13 或跑通真实 bootstrap/verdict 之前，不应对外宣称“完整反演闭环已落地”。

### 2.3 上一轮已完成（基线，不用重做）

| 项 | 证据 |
|---|---|
| `go build ./...` 通过 | 实测 exit 0（2026-11-16） |
| `skill/`、`resource/` 包可编译 | `go test ./toolCalling/` ok |
| `patchintel/` 三件套 | `go test ./patchintel/` ok |
| 四个补丁工具已挂载 | `agents/agent_builder.go:176-179,307-310` |
| langflow 场景模板 | `data/templates/langflow/template.md` |

---

## 3. Step 1（本轮唯一动作）：Cairn 架构与开发计划沉淀为 Skills

> 需求：R2。这是整个迭代的第一步：先让“怎么做”可被任何新会话加载，再动代码。

### 3.1 动作清单

- [x] **3.1.1** 初始化 Git 基线并连接 GitHub
  - 命令：`git init` → `.gitignore` → 拍平两个内嵌仓库 → `git commit` → `git push -u origin main`
  - 证据：本地 `7d0e635`；`git ls-remote origin main` → `7d0e635 refs/heads/main`；GitHub `default_branch=main`
  - 备注：SSH key `patchtrace-root@local` 认证成功（`Hi YaoanGuan!`）

- [x] **3.1.2** 创建 `cairn-architecture` Skill
  - 文件：`.claude/skills/cairn-architecture/SKILL.md`
  - 必须包含：四层组件、Fact/Intent/Hint、三类任务、lease 生命周期、唯一协议写入者
  - 证据：`grep -c '^---' .claude/skills/cairn-architecture/SKILL.md` → `2`
  - 证据：正文 `88` 行（`wc -l` 92 − frontmatter 4），≤120；`grep -nE '^## (四层组件|三个原语|三类任务|Intent 生命周期|Dispatcher 是唯一|导出与读取)'` → 6 个小节全部命中

- [x] **3.1.3** 补三份 references
  - 文件：`references/server-protocol.md`、`references/dispatcher-design.md`、`references/data-model.md`
  - 来源：`Cairn/docs/specs/server-protocol.md`、`Cairn/docs/specs/dispatcher-design.md`
  - 证据：`wc -l .claude/skills/cairn-architecture/references/*.md` → `193 / 184 / 118`

- [x] **3.1.4** 创建 `patchtrace-iteration` Skill
  - 文件：`.claude/skills/patchtrace-iteration/SKILL.md` + `references/workflow.md`
  - 必须包含：三文档读取顺序、TDD 步骤、todo 回写格式、证据要求
  - 证据：`grep -c '^---' ...` → `2`；正文 `52` 行；`grep -nE 'requirement\.md|task\.md|todo\.md|TDD|没有命令输出|阻塞'` → 路由与纪律全部命中

- [x] **3.1.5** 用新会话验证可读性
  - 方法：无上下文新会话，只读 `.claude/skills/cairn-architecture/SKILL.md`，回答四层组件/三原语/三任务/lease/唯一写入者/导出格式
  - 证据（fresh-context agent 回答要点，仅凭 SKILL.md）：四层＝Server(graph/)+Dispatcher(dispatch/)+Container(dockerManager/)+Worker(dispatch/worker/)；三原语＝Fact(节点)/Intent(边)/Hint(图外输入)；三任务＝bootstrap/reason/explore 及 complete|intent|空 输出契约；lease＝CAS 认领+心跳超时回收+409；导出＝yaml|timeline
  - 结论：六问全部答对，无“not stated”

- [x] **3.1.6** 提交
  - 命令：`git add .claude .gitignore docs/plans/todo.md && git commit -m "feat: add cairn-architecture and patchtrace-iteration skills"`
  - 证据：提交成功；`git status --porcelain` → 空（提交哈希见本轮报告）

### 3.2 完成标准（Definition of Done）

1. 两个 Skill 目录存在且 frontmatter 合法；
2. `cairn-architecture` 正文能在 120 行内说清四层与三任务，细节可跳 references；
3. `patchtrace-iteration` 能路由到三份文档并约束“无证据不打勾”；
4. Git 基线干净；
5. todo.md 本节全部打勾并附命令输出。

---

## 4. 后续 Task 跟踪（按 task.md 顺序执行）

> 每一节只展开“完成标准 + 验收命令”，步骤细节以 task.md 为准，避免两处漂移。

### Task 2 · 溯源图模型与存储（R3，闸门 G1）

- [x] `graph/` 包可编译；`Open/Close` 幂等
  - 证据：`CGO_ENABLED=0 go build ./...` → exit 0；`Close` 重复调用无错（评审未提异议）
- [x] 并发认领测试通过：`go test ./graph/ -run TestClaimIntentIsAtomic -race -v` → PASS (0.10s)
- [x] lease 回收测试通过：`go test ./graph/ -run TestLeaseExpiry -v` → PASS (0.07s)
- [x] block 环检测测试通过：`go test ./graph/ -run TestBlockEdgeCycleRejected -v` → PASS (0.10s)
- [x] 全包：`go test ./graph/ -race` → `ok AIxVuln/graph 1.559s`
- 完成标准：5 项全绿 + 一次提交 → `6f2a6b3`；独立评审 spec ✅ / quality Approved（7 Minor parked）。

### Task 3 · 溯源图查询与导出（R8，G1）

- [x] `Provenance` 返回 origin→finding 有序链
  - 证据：`TestProvenanceReturnsPatchToFindingPath` PASS；无路径 → `ErrNotFound`
- [x] `Gaps` 过滤被阻塞 Intent 并稳定排序
  - 证据：`TestGapsExcludesBlockedIntents` / `TestGapsExcludesClaimedBlocker` / `TestGapsSortsByCreatedAt` PASS；blocker 需 `state=resolved` 才解除阻塞
- [x] `ExportYAML` 四段结构；`ExportTimeline` 时间有序
  - 证据：`TestExportYAMLMatchesProtocolShape` / `TestTimelineIsChronological` PASS
- [x] `go test ./graph/ -race` → `ok AIxVuln/graph 2.197s`（16 个测试）
- 完成标准：独立评审 spec ✅ / quality Approved；修复轮复核 ADDRESSED。

### Task 4 · 图协议 API（R4，G1）

- [x] 生命周期路由全通（201/200）
  - 证据：`TestRoutesLifecycle` PASS；16 条路由全部注册于 `/api/graph`
- [x] 403/409/422 各有一条测试
  - 证据：`TestStoppedProjectRejectsWrites`(403) / `TestConflictStatusCodes`(409) / `TestInvalidBodyReturns422`(422) PASS
- [x] `go test ./graphserver/ -race` → `ok AIxVuln/graphserver`
  - 证据：8/8 PASS；`go vet ./graphserver/` → VET_EXIT=0
- 完成标准：独立评审 spec ✅ / quality Approved；6 Minor 延后至整支评审。

### Task 5 · 调度核心（R5，G2）

- [x] 初始态→bootstrap；新态势→reason；有 open intent→explore
  - 证据：`TestPlanPrefersBootstrapOnInitialProject` / `TestPlanReasonOnNewFacts` / `TestPlanReasonOnNewHints` / `TestPlanReasonWhenOpenIntentsDrained` / `TestPlanExploreWhenOpenIntentExists` PASS
- [x] 单项目 reason 唯一
  - 证据：`TestReasonOnlyOncePerProject` PASS（非空 `Project.Reason` 抑制 reason）
- [x] 配置缺项报错
  - 证据：`TestConfigRequiresTimeouts` PASS（缺 `explore.timeout` → error）
- [x] `go test ./dispatch/ -race` → `ok`
  - 证据：31 个顶层测试 + 4 子测试 PASS，`ok AIxVuln/dispatch 1.081s`；`go vet`/`gofmt` clean
- 完成标准：独立评审 spec ✅ / quality Approved（6 Minor 延后至整支评审）；runner 接线与并发上限按裁定归 Task 9。

### Task 6 · Worker driver（R6，G2）

- [x] `ParseResult` 覆盖合法/拒绝/缺字段/代码围栏
  - 证据：`TestParseResult*` 14 条 PASS（含拒绝、缺字段、代码围栏、bootstrap intents、complete.from）
- [x] native driver 能加载五份 prompt
  - 证据：`TestNativeExecuteRendersShippedTemplates` / `TestNativeConcludeRendersShippedTemplates` PASS；未解析占位符 → error
- [x] `go test ./dispatch/worker/` → `ok AIxVuln/dispatch/worker`
  - 证据：37 PASS；`-race` exit 0；`go vet` exit 0

### Task 7 · bootstrap（R7，G2）

- [x] diff → `patch_profile` Fact → N 个 inversion Intent
  - 证据：`TestBootstrapWritesPatchProfileAndExploitIdeaFacts`（两类 Fact）/ `TestBootstrapDeclaresInversionIntents`（2 条 from=[f001]）/ `TestBootstrapDeclaresOnlyMissingInversionIntents` PASS，`ok AIxVuln/dispatch/tasks 1.355s`
- [x] 无补丁源时明确报错且不写图
  - 证据：`TestBootstrapSkipsWhenNoPatchSource` PASS（error 且零 declare/claim/conclude/release）
- [x] `go test ./dispatch/tasks/ -run TestBootstrap -race` → `ok`
  - 证据：22 个 bootstrap 测试 PASS；`go vet` exit 0；`gofmt -l` 空
- 完成标准：评审 2 轮修复后全部 ADDRESSED；exploit_idea 事实经 `patchintel.Invert` 可注入 seam 写入，恢复路径可重入。

### Task 8 · reason（R8, R12，G3）

- [x] complete / intent / noop 三分支各一条测试
  - 证据：`TestReasonCompletesWhenGoalCovered` / `TestReasonDeclaresIntentWithValidFrom` / `TestReasonNoopWritesNothing` PASS；`accepted:false` 见 `TestReasonRejectsRefusedResult`
- [x] 非法 `from` 被拒绝
  - 证据：`TestReasonRejectsIntentWithUnknownFrom` PASS（`ErrInvalid`，零写入，释放租约）
- [x] prompt 含 gap 与 hint
  - 证据：`TestReasonPromptIncludesGapsAndHints` / `TestReasonPromptFallbackKeepsGraphContext` PASS（gap top-5、hint、graph YAML、fact ids、open intents）
- [x] `go test ./dispatch/tasks/ -run TestReason -race` → `ok`
  - 证据：40+ 测试 PASS（`ok AIxVuln/dispatch/tasks`、`ok AIxVuln/graph`）；`go vet`/`gofmt` 干净
- 完成标准：评审 1 轮修复后通过；`GapScore` 排序 OpenBlockers↑ → DepthFromOrigin↑ → IntentID↑。

### Task 9 · explore + 判决（R9, R10，G3）

- [x] 正常结论写 observation Fact
  - 证据：`TestExploreConcludesObservationFact` PASS（`ConcludeIntent(...,"observation")`）
- [x] 超时走 conclude fallback
  - 证据：`TestExploreTimeoutFallsBackToConclude` / `TestExploreTimeoutReleaseUsesDetachedContext` PASS（detached ctx 仍能落结论/释放）
- [x] 失败仍写客观结论
  - 证据：`TestExploreFailureStillWritesObjectiveFact` / `TestExploreSandboxVerificationFailureStillWritesFact` PASS
- [x] 无溯源路径的 finding 不能 complete
  - 证据：`TestCompleteRejectsFindingWithoutProvenance` PASS（`ErrInvalid`，零写入）；`origin` 例外
- [x] 经 dockerManager 实证（R9 核心）
  - 证据：`dockerManager/exec_capture.go` `ExecCapture`；`DockerSandboxRuntime` 注入 resolver；`TestExploreRunsSandboxVerification` PASS
- [x] `go test ./dispatch/tasks/ ./graph/ -race` → `ok`
  - 证据：`ok AIxVuln/dispatch/tasks`、`ok AIxVuln/graph`、`ok AIxVuln/dispatch/runtime`、`ok AIxVuln/dockerManager`；`go build ./...` exit 0；vet/gofmt 干净
- 完成标准：评审 1 轮（Critical+Important）修复后 scoped re-review 全部 ADDRESSED；提交 `59b12d5`+`ffb5a4e`；6 Minor 延后至整支评审。

### Task 10 · CE4 回流（R11，G3）

- [x] confirm_count 自增、置信度变化
  - 证据：`TestConfirmedVerdictIncrementsConfirmCount` PASS（1→2，v1 保留）；`TestFailedVerdictLowersConfidence` PASS（in-place）
- [x] 同 pattern 新版本 version+1，旧版保留
  - 证据：`TestNewPatternPromotedAsNewVersion` PASS；`rule_hit` 事实 + produce 边
- [x] `go test ./dispatch/tasks/ -run TestConfirmedVerdict -race` → `ok`
  - 证据：`ok AIxVuln/dispatch/tasks`、`ok AIxVuln/graph`、`ok AIxVuln/patchintel`、`ok AIxVuln/toolCalling`；vet/gofmt 干净
- 完成标准：评审 1 轮 3 项 Important 修复后 scoped re-review 全部 ADDRESSED；5 Minor 延后。

### Task 11 · 评测与端到端（R13–R16，G4）

- [x] patcheval 双条件 N≥3 产出三指标 JSON
  - 证据：`TestPatchevalRunsDualConditionAndReportsMetrics` PASS（6 runs，Metrics 三字段非零）；`TestPatchevalRequiresN` PASS；NDJSON 行 schema 由测试逐行断言
- [x] `graph_lane_enabled` 默认 `false`
  - 证据：`misc/ConfigDB.go` 默认值 `false`；`TestGraphLaneFlagDefaultsOff` PASS；main.go 仅在开启时启动车道
- [x] CVE-2025-3248 runbook 可从零复现
  - 证据：`docs/runbooks/cve-2025-3248.md`（靶标、diff 导入、lab UUID 作为图项目 id、沙箱容器 preflight、验收命令、timeline、降级路径）；车道路由已挂载且带 Bearer 鉴权；`GET /projects/:name` 暴露 `sandboxContainerID`（`TestGetProjectReportsSandboxContainerID` PASS）
  - 遗留：本环境无 Docker/LLM，真实 CVE 实弹未执行（裁定见 requirement §12）；离线等价闭环见 `dispatch/lane_outcome_test.go` 与 `dispatch/lane_e2e_test.go`
- [ ] `go build ./...` exit 0；`go test ./...` 无 FAIL
  - 证据：`go build ./...` exit 0；`go test ./...` 仅 `ProjectManager.TestEnsureProjectOverviewSandboxReusesExistingSandbox` FAIL（已在本分支 base 复现，与改造无关；未修）
- [ ] 新旧车道对照数据落盘（路径写在此处）
  - 现状：`eval.Report.JSON()` 可输出逐条对照 NDJSON；**未**落盘为仓库内固定路径产物（未打勾）
- [x] 最终评审修复轮（R10 证据闸门、verdict 溯源、R14 finding 口径、R11 规则库接线、R15 runbook、R6 health 窗口）
  - 证据：`f118081`..`e83793f`；scoped re-review 全部 ADDRESSED；`go build ./...` BUILD_OK；`go test ./graph/ ./graphserver/ ./dispatch/... ./eval/ -race` 全 `ok`

### Task F · 前端图驱动首页与旧入口保留（2026-11-17）

- [x] 三份 front 文档：`front_requirement.md` / `front_task.md` / `front_todo.md`
  - 证据：均含 §0“上一轮移除旧入口”复盘；R1–R16 → 前端落点矩阵；状态回写到 F7。
- [x] 品牌与首页：`ParchTrace | 寻迹 - Autonomous Penetration System` + `Trace the Patch, Uncover the Flaw.`
  - 证据：`webui/index.html` 1 处、`App.tsx` 标题 1 处 / slogan 3 处；`App.css` 含 `.aix-brand` / `.aix-slogan`。
- [x] `GraphOverview` 首页总览：指标条、01–07 模块、R1–R16 矩阵、图项目速览、旧入口 CTA
  - 证据：`GraphOverview.tsx`；只读聚合 `/api/graph/projects` + 最多 5 个 snapshot；R14 标注无 HTTP 端点、不伪造数据。
- [x] 旧能力红线：实验室列表 + 四种来源创建 + sandbox 自动执行入口保留在首页
  - 证据：`App.tsx` 中 `startProject/stopProject/enterProject/deleteProject` 绑定在 `4409–4421`；四来源按钮 `4455`；`#legacy-labs` 位于 `<GraphOverview>` 之后。
- [x] 构建闸门：`npx tsc --noEmit` exit 0；`npm run build` exit 0；`CGO_ENABLED=0 go build` exit 0
  - 证据：2026-11-17 三条命令均 exit 0；`go build` 输出 `go_build_exit=0`。
- [x] 推送：`f94a941` + `5810293` + `e008d0f` → `origin/main`
  - 证据：`git push origin HEAD:main` 输出 `6d918d2..f94a941`、`e8de8d2..5810293` 与 `5810293..e008d0f`。`e008d0f` 修复首页按 `created_at` 选取最新项目，`graphOrder.test.mjs` RED→GREEN（5/5）。
- [x] 重启与终版验证：9999 新二进制 + 首页 / 图面板截图
  - 证据：日志 `图调度车道已启动，配置: dispatch.yaml`；`/` 200、`/api/graph/projects` 401；服务端 bundle 含 R 矩阵与保留入口；`/tmp/r13/front-home-final.png`；图面板 `panel/legend/export_yaml/export_timeline/hint/intent/reason/capability/projects_loaded` 全 True。
  - 遗留：R14 仍无 HTTP 端点；Finding 报告后置。
- [x] 后端图数据深链适配：模块卡与项目速览选中目标项目，事实区按类型过滤
  - 证据：`GraphOverview` 传 `projectId + factType`，`GraphPanel` 支持 `initialProjectId` / `initialFactType`；隔离实例 CDP 11/11（速览选中 `probe-5`、模块 01 选中 `probe-6`、两类事实过滤互斥）；`tsc`/`npm build`/`go build` exit 0。

---

## 5. 当前阻塞

| 日期 | Task | 现象 | 需要谁决策 | 状态 |
|---|---|---|---|---|
| 2026-11-20 | R7/R10/R11/R15 | 真实 `graph.db` 无 `patch_profile`/`verdict`，`rules.db` 0 行；实弹未跑 | 用户提供 Docker + LLM 配置并批准实弹执行 | 阻塞 |
| 2026-11-20 | R1 | `git status` 未清（`dispatch/data/` 等 6 项未跟踪） | 用户决定未跟踪文件的去留/忽略规则 | 待决策 |
| 2026-11-20 | R13 | 上轮称“面板已补齐”，但 requirement §12 仍记后置；实为既有前端读图 API | 用户确认以 §12 为准，并在交付说明中写明 Finding 报告未交付 | 待决策 |

---

## 6. 节奏与时间盒

| 阶段 | 内容 | 预估（有测试脚手架） |
|---|---|---|
| 第 1 天 | Task 1–3（地基 + 图模型 + 查询） | 6–7 小时 |
| 第 2 天 | Task 4–5（协议 + 调度） | 6–7 小时 |
| 第 3 天 | Task 6–7（driver + bootstrap） | 5–6 小时 |
| 第 4 天 | Task 8–9（reason + explore） | 6–7 小时 |
| 第 5 天 | Task 10–11（回流 + 评测 + 端到端） | 6–8 小时 |

估算前提：单测无需真实 Docker / LLM；若需要实弹靶标或真实模型，单日预算按 ×1.5–2 计。

---

## 7. 状态回写规则

1. 只在有命令输出时打勾；输出贴在该项下一行。
2. 状态变化同步更新 §2 总览表。
3. 新增字段或改接口，先改 `requirement.md` §12，再改代码。
4. 每轮结束必须留下：`已完成 / 证据 / 下一轮唯一动作` 三行。
5. 连续两轮同一 Task 无进展 → 在 §5 记阻塞，换实现路径，不重复试同一招。
