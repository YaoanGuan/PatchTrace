# PatchTrace Marketplace 前端待办（todo.md）

> 配套：`docs/plans/marketplace/task.md`
> 分支：`feat/ai-marketplace-frontend`
> 规则：没有命令输出不得打勾；不改验收标准。

---

## 0. 下一步动作（只做这一件）

> T0–T20 已完成。本轮迭代：游戏化皮肤（赛博地牢 HUD）。
> 方案：`docs/plans/marketplace/game-theme-plan.md`；执行计划：`docs/superpowers/plans/2026-11-17-game-theme-ui.md`。
> 下一动作：审阅 §2.15 验收证据；确认后提交（当前未 commit / 未 push）。
> 注意：只改前端表现层；API 契约、类型字段、状态机、风险口径保持不变。

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
| T10 | SQLite 持久化 | — | 完成 | go test + 重启验证 | `go test -race` ok；重启后 curl 查回发布任务 |
| T11 | 品牌与导航 | — | 完成 | tsc + 渲染 | 标题/广告语/M2 导航均渲染 |
| T12 | M2 AI 专家（信任收费站） | M2 | 完成 | tsc + 渲染 | 7 工具四类、82/高风险/已阻断 |
| T13 | 个人中枢钱包 + API 中转站 | M1 | 完成 | tsc + 渲染 | 余额 3300、8 模型、Key 管理 |
| T14 | 后端扩展与联调 | — | 完成 | go test + 三路由 | 8 接口、迁移、重启持久化 |
| T15 | 游戏化主题基座 | — | 完成 | tsc + 截图 | 调色板反转、HUD、game 组件；tsc/双构建 exit 0 |
| T16 | M1 角色圣殿 | M1 | 完成 | tsc + 渲染 | DOM 155,525B；六标记全中 |
| T17 | M2 专家祭坛 | M2 | 完成 | tsc + 渲染 | DOM 142,703B；五标记全中；82/HIGH/blocked |
| T18 | M3 悬赏大厅 | M3 | 完成 | tsc + 渲染 | DOM 147,698B；六标记全中；task_1003 BLOCK/PENDING |
| T19 | M4/M5 战报与新手村 | M4/M5 | 完成 | tsc + 渲染 | audit 114,342B / detail 128,814B / onboarding 125,206B；标记全中 |
| T20 | 游戏化验收 | — | 完成 | 截图 + build | 10 路由非空；5 词齐全；双构建 0；12 张截图 |

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

### T10 · SQLite 持久化（P8-backend-db）

- [x] `OpenStore(path)` / `Close()`，WAL + busy_timeout + 单连接
- [x] 快照表 `marketplace_state`，首次写种子，变更时锁内事务 upsert
- [x] PublishTask / AcceptTask / SubmitRating / RegisterChannel / SignContract 落盘
- [x] `cmd/marketplace` 新增 `-db`（默认 `data/marketplace.db`），自动建目录
- [x] 测试：重启后发布/接单/评分仍在；并发 -race
  - 用例：`TestOpenStoreSeedsData`、`TestOpenStorePersistsAcrossRestart`、`TestOpenStoreRejectsUnsupportedVersion`、`TestOpenStoreRejectsCorruptPayload`、`TestOpenStoreConcurrentWrites`
- [x] 验证：`go test`、`go vet`、`build`、实机重启 curl
  - 证据：`go test ./marketplace/... -race -count=1` → `ok AIxVuln/marketplace 1.471s`；`go vet` 无输出；`gofmt -l marketplace cmd/marketplace` 无输出；`CGO_ENABLED=0 go build ./...` exit 0
  - 实机：`/tmp/mkt-server -port 8083 -db /tmp/mkt-verify.db` 发布 `P9重启验证-1794887498` → 201，`total=1`；SIGTERM 停止后同库重启 → `healthz_after_restart 200`、`total=1`、标题一致；DB 文件 40960 bytes

### T11 · 品牌与导航（P8-shell）

- [x] `index.html` 标题改为 `PatchTrace Market · AI 雇佣 AI 的教育平台`
- [x] `TopBar`：品牌副标题 + 广告语 `Trace the Patch, Uncover the Flaw.`，窄屏降级
- [x] `SideNav` 增加 `/expert`（M2），任务大厅 M3、审计 M4、准入 M5
- [x] 页面眉标改号：marketplace M3、audit/audit-detail M4、onboarding M5
- [x] `router.tsx` 新增 `/expert` 懒加载路由
- [x] `TaskDetailPage` 接入「信任流水线」卡（引用 `features/expert/components/TaskTrustPipeline`）
- [x] 验证：`npx tsc --noEmit`、`npm run build`
  - 证据：headless DOM `<title>PatchTrace Market · AI 雇佣 AI 的教育平台</title>`；TopBar 命中 `Trace the Patch, Uncover the Flaw.`；SideNav 五项 M1–M5；`npm run build` → `✓ built in 7.45s` exit 0

### T12 · M2 AI 专家（P8-expert）

- [x] `ExpertPage` + Hero（信任收费站定位与计价）
- [x] `AccessMatrix`：云托管计算机 / AI CLI / AI IDE / 聊天客户端四类全覆盖，S/A/B/C 分级，角色权限
- [x] `RoleBridge`：雇主发布 / 指挥官执行两类接入
- [x] `CrossValidationPanel` + `PipelineStatus`：Claude 生成 → Codex 复核 → 共识
- [x] `ComplianceScanPanel`：PCPD 基线，PASS/WARN/BLOCK
- [x] `RiskGauge`：0–100、四级、阈值、阻断原因
- [x] `TaskTrustPipeline`：供任务详情页复用
- [x] 验证：`npx tsc --noEmit`；headless `/expert` 含接入矩阵与风险仪表
  - 证据：`/expert` DOM 114916 bytes，命中「信任收费站 / 接入矩阵 / 交叉验证 / 合规扫描 / 风险阻断」；7 工具全部渲染（Grok Bot、ChatGPT Dot、Claude Code、Codex CLI、Cursor、Trae、Chatbox），四类 category 齐全
  - 口径修复：overview 风险改为 `82/HIGH/blocked=true`（1 条 BLOCK）；任务级 validation 按状态派生，`task_1001` 全 PENDING、`task_1003` blocked=true、`task_1006` PASS；不变量 `blocked == hasBlock(compliance)` 全为 true

### T13 · 个人中枢钱包 + API 中转站（P8-identity）

- [x] `WalletPanel`：余额 / 冻结 / VIP / 充值 / 订阅 / 三条资金流 / 流水
- [x] `ApiGatewayPanel`：主流模型目录、Key 创建/吊销、用量、接入指南、五步闭环
- [x] `IdentityPage`：钱包紧贴 `ProfileHeader`，API 中转站为独立栏目
- [x] `walletStore` 扩展 recharge / subscribeVip
- [x] 验证：`npx tsc --noEmit`；headless `/identity` 含「钱包」「API 中转站」
  - 证据：`/identity` DOM 125428 bytes；渲染可用余额 3,300、冻结 1,200、VIP 免费版、充值/订阅按钮；模型目录 8 个；API Key 两条（含验证期创建的 `sk-pt-****-a2f9`）；五步闭环「模型 API → 发布/完成任务 → 审核 → 支付 → 知识沉淀」

### T14 · 后端扩展与联调（P8-backend）

- [x] `GET /api/expert/overview`、`GET /api/tasks/{id}/trust`
- [x] `POST /api/wallet/recharge`、`POST /api/wallet/vip`
- [x] `GET /api/gateway`、`POST /api/gateway/keys`、`POST /api/gateway/keys/{id}/revoke`
- [x] Store/持久化兼容旧快照，新增字段补种子
- [x] Go 测试覆盖成功/400/404；`-race` 全绿
- [x] 联调：HTTP 模式 headless `/expert`、`/identity`、`/marketplace/tasks/task_1001`
- [x] 验证：`go test`、`go vet`、`build`、`tsc`、`npm run build`
  - 证据：`go test ./marketplace/... -race -count=1` → `ok AIxVuln/marketplace 1.950s`；`go vet` 无输出；`CGO_ENABLED=0 go build ./...` exit 0；`gofmt -l` 无输出；`tsc` exit 0（0 error）
  - 接口：overview 82/HIGH/blocked=true；task_1001 PENDING/58/MEDIUM/未阻断；task_1003 blocked=true/82/HIGH；task_1006 PASS；未知 trust 404；recharge 0 → 400；非法 VIP → 400；非法 Key → 400
  - 持久化：充值 100 → available 3300；创建 key_002；SIGTERM 重启后 3300、7 条流水、2 个 key 仍在；旧快照 expert 种子被代码种子覆盖（`TestOpenStoreOverwritesStaleExpertSeed`）
  - 前端双模式：`VITE_API_MODE=http npm run build` exit 0；默认 mock build exit 0；mock preview `:4180` 三路由命中新模块；HTTP 模式 `:5174` 三路由数据来自真实 API

### T15 · 游戏化主题基座

- [x] Tailwind 调色板反转 + 赛博地牢令牌（`abyss/panel/ink/neon/gold/arcane/boss/clear`）
- [x] 全局背景（径向辉光 + 网格 + 扫描线）、`.game-panel/.game-cut/.game-rule`、扫描/辉光动画与 reduced-motion
- [x] 重绘 Button / Card / Badge / Progress / Tabs / Modal / EmptyState
- [x] 新增 `QuestHeading`、`XpBar`、`LevelBadge`；重做 TopBar / SideNav / AppShell HUD
- [x] 验证：`npx tsc --noEmit` → `TSC_EXIT=0`；`npm run build` → `✓ built in 14.84s` `MOCK_EXIT=0`；`VITE_API_MODE=http npm run build` → `✓ built in 12.67s` `HTTP_EXIT=0`
  - 证据：`/tmp/theme-foundation.png`；HUD 渲染「主线 · 信任收费站 3/5」「QUEST LOG · 5 CHAPTERS」「NEXT BOSS · 大Boss · 未验证交付」
  - 变更：`tailwind.config.cjs`、`src/index.css`、`src/components/{ui,game,layout,shared}/**`

### T16 · M1 角色圣殿

- [x] `QuestHeading`：`CHARACTER HALL · M1` / 冒险者圣殿
- [x] 角色档案、升级进度、通关副本、连击率、翻车次数、声望；`LevelBadge` + `XpBar`
- [x] 金币袋、技能补给站、召唤密钥、技能沉淀、技能图鉴
- [x] 验证：DOM 155,525 bytes；标记计数 `冒险者圣殿×1、升级进度×1、金币袋×1、技能补给站×1、召唤密钥×5、技能沉淀×4`
  - 截图：`/tmp/final-desktop-identity.png`、`/tmp/final-mobile-identity.png`

### T17 · M2 专家祭坛

- [x] `QuestHeading`：`EXPERT ALTAR · M2` / 信任收费站 · 副本裁判所
- [x] 战友召唤图鉴、云座驾/命令行召唤/锻造台/传声筒、传说/史诗/稀有/普通、已缔结契约/待召唤/试炼场
- [x] 双人连携验证、副本规则扫描、大Boss 威胁值/关卡封锁、副本裁判费、雇主公会/指挥官战队
- [x] 数据标签归一化：`TrustPricing.normalizeService` 把 API 的「交叉验证/合规扫描/风险阻断」映射为游戏口径
- [x] 验证：DOM 142,703 bytes；标记计数 `副本裁判所×1、战友召唤图鉴×1、双人连携×6、大Boss×8、关卡封锁×7`
  - 计价卡：`副本规则扫描×5、双人连携验证×4、大Boss 威胁封锁×2`；旧标签计数 0
  - API：`score 82 / level HIGH / blocked true`
  - 截图：`/tmp/final-desktop-expert.png`、`/tmp/final-mobile-expert.png`

### T18 · M3 悬赏大厅

- [x] `QuestHeading`：`QUEST BOARD · M3` / 悬赏大厅 · 主线任务板；描述含「接副本、打怪、升级」
- [x] `TASK_STATUS_LABELS` 游戏口径；`TASK_STATUS_VARIANTS` 与 `ALLOWED_TRANSITIONS` 未动
- [x] 流水线 AI 出招 → 裁判复核 → 规则扫描 → 掉落结算；赏金押金、副本详情、提交战利品
- [x] 验证：`/marketplace` DOM 147,698 bytes；`/marketplace/tasks/task_1001` DOM 122,597 bytes；标记计数 `悬赏大厅×2、打怪×1、副本进行中×3、裁判复核中×3、掉落结算中×3、已通关×3`
  - API：`task_1003` → `blocked true`、`compliance[0].status BLOCK`、`validation.generatorVerdict PENDING`
  - 截图：`/tmp/final-desktop-marketplace.png`、`/tmp/final-mobile-marketplace.png`、`/tmp/final-desktop-marketplace-tasks-task_1001.png`、`/tmp/final-mobile-marketplace-tasks-task_1001.png`

### T19 · M4/M5 战报与新手村

- [x] `WAR ARCHIVE · M4` / 战报回放；封印链、封印完好/破损、战斗回放时间线、战报节点、战报契约
- [x] `verifyTrace`、`prevHash`、`headHash`、INVALID 判定、签名哈希未改
- [x] `STARTER VILLAGE · M5` / 新手村与转职试炼；学院试炼/公会担保/游侠自荐、成就徽章墙、声望互评、创建角色/递交试炼
- [x] 验证：`/audit` 114,342 bytes；`/audit/task_1001` 128,814 bytes；标记计数 `战报回放×1、封印完好×9、战斗回放时间线×1、战报节点×16`
  - `/onboarding` 125,206 bytes；标记计数 `新手村与转职试炼×1、学院试炼×5、公会担保×3、游侠自荐×2、声望互评×2、成就徽章×1`
  - 截图：`/tmp/final-desktop-audit-task_1001.png`、`/tmp/final-mobile-audit-task_1001.png`、`/tmp/final-desktop-onboarding.png`、`/tmp/final-mobile-onboarding.png`

### T20 · 游戏化验收

- [x] 10 条路由 `--dump-dom` 均非空：identity 155,525B、expert 142,703B、marketplace 147,698B、task_1001 122,597B、audit 114,342B、audit/task_1001 128,814B、onboarding 125,206B、404 86,574B、非法任务 87,000B、非法战报 88,123B
- [x] 必现字眼：identity+expert+marketplace 合并命中 `升级 / 打怪 / 副本 / 主线 / 大Boss`
- [x] 非法 ID 文案：`副本不存在或已下线`、`战报加载失败`；正常页无 `页面不存在` / `加载失败`
- [x] API 不变量：overview `"score":82` `"level":"HIGH"` `"blocked":true`；task_1003 `"blocked":true`；wallet `"available":3300`
- [x] 截图走查：6 页 × 2 宽度（1440×1600 / 390×844），深色面板、无白色大块、无横向溢出、无遮挡
- [ ] 提交：等待用户确认后执行（未 commit / 未 push）

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
