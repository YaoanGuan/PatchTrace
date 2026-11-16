# PatchTrace Marketplace 前端实施计划（task.md）

> 迭代名：PatchTrace · AI 雇佣 AI 撮合与教育平台（前端）
> 分支：`feat/ai-marketplace-frontend`
> 技术栈：React 18 + TypeScript + Vite 5 + react-router-dom 6 + Zustand 5 + TailwindCSS 3
> 产物位置：`webui-marketplace/`（独立前端应用，不侵入既有 `webui/` 与 Go 后端）
> 配套：`docs/plans/marketplace/todo.md`（进度与验收）
> 状态：T0 设计完成，进入 T1

---

## 0. 定位与边界

平台定位：任何人都可以发布任务（雇主），任何人也可以领取任务并调度自己的 AI Agent 完成（AI 指挥官）。平台提供任务封装、AI 交叉审核、资金托管、执行证据链与双向信誉。

本轮只做**前端架构 + 可运行界面 + 类型化 mock API**；不接 Go 后端，不改 `webui/`，不改嵌入的 `webui/dist`。后端契约以本文档 §4 的 JSON 字段为唯一口径，后续接真实 API 时只替换 `src/lib/api/client.ts` 的实现。

---

## 1. 顶层路由

| 路径 | 页面 | 模块 |
|---|---|---|
| `/` | 重定向到 `/marketplace` | — |
| `/identity` | `IdentityPage`（个人中枢与能力认证） | M1 |
| `/marketplace` | `MarketplacePage`（任务撮合大厅） | M2 |
| `/marketplace/tasks/:taskId` | `TaskDetailPage`（任务详情 + 流水线 + 托管） | M2 |
| `/audit` | `AuditPage`（审计列表与证据链入口） | M3 |
| `/audit/:taskId` | `AuditDetailPage`（单任务证据链 + 免责契约） | M3 |
| `/onboarding` | `OnboardingPage`（准入与信誉） | M4 |
| `*` | `NotFoundPage` | — |

路由实现：`src/app/router.tsx`，使用 `createBrowserRouter` + `React.lazy` 按页懒加载；`AppShell` 作为父路由承载顶栏、侧栏与全局弹窗。

---

## 2. 全局状态（Zustand）

四个独立 slice，按领域切分，禁止跨 slice 直接读写内部字段。

### 2.1 `sessionStore` — 身份与视角

```ts
type UserRole = 'EMPLOYER' | 'COMMANDER';
type Channel = 'UNIVERSITY' | 'ENTERPRISE' | 'FREELANCER';

interface SessionState {
  currentUser: User | null;
  activeRole: UserRole;              // 雇主视角 / 指挥官视角
  pendingRole: UserRole | null;      // 切换时的确认态
  setActiveRole(role: UserRole): void;
  toggleRole(): void;
  loadProfile(): Promise<void>;
}
```

- `activeRole` 只影响视角与可执行动作，不改变账号身份（一个人可同时是发单方与接单方）。
- 切换视角时 `RoleSwitcher` 触发一次确认，写 `pendingRole`，确认后调用 `setActiveRole`。

### 2.2 `taskStore` — 任务与状态机

```ts
type TaskStatus =
  | 'DRAFT' | 'PUBLISHED' | 'ACCEPTED' | 'IN_PROGRESS' | 'SUBMITTED'
  | 'AI_VALIDATING' | 'VALIDATION_FAILED' | 'COMPLIANCE_SCAN'
  | 'SETTLING' | 'COMPLETED' | 'DISPUTED' | 'REFUNDED' | 'CANCELLED';

interface TaskState {
  tasks: TaskSummary[];
  filters: TaskFilters;
  selectedTask: TaskDetail | null;
  loading: boolean;
  fetchTasks(filters?: Partial<TaskFilters>): Promise<void>;
  fetchTask(id: string): Promise<void>;
  publishTask(input: PublishTaskInput): Promise<TaskDetail>;
  acceptTask(id: string): Promise<void>;
  advanceStage(id: string, next: TaskStatus): void;
}
```

**状态流转（唯一合法迁移表）**

```
DRAFT ──publish──▶ PUBLISHED ──accept──▶ ACCEPTED ──start──▶ IN_PROGRESS
IN_PROGRESS ──submit──▶ SUBMITTED ──▶ AI_VALIDATING ──▶ COMPLIANCE_SCAN ──▶ SETTLING ──▶ COMPLETED
AI_VALIDATING ──reject──▶ VALIDATION_FAILED ──rework──▶ IN_PROGRESS
PUBLISHED/ACCEPTED/IN_PROGRESS ──dispute──▶ DISPUTED ──arbitrate──▶ COMPLETED | REFUNDED
PUBLISHED ──cancel──▶ CANCELLED
```

`ALLOWED_TRANSITIONS: Record<TaskStatus, TaskStatus[]>` 在 `src/types/domain.ts` 中集中声明；`advanceStage` 遇到非法迁移必须抛错，不允许静默写入。

### 2.3 `walletStore` — 资金与托管

```ts
interface WalletState {
  balance: { available: number; frozen: number; currency: 'CNY' };
  transactions: WalletTransaction[];
  refresh(): Promise<void>;
  freezeForTask(taskId: string, amount: number): Promise<void>;
  settleTask(taskId: string): Promise<void>;
}
```

### 2.4 `uiStore` — 全局 UI 态

```ts
interface UiState {
  legalModal: { open: boolean; taskId: string | null };
  toasts: Toast[];
  openLegalModal(taskId: string): void;
  closeLegalModal(): void;
  pushToast(t: Toast): void;
  dismissToast(id: string): void;
}
```

---

## 3. 组件树

### 3.0 应用外壳

```
<App>
└─ <RouterProvider>
   └─ <AppShell>
      ├─ <TopBar>
      │  ├─ <Brand/>
      │  ├─ <RoleSwitcher/>
      │  └─ <WalletChip/>
      ├─ <SideNav/>            // 4 个模块入口
      ├─ <main><Outlet/></main>
      ├─ <LegalLiabilityModal/> // 全局，由 uiStore.legalModal 驱动
      └─ <ToastViewport/>
```

### 3.1 模块 1 · 个人中枢与能力认证 `IdentityPage`

```
<IdentityPage>
├─ <ProfileHeader>              // 头像、昵称、等级、信誉分
│  └─ <ReputationPill/>
├─ <RoleSwitcher variant="panel"/>
├─ <SkillRadarChart>            // AI 指令精准度 / 任务闭环 / 成本控制 / 风险控制 / 证据质量
│  └─ <SkillLegend/>
├─ <ExpertiseRepository>
│  ├─ <ExpertiseTabs/>          // Agent 能力 / 数据资产 / 提示词模板
│  ├─ <ExpertiseCard/> × n
│  └─ <DataAssetCard/> × n
├─ <TrustBadgeWall>             // 高校学生认证 / 企业认证 / 任务通关认证
│  └─ <TrustBadge/> × n
└─ <CredentialUploadDialog/>    // 证书/学籍/工牌上传
```

### 3.2 模块 2 · 任务撮合大厅与 AI 流水线 `MarketplacePage` / `TaskDetailPage`

```
<MarketplacePage>
├─ <MarketplaceHeader/>         // 视角提示：当前为雇主/指挥官
├─ <TaskMarketplace>
│  ├─ <TaskFilters>             // 类别、状态、赏金区间、技能
│  ├─ <TaskPublishForm>         // 雇主视角可见
│  │  ├─ <RewardInput/>         // 触发资金校验
│  │  └─ <ComplianceAck/>       // 发布前合规勾选
│  ├─ <TaskList>
│  │  └─ <TaskCard/> × n        // 状态徽标、赏金、发布者信誉
│  └─ <AcceptTaskPanel/>        // 指挥官视角可见
├─ <AIValidationPipeline>
│  ├─ <PipelineStage/> × n      // AI 生成 → 平台审核 AI 复核 → 合规扫描 → 结算
│  ├─ <ValidationVerdictCard/>
│  └─ <CrossCheckBadge/>        // 交叉验证标识
└─ <EscrowWallet>
   ├─ <BalanceSummary/>         // 可用 / 冻结
   ├─ <EscrowTaskRow/> × n
   └─ <TransactionList/>

<TaskDetailPage>
├─ <TaskSummary/>
├─ <AIValidationPipeline/>
├─ <EscrowWallet/>
├─ <AcceptTaskPanel/>           // 或 <SubmitWorkPanel/>，按 activeRole 与任务状态切换
└─ <AuditLinkCard/>             // 跳转 /audit/:taskId
```

### 3.3 模块 3 · 责任隔离与防篡改审计 `AuditPage` / `AuditDetailPage`

```
<AuditPage>
├─ <AuditFilterBar/>
├─ <AuditTaskList>
│  └─ <AuditTaskRow/> × n       // 任务、哈希链状态、最近节点时间
└─ <HashChainSummary/>

<AuditDetailPage>
├─ <TraceabilityTimeline>
│  ├─ <TraceNode type="REQUIREMENT"/>
│  ├─ <TraceNode type="ACCEPTANCE"/>
│  ├─ <TraceNode type="MODEL_CALL"/>    // 调用的模型、成本、耗时
│  ├─ <TraceNode type="ACTION"/>        // 具体执行动作
│  ├─ <TraceNode type="OUTPUT"/>
│  └─ <TraceNode type="VALIDATION"/>
├─ <HashChainStatus/>           // 节点 prevHash/hash 链校验结果
├─ <ComplianceScanReport/>
└─ <LegalLiabilityModal>
   ├─ <LiabilityChecklist/>     // 三方责任边界：雇主 / 指挥官 / 平台
   ├─ <ContractSignaturePanel/> // 签署人 + 签名字段
   └─ <SignatureHash/>
```

### 3.4 模块 4 · 准入机制与生态信誉 `OnboardingPage`

```
<OnboardingPage>
├─ <MultiChannelRegistration>
│  ├─ <ChannelTabs/>            // 高校 / 企业白名单 / 自由职业
│  ├─ <UniversityVerificationForm/>   // 学籍号、学校邮箱、学生证
│  ├─ <EnterpriseWhitelistForm/>      // 企业域名邮箱、工牌、统一社会信用代码
│  └─ <FreelancerForm/>               // 实名、作品集、历史评价
├─ <TrustBadgeWall/>
└─ <ReputationSystem>
   ├─ <ReputationScoreCard/>    // 双向评分总览
   ├─ <RatingForm direction="EMPLOYER_TO_COMMANDER"/>
   ├─ <RatingForm direction="COMMANDER_TO_EMPLOYER"/>
   └─ <RatingHistory/>
```

---

## 4. 核心 API 契约（mock 实现，字段即类型）

统一前缀 `/api`。前端 `src/types/api.ts` 与之一一对应；mock 在 `src/lib/api/mock.ts`。

### 4.1 `GET /api/me`

```json
{
  "id": "usr_001",
  "displayName": "林知远",
  "avatarUrl": "/avatars/u1.png",
  "activeRole": "COMMANDER",
  "roles": ["EMPLOYER", "COMMANDER"],
  "verificationChannel": "UNIVERSITY",
  "reputation": {
    "score": 4.8,
    "completedTasks": 32,
    "onTimeRate": 0.97,
    "disputes": 1
  },
  "skills": [
    { "key": "INSTRUCTION_PRECISION", "label": "AI指令精准度", "score": 88 },
    { "key": "TASK_CLOSURE", "label": "任务闭环能力", "score": 76 },
    { "key": "COST_EFFICIENCY", "label": "成本控制", "score": 81 },
    { "key": "RISK_CONTROL", "label": "风险控制", "score": 72 },
    { "key": "EVIDENCE_QUALITY", "label": "证据质量", "score": 84 }
  ],
  "expertise": [
    {
      "id": "exp_001",
      "kind": "AGENT",
      "title": "代码审计 Agent 编排",
      "level": "L3",
      "usageCount": 42,
      "updatedAt": "2026-11-01T09:20:00Z"
    }
  ],
  "badges": [
    {
      "id": "bdg_001",
      "type": "UNIVERSITY",
      "label": "高校学生认证",
      "issuer": "PatchTrace × 某高校",
      "verifiedAt": "2026-09-12T00:00:00Z",
      "status": "VERIFIED"
    }
  ]
}
```

### 4.2 `GET /api/tasks?status=&keyword=&rewardMin=&rewardMax=&page=`

```json
{
  "items": [
    {
      "id": "task_1001",
      "title": "为一款 IoT 固件编写协议模糊测试 Agent",
      "category": "SECURITY",
      "status": "PUBLISHED",
      "reward": { "amount": 1200, "currency": "CNY" },
      "deadline": "2026-11-30T12:00:00Z",
      "publisher": { "id": "usr_002", "displayName": "某安全实验室", "reputationScore": 4.6 },
      "requiredSkills": ["AGENT_ORCHESTRATION", "FUZZING"],
      "acceptCount": 3,
      "validationScore": 87
    }
  ],
  "total": 128,
  "page": 1,
  "pageSize": 20
}
```

### 4.3 `POST /api/tasks`

请求：

```json
{
  "title": "……",
  "description": "……",
  "category": "SECURITY",
  "reward": { "amount": 1200, "currency": "CNY" },
  "deadline": "2026-11-30T12:00:00Z",
  "acceptanceCriteria": ["可复现脚本", "执行日志", "结论报告"],
  "requiredSkills": ["AGENT_ORCHESTRATION"],
  "complianceAck": true
}
```

响应：`TaskDetail`（见 4.4）。

### 4.4 `GET /api/tasks/:taskId`

```json
{
  "id": "task_1001",
  "title": "……",
  "description": "……",
  "category": "SECURITY",
  "status": "AI_VALIDATING",
  "publisher": { "id": "usr_002", "displayName": "……", "reputationScore": 4.6 },
  "assignee": { "id": "usr_001", "displayName": "林知远", "reputationScore": 4.8 },
  "reward": { "amount": 1200, "currency": "CNY" },
  "escrow": { "state": "FROZEN", "amount": 1200, "frozenAt": "2026-11-16T02:00:00Z" },
  "pipeline": {
    "currentStage": "AI_REVIEW",
    "stages": [
      { "key": "AI_GENERATION", "status": "DONE", "at": "2026-11-16T02:10:00Z", "by": "agent://commander-01", "summary": "生成 3 条候选路径" },
      { "key": "AI_REVIEW", "status": "RUNNING", "at": "2026-11-16T02:20:00Z", "summary": "平台审核 AI 交叉复核" },
      { "key": "COMPLIANCE_SCAN", "status": "PENDING" },
      { "key": "SETTLEMENT", "status": "PENDING" }
    ]
  },
  "requiredSkills": ["AGENT_ORCHESTRATION"],
  "createdAt": "2026-11-15T10:00:00Z",
  "deadline": "2026-11-30T12:00:00Z"
}
```

### 4.5 `POST /api/tasks/:taskId/accept`

```json
{ "taskId": "task_1001", "status": "ACCEPTED", "leaseExpiresAt": "2026-11-16T04:20:00Z" }
```

### 4.6 `GET /api/tasks/:taskId/trace`

```json
{
  "taskId": "task_1001",
  "headHash": "sha256:9f2c…",
  "nodes": [
    {
      "id": "node_001",
      "type": "REQUIREMENT",
      "actor": { "kind": "USER", "id": "usr_002", "label": "某安全实验室" },
      "at": "2026-11-15T10:00:00Z",
      "summary": "发布任务并冻结赏金",
      "evidence": { "hash": "sha256:1a2b…", "payloadRef": "blob://task/1001/req", "model": null, "cost": 0 },
      "prevHash": null,
      "hash": "sha256:1a2b…"
    },
    {
      "id": "node_002",
      "type": "MODEL_CALL",
      "actor": { "kind": "AGENT", "id": "agent://commander-01", "label": "Claude Sonnet" },
      "at": "2026-11-16T02:11:00Z",
      "summary": "生成候选测试计划",
      "evidence": { "hash": "sha256:7c3d…", "payloadRef": "blob://trace/1001/step2", "model": "claude-sonnet-5-5", "cost": 0.12 },
      "prevHash": "sha256:1a2b…",
      "hash": "sha256:7c3d…"
    }
  ]
}
```

### 4.7 `GET /api/wallet`

```json
{
  "balance": { "available": 3200, "frozen": 1200, "currency": "CNY" },
  "transactions": [
    {
      "id": "tx_001",
      "type": "ESCROW_FREEZE",
      "amount": 1200,
      "taskId": "task_1001",
      "at": "2026-11-15T10:00:00Z",
      "status": "SETTLED"
    }
  ]
}
```

### 4.8 信誉与准入

- `GET /api/reputation/:userId` → `{ userId, score, dimensions: [{key,label,score}], ratings: [...] }`
- `POST /api/ratings` → `{ taskId, direction: "EMPLOYER_TO_COMMANDER" | "COMMANDER_TO_EMPLOYER", score, tags, comment }`
- `POST /api/auth/register/:channel` → `{ channel, evidence: { studentId?, schoolEmail?, workEmail?, realName? }, status: "PENDING" | "VERIFIED" }`
- `POST /api/tasks/:taskId/contracts/sign` → `{ taskId, signerRole, clausesAccepted, signatureHash, signedAt }`

---

## 5. 目录结构（文件域）

```
webui-marketplace/
├─ package.json / tsconfig*.json / vite.config.ts / index.html / tailwind.config.cjs / postcss.config.cjs
└─ src/
   ├─ main.tsx
   ├─ index.css
   ├─ app/
   │  ├─ App.tsx
   │  ├─ router.tsx
   │  ├─ store/{sessionStore,taskStore,walletStore,uiStore,index}.ts
   │  └─ pages/NotFoundPage.tsx
   ├─ types/{domain.ts,api.ts}
   ├─ lib/{cn.ts,api/client.ts,api/mock.ts}
   ├─ components/
   │  ├─ ui/{Button,Card,Badge,Modal,Tabs,Progress,EmptyState}.tsx
   │  ├─ layout/{AppShell,TopBar,SideNav}.tsx
   │  └─ shared/{RoleSwitcher,TrustBadgeWall,WalletChip}.tsx
   └─ features/
      ├─ identity/pages/IdentityPage.tsx
      ├─ marketplace/pages/{MarketplacePage,TaskDetailPage}.tsx
      ├─ audit/pages/{AuditPage,AuditDetailPage}.tsx
      └─ onboarding/pages/OnboardingPage.tsx
```

---

## 6. 任务拆解与验收

### T1 · 地基（P8-foundation）

- 产出：`webui-marketplace/` 全部脚手架、路由、四个 store、类型、mock API、UI 基础组件、AppShell、共享组件、7 个占位页面。
- DONE：`npm install` 成功；`npx tsc --noEmit` 通过；`npm run build` 通过。
- 文件域：`webui-marketplace/` 下除 `src/features/**` 外的全部文件；`src/features/**` 只允许占位页面。
- 禁区：不改 `webui/`、Go 代码、既有 `docs/plans/*.md`；不接真实后端。

### T2 · 模块 1 身份中枢（P8-identity）

- 文件域：`src/features/identity/**`（占位页替换 + 组件）。
- 只读依赖：`src/components/**`、`src/app/store/**`、`src/types/**`、`src/lib/api/**`。
- DONE：`npx tsc --noEmit` 通过；页面渲染 `RoleSwitcher`、`SkillRadarChart`、`ExpertiseRepository`、`TrustBadgeWall`。

### T3 · 模块 2 撮合大厅（P8-marketplace）

- 文件域：`src/features/marketplace/**`。
- DONE：`TaskMarketplace` 支持筛选 + 发布表单 + 接单面板；`AIValidationPipeline` 四阶段状态；`EscrowWallet` 余额与流水。

### T4 · 模块 3 审计（P8-audit）

- 文件域：`src/features/audit/**`。
- DONE：`TraceabilityTimeline` 按 6 类节点渲染并展示 hash 链；`LegalLiabilityModal` 可打开、勾选条款、生成签名哈希。

### T5 · 模块 4 准入信誉（P8-onboarding）

- 文件域：`src/features/onboarding/**`。
- DONE：三通道注册表单切换；双向评分表单与历史列表。

### T6 · 集成验收（P9）

- DONE：`npx tsc --noEmit`、`npm run build` 全绿；路由清单与 4 模块组件树对照无缺项；`git commit` 到 `feat/ai-marketplace-frontend`。
- 证据：命令输出贴入 `docs/plans/marketplace/todo.md`。

---

## 7. 依赖与约束

- 新增依赖：`react-router-dom@6`、`zustand@5`、`lucide-react`；其余沿用 React 18 / Tailwind 3。
- 不引入 UI 组件库（Radix 可选，非必需）；样式用 Tailwind 工具类，不写自定义 CSS 主题。
- 不写单元测试（本轮只验收类型与构建）；组件保持纯展示 + store 订阅。
- 不修改验收标准；任何状态变化先回写 `todo.md`。

---

## 8. T7–T8 · 真实 API 实现

### 目标

把前端的内存 mock 替换为真实 HTTP 契约实现，前后端字段严格对齐 §4。

### 架构决策

- 新增独立 Go 服务：包 `marketplace/` + 入口 `cmd/marketplace/main.go`，默认监听 `:8081`。
- 不挂进既有 `Web.Server`：那边所有 `/api` 路由要求 Bearer 鉴权，而 marketplace 前端是独立产品，避免把两个产品的鉴权模型耦在一起。
- 存储：进程内内存存储（`sync.RWMutex`），启动时按 §4 种子数据初始化；重启即重置。真实持久化（SQLite）不在本轮。
- 仅用标准库 `net/http` + `encoding/json`，不新增 Go 依赖。
- CORS 允许 `http://localhost:5174`、`http://localhost:4180` 与同源请求。

### T7 · Go API 服务

生效接口（与 §4 一一对应）：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/me` | 当前用户与能力认证 |
| GET | `/api/tasks` | 列表，支持 `status`/`keyword`/`rewardMin`/`rewardMax`/`page` |
| POST | `/api/tasks` | 发布任务，冻结赏金，`DRAFT→PUBLISHED` |
| GET | `/api/tasks/:id` | 详情（含 pipeline/escrow） |
| POST | `/api/tasks/:id/accept` | 接单，返回 `leaseExpiresAt` |
| GET | `/api/tasks/:id/trace` | 证据链，`task_1001` 6 节点完整链 |
| GET | `/api/wallet` | 余额与流水 |
| GET | `/api/reputation/:userId` | 信誉维度与历史评分 |
| POST | `/api/ratings` | 双向评分，追加历史 |
| POST | `/api/auth/register/:channel` | 三通道注册，返回 `PENDING`/`VERIFIED` |
| POST | `/api/tasks/:id/contracts/sign` | 契约签署，返回 `signatureHash` |
| GET | `/api/healthz` | 健康检查 |

约束：

- `POST /api/tasks` 必须在服务端再次校验 `complianceAck=true`、`reward.amount > 0`、必填字段；非法请求返回 400 + `{"error":"..."}`，不得 panic。
- 状态迁移复用 `ALLOWED_TRANSITIONS` 的口径；非法迁移返回 409。
- 所有时间用 RFC3339 UTC。
- 错误响应统一 `{"error": "..."}`，成功响应与 §4 字段完全一致。

DONE：
- `go test ./marketplace/...` 全绿（覆盖 11 个接口 + 非法参数 + 非法迁移）。
- `go vet ./marketplace/... ./cmd/marketplace/` 干净。
- `go run ./cmd/marketplace -port 8081` 后，curl 矩阵全部符合预期。

### T8 · 前端 HTTP 客户端

- `src/lib/api/client.ts`：保留 mock，新增 HTTP 实现；切换开关 `import.meta.env.VITE_API_MODE === 'http'`，基址 `import.meta.env.VITE_API_BASE_URL ?? ''`（默认同源）。
- HTTP 实现必须与 mock 函数签名、返回 DTO 完全一致；非 2xx 抛出带 `status` 与后端 `error` 文本的 `Error`。
- `vite.config.ts`：`server.proxy` 与 `preview.proxy` 把 `/api` 转发到 `http://127.0.0.1:8081`。
- 新增 `.env.example`：`VITE_API_MODE=http`、`VITE_API_BASE_URL=`。
- DONE：`npx tsc --noEmit` exit 0；`npm run build` exit 0；`VITE_API_MODE=http npm run build` exit 0。

### T9 · 联调验收

- 启动 Go 服务 + HTTP 模式前端，6 条路由 headless 渲染仍通过。
- curl 抽查：发布任务后列表 +1、接单后状态 `ACCEPTED`、签署返回 `sha256:` 哈希。
- 提交并推送分支。
