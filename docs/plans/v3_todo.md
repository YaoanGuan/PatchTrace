# PatchTrace v3 执行跟踪（v3_todo）

当前轮次：v3 第 0 轮（设计已拍板，实施未开始）· 2026-11-20
规格：`v3_re.md`（需求 V3-R1…R16）· 计划：`v3_task.md`（任务 T0…T16）

## 0. 下一轮唯一动作

**Task 1 Step 1：在 `decision/decider_test.go` 写下 `TestJudgeBool/Choice/Rank/Score` 四个失败测试。**

理由：T0 被 Docker/LLM 阻塞，但 C1（T1–T4）完全离线，先动不依赖环境。T0 并行推进，未跑通前 C2 不得验收。

> 执行纪律：本文件的状态列只认命令输出。没有“本轮跑过的命令 + 原始输出”，一律记 `未开始`，不写“基本完成”。

## 1. 执行协议

- 一轮只做一个 Task；做完即回写本文件的状态与证据。
- 每个 Task 走 `红 → 验红 → 实现 → 验绿 → 提交` 五步；提交只在你明确要求时执行。
- 状态词只有四个：`未开始` / `进行中` / `阻塞` / `已验收`。
- `已验收` 的门槛：Task 内全部测试命令输出 PASS，且证据块贴在本文件对应小节。
- 任何闸门（G0–G5）未过，对应能力不得在本文或对用户的汇报里写“已落地”。

## 2. 总览

| Task | 阶段 | 交付 | 对应需求 | 闸门 | 状态 |
|---|---|---|---|---|---|
| T0 | 基线 | 真实 CVE 跑通记录 | R14 前置 | G0 | 阻塞（Docker+LLM） |
| T1 | C1 | `decision/` 契约与启发式 | R1 | G1 | 未开始 |
| T2 | C1 | claim 落库与签名 | R2 | G1 | 未开始 |
| T3 | C1 | `evidence/` 证据信封 | R3 | G2 | 未开始 |
| T4 | C1 | planner/executor 接入 + flag | R1/R15 | G1 | 未开始 |
| T5 | C2 | goja 运行时 + 静态检查 | R4/R16 | G2 | 未开始 |
| T6 | C2 | 短路/落回/quarantine | R5/R16 | G3 | 未开始 |
| T7 | C2 | 内置 reflex ×3（http/evidence/stop） | R8 | G3 | 未开始 |
| T8 | C2 | 生成管线（阈值 + 5 并行） | R6 | G3 | 未开始 |
| T9 | C2 | 回放/入库/版本/置信度 | R7 | G3 | 未开始 |
| T10 | C3 | 命令护栏 fail-closed | R9 | G4 | 未开始 |
| T11 | C3 | POC 安全分级 | R10 | G4 | 未开始 |
| T12 | C3 | 指纹路由 + fingerprint_lookup | R11/R8 | G4 | 未开始 |
| T13 | C3 | 旁路优化器 + A/B | R12 | G5 | 未开始 |
| T14 | C3 | eval 指标 | R13 | G5 | 未开始 |
| T15 | C3 | 真实渗透验收 | R14 | G5 | 未开始 |
| T16 | 收口 | flag 矩阵/构建/可观测 | R15 | G1–G5 | 未开始 |

## 3. 闸门

| 闸门 | 条件 | 过闸证据 |
|---|---|---|
| G0 | `patch_profile≥1`、`verdict≥1`、`rules≥1`、≥3 同签名 claim | T0 runbook + SQL 输出 |
| G1 | 四类判断 + claim 审计 + 签名稳定 + flag 关闭行为不变 | `go test ./decision/ ./dispatch/ -race` |
| G2 | 证据契约 + 运行时权限 | `go test ./evidence/ ./reflex/ -race` |
| G3 | 命中/落回/quarantine + 内置 reflex + parity=0 | `go test ./dispatch/ ./reflex/... -race` |
| G4 | 护栏 + POC 分级 + 指纹路由 | `go test ./safety/ ./fingerprint/ -race` |
| G5 | 优化器 + 指标 + 真实验收 | eval JSON + T15 runbook |

## 4. 任务清单与证据

### T0 真实基线（阻塞）

- [ ] 环境预检：`docker info` 成功；`/api/graph/projects` 带 token 返回 200；`main_setting.MODEL` 非空
- [ ] 建真实 diff 的图项目（`bootstrap_enabled=true`）
- [ ] 跑完 bootstrap → reason → explore，导出 yaml + timeline
- [ ] SQL 验收达标
- [ ] 写 `docs/runbooks/cve-2025-3248-run-2026-11-20.md`

阻塞原因：本机当前无可用 Docker daemon / 模型配置（2026-11-20 复核结论）。
证据：_待填_

### T1 decision 契约

- [ ] 四个 `TestJudge*` 失败测试
- [ ] `go test ./decision/ -run TestJudge -v` 确认 FAIL
- [ ] 实现 `Claim/Result/Decider/Heuristic`
- [ ] `go test ./decision/ -race` PASS
- [ ] （待用户要求）提交

证据：_待填_

### T2 claim 落库与签名

- [ ] `TestSignature*`、`TestClaimStoreConcurrentSave`
- [ ] 验红
- [ ] SQLite `claims` 表 + `Signature()`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T3 evidence 信封

- [ ] 五个测试（capture/redact/parse/reject/truncate）
- [ ] 验红
- [ ] `Capture/Encode/Parse/Redact`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T4 接入与 flag

- [ ] 五个 dispatch 测试（不变/延后/优先级/单次记录/失败回落）
- [ ] 验红
- [ ] `misc.reflex_enabled=false` 等 8 个配置项
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T5 goja 运行时

- [ ] 六个测试（越权/超时/输出上限/无宿主/证据/静态检查）
- [ ] 验红
- [ ] 白名单原语 + `vm.Interrupt` + parser 静态检查
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T6 短路与落回

- [ ] 六个测试（跳过 LLM/落回/quarantine/无双写/parity/stop）
- [ ] 验红
- [ ] Matcher + ApplyFunc 注入 + 计数
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T7 内置 reflex

- [ ] 四个测试（http/evidence/stop/显式命令）
- [ ] 验红
- [ ] `http_probe`、`evidence_collect`、`stop_on_marker`
- [ ] 验绿
- [ ] （待用户要求）提交
- [ ] T0 跑通后补真实调用记录（G3 要求）

证据：_待填_

### T8 生成管线

- [ ] 五个测试（阈值/5 并行/越权拒绝/去重/token 计数）
- [ ] 验红
- [ ] `Generator` + `prompts/reflex/generate.md`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T9 回放与版本

- [ ] 五个测试（parity 拒绝/升版/quarantine/置信度/退役）
- [ ] 验红
- [ ] `Replay/Promote/Quarantine`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T10 命令护栏

- [ ] 四个测试（fail-closed/危险拒绝/未知审批/覆盖留痕）
- [ ] 验红
- [ ] `Guardrail` 接入 `aix.exec` 与 explore
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T11 POC 分级

- [ ] 五个测试（validation 拦截/unknown 默认/显式 full/分类失败/覆盖审计）
- [ ] 验红
- [ ] `ClassifyPOC` + `engagement_mode`
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T12 指纹路由

- [ ] 四个测试（命中/无命中回退/不伪造/内置 reflex）
- [ ] 验红
- [ ] `Router` + `fingerprint_lookup`（第 4 条内置 reflex）
- [ ] 验绿
- [ ] （待用户要求）提交
- [ ] T0 跑通后补 `fingerprint_lookup` 真实调用记录（R8 要求四条各有记录）

证据：_待填_

### T13 优化器

- [ ] 四个测试（样本不足/A/B 胜出/回滚/成本计数）
- [ ] 验红
- [ ] 1/10 采样 + A/B
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T14 指标

- [ ] 四个测试（短路率/token 每 finding/parity/finding 口径）
- [ ] 验红
- [ ] `V3Metrics` + JSON 输出
- [ ] 验绿
- [ ] （待用户要求）提交

证据：_待填_

### T15 真实验收

- [ ] T0 同场景、开 v3 flag 重跑
- [ ] `short_circuit_rate≥20%`、token/finding ↓≥25%、finding 不降、`safety_incidents=0`
- [ ] 写 runbook 并贴原始输出
- [ ] 未达标不开 G5

证据：_待填_

### T16 收口

- [ ] flag 矩阵测试（全关/单开/全开）
- [ ] `CGO_ENABLED=0 go build ./...` exit 0
- [ ] 相关包 `go test -race` 全绿
- [ ] 红线检查：无 `import "AIxVuln/Web"`；工厂下标未动；prompt 无流程分支
- [ ] 日志字段：claim id、signature、reflex id/version、quarantine 原因

证据：_待填_

## 5. 证据模板

```text
### <Task> 证据 · <日期>
命令：`<完整命令>`
退出码：<0/N>
输出（粘贴原始、含 PASS/FAIL 行）：
<...>
结论：<已验收 | 未达标 + 原因>
```

## 6. 阻塞与风险

| 项 | 状态 | 影响 | 解除条件 |
|---|---|---|---|
| T0 Docker/LLM | 阻塞 | G0/G3 真实验收、T15 全部不可做 | daemon 可用 + 模型配置 |
| git 工作区脏 | 已知 | R1 兼容性证据不完整 | 提交或清理用户批准 |
| 生成式 reflex 无真实样本 | 依赖 T0 | T8/T13 只能单测验收 | T0 产出 ≥3 同签名 claim |

## 7. 决策记录

| 日期 | 决策 | 理由 |
|---|---|---|
| 2026-11-20 | 选 C（Model × Harness 自进化） | 用户拍板 |
| 2026-11-20 | goja 作为 reflex 引擎 | 纯 Go、单二进制、可中断 |
| 2026-11-20 | reflex 只读图 + 执行命令；写图由 runner 统一做 | 避免双写与图协议分叉 |
| 2026-11-20 | 内置 reflex 先于生成式 | 生成式依赖真实样本 |
| 2026-11-20 | 首个验收场景 CVE-2025-3248 | 与现有 runbook/数据一致 |

## 8. 验收口径

- **verified finding**：observation（带 Evidence 信封）+ support 关系的 verdict + 可复现命令。旧代理口径只记 `legacy_proxy_finding`，不计入 v3 指标。
- **短路成功**：reflex 命中且产出证据、未调用主模型、parity=0。
- **安全事件**：`safety_incidents>0` 即本轮不通过，不论其他指标多好。
