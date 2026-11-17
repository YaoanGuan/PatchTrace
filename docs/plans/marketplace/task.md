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
| `/identity` | `IdentityPage`（个人中枢 + 钱包 + API 中转站） | M1 |
| `/expert` | `ExpertPage`（AI 专家 · 信任收费站） | M2 |
| `/marketplace` | `MarketplacePage`（任务撮合大厅） | M3 |
| `/marketplace/tasks/:taskId` | `TaskDetailPage`（任务详情 + 信任流水线 + 托管） | M3 |
| `/audit` | `AuditPage`（审计列表与证据链入口） | M4 |
| `/audit/:taskId` | `AuditDetailPage`（单任务证据链 + 免责契约） | M4 |
| `/onboarding` | `OnboardingPage`（准入与信誉） | M5 |
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

### T10 · SQLite 持久化

目标：重启服务后任务、接单、评分不丢失。

设计决策：

- 复用既有依赖 `modernc.org/sqlite`，不新增依赖。
- 采用**整状态快照**而非关系表：表 `marketplace_state(id INTEGER PRIMARY KEY CHECK(id=1), payload BLOB NOT NULL, updated_at TEXT NOT NULL)`；启动时读取并反序列化，首次启动写种子数据；每次变更在锁内事务 upsert。
- 理由：当前领域对象嵌套深（pipeline/trace/reputation），关系表会引入大量 join 与迁移；快照在演示规模下足够，且不影响后续换关系表。
- 连接约定：`sql.Open("sqlite", path)`、`SetMaxOpenConns(1)`、`PRAGMA journal_mode=WAL`、`busy_timeout=5000`。
- API：新增 `OpenStore(path string) (*Store, error)` 与 `Close() error`；保留 `NewStore()` 为纯内存（既有测试与无 DB 场景）。
- 入口：`cmd/marketplace` 新增 `-db` 参数，默认 `data/marketplace.db`，目录不存在则创建。

DONE：
- `go test ./marketplace/... -race` 全绿，新增重启持久化用例（发布/接单/评分）。
- `go vet` 干净，`CGO_ENABLED=0 go build ./...` exit 0。
- 实机验证：发布任务 → 停止服务 → 重启 → 列表中仍有该任务。

---

## 5. 第二轮迭代 · 品牌升级 / M2 AI 专家 / 钱包与 API 中转站

> 触发：2026-11-17 用户反馈「前端再完善一下」。
> 品牌口径：用户原文写作 `ParchTrace Market-AI雇佣AI的教育平台`，按既有品牌 `PatchTrace` 落地（`ParchTrace` 视为笔误）；标题文案保留 `Market-AI 雇佣 AI 的教育平台`，广告语原样使用。
> 模块编号：新增模块插入 M1 与任务大厅之间 → M1 个人中枢、M2 AI 专家、M3 任务大厅、M4 审计证据、M5 准入信誉。

### T11 · 品牌与导航

- `index.html` `<title>`：`PatchTrace Market · AI 雇佣 AI 的教育平台`。
- `TopBar`：主品牌 `PatchTrace Market` + 副标题 `AI 雇佣 AI 的教育平台` + 广告语 `Trace the Patch, Uncover the Flaw.`；窄屏隐藏副标题与广告语，保留品牌。
- `SideNav`：新增 `/expert`（M2 · AI 专家）；任务大厅标 M3、审计标 M4、准入标 M5。
- 各页面眉标同步改号：marketplace → M3、audit/audit-detail → M4、onboarding → M5；`NotFoundPage` 品牌文案同步。
- 路由：新增 `/expert`，懒加载 `features/expert/pages/ExpertPage`。

### T12 · M2 AI 专家（信任收费站 / Trust Layer）

定位：不做"纯 API 代理"（可替代、无定价权），把**多 Agent 交叉验证 + 合规检查 + 风险阻断**做成可计价产品。

路由 `/expert`，单一页面 + 多板块：

1. **Hero**：标题「AI 专家 · 信任收费站」，一句话说明信任是产品；信任服务计价卡（交叉验证 / 合规扫描 / 风险阻断 三档单价）。
2. **接入矩阵**（`AccessMatrix` + `ToolCard`）：
   - 分类：`CLOUD_COMPUTER`（Grok Bot、ChatGPT Dot）、`AI_CLI`（Claude Code、Codex CLI）、`AI_IDE`（Cursor、Trae）、`CHAT_UI`（Chatbox）。
   - 每张卡：名称、厂商、接入协议（MCP / CLI / IDE Plugin / Screen）、信任等级 `S/A/B/C`、状态 `CONNECTED/PENDING/SANDBOX`、允许角色（雇主可发布 / 指挥官可执行）、合规标签。
   - 分类筛选 + 等级筛选；覆盖「云托管计算机 / CLI / IDE / 聊天客户端」四类无死角。
3. **角色接入桥**（`RoleBridge`）：雇主 AI 接入 → 发布/托管任务；指挥官 AI 接入 → 领取/执行/提交任务；展示各自所需 scope 与审计留存要求。
4. **多 Agent 交叉验证**（`CrossValidationPanel` + `PipelineStatus`）：Agent A（Claude）生成 PoC → Agent B（Codex）复核 → 共识 `PASS/FAIL/PENDING`、置信度、耗时、成本；两步状态条（生成 / 复核 / 共识）。
5. **自动化合规扫描**（`ComplianceScanPanel`）：基于 PCPD 基线的规则命中表，字段 `rule / severity / status(PASS|WARN|BLOCK) / detail`；`BLOCK` 必须显式阻断任务。
6. **风险定级与阻断**（`RiskGauge`）：0–100 分、四级 `LOW/MEDIUM/HIGH/CRITICAL`、阈值线、阻断状态、触发原因列表。
7. **任务详情页集成**：`TaskDetailPage` 新增「信任流水线」卡（组件 `features/expert/components/TaskTrustPipeline.tsx`），消费 `GET /api/tasks/:id/trust`，展示 `PipelineStatus` + `RiskGauge`，失败时降级为静态说明，不得整页报错。

新增类型（`src/types/domain.ts`）：

```ts
export type AiToolCategory = 'CLOUD_COMPUTER' | 'AI_CLI' | 'AI_IDE' | 'CHAT_UI';
export type AiToolGrade = 'S' | 'A' | 'B' | 'C';
export type AiToolStatus = 'CONNECTED' | 'PENDING' | 'SANDBOX';
export interface AiToolIntegration {
  id: string; name: string; vendor: string; category: AiToolCategory;
  protocol: string; grade: AiToolGrade; status: AiToolStatus;
  roles: UserRole[]; permissions: string[]; compliance: string[];
}
export type ValidationVerdict = 'PASS' | 'FAIL' | 'PENDING';
export interface CrossValidation {
  taskId: string; generator: string; generatorModel: string; generatorVerdict: ValidationVerdict;
  reviewer: string; reviewerModel: string; reviewerVerdict: ValidationVerdict;
  consensus: ValidationVerdict; confidence: number; cost: number; elapsedMs: number; at: string;
}
export type ComplianceSeverity = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';
export interface ComplianceFinding {
  id: string; rule: string; severity: ComplianceSeverity;
  status: 'PASS' | 'WARN' | 'BLOCK'; detail: string;
}
export type RiskLevel = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';
export interface RiskAssessment {
  score: number; level: RiskLevel; threshold: number; blocked: boolean; reasons: string[];
}
export interface ExpertOverview {
  tools: AiToolIntegration[]; validation: CrossValidation;
  compliance: ComplianceFinding[]; risk: RiskAssessment;
  pricing: { service: string; unit: string; price: number }[];
}
export interface TaskTrust {
  taskId: string; validation: CrossValidation;
  compliance: ComplianceFinding[]; risk: RiskAssessment;
}
```

API：

| Method | Path | 响应 |
|---|---|---|
| GET | `/api/expert/overview` | `ExpertOverview` |
| GET | `/api/tasks/{id}/trust` | `TaskTrust`（未知任务 404） |

口径约束：`validation` 必须按任务状态派生——未进入验证（PUBLISHED/ACCEPTED/IN_PROGRESS 等）返回 `PENDING`，不得伪造 `PASS`；任何 `compliance` 命中 `BLOCK` 时 `risk.blocked` 必须为 `true`，否则为 `false`。

### T13 · 个人中枢：钱包 + API 中转站

**钱包**（`WalletPanel`，放在 `ProfileHeader` 正下方）：

- 余额卡：可用 / 冻结 / VIP 等级 / 到期时间；按钮「充值」「订阅 VIP」。
- 三条资金流说明：雇主托管支付、指挥官收入、平台 AI 增值服务费（按任务抽成）。
- 近 10 笔流水，复用 `WalletTransaction`。
- 交互：充值弹窗（金额 -> `POST /api/wallet/recharge`，mock 同步）；VIP 弹窗（`PRO/ENTERPRISE` -> `POST /api/wallet/vip`）；成功后刷新钱包并 toast。

**API 中转站**（`ApiGatewayPanel`，钱包下方独立栏目）：

- 主流模型目录：DeepSeek、Qwen、GLM、Kimi、MiniMax、Claude、GPT、Gemini；字段 `provider / name / contextWindow / inputPricePerMTok / outputPricePerMTok / latencyMs / status / capabilities`。
- API Key 管理：列表（打码）、创建（返回一次完整 key，仅显示一次）、吊销。
- 用量卡：本期 tokens 入/出、请求数、花费。
- 接入指南：Claude Code / Codex CLI / Cursor / Chatbox 的 Base URL 与模型名示例。
- 闭环条：`模型 API → 发布/完成任务 → 审核 → 支付 → 知识沉淀`。

新增类型：

```ts
export interface WalletVip { level: 'FREE' | 'PRO' | 'ENTERPRISE'; expiresAt: string | null; feeRate: number; }
export interface WalletOverview { balance: WalletBalance; transactions: WalletTransaction[]; vip: WalletVip; }
export interface RechargeInput { amount: number; }
export interface SubscribeVipInput { level: 'PRO' | 'ENTERPRISE'; }
export interface ModelCatalogItem {
  id: string; provider: string; name: string; contextWindow: number;
  inputPricePerMTok: number; outputPricePerMTok: number; latencyMs: number;
  status: 'ONLINE' | 'DEGRADED'; capabilities: string[];
}
export interface ApiKeyRecord { id: string; label: string; maskedKey: string; createdAt: string; lastUsedAt: string | null; scopes: string[]; status: 'ACTIVE' | 'REVOKED'; }
export interface ApiUsage { period: string; tokensIn: number; tokensOut: number; requests: number; spend: number; }
export interface ApiGatewayOverview { models: ModelCatalogItem[]; keys: ApiKeyRecord[]; usage: ApiUsage; }
export interface CreateApiKeyInput { label: string; scopes: string[]; }
export interface CreateApiKeyResponse { key: ApiKeyRecord; plaintextKey: string; }
```

API：

| Method | Path | 说明 |
|---|---|---|
| GET | `/api/wallet` | 扩展返回 `vip`（向后兼容：新增字段） |
| POST | `/api/wallet/recharge` | `{amount}` > 0；返回更新后的 `WalletOverview`；400 非法 |
| POST | `/api/wallet/vip` | `{level}` ∈ PRO/ENTERPRISE；返回更新后的 `WalletOverview` |
| GET | `/api/gateway` | `ApiGatewayOverview` |
| POST | `/api/gateway/keys` | `{label, scopes}` → `CreateApiKeyResponse`（201） |
| POST | `/api/gateway/keys/{id}/revoke` | 返回更新后的 `ApiKeyRecord`；未知 404 |

### T14 · 后端扩展与联调

- `marketplace/expert.go`、`marketplace/gateway.go`：种子数据与 handler；Store 新增 `expert ExpertState`、`gateway GatewayState` 字段；钱包新增 VIP 字段。
- 持久化兼容：旧快照缺少新字段时，`OpenStore` 加载后补种子（不 bump 版本、不重置用户数据）。
- 新增测试：专家总览、任务信任、充值、VIP、API Key 创建/吊销、400/404。
- 联调：`go test -race`、`go vet`、`CGO_ENABLED=0 go build ./...`、`tsc --noEmit`、`npm run build`、HTTP 模式 headless 渲染 `/expert`、`/identity`、`/marketplace/tasks/task_1001`。

DONE：上述命令全绿；重启后新数据仍在；截图/摘录存档到 todo.md。
