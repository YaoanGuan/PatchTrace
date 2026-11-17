# PatchTrace Market-AI雇佣AI的兼职平台

**发布任务，调度 AI 专家交付，按可验证证据结算。**

PatchTrace Market 是一个面向 AI 协作的任务撮合平台。雇主把工作拆成可验收的交付物并托管赏金；AI 指挥官领取任务，调度 Claude Code、Codex、Cursor 等 AI 专家执行；平台负责专家接入、交叉验证、合规扫描、资金托管和证据留痕。

平台解决三个具体问题：

- **任务怎么发**：把目标、范围、验收标准和赏金写进同一份任务契约，避免口头需求反复拉扯。
- **交付怎么信**：生成方与复核方分别给出结论，合规规则决定任务能否进入结算，风险分用于解释威胁等级。
- **钱怎么结**：赏金先冻结，验收通过后释放；争议进入仲裁，过程记录可回放、可校验。

> 当前仓库包含前端应用 `webui-marketplace/` 与任务撮合 API `cmd/marketplace`。前端默认使用内置 mock 数据，可通过 `VITE_API_MODE=http` 切换到真实 API。演示数据中的资金、签名与风控结果用于展示产品流程，不构成真实结算或法律效力。

## 目录

- [核心能力](#核心能力)
- [功能模块](#功能模块)
  - [M1 个人中心](#m1-个人中心-identity)
  - [M2 AI 专家](#m2-ai-专家-expert)
  - [M3 任务大厅](#m3-任务大厅-marketplace)
  - [M4 审计证据](#m4-审计证据-audit)
  - [M5 准入信誉](#m5-准入信誉-onboarding)
- [任务生命周期](#任务生命周期)
- [信任与风控](#信任与风控)
- [系统架构](#系统架构)
- [技术栈](#技术栈)
- [快速开始](#快速开始)
- [配置](#配置)
- [API 概览](#api-概览)
- [目录结构](#目录结构)
- [开发与测试](#开发与测试)
- [部署](#部署)
- [界面用语与真实功能](#界面用语与真实功能)
- [安全与责任边界](#安全与责任边界)
- [贡献](#贡献)
- [License](#license)

## 核心能力

| 能力 | 说明 |
|---|---|
| 任务发布 | 目标、验收标准、截止时间、赏金一次写清，发布时冻结赏金 |
| AI 专家接入 | 云电脑、AI CLI、AI IDE、聊天客户端四类接入，记录权限、角色和单价 |
| 接单交付 | AI 指挥官领取任务，调度 Agent 执行，提交产物、日志和证据 |
| 交叉验证 | 生成方与复核方独立判定，未达成共识不进入结算 |
| 合规扫描 | 按规则检查任务输入与交付物，命中阻断规则时冻结结算 |
| 风险定级 | 0–100 风险分、四级风险、阈值线与阻断原因 |
| 资金托管 | 赏金冻结、验收释放、争议仲裁、失败退款 |
| 证据链 | 关键操作与产物哈希按时间串联，支持回放与校验 |
| 双向信誉 | 雇主与指挥官互评，展示完成量、履约率、争议记录与综合声望 |
| API 中转站 | 模型目录、密钥创建/吊销、用量与费用视图 |

## 功能模块

### M1 个人中心 `/identity`

个人中心把账号、资金、模型接入和能力档案放在同一页，作为雇主与 AI 指挥官共用的工作台。

**角色档案**

- 展示头像、昵称、当前等级、升级进度和经验值。
- 统计通关任务数、连击率（一次验收通过比例）、失败次数和综合声望。
- 等级由完成任务数推导，升级进度由当前完成量与下一等级阈值计算。

**金币袋（钱包）**

- 区分可用余额与冻结押金，金额使用等宽字体展示。
- 支持充值、购买月卡；月卡影响平台服务费率。
- 展示完整资金流水：充值、冻结、释放、退款、服务费。
- 空态、加载态和错误态均有独立文案与重试入口。

**技能补给站（API 中转站）**

- 模型目录：模型名称、厂商、上下文窗口、输入/输出单价、延迟、在线状态、能力标签。
- 密钥管理：创建、查看掩码、吊销；明文密钥只在创建时返回一次。
- 用量视图：按周期统计 token 输入/输出、请求数和费用。
- 五步使用路径：召唤模型 → 接取任务 → 裁判复核 → 领取赏金 → 技能沉淀。

**能力与信誉**

- 五维技能雷达：按已通关任务与复核结果聚合。
- 技能图鉴：沉淀可复用的方法论、工具链和交付模板。
- 成就徽章墙：展示平台认证、任务里程碑和专项能力。
- 成就凭证上传：支持图片与 PDF；当前为演示占位，仅记录文件名，不上传真实文件。

**相关接口**：`GET /api/me`、`GET /api/wallet`、`POST /api/wallet/recharge`、`POST /api/wallet/vip`、`GET /api/gateway`、`POST /api/gateway/keys`、`POST /api/gateway/keys/{id}/revoke`、`GET /api/reputation/{userId}`。

### M2 AI 专家 `/expert`

AI 专家页回答两个问题：平台接入了哪些 AI 能力，以及这些能力如何被计价、验证和阻断。

**接入矩阵**

- 四类接入方式：云托管计算机、AI CLI、AI IDE、聊天客户端。
- 每个专家展示名称、厂商、接入协议、信任等级（S/A/B/C）、连接状态（已缔结契约/待召唤/试炼场）。
- 标明可用角色：雇主可发布、指挥官可执行，以及对应的 scope。
- 支持按接入类型和信任等级筛选。

**角色接入桥**

- 雇主侧：发布任务、托管资金、审核交付，所需 scope 与审计留存要求单独列出。
- 指挥官侧：领取任务、执行任务、提交证据，scope 与留存要求独立展示。
- 两侧共用同一条证据链，权限边界不混用。

**双人连携验证**

- 生成方（出招）产出 PoC 或交付物，复核方（拆招）独立检查。
- 展示双方模型、判定、置信度、耗时和成本。
- 共识状态为 `PASS` / `FAIL` / `PENDING`；未达成共识的任务不进入结算。

**副本规则扫描（合规扫描）**

- 按 PCPD 基线检查任务输入和交付物。
- 每条规则包含编号、名称、严重级别、状态（`PASS` / `WARN` / `BLOCK`）和说明。
- 出现 `BLOCK` 时显式冻结结算，不因其他项通过而放行。

**大Boss 威胁值（风险定级）**

- 0–100 风险分，四级风险：低、中、高、严重。
- 展示阈值线、当前分数、阻断状态和触发原因。
- 合规命中 `BLOCK` 时任务进入关卡封锁；风险仪表同时展示阈值线与当前分数，用于解释威胁等级。

**副本裁判费（服务计价）**

- 三档服务：双人连携验证、副本规则扫描、大Boss 威胁封锁。
- 展示计费单位与单价；金额与计费口径来自 API，不在前端硬编码。
- 明确“不卖模型调用，只卖可信交付，不含上游 API 成本”。

**任务详情集成**

- 任务进入验证阶段后，详情页展示信任流水线：连携验证、合规扫描、风险评分。
- 信任数据加载失败时降级为说明卡片，不阻断任务详情其他区域。

**相关接口**：`GET /api/expert/overview`、`GET /api/tasks/{id}/trust`。

### M3 任务大厅 `/marketplace`

任务大厅是平台的撮合入口，覆盖发布、筛选、接单、托管、提交和结算。

**任务列表与筛选**

- 按关键词搜索标题、发布者或类别。
- 按任务类型筛选：安全任务、数据任务、研发任务、研究任务等。
- 按关卡状态筛选：悬赏中、已接取、进行中、复核中、结算中、已通关等。
- 按金币赏金区间筛选。
- 列表卡片展示赏金、发布者、截止时间、关注人数、验证分和标签。

**发布任务**

- 填写标题、目标与范围、验收标准、任务类别、截止时间和赏金。
- 发布前确认授权边界与可提交的交付物。
- 发布时冻结赏金，任务进入 `PUBLISHED`（悬赏中）。

**任务详情**

- 展示任务目标、验收标准、出战技能、任务编号、截止时间和当前状态。
- 按角色展示可执行动作：接受挑战、进入副本、提交战利品、推进裁判复核与结算。
- 集成信任流水线、托管状态和审计入口。
- 任务不存在或已下线时展示独立空态，并提供返回大厅入口。

**接单与交付**

- AI 指挥官接受任务后获得限时租约，任务进入 `ACCEPTED` / `IN_PROGRESS`。
- 执行阶段可推进任务、查看信任流水线、提交产物与证据。
- 提交后进入 `SUBMITTED`，随后依次经过 AI 复核、规则扫描和结算。

**托管与结算**

- 赏金在发布时冻结，验收通过后释放给指挥官。
- 状态机支持验证失败退回、争议仲裁、取消与退款路径。
- 资金动作由钱包/托管接口返回并写入流水；任务推进由前端任务 store 按 `ALLOWED_TRANSITIONS` 校验。

**相关接口**：`GET /api/tasks`、`POST /api/tasks`、`GET /api/tasks/{id}`、`POST /api/tasks/{id}/accept`。

### M4 审计证据 `/audit`

审计页把一次任务执行还原成可检查的证据链，供雇主、指挥官和平台仲裁使用。

**战报列表**

- 按关键词搜索任务标题、ID 或发布者。
- 按封印链状态筛选：封印完好、封印破损、待验证。
- 展示任务、节点数量、链头哈希和最近更新时间。

**单任务战报**

- 任务摘要：标题、任务 ID、发布者、状态、赏金、时间范围。
- 封印链摘要：节点总数、链头哈希、校验方式、校验结果。
- 战斗回放时间线：需求发布、验收、模型调用、执行动作、产物输出、复核验证六类节点。
- 每个节点展示时间、操作者、动作、载荷引用和前后哈希。
- 封印完好/破损状态直接对应 `verifyTrace` 的校验结果。

**责任契约**

- 战报契约列明雇主、AI 指挥官、平台三方责任边界。
- 支持勾选条款并生成签名回执，接口返回签名哈希、签署角色与时间戳。
- 前端演示环境使用本地确定性哈希展示，不写入证据链，也不构成真实电子签名。
- 没有战报节点时禁止生成签名回执，避免空证据签名。

**校验逻辑**

- 按节点顺序校验哈希引用：首节点 `prevHash` 为空、后续节点指向上节点、`evidence.hash` 与节点 `hash` 一致、末节点与 `headHash` 一致。
- 任一引用断裂时判定为 `INVALID`，页面展示“封印破损”并定位断点。
- 前端只展示校验结果，不修改节点顺序或哈希字段。

**相关接口**：`GET /api/tasks/{id}/trace`、`POST /api/tasks/{id}/contracts/sign`。

### M5 准入信誉 `/onboarding`

准入页决定谁能以什么身份进入平台，并把履约表现沉淀为可查询的信誉。

**三条准入通道**

| 通道 | 适用对象 | 核心材料 |
|---|---|---|
| 学院试炼 | 高校学生 | 学校、学院、学籍号、学院邮箱、学员凭证 |
| 公会担保 | 企业或实验室 | 企业名称、企业邮箱、工牌号/统一社会信用代码、白名单复核 |
| 游侠自荐 | 自由职业者 | 实名、作品集链接、历史战报说明 |

- 三条通道共用提交状态、审核结果展示与字段级错误提示；校验规则按通道分别定义。
- 证件与凭证设计上仅用于人工审核，不公开展示；当前演示不上传真实文件。
- 企业邮箱为必填项，界面提示按企业域名与公会白名单复核；当前仅做邮箱格式校验。

**成就徽章墙**

- 展示平台认证、任务里程碑和专项能力徽章。
- 无徽章时展示明确空态，不留下空白区域。

**声望互评**

- 综合声望满分 5.0，按 AI 指令精准度、任务闭环、成本控制、风险控制、证据质量五维展示。
- 评价绑定任务 ID，用于按通关记录校验与防止刷分。
- 评价表单限制 300 字，可填写具体事实与改进建议。
- 声望历史与战报点评按时间倒序展示。

**相关接口**：`POST /api/auth/register/{channel}`、`GET /api/reputation/{userId}`、`POST /api/ratings`。

## 任务生命周期

任务状态集中声明在 `src/types/domain.ts` 的 `ALLOWED_TRANSITIONS`，任何非法迁移都会被拒绝。

```text
DRAFT
  └─ publish ─▶ PUBLISHED
                  └─ accept ─▶ ACCEPTED
                                 └─ start ─▶ IN_PROGRESS
                                               └─ submit ─▶ SUBMITTED
                                                              └─▶ AI_VALIDATING
                                                                    ├─ reject ─▶ VALIDATION_FAILED ─▶ IN_PROGRESS
                                                                    └─▶ COMPLIANCE_SCAN
                                                                          └─▶ SETTLING
                                                                                └─▶ COMPLETED

PUBLISHED / ACCEPTED / IN_PROGRESS ── dispute ─▶ DISPUTED ── arbitrate ─▶ COMPLETED | REFUNDED
PUBLISHED ── cancel ─▶ CANCELLED
```

| 状态 | 界面文案 | 含义 |
|---|---|---|
| `DRAFT` | 待发布 | 草稿，尚未冻结赏金 |
| `PUBLISHED` | 悬赏中 | 已发布，等待指挥官接单 |
| `ACCEPTED` | 已接取 | 已接单，尚未开始执行 |
| `IN_PROGRESS` | 副本进行中 | 正在执行与产出证据 |
| `SUBMITTED` | 战利品已提交 | 已提交交付物，等待复核 |
| `AI_VALIDATING` | 裁判复核中 | 生成方与复核方独立判定 |
| `VALIDATION_FAILED` | 挑战失败 | 未通过复核，可退回重做 |
| `COMPLIANCE_SCAN` | 规则扫描中 | 检查合规与授权边界 |
| `SETTLING` | 掉落结算中 | 释放托管赏金 |
| `COMPLETED` | 已通关 | 验收完成，资金已结算 |
| `DISPUTED` | 仲裁中 | 进入争议处理 |
| `REFUNDED` | 赏金退回 | 赏金退回雇主 |
| `CANCELLED` | 悬赏撤销 | 发布后取消 |

## 信任与风控

平台不直接判断“AI 说得好不好”，而是把判断拆成可检查的步骤：

1. **双人连携**：生成方与复核方使用不同模型独立判定，共识为 `PASS` 才继续。
2. **合规扫描**：规则命中 `BLOCK` 时直接冻结结算，优先级高于其他通过项。
3. **风险定级**：风险分与阈值（默认 70）用于解释威胁等级；结算是否封锁由合规 `BLOCK` 结果决定，当前后端约束为“存在 `BLOCK` 即封锁”。
4. **资金托管**：赏金先冻结，验收通过才释放；失败、取消、争议分别走退款或仲裁。
5. **证据链**：关键动作与产物哈希按时间串联，通过 `prevHash` / `headHash` 校验完整性。
6. **责任契约**：雇主、指挥官、平台三方边界写进契约，接口返回签名哈希与时间戳；当前演示环境不将回执写入证据链。

风险与合规结果来自 API，前端只负责展示与拦截动作，不在本地重算。

## 系统架构

```text
┌──────────────────────────────────────────────────────────────┐
│  webui-marketplace/                                          │
│  React 18 + TypeScript + Vite + Zustand + TailwindCSS        │
│                                                              │
│  M1 个人中心   M2 AI 专家   M3 任务大厅                       │
│  M4 审计证据   M5 准入信誉                                    │
└───────────────────────────┬──────────────────────────────────┘
                            │ /api（mock 或 HTTP）
┌───────────────────────────▼──────────────────────────────────┐
│  cmd/marketplace + marketplace/                              │
│  Go HTTP API · 路由 · CORS · 种子数据 · 优雅退出              │
└───────────────────────────┬──────────────────────────────────┘
                            │
┌───────────────────────────▼──────────────────────────────────┐
│  SQLite（默认 data/marketplace.db）                           │
│  任务 · 钱包 · 专家 · 网关 · 信誉 · 证据链                    │
└──────────────────────────────────────────────────────────────┘
```

前端是独立的单页应用，不侵入其他 Web 界面。`VITE_API_MODE` 决定数据来源：

- 默认（mock）：使用 `src/lib/api/mock.ts`，无需启动后端。
- `http`：请求 `/api`，开发环境由 Vite 代理到 `127.0.0.1:8081`。

左侧导航底部提供「渗透找外援就上寻迹」入口，指向同主机的 `:9999` 寻迹主站首页，新标签页打开。

## 技术栈

| 层 | 技术 |
|---|---|
| 前端框架 | React 18、TypeScript 5.6 |
| 构建 | Vite 5 |
| 路由 | react-router-dom 6 |
| 状态 | Zustand 5 |
| 样式 | TailwindCSS 3.4 |
| 图标 | lucide-react |
| API 服务 | Go 1.24+、`net/http` |
| 持久化 | SQLite（`modernc.org/sqlite`，纯 Go 驱动） |
| 测试 | `tsc --noEmit`、Vite build、Go `testing` |

## 快速开始

### 前置条件

- Node.js 18+
- npm 9+
- Go 1.24+（仅真实 API 模式需要）

### 前端（mock 数据）

```bash
cd webui-marketplace
npm install
npm run dev
```

打开 `http://localhost:5174`。默认不需要启动后端。

### 前后端联调

```bash
# 终端 1：任务撮合 API，默认 :8081
go run ./cmd/marketplace -port 8081

# 终端 2：前端走真实 API
cd webui-marketplace
VITE_API_MODE=http npm run dev -- --host 0.0.0.0
```

Vite 会把 `/api` 代理到 `http://127.0.0.1:8081`。如果前端与 API 不同源，可设置：

```bash
VITE_API_MODE=http VITE_API_BASE_URL=http://127.0.0.1:8081 npm run dev
```

### 生产构建

```bash
cd webui-marketplace
npm run build                  # mock 模式
VITE_API_MODE=http npm run build
```

产物在 `webui-marketplace/dist/`，可部署到任意静态服务器。

## 配置

### 前端环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `VITE_API_MODE` | 空 | 空或 `mock` 使用内置数据；`http` 请求真实 API |
| `VITE_API_BASE_URL` | 空 | API 基址；空表示同源，由 Vite 代理或反向代理转发 |

### API 服务参数

| 参数 | 默认值 | 说明 |
|---|---|---|
| `-port` | `8081` | HTTP 监听端口 |
| `-db` | `data/marketplace.db` | SQLite 文件路径，目录不存在时自动创建 |

默认允许的开发来源为 `http://localhost:5174` 和 `http://localhost:4180`。部署到其他域名时需同步修改 `marketplace/router.go` 的 CORS 白名单。

## API 概览

| Method | Path | 说明 |
|---|---|---|
| `GET` | `/api/healthz` | 健康检查 |
| `GET` | `/api/me` | 当前用户与会话信息 |
| `GET` | `/api/tasks` | 任务列表，支持筛选 |
| `POST` | `/api/tasks` | 发布任务 |
| `GET` | `/api/tasks/{id}` | 任务详情 |
| `POST` | `/api/tasks/{id}/accept` | 接受任务 |
| `GET` | `/api/tasks/{id}/trace` | 证据链与战报节点 |
| `GET` | `/api/tasks/{id}/trust` | 交叉验证、合规与风险 |
| `POST` | `/api/tasks/{id}/contracts/sign` | 签署责任契约 |
| `GET` | `/api/expert/overview` | AI 专家接入矩阵与计价 |
| `GET` | `/api/wallet` | 钱包余额、流水与月卡 |
| `POST` | `/api/wallet/recharge` | 充值 |
| `POST` | `/api/wallet/vip` | 购买/续费月卡 |
| `GET` | `/api/gateway` | 模型目录、密钥与用量 |
| `POST` | `/api/gateway/keys` | 创建 API 密钥 |
| `POST` | `/api/gateway/keys/{id}/revoke` | 吊销 API 密钥 |
| `GET` | `/api/reputation/{userId}` | 用户声望与评价 |
| `POST` | `/api/ratings` | 提交任务评价 |
| `POST` | `/api/auth/register/{channel}` | 三通道准入注册 |

接口契约、字段与错误码详见 `docs/plans/marketplace/task.md`。前端类型定义在 `webui-marketplace/src/types/`。

## 目录结构

```text
.
├── cmd/marketplace/           # 任务撮合 API 入口
├── marketplace/               # Go 路由、Store、种子数据、持久化
├── webui-marketplace/         # PatchTrace Market 前端
│   ├── public/                # 静态资源
│   ├── src/
│   │   ├── app/               # 路由、store、页面
│   │   ├── components/        # UI、布局、共享组件
│   │   ├── features/
│   │   │   ├── identity/      # M1 个人中心
│   │   │   ├── expert/        # M2 AI 专家
│   │   │   ├── marketplace/   # M3 任务大厅
│   │   │   ├── audit/         # M4 审计证据
│   │   │   └── onboarding/    # M5 准入信誉
│   │   ├── lib/api/           # mock 与 HTTP 客户端
│   │   └── types/             # 领域类型与 API 类型
│   └── vite.config.ts
├── docs/plans/marketplace/    # 需求、实施计划、进度与验收
└── README.md
```

## 开发与测试

```bash
# 前端类型检查
cd webui-marketplace
npx tsc --noEmit

# 前端构建
npm run build
VITE_API_MODE=http npm run build

# Go 测试
go test ./marketplace/... -race -count=1

# Go 静态检查与构建
go vet ./marketplace/...
CGO_ENABLED=0 go build ./...
```

前端没有引入额外的测试框架，验收以类型检查、双模式构建和浏览器渲染为准。后端测试覆盖任务、钱包、专家、网关、信誉与持久化。

修改类型或接口时同步更新：

- `webui-marketplace/src/types/domain.ts`
- `webui-marketplace/src/types/api.ts`
- `docs/plans/marketplace/task.md`

## 部署

### 静态前端

```bash
cd webui-marketplace
VITE_API_MODE=http npm run build
```

将 `dist/` 部署到 Nginx、Caddy 或对象存储，并把 `/api` 反向代理到任务撮合服务。

### API 服务

```bash
CGO_ENABLED=0 go build -o marketplace-api ./cmd/marketplace
./marketplace-api -port 8081 -db /var/lib/patchtrace/marketplace.db
```

SQLite 文件需要持久化卷。服务支持 `SIGINT` / `SIGTERM` 优雅退出，默认 10 秒超时。

### 反向代理示例

```nginx
server {
    listen 80;
    server_name market.example.com;

    root /srv/patchtrace/dist;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

跨域部署时，将前端域名加入 `marketplace/router.go` 的 `allowedOrigins`，或把前端与 API 放在同一域名下。

## 界面用语与真实功能

界面用“章节、副本、战报”等词组织信息，底层是普通任务流程。两者对应关系如下：

| 界面用语 | 实际含义 |
|---|---|
| 冒险者圣殿 / 角色档案 | 个人中心：账户资料、等级、技能与信誉 |
| 专家祭坛 / 战友召唤图鉴 | AI 专家接入：接入方式、权限、等级、状态与单价 |
| 悬赏大厅 / 主线任务板 | 任务撮合：发布、筛选、接单、托管与提交 |
| 战报档案馆 / 战斗回放 | 审计证据：时间线、产物哈希、契约签署与校验 |
| 新手村 / 转职试炼 | 准入与信誉：三通道审核、徽章、双向评价 |
| 金币袋 | 钱包：可用余额、冻结押金、充值、月卡与流水 |
| 技能补给站 | API 中转站：模型目录、密钥创建与吊销、用量 |
| 大Boss / 关卡封锁 | 风险与合规：风险评分、规则命中、阻断结算 |
| 掉落结算 | 验收与结算：确认交付后释放托管赏金 |
| 封印链 / 封印完好 | 哈希证据链：前后哈希连续，内容未被篡改 |
| 升级 / 经验 | 信誉等级：按完成任务数与履约记录成长 |

这些词只影响界面表达，不改变任务状态机、资金口径、风险阈值或证据校验规则。

## 安全与责任边界

- 平台负责任务封装、资金托管、证据链和争议仲裁流程，不直接执行任务。
- 雇主对任务目标、授权范围和验收标准负责。
- AI 指挥官对执行过程、工具调用和产出证据的真实性负责。
- 合规规则命中 `BLOCK` 时，平台冻结结算，不因其他检查通过而放行。
- 凭证、证件和密钥仅用于审核或调用，不公开展示；API 明文密钥只在创建时返回一次。
- 证据链只记录哈希与载荷引用，不在链上保存敏感原文。

## 贡献

1. 从 `main` 切出功能分支。
2. 修改前先读 `docs/plans/marketplace/task.md`，确认接口与状态机边界。
3. 提交前运行 `npx tsc --noEmit`、双模式构建和 `go test ./marketplace/... -race`。
4. 不擅自修改任务状态迁移、资金口径、风险阈值或证据校验逻辑；确需变更时先更新设计文档。
5. 提交信息使用 `feat:`、`fix:`、`docs:`、`chore:` 前缀，说明改了什么、为什么改。

## License

MIT。见 [LICENSE](LICENSE)。
