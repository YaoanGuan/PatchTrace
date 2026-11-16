# PatchTrace Marketplace 前端待办（todo.md）

> 配套：`docs/plans/marketplace/task.md`
> 分支：`feat/ai-marketplace-frontend`
> 规则：没有命令输出不得打勾；不改验收标准。

---

## 0. 下一步动作（只做这一件）

> T0–T6 全部完成：设计、地基、4 个业务模块、集成验收。
> 证据：全量 `tsc --noEmit` 0 错误；`npm run build` exit 0；6 条路由 headless 渲染通过。
> 下一动作：`cd webui-marketplace && npm run dev`（端口 5174），浏览器人工走查 4 个模块的交互。

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
