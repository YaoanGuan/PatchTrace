# PatchTrace v3 需求：JEV 决策 × Reflex 自进化执行系统（v3_re）

> 配套：`v3_方案.md`（选型与原始思路）、`v3_task.md`（实施计划）、`v3_todo.md`（执行跟踪）。
> 本文件定义“做什么、为什么、怎么算通过”，不定义代码细节。

---

## 0. 战略输入（P10）

### 方向

把渗透测试执行链中**机械、重复、可判定**的部分，从「每次调用 LLM 重新思考」改成「决策模型路由 + Reflex 程序短路」，在不降低证据质量的前提下，降低 token 成本与到达验证的时间；未命中或证据不足时，仍回落到现有 LLM 车道。

### 成功标准（可量化）

| 指标 | 目标（首版） | 口径 |
|---|---|---|
| `tokens_per_verified_finding` | 比 T0 基线下降 ≥25% | 只统计真实 finding，不用代理口径 |
| `short_circuit_rate` | ≥20%（目标 ≥35%） | 命中 reflex 且证据与 LLM 路径一致的任务占比 |
| `parity_failures` | 0 | reflex 与 LLM 路径结果不一致的次数 |
| `time_to_verified_finding` | 下降 ≥20% | 项目创建到第一条 verified finding |
| `verified_findings` | 不低于基线 | 不得用省 token 换少发现 |
| `safety_incidents` | 0 | 越权命令、绕过审批、越界目标 |
| `reflex_regression_rate` | <5% | 升版后回放失败率 |

### 约束

- 技术：Go 1.24、单二进制、`CGO_ENABLED=0`、SQLite（`modernc.org/sqlite`）、不 import `Web/`。
- 兼容：`misc.reflex_enabled=false` 时与当前车道逐字节行为一致；不修改 graph 协议与任务状态机。
- 安全：只在授权靶标与沙箱内执行；审批流不可绕过；所有 reflex 执行留审计。
- 资源：决策默认不调用主 LLM；生成与优化成本必须计入 token 账。

### 风险预判

1. **短路固化错误经验** → 回放调试 + parity 闸门 + quarantine + 版本回滚。
2. **决策误杀高价值 intent** → 低置信度一律回落；决策只路由，不判漏洞。
3. **reflex 逃逸沙箱/绕过审批** → goja 白名单 + 静态检查 + 注入式审批器，默认拒绝。
4. **省 token 但 finding 变少** → `verified_findings` 设为硬门槛，不达标即回滚。
5. **签名过拟合导致零命中** → 签名只含结构化特征，跨项目归一化，上线前用历史 claim 回放验证。

### 不做什么（本版）

- 不做多租户、权限模型重构、向量规则库。
- 不做 UI 大改；只加只读观测字段。
- 不做自动外网攻击、不做超出授权范围的扫描。
- 不做新的存储引擎；不重写 graph/dispatch。
- 不把流程分支写进 prompt；模型仍不可见状态机。

### 工作流编制（P9 视角）

| 工作流 | 范围 | 交付 |
|---|---|---|
| P9-A 决策与证据 | `decision/`、`evidence/`、planner 接入 | Claim 契约、签名、证据信封 |
| P9-B Reflex | `reflex/`、executor 短路、生成与版本 | 运行时、内置 reflex、生成管线 |
| P9-C 安全与评测 | `safety/`、`fingerprint/`、`reflexopt/`、eval | 护栏、POC 分级、指纹路由、A/B |

接口边界：A 产出 `Claim/Result/Evidence`；B 只消费这些类型并返回 `Outcome`；C 只通过 B 的匹配器与检查器接入，不改协议。

---

## 1. 背景与问题

### 1.1 来源思路（翻译）

- **JEV 决策模型**：decode 不再生成整段文字，而是输出布尔、单选、排序、分数；本质是分类器 + 路由。
- **Claim**：一次决策 = 内容 + 选项 + 结果，可审计、可回放。
- **Reflex**：把重复执行经验封装成程序；同场景直接短路，可中途终止；不占 prompt，避免 skills 上下文污染。
- **自进化**：reflex 由多个 claim 生成、调试、入库、命中、淘汰；每 10 次抽 1 次用旁路模型优化自身。
- **护栏/指纹/POC**：同一分类能力可用于三种地方——拦危险动作、从文本提取指纹来源、对 POC 做安全分级。

### 1.2 当前仓库事实（2026-11-20 审计）

- `data/graph.db`：2 个项目、96 节点、106 条边，**全部是 `produce` 边**；`patch_profile`、`verdict`、`finding` 均为 0；`rules.db` 0 行。
- 前端能看到一张 observation/gap 图，但无法从结构上证明“补丁 → 约束 → 思路 → 观测 → 判决”。
- 因此 v3 的第一前提不是加 reflex，而是**先有真实、可重复的渗透执行基线**（T0）。

### 1.3 为什么现在做

- 渗透规模化后 token 是主要成本；原文口径约 1/3 调用是机械的。
- 现有 native driver 是一次性补全，没有工具循环，最适合在外部套决策与短路。
- eval 已有 `tokens_per_finding`、`avg_trial_rounds`、`inversion_success_rate`，能验证收益。

---

## 2. 目标 / 非目标 / 反目标

### 2.1 目标

1. 让重复的渗透动作（探测、取证据、指纹确认、已知结论终止）变成可复用 reflex。
2. 让「下一步做什么」由廉价的决策层先判，减少主模型调用。
3. 保证 reflex 路径与 LLM 路径产出**同构证据**，可审计、可回放、可对比。
4. 把护栏、POC 分级、指纹路由接进真实渗透流程，而不是只做 benchmark。

### 2.2 非目标

- 不追求全自动漏洞挖掘；人仍是授权与高风险动作的最终决策者。
- 不用 reflex 替代 verdict；reflex 只能产出 observation/候选结论，verdict 仍需证据闸门。
- 不优化“看起来像 finding”的文本；只认真实命令与输出。

### 2.3 反目标（即使技术可行也算失败）

- 省了 token 但 verified finding 减少。
- reflex 命中率靠放宽签名造假。
- 决策层直接断言“存在漏洞”而不执行命令。
- 护栏/ POC 分级误判导致越权或越界执行。

---

## 3. 用户与场景

### 3.1 用户

- 执行授权渗透测试的安全工程师（operator）。
- 审阅证据与报告的安全负责人（reviewer）。
- 维护 reflex 库与规则库的平台工程师。

### 3.2 必测场景

| 场景 | 输入 | 期望 | 验收证据 |
|---|---|---|---|
| S1 补丁反演 | Langflow 修复 diff | bootstrap 产出 `patch_profile` + `exploit_idea`；explore 产出 `observation`；verdict 带 support/refute | timeline + 事实/边 + 命令输出 |
| S2 指纹到 POC | 目标响应文本 | 指纹路由到组件/版本，命中规则或 POC；无命中回退现有搜索 | 路由记录 + 命中依据 |
| S3 重复验证短路 | 同签名的探测动作重复 ≥3 次 | 第 4 次起命中 reflex，零主模型调用，证据与 LLM 路径一致 | `reflex_runs` + parity 对比 |
| S4 反证与误报 | 命令显示入口不存在/被阻断 | 写客观失败 observation，refute 对应 intent，不得 complete | 失败结论 + refute 边 |
| S5 人类 Hint | 操作者注入判断 | 下一次 reason/explore 可见；reflex 命中时也不得跳过 hint 语义 | export yaml + prompt 记录 |

---

## 4. 概念契约

### 4.1 JEV 决策

```go
type Judgment int

const (
    JudgeBool Judgment = iota // 是/否
    JudgeChoice               // n 选一
    JudgeRank                 // 排序
    JudgeScore                // 打分
)

type Claim struct {
    ID        string
    ProjectID string
    TaskType  string            // bootstrap | reason | explore | guardrail | poc | fingerprint
    Point     string            // binding（工具调用准入）| completion（报告后是否完成）
    Signature string
    Content   string            // 判断什么（客观描述）
    Options   []string          // 封闭选项；必须含 "defer"
    Features  map[string]any    // 结构化特征，不含目标隐私
}

type Result struct {
    Bool          bool
    Choice        string             // 选中的选项；reflex 候选以 reflex ID 出现
    Rank          []string           // 按概率降序
    Probabilities map[string]float64 // 与 Rank 对齐，和 ≤ 1
    Score         float64
    Confidence    float64
}

type Decider interface {
    Judge(ctx context.Context, c Claim) (Result, error)
}
```

规则：

- 默认 `HeuristicDecider`（本地、确定性、零 token）；模型决策必须走 `decision` 配置段、按签名缓存、有预算上限。判断是一组封闭选择，允许本地小模型回答。
- **两个判断点**：`binding` 在每次工具调用前问“允许执行还是 defer”；`completion` 在报告返回后问“完成还是继续”。其余判断（bootstrap/reason 路由）复用同一契约。
- 选项按概率排序输出（例：reflex `rb3218…` 86% / `defer` 14%，confidence 72%）；`defer` 表示“本 reflex 不适用，交回 Reason”。
- 决策**只回答“下一步”**，不得输出“存在漏洞”。
- 未知 claim 返回 `ErrInvalid`；决策服务不可用返回 `ErrUnavailable` 并回落。

### 4.2 签名

```go
type Features struct {
    FactCount, IntentCount, OpenIntents, ObservationCount int
    TypeHistogram map[string]int
    IntentClass   string // gap | exploit_idea | human_intent | ...
    ToolClass     string // http | shell | file | browser | ...
    CommandClass  string // 归一化命令模板（目标名替换为占位符）
    ResponseClass string // 状态桶/内容类型/长度桶/标记集合
}
```

签名 = `sha256(canonical_json(features))[:16]`；不得包含 URL、域名、凭据、完整响应体。

### 4.3 证据信封（Evidence）

```go
type Record struct {
    Command    string    `json:"command"`
    ExitCode   int       `json:"exit_code"`
    Stdout     string    `json:"stdout"`
    Stderr     string    `json:"stderr"`
    SandboxID  string    `json:"sandbox_id"`
    StartedAt  time.Time `json:"started_at"`
    EndedAt    time.Time `json:"ended_at"`
    OutputHash string    `json:"output_hash"`
    Truncated  bool      `json:"truncated"`
    Redacted   []string  `json:"redacted"`
}
```

落库前必须脱敏：`Authorization`、`Cookie`、`Set-Cookie`、API Key、密码字段。observation fact 的 content 保留人类摘要，并附可解析的 evidence 块；不改变 graph 协议。

### 4.4 Reflex

```go
type Reflex struct {
    ID, Name, Signature string
    Language            string // js（goja）
    Entry               string // 固定为 "run"
    ArgsSchema          string // JSON Schema；描述 a 的字段，不含目标隐私
    Source              string
    Version             int
    Status              string // active | quarantined | retired
    Confidence          float64
    Hits, Confirms, Fails int
}

type Outcome struct {
    Evidence   evidence.Record
    Fact       string          // 人类可读结论
    Stop       bool            // 终止当前链
    Verdict    *VerdictInput   // 可选，仍由 runner 走证据闸门
    TokensUsed int64
}
```

reflex 是**参数化程序**，入口固定 `function run(a, read, step, occurrence)`：

```js
function run(a, read, step, occurrence) {
  var request = {name: 'bash', arguments: {
    command: command('playwright', a), read: read, step: step, occurrence: occurrence,
  }};
  if (read) { request.step = step; request.occurrence = occurrence; }
  return execute(request);
}
```

- `a` 是本次任务参数（如 `url`/`fields`）；同一段程序换一组 `a` 即复用，**程序不变**。签名只描述“什么时候适用”，不包含 `a` 的具体值。
- `execute(request)` 是 harness 工具边界：先过 `binding` 判断（accept/defer），再落到沙箱/工具执行；reflex 不自带网络、不直接调用 Docker。
- 程序内部保留运行时判断（循环、条件、读回校验）；未覆盖情形 `return {defer: true}` 交回 Reason。
- reflex 只返回 Outcome 或 defer；**写图由现有 runner 执行**，reflex 不得直接调用 graph 写接口。

形成路径（与图片一致）：

| 阶段 | 动作 | 本仓库落点 |
|---|---|---|
| 01 真实任务 | 当前任务、约束、真实工具结果 | explore 的一次完整 claim + evidence |
| 02 提炼判断 | 形成 Claim，明确适用条件 | `claims` 表 + `Signature()` |
| 03 构造 Reflex | 组织处理逻辑，保留运行时判断 | LLM 生成 `run()` 源码 + ArgsSchema |
| 04 校验入库 | 验证可执行性，进入能力候选库 | Replay parity=100% → `reflexes` → candidate→published |

Skill 与 Reflex 的区别：Skill 是经验文本，每次读入后重新理解、重新规划；Reflex 是已编译路径，短路“重新理解/重新规划”，只在判断点问 JEV。

---

## 5. 需求与验收

### V3-R1 决策契约与默认零 token

- 实现 `decision/` 四类判断；默认启发式；模型决策可选、按签名缓存、有预算。
- 覆盖两个判断点：`binding`（工具调用准入，accept/defer）与 `completion`（报告后是否完成）；reflex 候选以选项形式参与排序。
- 验收：四类单测；同输入同输出；未知 claim 报错；binding/completion 各有用例；选项概率和 ≤1 且 `defer` 恒在选项集；flag 关闭时 planner 行为与基线一致。

### V3-R2 Claim 审计与签名稳定性

- 每次决策落 `claims` 表；签名跨项目稳定、对图变化敏感。
- 验收：同形不同文本同签名；多一个 fact 签名变化；并发写入无冲突；可用历史 claim 回放。

### V3-R3 渗透证据契约

- 所有 explore/reflex 产生的 observation 必须带 Evidence；拒绝文本不算成功。
- 验收：缺命令/缺退出码/缺 sandbox id 的记录被拒；脱敏字段不落库；旧格式可读。

### V3-R4 Reflex 运行时与权限

- goja 运行时；白名单原语；超时；输出上限；静态检查。
- 验收：越权原语拒绝；超时终止；无网络/宿主对象；异常落审计。

### V3-R5 短路、落回与隔离

- 命中执行、失败落回、连续失败 quarantine；不得双写事实。
- reflex 内每次工具调用前过 `binding` 判断；`defer` 或拒绝即中止本次 reflex 并交回 Reason。
- 报告返回后过 `completion` 判断，决定写 fact/verdict 还是继续。
- 验收：命中成功零主模型调用；`binding=defer` 时只执行零次工具调用；失败仅计一次且落回；quarantine 后不命中；证据与 LLM 路径 parity。

### V3-R6 Reflex 生成与调试

- 同签名成功 ≥3 次触发；5 claim 并行；候选必须通过回放调试才能入库。
- 生成物必须是 `function run(a, read, step, occurrence)`，附带 ArgsSchema；只描述适用条件，不写死目标参数。
- 验收：不足阈值不生成；候选缺 run/stop/越权被拒；同程序换 `a` 复用通过；回放不一致拒绝入库。

### V3-R7 版本、置信度与淘汰

- 版本 +1 保留旧版；置信度随 hit/confirm/fail 更新；可人工/自动退役。
- 验收：升版不覆盖旧版；失败降置信度；退役后不再匹配。

### V3-R8 内置渗透 Reflex（先于生成式）

首版必须内置并测试至少四条，保证没有生成能力也能产生价值：

1. `http_probe`：请求 URL，记录状态/头/正文摘要与证据。
2. `fingerprint_lookup`：响应文本 → 指纹/规则/POC 候选。
3. `evidence_collect`：执行给定命令并落证据，失败也落客观结论。
4. `stop_on_marker`：标记不存在/入口 404 时终止当前链并 refute。

验收：每条有单测 + 在 T0 场景的一次真实调用记录。

### V3-R9 命令护栏（fail-closed）

- 分类：`safe` / `approval` / `forbidden`；未知或分类失败 → `approval`。
- 验收：危险命令不被放行；审批流不绕过；误判可人工覆盖并留痕。

### V3-R10 POC 安全分级（OPESEC 思路）

- 对 POC 分 `validation_only` / `exploit_capable` / `unknown`；unknown 默认仅验证。
- 是否允许 `exploit_capable` 由 `misc.engagement_mode`（`validation` 默认 / `full`）决定。
- 验收：validation 模式下攻击型 POC 不进执行路径；人工审核可改级；审计可查。

### V3-R11 指纹路由（fingers 思路）

- 响应关键字 → 组件/版本 → 规则/POC；不逐条正则扫描；无命中回退现有搜索。
- 验收：关键字命中正确；无命中不伪造；路由依据入库。

### V3-R12 自进化优化器

- 每 10 次抽样；旁路模型评估/改写；A/B 双条件；胜者升版，败者保留。
- 验收：样本不足不触发；升版需 A/B 胜出；回滚可用；优化成本计入 token 账。

### V3-R13 指标与对照评测

- eval 增加：`short_circuit_rate`、`reflex_tokens_saved`、`parity_failures`、`time_to_verified_finding`、`reflex_regression_rate`、`safety_incidents`。
- 验收：开关前后同场景对比；输出 JSON；finding 口径为 verified，不用代理。

### V3-R14 真实渗透验收

- CVE-2025-3248：真实 diff → bootstrap → explore → verdict → rule；≥1 verified finding；≥1 规则；≥3 次 reflex 命中。
- 验收：runbook 记录命令与原始输出；timeline 可回放；失败点也要记录。

### V3-R15 兼容、灰度与可观测

- 所有能力独立 flag 默认关；关闭时行为不变；单二进制；SQLite；不 import `Web/`。
- 可观测：claim/reflex 运行记录、quarantine 原因、匹配签名、证据 hash。
- 验收：flag 矩阵测试；`go build ./...` exit 0；`go test ./...` 相关包全绿。

### V3-R16 人类控制

- 审批不绕过；一键停用 reflex；quarantine 人工复核；Hint 始终优先于 reflex 短路。
- 验收：停用后立即回 LLM 路径；Hint 存在时同签名不得直接短路（需把 Hint 纳入签名或强制 reason）；reflex 返回 `defer` 时必须回 Reason，不得在同一任务内重试同一版本。

---

## 6. 数据模型（`data/reflex.db`）

```sql
CREATE TABLE claims (
  id TEXT PRIMARY KEY, project_id TEXT, task_type TEXT, point TEXT, signature TEXT,
  content TEXT, options_json TEXT, features_json TEXT, result_json TEXT,
  cost_tokens INTEGER, latency_ms INTEGER, outcome TEXT, created_at TEXT
);

CREATE TABLE reflexes (
  id TEXT PRIMARY KEY, name TEXT, signature TEXT, language TEXT, entry TEXT,
  args_schema_json TEXT, source TEXT,
  version INTEGER, status TEXT, confidence REAL,
  hit_count INTEGER, confirm_count INTEGER, fail_count INTEGER,
  created_at TEXT, updated_at TEXT
);

CREATE TABLE reflex_runs (
  id TEXT PRIMARY KEY, reflex_id TEXT, project_id TEXT, intent_id TEXT,
  claim_id TEXT, success INTEGER, tokens_saved INTEGER, parity TEXT,
  evidence_hash TEXT, error TEXT, created_at TEXT
);

CREATE TABLE guardrail_labels (
  id TEXT PRIMARY KEY, command_class TEXT, command_hash TEXT,
  label TEXT, confidence REAL, source TEXT, reviewed_by TEXT, created_at TEXT
);

CREATE TABLE poc_labels (
  id TEXT PRIMARY KEY, poc_path TEXT, class TEXT, confidence REAL,
  engagement_mode TEXT, reviewed_by TEXT, created_at TEXT
);

CREATE TABLE fingerprints (
  id TEXT PRIMARY KEY, product TEXT, version TEXT, source_keyword TEXT,
  evidence TEXT, confidence REAL, created_at TEXT
);
```

独立库，不改 `graph.db`；删除即回滚。

---

## 7. 安全与合规边界

- 仅授权靶标；目标范围由 lab/project 记录，reflex 不得跨项目复用具体主机。
- 所有命令在 sandbox runtime 内执行；宿主命令一律 `approval`，不得由 reflex 直接执行。
- 证据落库前脱敏；日志不得打印凭据。
- POC 分级只决定“是否可执行”，不改变授权；`full` 模式需显式配置并留痕。
- 任何护栏/分类器异常 → fail-closed。

---

## 8. 指标与闸门

| 闸门 | 条件 | 对应需求 |
|---|---|---|
| G0 基线 | T0 真实跑出 patch_profile/verdict/rule | R14 前置 |
| G1 决策 | 四类判断 + claim 审计 + 签名稳定 | R1, R2 |
| G2 证据 | 证据契约 + 运行时权限 | R3, R4, R16 |
| G3 短路 | 命中/落回/quarantine + 内置 reflex + parity | R5–R8 |
| G4 安全路由 | 护栏 + POC 分级 + 指纹路由 | R9–R11 |
| G5 自进化 | 优化器 + 指标 + 真实验收 | R12–R14 |

任一闸门未过，不得把对应能力标为“已落地”。

---

## 9. 失败模式与降级

| 场景 | 行为 |
|---|---|
| 决策服务不可用 | 回落 LLM；记 `decision_unavailable` |
| 签名无命中 | 正常 LLM 路径；继续积累 claim |
| reflex 执行失败 | 落回 LLM；`fail_count+1`；连续失败 quarantine |
| reflex 证据不一致 | parity 失败；quarantine；保留旧版 |
| 生成候选不合格 | 拒绝入库；记录原因 |
| 护栏异常 | 按 `approval` 处理，不自动放行 |
| POC 分级不确定 | 按 `validation_only` |
| 指纹无命中 | 回退规则搜索；不伪造 |
| 优化器回退 | 保留旧版；A/B 数据留档 |
| 环境缺 Docker/LLM | T0 阻塞；不进入 C2 验收 |

---

## 10. 交付物

1. 三份文档：`v3_re.md` / `v3_task.md` / `v3_todo.md`。
2. 代码：`decision/`、`evidence/`、`reflex/`、`safety/`、`fingerprint/`、`reflexopt/`，及 dispatch/eval 接入。
3. 内置 reflex ≥4 条；生成式管线；版本与 quarantine。
4. 真实运行记录：`docs/runbooks/cve-2025-3248-run-<date>.md`。
5. eval 对照 JSON 与指标报告。

---

## 11. 变更记录

| 日期 | 变更 | 原因 |
|---|---|---|
| 2026-11-20 | 选定方案 C；拆出 v3_re/task/todo | 用户拍板，要求可实际作用于渗透测试 |
| 2026-11-20 | 增加 Evidence 契约与 verified finding 口径 | 现有 finding 为代理口径，无法证明真实利用 |
| 2026-11-20 | 内置 reflex 先于生成式 reflex | 生成式依赖真实样本；先交付确定性价值 |
| 2026-11-20 | 决策默认零 token | 防止“用决策模型又烧一遍 token” |
| 2026-11-20 | 增加 binding/completion 两个判断点 | `jev+reflex` 图片：工具准入与完成判定是 harness 的两个真实调用位置 |
| 2026-11-20 | Reflex 固定入口 `run(a, read, step, occurrence)`，参数与程序分离 | 同一段程序换参数复用，签名只描述适用条件 |
| 2026-11-20 | 决策输出改为带概率的排序选项，`defer` 恒在选项集 | 图片展示 86%/14% 排序与“不适用交回 Reason” |
