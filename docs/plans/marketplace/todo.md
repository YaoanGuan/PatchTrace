# PatchTrace Marketplace 前端待办（todo.md）

> 配套：`docs/plans/marketplace/task.md`
> 分支：`feat/ai-marketplace-frontend`
> 规则：没有命令输出不得打勾；不改验收标准。

---

## 0. 下一步动作（只做这一件）

> T0–T9 全部完成：设计、地基、4 个业务模块、Go API 契约、前端 HTTP 接入、端到端联调。
> 证据：`go test ./marketplace/... -race` ok；`tsc` exit 0；`npm run build` exit 0；HTTP 模式渲染出 curl 新建任务。
> 下一动作：两个终端分别执行 `go run ./cmd/marketplace -port 8081` 与 `cd webui-marketplace && VITE_API_MODE=http npm run dev`，浏览器走查。
> 注意：memory 存储重启即重置；默认 mock 模式仍可单独运行前端。

---

## 1. 进度总览

| # | Task | 模块 | 状态 | 验收 | 证据 |
|---|---|---|---|---|---|
| T0 | 架构设计与计划 | — | 完成 | 文档评审 | `docs/plans/marketplace/task.md` |
| T1 | 地基：脚手架 + 路由 + store + mock API + 外壳 | — | 完成 | tsc + build | `npx tsc --noEmit` exit 0；`npm run build` exit 0 |
| T2 | 身份中枢 | M1 | 完成 | tsc | `src/features/identity` 零错误；headless 渲染含五维雷达/徽章墙 |
| T3 | 撮合大厅 + 流水线 + 托管 | M2 | 完成 | tsc | `src/features/marketplace` 零错误；headless 渲染含流水线/托管 |
| T4 | 审计与免责契约 | M3 | 完成 | tsc | `src/features/audit` 零错误；篡改链路测试 INVALID |
| T5 | 准入与双向信誉 | M4 | 完成 | tsc | `src/features/onboarding` 零错误；headless 渲染含三通道/评分 |
| T6 | 集成验收 + 提交 | — | 完成 | tsc + build + commit | 全量 tsc 0 错误；build exit 0；7 路由渲染通过 |
| T7 | Go API 服务 | — | 完成 | go test + curl | `go test -race` ok；11 接口 curl 矩阵通过 |
| T8 | 前端 HTTP 客户端 | — | 完成 | tsc + build | tsc/build/HTTP 模式构建均 exit 0 |
| T9 | 前后端联调 | — | 完成 | 渲染 + curl | HTTP 模式渲染 9 任务（含 curl 新建）；证据链完整 |

---

## 2. 分任务清单

### T1 · 地基（P8-foundation）

- [x] 脚手架：`package.json`、`tsconfig*.json`、`vite.config.ts`、`index.html`、`tailwind.config.cjs`、`postcss.config.cjs`
- [x] 四个 Zustand store：session / task / wallet / ui；`ALLOWED_TRANSITIONS` 集中声明
- [x] 类型：`src/types/domain.ts`、`src/types/api.ts` 与 task.md §4 字段一致
- [x] mock API：`src/lib/api/client.ts` + `mock.ts`，覆盖 task.md §4.1–§4.8
- [x] UI 基础组件：Button / Card / Badge / Modal / Tabs / Progress / EmptyState
- [x] 外壳：`AppShell`、`TopBar`、`SideNav`、`RoleSwitcher`、`TrustBadgeWall`、`WalletChip`
- [x] 路由：8 条路由 + 懒加载 + 7 个占位页面
- [x] 验证：`npm install` → `npx tsc --noEmit` → `npm run build`
  - 证据：`npm install` added 139 packages exit 0；`npx tsc --noEmit` → `TSC_EXIT=0`；`npm run build` → `✓ built in 8.54s` `BUILD_EXIT=0`（P9 复核）
  - 备注：npm 12 拦截 esbuild postinstall，但本机构建正常；换机若报 esbuild binary 缺失，执行 `npm install-scripts approve esbuild`

### T2 · 身份中枢（P8-identity）

- [x] `ProfileHeader` + `ReputationPill`
- [x] `SkillRadarChart`（5 维）+ `SkillLegend`
- [x] `ExpertiseRepository`（Tabs + ExpertiseCard + DataAssetCard）
- [x] `TrustBadgeWall` 接入 + `CredentialUploadDialog`
- [x] 验证：`npx tsc --noEmit`
  - 证据：identity 目录 0 错误；`/identity` headless DOM 含「个人中枢」「AI指令精准度」「Agent 能力」

### T3 · 撮合大厅（P8-marketplace）

- [x] `MarketplacePage`：`TaskMarketplace` + `TaskFilters` + `TaskPublishForm` + `TaskList/TaskCard` + `AcceptTaskPanel`
- [x] `AIValidationPipeline`：四阶段状态 + `ValidationVerdictCard` + `CrossCheckBadge`
- [x] `EscrowWallet`：`BalanceSummary` + `EscrowTaskRow` + `TransactionList`
- [x] `TaskDetailPage`：摘要 + 流水线 + 托管 + 动作面板 + 审计入口
- [x] 验证：`npx tsc --noEmit`
  - 证据：marketplace 目录 0 错误；`/marketplace` 与 `/marketplace/tasks/task_1001` headless DOM 含流水线/赏金/托管

### T4 · 审计（P8-audit）

- [x] `AuditPage`：筛选 + 任务列表 + 哈希链摘要
- [x] `TraceabilityTimeline`：6 类节点 + 证据字段 + `HashChainStatus`
- [x] `LegalLiabilityModal`：三方责任清单 + 条款勾选 + 签名哈希
- [x] `ComplianceScanReport`
- [x] 验证：`npx tsc --noEmit`
  - 证据：audit 目录 0 错误；`/audit/task_1001` headless DOM 含「证据链」「哈希链」「链完整」；篡改 prevHash/headHash 三场景均判 INVALID

### T5 · 准入信誉（P8-onboarding）

- [x] `MultiChannelRegistration`：三通道 Tabs + 三套表单
- [x] `ReputationSystem`：`ReputationScoreCard` + 双向 `RatingForm` + `RatingHistory`
- [x] 验证：`npx tsc --noEmit`
  - 证据：onboarding 目录 0 错误；`/onboarding` headless DOM 含高校/企业白名单/自由职业与双向评分

### T6 · 集成验收（P9）

- [x] `npx tsc --noEmit` 通过
  - 证据：`TSC_EXIT=0`，`grep -c "error TS"` → 0
- [x] `npm run build` 通过
  - 证据：`✓ built in 9.05s`，`BUILD_EXIT=0`
- [x] 路由清单与 task.md §1 一致
  - 证据：7 个页面文件齐备；6 条实际路由 headless 渲染出模块内容（identity/marketplace/task/audit/audit-detail/onboarding）
- [x] 4 模块组件树与 task.md §3 对照无缺项
  - 证据：`src` 下 72 个 `.tsx`；marker grep 命中各模块核心组件文案
- [x] 提交到 `feat/ai-marketplace-frontend`
  - 证据：`b086f92 feat(marketplace): add AI task marketplace frontend`；已推送 `origin/feat/ai-marketplace-frontend`（104 文件）
- [x] 视觉走查与一致性修复
  - 证据：6 路由 headless 截图；修复状态文案双口径（PUBLISHED 统一「招募中」）、接单卡徽标竖排、哈希卡换行；`tsc` exit 0、`build` exit 0

### T7 · Go API 服务（P8-backend）

- [x] `marketplace/` 包：types / store / handlers / router，字段对齐 task.md §4
- [x] `cmd/marketplace/main.go`：`-port` 默认 8081，CORS，优雅退出
- [x] 种子数据：7 条任务、task_1001 6 节点链、钱包、信誉
- [x] 11 个接口 + healthz，非法参数 400、非法迁移 409、错误体 `{"error":...}`
- [x] `go test ./marketplace/...` 覆盖接口与错误路径
- [x] 验证：`go test`、`go vet`、curl 矩阵
  - 证据：`go test ./marketplace/... -race -count=1` → `ok AIxVuln/marketplace 1.428s`；`go vet` 无输出；`CGO_ENABLED=0 go build ./...` exit 0
  - curl：healthz 200、me 200、tasks 7 条、publish 201（task_1008）、accept 200（lease +2h）、sign 200（sha256 确定性）、bad ack 400、非法迁移 409、未知任务 404

### T8 · 前端 HTTP 客户端（P8-frontend）

- [x] `client.ts` 新增 HTTP 实现，`VITE_API_MODE=http` 切换，签名与 mock 一致
- [x] `vite.config.ts` dev/preview 代理 `/api` → `127.0.0.1:8081`
- [x] `.env.example`
- [x] 验证：`tsc`、mock 构建、HTTP 模式构建
  - 证据：`npx tsc --noEmit` → `TSC_EXIT=0`；`npm run build` → exit 0；`VITE_API_MODE=http npm run build` → exit 0（产物含 `/api/me`，mock 构建不含）

### T9 · 前后端联调（P9）

- [x] Go 服务启动 + HTTP 模式前端渲染关键路由
  - 证据：preview 代理 `/api/healthz` 200；`/marketplace` 渲染 9 张任务卡（7 种子 + 2 条 curl 新建，含 `E2E-联调-*`）；`/audit/task_1001` 显示「链完整」；`/identity` 显示真实 `/api/me` 数据（mock 模式 6 路由渲染已在 T6 覆盖）
- [x] curl 抽查发布/接单/签署
  - 证据：publish 201 → task_1008；accept 200 lease `+2h`；sign 200 两次哈希一致
- [x] 提交并推送
  - 证据：见 `git log feat/ai-marketplace-frontend` 最新提交

---

## 3. 阻塞

| 日期 | Task | 现象 | 需要谁决策 | 状态 |
|---|---|---|---|---|
| — | — | 无 | — | — |

---

## 4. 状态回写格式

```text
- [x] T? 步骤
  证据：<命令> → <关键输出一行>
  变更：<文件列表>
  下一步：<两分钟内可执行的动作>
```
