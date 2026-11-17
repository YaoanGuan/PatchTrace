# PatchTrace Market 游戏化 UI 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 PatchTrace Market 从前端企业后台视觉改造成「AI 副本闯关」世界：深色赛博 HUD、升级/金币/主线、副本/打怪/大Boss 叙事，同时不改变任何 API、类型、状态机与数据口径。

**Architecture:** 用 Tailwind 调色板反转一次性覆盖 600+ 处浅色类名，再用基础组件（Card/Button/Badge/Progress/Tabs/Modal）统一深色皮肤；新增 `QuestHeading`/`XpBar`/`LevelBadge` 三个共享游戏组件；最后按 M1–M5 模块替换文案与关键视觉。所有模块共享同一套术语表与色彩令牌。

**Tech Stack:** React 18、TypeScript 5.6、Vite 5.4、TailwindCSS 3.4、Zustand 5、lucide-react、Headless Chromium。

**Spec:** `docs/plans/marketplace/game-theme-plan.md`

## Global Constraints

- 只改前端表现层：不改 `src/types/**`、`src/lib/api/**`、Go 后端、mock 数据、任务状态机、风险与合规判定。
- 不新增依赖、不引入外部字体或网络资源。
- 禁止新增 `bg-white`；面板统一用 `bg-panel` / `game-panel`。
- 必须出现并一致使用：`升级`、`打怪`、`副本`、`主线`、`大Boss`。
- 合规、免责、资金托管正文保留原意；游戏化只改标题与短标签。
- 所有命令在 `/root/PatchTrace/webui-marketplace` 执行。
- 前端无测试框架；验收命令为 `tsc`、`vite build`、Headless Chromium DOM/截图、`curl` API 不变量。
- 提交与推送只在用户明确确认后执行。

## Review Focus

1. **360–390px 窄屏**：HUD、侧栏、状态徽标、中文长标签不得溢出或互相遮挡；预期可横向滚动导航、正文不裁切。
2. **API 缺字段/失败**：`/expert` overview 失败、`/identity` 无徽章、`/marketplace` 空列表、非法 `taskId` 时仍显示游戏化空态，不出现 `NaN`、空白或伪造通过。
3. **超长标题与哈希**：任务标题、32 位哈希、模型名在卡片中换行或截断，不得撑破网格；预期哈希保持 `font-mono`。
4. **状态语义**：`task_1003` 必须显示「大Boss / 关卡封锁」，不得出现「通关」；任何 BLOCK 都必须对应 `blocked=true`。
5. **对比度与动效**：次要文字不低于 `ink-muted`；`prefers-reduced-motion` 下动画停止；预期 6 张关键页无低对比文字。

---

### Task 1: 主题基座（调色板反转 + 基础组件）

**Files:**
- Modify: `tailwind.config.cjs`
- Modify: `src/index.css`
- Modify: `src/components/ui/Card.tsx`、`Button.tsx`、`Badge.tsx`、`Progress.tsx`、`Tabs.tsx`、`Modal.tsx`、`EmptyState.tsx`
- Test: Headless 截图 + `tsc`/`build`

**Interfaces:**
- Produces: Tailwind 令牌 `abyss`、`panel`/`panel-soft`/`panel-line`、`ink`/`ink-muted`/`ink-dim`、`neon`、`gold`、`arcane`、`boss`、`clear`；CSS 类 `.game-panel`、`.game-cut`、`.game-rule`；`Card` 默认应用 `.game-panel`。
- Consumes: 无。

- [x] **Step 1: 反转调色板并新增令牌**

修改 `tailwind.config.cjs`：`slate` 50→900 反向映射为深→浅；`indigo` 映射青色；`emerald/amber/rose/sky/orange` 的 50–200 改为深色半透明底、300–700 改为亮色文字；新增 `boxShadow.glow/gold/boss`、`backgroundImage.grid/quest-radial`、`backgroundSize.grid-size`、`pulse-glow/scan/shine` 动画。

- [x] **Step 2: 增加全局背景与面板样式**

`src/index.css`：`body` 使用 `abyss` + 径向光晕；`::selection` 金色；深色滚动条；`@media (prefers-reduced-motion: reduce)` 关闭动画；`.game-panel` 深色渐变 + 四角括号；`.game-cut` 切角；`.game-rule` 渐变分隔线。

- [x] **Step 3: 重绘 7 个基础组件**

`Button` primary 金色渐变 + 深色文字 + `game-cut`；`Card` 使用 `game-panel`；`Badge` 半透明描边；`Progress` 金色→青色 XP 条；`Tabs` 深色分段；`Modal` 遮罩 `bg-black/75`、面板 `game-panel`；`EmptyState` 深色虚线框。

- [x] **Step 4: 验证类型与构建**

Run: `npx tsc --noEmit && npm run build`
Expected: 两条命令 exit 0；构建产物含深色类名。

- [x] **Step 5: 截图确认基座**

Run: `chromium --headless --disable-gpu --no-sandbox --hide-scrollbars --window-size=1440,1600 --screenshot=/tmp/theme-foundation.png --virtual-time-budget=6000 http://localhost:5174/marketplace`
Expected: 深色背景、无白色大面板、金色按钮、青色强调。

### Task 2: 游戏外壳与共享组件

**Files:**
- Create: `src/components/game/QuestHeading.tsx`、`XpBar.tsx`、`LevelBadge.tsx`
- Modify: `src/components/layout/TopBar.tsx`、`SideNav.tsx`、`AppShell.tsx`
- Modify: `src/components/shared/WalletChip.tsx`、`RoleSwitcher.tsx`、`TrustBadgeWall.tsx`
- Modify: `src/app/pages/NotFoundPage.tsx`、`src/app/router.tsx`
- Test: Headless DOM + 截图

**Interfaces:**
- Produces:
  - `QuestHeading({ chapter: string; title: string; description?: string; badge?: ReactNode; className?: string })`
  - `XpBar({ value: number; label?: string; compact?: boolean; className?: string })`
  - `LevelBadge({ code: string; title: string; size?: 'sm' | 'md'; className?: string })`
- Consumes: Task 1 的令牌与 `.game-panel`。

- [x] **Step 1: 实现三个共享组件**

按上述签名创建文件；`QuestHeading` 输出 `MAIN QUEST` 风格眉标 + 中文标题 + 描述 + 右侧槽位；`XpBar` 金色→青色渐变 + 流光；`LevelBadge` 金色描边 + `Sparkles` 图标。

- [x] **Step 2: 重做 HUD 外壳**

`TopBar`：左侧品牌 + `AI 雇佣 AI 的副本闯关平台`；中间 `主线 · 信任收费站 3/5` + XP 条；右侧 `LevelBadge` + `XpBar` + 金币袋 + 身份切换；底部分隔线 + `Trace the Patch, Uncover the Flaw.`。
`SideNav`：`QUEST LOG · 5 CHAPTERS`；五项改为角色圣殿/专家祭坛/悬赏大厅/战报档案馆/新手村；底部 `NEXT BOSS · 大Boss · 未验证交付`。
`AppShell`：固定 `GameBackdrop`（网格 + 光晕 + 顶部扫描线）；toast 改深色。

- [x] **Step 3: 统一共享组件与 404**

`WalletChip` 改金币袋；`RoleSwitcher` 改「雇主公会 / 指挥官战队」；`TrustBadgeWall` 改成就徽章；`NotFoundPage` 改「这条副本路线不存在」；路由 fallback 改「副本加载中…」。

- [x] **Step 4: 验证 DOM 标记**

Run: `chromium --headless --disable-gpu --no-sandbox --dump-dom --virtual-time-budget=6000 http://localhost:5174/marketplace | grep -o '主线 · 信任收费站\|QUEST LOG\|大Boss · 未验证交付' | sort -u`
Expected: 三个标记全部输出。

- [x] **Step 5: 窄屏检查**

Run: 同命令加 `--window-size=390,844 --screenshot=/tmp/shell-mobile.png`
Expected: 侧栏横向可滚动，HUD 不重叠，正文无水平溢出。

### Task 3: M1 角色圣殿

**Files:**
- Modify: `src/features/identity/**`（IdentityPage、ProfileHeader、WalletPanel、ApiGatewayPanel、SkillRadarChart、ExpertiseRepository、CredentialUploadDialog、types.ts）
- Test: `tsc` + Headless `/identity`

**Interfaces:**
- Consumes: `QuestHeading`、`LevelBadge`、`XpBar`（Task 2）。
- Produces: 无新导出；保留既有 props/store 行为。

- [x] **Step 1: 写失败检查**

Run: `grep -r "冒险者圣殿\|升级进度\|金币袋\|技能补给站\|召唤密钥" src/features/identity | wc -l`
Expected: `0`。

- [x] **Step 2: 替换页面标题与档案**

`IdentityPage` 用 `QuestHeading`：`CHARACTER HALL · M1` / `冒险者圣殿`。
`ProfileHeader`：角色档案、升级进度、通关副本、连击率、翻车次数、声望；接入 `LevelBadge` + `XpBar`。

- [x] **Step 3: 金币袋与技能补给站**

按 Spec §4.2 替换钱包、API 中转站、雷达、技能图鉴、成就徽章文案；API 五步闭环固定为 `召唤模型 → 接取副本 → 裁判复核 → 领取赏金 → 技能沉淀`；金额与数值 `font-mono`。

- [x] **Step 4: 验证类型**

Run: `npx tsc --noEmit`
Expected: exit 0。

- [x] **Step 5: 验证渲染标记**

Run: `chromium --headless --disable-gpu --no-sandbox --dump-dom --virtual-time-budget=6000 http://localhost:5174/identity | grep -o '冒险者圣殿\|升级进度\|金币袋\|技能补给站\|召唤密钥\|技能沉淀' | sort -u`
Expected: 六个标记全部输出。

- [x] **Step 6: 视觉检查**

Run: 截图 `1440x1600` 与 `390x844`。
Expected: 深色面板、金币金色、无白色大块、窄屏不溢出；无徽章时显示「暂无成就徽章」空态而不是空白。

### Task 4: M2 专家祭坛

**Files:**
- Modify: `src/features/expert/**`（ExpertPage、AccessMatrix、ToolCard、RoleBridge、CrossValidationPanel、PipelineStatus、ComplianceScanPanel、RiskGauge、TaskTrustPipeline、TrustPricing、labels.ts）
- Test: `tsc` + Headless `/expert` + `curl /api/expert/overview`

**Interfaces:**
- Consumes: `QuestHeading`、Task 1 状态色。
- Produces: 无新导出；保留 `ExpertOverview`、`TaskTrust` 数据形状与 `blocked == hasBlock(compliance)` 展示。

- [x] **Step 1: 写失败检查**

Run: `grep -r "副本裁判所\|战友召唤图鉴\|双人连携\|大Boss\|关卡封锁" src/features/expert | wc -l`
Expected: `0`。

- [x] **Step 2: 改写标题与接入矩阵**

`ExpertPage` 用 `QuestHeading`：`EXPERT ALTAR · M2` / `信任收费站 · 副本裁判所`。
分类显示：云座驾（云托管计算机）、命令行召唤（AI CLI）、锻造台（AI IDE）、传声筒（聊天客户端）；等级 S/A/B/C → 传说/史诗/稀有/普通；状态 → 已缔结契约/待召唤/试炼场。

- [x] **Step 3: 连携、规则扫描与大Boss**

角色桥 → 雇主公会/指挥官战队；交叉验证 → 双人连携验证（Claude 出招 / Codex 拆招）；合规 → 副本规则扫描（低危/警戒/高危/灭团级）；风险 → 大Boss 威胁值，阻断 → 关卡封锁；定价 → 副本裁判费。

- [x] **Step 4: 验证类型与数据不变量**

Run: `npx tsc --noEmit`
Expected: exit 0。
Run: `curl -s http://127.0.0.1:8081/api/expert/overview | grep -o '"score":82\|"level":"HIGH"\|"blocked":true' | sort -u`
Expected: 三项全部输出。

- [x] **Step 5: 验证渲染标记**

Run: `chromium ... --dump-dom http://localhost:5174/expert | grep -o '副本裁判所\|战友召唤图鉴\|双人连携\|大Boss\|关卡封锁' | sort -u`
Expected: 五个标记全部输出；风险卡与合规卡颜色分别为 `boss` 与 `amber/rose`；代码中 `phase === 'error'` 分支仍保留 Hero 与重试按钮，不因游戏化被删。

### Task 5: M3 悬赏大厅

**Files:**
- Modify: `src/features/marketplace/**`（22 个页面/组件）
- Modify: `src/lib/taskStatus.ts`
- Test: `tsc` + Headless `/marketplace`、`/marketplace/tasks/task_1001` + `curl /api/tasks/{id}/trust`

**Interfaces:**
- Consumes: `QuestHeading`、`Card`、Task 1 状态色。
- Produces: `TASK_STATUS_LABELS` 新口径（唯一权威）；不改变 `ALLOWED_TRANSITIONS`。

- [x] **Step 1: 写失败检查**

Run: `grep -r "悬赏大厅\|打怪\|副本进行中\|裁判复核中\|已通关" src/features/marketplace src/lib/taskStatus.ts | wc -l`
Expected: `0`。

- [x] **Step 2: 统一状态标签**

按 Spec §4.4 表格重写 `TASK_STATUS_LABELS`：待发布/悬赏中/已接取/副本进行中/战利品已提交/裁判复核中/挑战失败/规则扫描中/掉落结算中/已通关/仲裁中/赏金退回/悬赏撤销。`TASK_STATUS_VARIANTS` 保持语义不变。

- [x] **Step 3: 改写大厅与详情**

`MarketplaceHeader` → `QUEST BOARD · M3` / `悬赏大厅 · 主线任务板`，描述含「接副本、打怪、升级」；发布/接单/提交/托管/审计入口按 Spec §4.4 替换；流水线阶段 → AI 出招/裁判复核/规则扫描/掉落结算；判定 → 通关/挑战失败/待定。

- [x] **Step 4: 验证类型与状态不变量**

Run: `npx tsc --noEmit`
Expected: exit 0。
Run: `curl -s http://127.0.0.1:8081/api/tasks/task_1003/trust | grep -o '"blocked":true\|"status":"BLOCK"' | sort -u`
Expected: 两项输出；`validation.generatorVerdict` 仍为 `PENDING`。合规项 `cmp_004` 的 `PASS` 属既有数据，不计为回归。

- [x] **Step 5: 验证渲染标记**

Run: `chromium ... --dump-dom http://localhost:5174/marketplace | grep -o '悬赏大厅\|打怪\|副本进行中\|裁判复核中\|掉落结算中\|已通关' | sort -u`
Expected: 六个标记全部输出；详情页命中「副本详情」「提交战利品」。

### Task 6: M4 战报档案馆

**Files:**
- Modify: `src/features/audit/**`（17 个页面/组件）
- Test: `tsc` + Headless `/audit`、`/audit/task_1001`

**Interfaces:**
- Consumes: `QuestHeading`、Task 1 状态色。
- Produces: 无新导出；保留 `verifyTrace` 判定与节点顺序。

- [x] **Step 1: 写失败检查**

Run: `grep -r "战报回放\|封印链\|封印完好\|战报契约" src/features/audit | wc -l`
Expected: `0`。

- [x] **Step 2: 改写列表与详情**

`AuditPage` → `WAR ARCHIVE · M4` / `战报回放`；单任务 → 单副本战报；时间线 → 战斗回放时间线；节点 → 战报节点；哈希链 → 封印链；状态 → 封印完好/封印破损/待验证；免责契约标题 → 战报契约，正文保留三方责任。

- [x] **Step 3: 保留校验逻辑**

只改文案与颜色；`verifyTrace`、`prevHash`、`headHash`、INVALID 判定、签名哈希一律不动。

- [x] **Step 4: 验证类型与链路**

Run: `npx tsc --noEmit`
Expected: exit 0。
Run: `chromium ... --dump-dom http://localhost:5174/audit/task_1001 | grep -o '战报回放\|封印完好\|战斗回放时间线\|战报节点' | sort -u`
Expected: 四个标记全部输出。

- [x] **Step 5: 篡改回归**

在浏览器外复用现有 `verifyTrace` 判定（不新增测试框架）：保持 `docs/plans/marketplace/todo.md` 记录的「篡改 prevHash/headHash 三场景均 INVALID」结论；本次只改文案，若不成立则回滚相关文件。

### Task 7: M5 新手村

**Files:**
- Modify: `src/features/onboarding/**`（15 个页面/组件）
- Test: `tsc` + Headless `/onboarding`

**Interfaces:**
- Consumes: `QuestHeading`、`Card`、`TrustBadgeWall`。
- Produces: 无新导出；保留三通道表单校验、注册 hook、评分历史。

- [x] **Step 1: 写失败检查**

Run: `grep -r "新手村\|转职试炼\|学院试炼\|公会担保\|游侠自荐\|声望互评" src/features/onboarding | wc -l`
Expected: `0`。

- [x] **Step 2: 改口径**

`OnboardingPage` → `STARTER VILLAGE · M5` / `新手村与转职试炼`；三通道 → 学院试炼（高校学生）/ 公会担保（企业白名单）/ 游侠自荐（自由职业者）；信任徽章墙 → 成就徽章墙；双向信誉 → 声望互评；注册 → 创建角色；提交审核 → 递交试炼。

- [x] **Step 3: 保留表单与评分**

字段、校验、`useChannelRegistration`、`ReputationSystem`、评分历史与 toast 全部保持行为不变。

- [x] **Step 4: 验证类型与渲染**

Run: `npx tsc --noEmit`
Expected: exit 0。
Run: `chromium ... --dump-dom http://localhost:5174/onboarding | grep -o '新手村与转职试炼\|学院试炼\|公会担保\|游侠自荐\|声望互评\|成就徽章' | sort -u`
Expected: 六个标记全部输出。

### Task 8: 全量验收

**Files:**
- Modify: `docs/plans/marketplace/todo.md`（证据回写）
- Test: 全路由 + 双构建模式 + API 不变量

**Interfaces:**
- Consumes: Task 1–7 全部产物。
- Produces: 可交付的验收证据。

- [x] **Step 1: 类型与双模式构建**

Run: `npx tsc --noEmit && npm run build && VITE_API_MODE=http npm run build`
Expected: 三条命令 exit 0。

- [x] **Step 2: 8 条路由渲染**

对 `/identity`、`/expert`、`/marketplace`、`/marketplace/tasks/task_1001`、`/audit`、`/audit/task_1001`、`/onboarding`、`/not-found` 执行 `--dump-dom`；另访问 `/marketplace/tasks/not-exist` 与 `/audit/not-exist`。
Expected: 每页非空；正常页无 `页面不存在`、无 `加载失败` 降级文案；非法 ID 页分别显示「副本不存在或已下线」与「战报加载失败」，均有游戏化返回入口。

- [x] **Step 3: 必现字眼检查**

Run: 对 `/identity`、`/expert`、`/marketplace` 的 DOM 合并后执行 `grep -o '升级\|打怪\|副本\|主线\|大Boss' | sort -u`
Expected: 五个词全部输出。

- [x] **Step 4: API 不变量**

Run:
```bash
curl -s http://127.0.0.1:8081/api/expert/overview | grep -o '"score":82\|"level":"HIGH"\|"blocked":true'
curl -s http://127.0.0.1:8081/api/tasks/task_1003/trust | grep -o '"blocked":true'
curl -s http://127.0.0.1:8081/api/wallet | grep -o '"available":3300'
```
Expected: 三组输出全部命中。

- [x] **Step 5: 截图走查**

Run: 6 张关键页在 `1440x1600` 与 `390x844` 下截图。
Expected: 无白色大面板、无低对比正文、无横向溢出、无内容遮挡。

- [x] **Step 6: 回写证据**

证据已写入 `docs/plans/marketplace/todo.md` §T15–T20。

- [ ] **Step 7: 提交（需用户确认）**

经用户确认后：
```bash
git add webui-marketplace docs/plans/marketplace docs/superpowers/plans
git commit -m "feat(marketplace): gamify UI with cyber dungeon theme"
```
Expected: 工作区干净；提交包含主题、五模块文案与验收证据。
