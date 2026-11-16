# PatchTrace 待办与进度跟踪（todo.md）

> 配套：`docs/plans/requirement.md`（需求）、`docs/plans/task.md`（实施计划）
> 更新规则：每完成一步立即回写；没有命令输出就不打勾。
> 当前轮次：第 1 轮 · 2026-11-16

---

## 0. 下一步动作（只做这一件）

> **Task 4 已完成（G1 图协议 API 通过）**：提交 `fc3fc1e`，独立评审 spec ✅ / quality Approved，0 Critical / 0 Important。
> `go test ./graphserver/ -race` → 8/8 PASS；`go build ./...` exit 0。
> 下一动作：按预审裁定顺序执行 Task 6 · Step 1，写 `dispatch/worker/native_test.go` 失败测试（ParseResult 契约）。

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
| 5 | 调度核心 | R5 | 未开始 | G2 | 预审裁定：T6 先行 |
| 6 | Worker driver 与 native 实现 | R6 | 进行中 | G2 | — |
| 7 | bootstrap 补丁反演任务 | R7 | 未开始 | G2 | — |
| 8 | reason 溯源图推理任务 | R8, R12 | 未开始 | G3 | — |
| 9 | explore 沙箱实证与判决 | R9, R10 | 未开始 | G3 | — |
| 10 | CE4 规则回流 | R11 | 未开始 | G3 | — |
| 11 | 评测、端到端与灰度 | R13–R16 | 未开始 | G4 | — |

状态取值：`未开始` / `进行中` / `阻塞` / `完成`。

### 2.1 上一轮已完成（基线，不用重做）

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

- [ ] 初始态→bootstrap；新态势→reason；有 open intent→explore
- [ ] 单项目 reason 唯一
- [ ] 配置缺项报错
- [ ] `go test ./dispatch/ -race` → `ok`

### Task 6 · Worker driver（R6，G2）

- [ ] `ParseResult` 覆盖合法/拒绝/缺字段/代码围栏
- [ ] native driver 能加载五份 prompt
- [ ] `go test ./dispatch/worker/` → `ok`

### Task 7 · bootstrap（R7，G2）

- [ ] diff → `patch_profile` Fact → N 个 inversion Intent
- [ ] 无补丁源时明确报错且不写图
- [ ] `go test ./dispatch/tasks/ -run TestBootstrap -race` → `ok`

### Task 8 · reason（R8, R12，G3）

- [ ] complete / intent / noop 三分支各一条测试
- [ ] 非法 `from` 被拒绝
- [ ] prompt 含 gap 与 hint
- [ ] `go test ./dispatch/tasks/ -run TestReason -race` → `ok`

### Task 9 · explore + 判决（R9, R10，G3）

- [ ] 正常结论写 observation Fact
- [ ] 超时走 conclude fallback
- [ ] 失败仍写客观结论
- [ ] 无溯源路径的 finding 不能 complete
- [ ] `go test ./dispatch/tasks/ ./graph/ -race` → `ok`

### Task 10 · CE4 回流（R11，G3）

- [ ] confirm_count 自增、置信度变化
- [ ] 同 pattern 新版本 version+1，旧版保留
- [ ] `go test ./dispatch/tasks/ -run TestConfirmedVerdict -race` → `ok`

### Task 11 · 评测与端到端（R13–R16，G4）

- [ ] patcheval 双条件 N≥3 产出三指标 JSON
- [ ] `graph_lane_enabled` 默认 `false`
- [ ] CVE-2025-3248 runbook 可从零复现
- [ ] `go build ./...` exit 0；`go test ./...` 无 FAIL
- [ ] 新旧车道对照数据落盘（路径写在此处）

---

## 5. 当前阻塞

| 日期 | Task | 现象 | 需要谁决策 | 状态 |
|---|---|---|---|---|
| — | — | 无 | — | — |

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
