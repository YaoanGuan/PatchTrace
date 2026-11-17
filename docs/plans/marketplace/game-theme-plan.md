# PatchTrace Market · 游戏化美化方案（game-theme-plan.md）

> 配套：`docs/plans/marketplace/task.md`、`todo.md`
> 分支：`feat/ai-marketplace-frontend`
> 约束：只改前端表现层。不改 API 契约、不改类型字段、不改任务状态机、不改风险/合规口径、不改后端。

---

## 1. 目标与非目标

**目标**：把「AI 雇佣 AI 的教育平台」包装成一场可感知的副本闯关：用户是冒险者，任务是副本，交付是打怪，审核是裁判，结算是掉落，个人成长是升级。

**必须出现的叙事字眼**：升级、打怪、副本、主线、大Boss。

**非目标**：
- 不新增业务数据或伪造进度。
- 不把合规、免责、资金托管等严肃文案改成纯玩笑；标题游戏化，正文保留法律与责任边界。
- 不引入外部字体、图片或网络依赖。

---

## 2. 美术方向：赛博地牢 HUD（Cyber Dungeon HUD）

**一句话**：深空底色 + 青色能量 + 金色赏金 + 切角面板 + 扫描线；像一台挂在冒险者公会墙上的全息任务终端。

### 2.1 色彩令牌

| 令牌 | 值 | 用途 |
|---|---|---|
| `abyss` | `#05070f` | 页面最底层 |
| `panel` | `#0b1224` | 卡片 / 面板 |
| `panel-soft` | `#0f1830` | 内嵌区块、输入框 |
| `panel-line` | `#1e2a48` | 面板描边 |
| `ink` | `#e8edff` | 主文字 |
| `ink-muted` | `#9aa5c6` | 次要文字 |
| `ink-dim` | `#6f7c9e` | 辅助文字 |
| `neon` | `#38e1ff` | 科技 / 链接 / 选中 |
| `gold` | `#f6c453` | 赏金 / 主按钮 / 升级 |
| `arcane` | `#a78bfa` | AI / 召唤 / 技能 |
| `boss` | `#fb7185` | 大Boss / 危险 / 阻断 |
| `clear` | `#34d399` | 通关 / 通过 |

状态色保留语义，但全部改成深色底上的半透明徽标。

### 2.2 视觉签名

1. **面板**：半透明深色 + 1px 冷色描边 + 顶部 1px 渐变高光 + 四角切角括号。
2. **主按钮**：金色渐变、深色文字、外发光；次按钮：青色描边、透明底。
3. **标题**：英文眉标 `MAIN QUEST` / `DUNGEON` / `QUEST LOG`，中文主标题加粗；数字用等宽字体。
4. **进度**：金色→青色渐变 XP 条，带流光；危险条用 `boss` 色。
5. **背景**：深空径向光晕 + 细网格 + 顶部扫描线，固定在 `AppShell` 底层，不遮挡内容。
6. **动效**：仅 `pulse-glow`（当前主线）、`scan`（Hero 扫描线）、`shine`（XP 条）。尊重 `prefers-reduced-motion`。

---

## 3. 全局主题策略（少改文件、不改语义）

现有 67 个 `.tsx`、600+ 处浅色类名。采用三层策略：

### 3.1 Tailwind 调色板反转

在 `tailwind.config.cjs` 中重定义：

- `slate`：50 最深 → 900 最亮。`bg-slate-50` 自动变深底，`text-slate-900` 自动变亮字，`border-slate-200` 自动变暗描边，连 `hover:` / `ring:` / `divide:` 一起生效。
- `indigo`：映射为青色能量系（50 = 青色 10% 透明，600/700 = 亮青）。
- `emerald` / `amber` / `rose` / `sky` / `orange`：50/100/200 改为半透明深色底，300–700 改为亮色文字。状态徽标无需逐个改类名。
- 新增 `panel` / `ink` / `gold` / `neon` / `arcane` / `boss` / `abyss` 令牌、`glow` 阴影、`grid` 背景、三个 keyframes。

### 3.2 `bg-white` 机械替换

全仓库 `bg-white` → `bg-panel`，`bg-white/90` → `bg-panel/90`。`text-white` 不动（仍用于彩色按钮与头像）。

### 3.3 基础组件重绘

`Card` / `Button` / `Badge` / `Progress` / `Tabs` / `Modal` / `EmptyState` 统一改深色皮肤。所有业务组件自动继承。

**例外**：`Modal` 遮罩从 `bg-slate-900/50` 改为 `bg-black/75`，避免反转后变亮。

---

## 4. 叙事文案映射

标题与短标签游戏化；正文中的责任、合规、资金说明保留原意，只换语气。

### 4.1 全局外壳

| 位置 | 原本文案 | 游戏化文案 |
|---|---|---|
| 站点标题 | PatchTrace Market · AI 雇佣 AI 的教育平台 | PatchTrace Market · AI 雇佣 AI 的副本闯关平台 |
| 顶栏 | 品牌 + 广告语 | 保留 `Trace the Patch, Uncover the Flaw.`，右侧加 HUD：`Lv.12` / 升级进度 / 金币 / 主线 |
| 顶栏主线 | — | `主线 · 信任收费站 3/5` |
| 侧栏 | 个人中枢 / AI 专家 / 任务大厅 / 审计证据 / 准入信誉 | 角色圣殿 / 专家祭坛 / 悬赏大厅 / 战报档案馆 / 新手村 |
| 侧栏分组 | — | `QUEST LOG · 任务日志` |

### 4.2 M1 角色圣殿

| 原 | 新 |
|---|---|
| 个人中枢 | 冒险者圣殿 |
| 个人档案 / 等级进度 | 角色档案 / 升级进度 |
| 完成任务 | 通关副本 |
| 准时率 | 连击率 |
| 争议 | 翻车次数 |
| 信誉分 | 声望 |
| 钱包 | 金币袋 |
| 可用余额 / 冻结 | 可用金币 / 押金冻结 |
| 充值 / 订阅 VIP | 充值金币 / 购买月卡 |
| 流水 | 金币流水 |
| API 中转站 | 技能补给站 |
| 模型目录 | 可召唤的 AI 战友 |
| API Key | 召唤密钥 |
| 创建 / 吊销 Key | 缔结契约 / 解除契约 |
| 用量 | 魔力消耗 |
| 五步闭环 | 召唤模型 → 接取副本 → 裁判复核 → 领取赏金 → 技能沉淀 |
| 能力雷达 | 技能树雷达 |
| 专家库 | 技能图鉴 |
| 信任凭证 | 成就徽章 |

### 4.3 M2 专家祭坛

| 原 | 新 |
|---|---|
| AI 专家 | 专家祭坛 |
| 信任收费站（保留） | 信任收费站 · 副本裁判所 |
| 接入矩阵 | 战友召唤图鉴 |
| 云托管计算机 / AI CLI / AI IDE / 聊天客户端 | 云座驾 / 命令行召唤 / 锻造台 / 传声筒 |
| S/A/B/C 级 | 传说 / 史诗 / 稀有 / 普通（保留字母） |
| 已连接 / 待接入 / 沙箱环境 | 已缔结契约 / 待召唤 / 试炼场 |
| 角色接入桥 | 雇主公会 / 指挥官战队 |
| 交叉验证 | 双人连携验证 |
| Claude 生成 / Codex 复核 | Claude 出招 / Codex 拆招 |
| 合规扫描 | 副本规则扫描 |
| 风险定级与阻断 | 大Boss 威胁与关卡封锁 |
| 通过 / 未通过 / 待定 | 通关 / 挑战失败 / 待定 |
| 按任务计费 | 按副本收取裁判费 |

### 4.4 M3 悬赏大厅

| 原 | 新 |
|---|---|
| 任务撮合大厅 | 悬赏大厅 · 主线任务板 |
| 任务大厅 | 悬赏大厅 |
| 发布任务 | 发布悬赏 |
| 接单 | 接受挑战 |
| 提交交付 | 提交战利品 |
| 赏金 / 托管 | 金币赏金 / 赏金押金 |
| 任务详情 | 副本详情 |
| AI 验证流水线 | 关卡进度 |
| 审计入口 | 战报回放 |

任务状态统一改为：

| 状态 | 新标签 |
|---|---|
| DRAFT | 待发布 |
| PUBLISHED | 悬赏中 |
| ACCEPTED | 已接取 |
| IN_PROGRESS | 副本进行中 |
| SUBMITTED | 战利品已提交 |
| AI_VALIDATING | 裁判复核中 |
| VALIDATION_FAILED | 挑战失败 |
| COMPLIANCE_SCAN | 规则扫描中 |
| SETTLING | 掉落结算中 |
| COMPLETED | 已通关 |
| DISPUTED | 仲裁中 |
| REFUNDED | 赏金退回 |
| CANCELLED | 悬赏撤销 |

### 4.5 M4 战报档案馆

| 原 | 新 |
|---|---|
| 审计证据 | 战报回放 |
| 单任务证据链 | 单副本战报 |
| 可追溯时间线 | 战斗回放时间线 |
| 证据链节点 | 战报节点 |
| 哈希链 / 链完整 / 链断裂 | 封印链 / 封印完好 / 封印破损 |
| 免责契约 | 战报契约（正文保留三方责任） |
| 合规扫描报告 | 副本规则报告 |

### 4.6 M5 新手村

| 原 | 新 |
|---|---|
| 准入与信誉 | 新手村与转职试炼 |
| 高校学生通道 | 学院试炼 |
| 企业白名单通道 | 公会担保 |
| 自由职业者通道 | 游侠自荐 |
| 信任徽章墙 | 成就徽章墙 |
| 双向信誉体系 | 声望互评 |
| 评分 / 评价 | 战绩评分 / 战报点评 |
| 注册 | 创建角色 |

---

## 5. 新增共享组件

`src/components/game/`：

1. `QuestHeading.tsx`
   ```ts
   interface QuestHeadingProps {
     chapter: string;      // "MAIN QUEST · M3"
     title: string;        // "主线任务板"
     description?: string;
     badge?: ReactNode;    // 右侧奖励/难度
   }
   ```
2. `XpBar.tsx`
   ```ts
   interface XpBarProps { value: number; label?: string; compact?: boolean; }
   ```
3. `LevelBadge.tsx`
   ```ts
   interface LevelBadgeProps { code: string; title: string; size?: 'sm' | 'md'; }
   ```

`QuestHeading` 供五个模块页统一使用，保证章节眉标、标题、描述、右侧槽位一致。

---

## 6. 文件清单与执行分工

### T15 主题基座（P9 直接执行）

- `tailwind.config.cjs`：令牌、调色板反转、阴影、背景、动效。
- `src/index.css`：body 背景、网格/光晕、`.game-panel`、滚动条、选区、reduced-motion。
- `src/components/ui/*`：7 个基础组件。
- `src/components/game/*`：3 个新组件。
- `TopBar` / `SideNav` / `AppShell` / `WalletChip` / `RoleSwitcher` / `TrustBadgeWall`。
- 全仓库 `bg-white` → `bg-panel`。
- `index.html` 标题与 theme-color。

### T16 M1 角色圣殿（P8-identity）

`IdentityPage`、`ProfileHeader`、`WalletPanel`、`ApiGatewayPanel`、`SkillRadarChart`、`ExpertiseRepository`、`CredentialUploadDialog`。

### T17 M2 专家祭坛（P8-expert）

`ExpertPage`、`AccessMatrix`、`ToolCard`、`RoleBridge`、`CrossValidationPanel`、`PipelineStatus`、`ComplianceScanPanel`、`RiskGauge`、`TaskTrustPipeline`、`TrustPricing`、`labels.ts`。

### T18 M3 悬赏大厅（P8-marketplace）

`MarketplacePage`、`MarketplaceHeader`、`TaskMarketplace`、`TaskFilters`、`TaskList`、`TaskCard`、`TaskPublishForm`、`TaskSummary`、`AIValidationPipeline`、`ValidationVerdictCard`、`PipelineStage`、`AcceptTaskPanel`、`SubmitWorkPanel`、`EscrowWallet`、`BalanceSummary`、`EscrowTaskRow`、`TransactionList`、`AuditLinkCard`、`RewardInput`、`ComplianceAck`、`TaskDetailPage`、`lib/taskStatus.ts`。

### T19 M4 + M5（P8-audit / P8-onboarding）

- 审计：`AuditPage`、`AuditDetailPage` 与 `features/audit/components/*`。
- 准入：`OnboardingPage`、`MultiChannelRegistration`、`ChannelTabs`、三套表单、`ReputationSystem`、`ReputationScoreCard`、`RatingForm`、`RatingHistory`、`RegistrationStatusBanner`、`constants.ts`。

### T20 验收（P9）

- `npx tsc --noEmit` exit 0。
- `npm run build` exit 0（mock + HTTP 两种模式）。
- 8 条路由 headless 渲染，命中 `升级`、`副本`、`主线`、`大Boss`、`打怪`。
- 6 张关键页截图确认无白色大面板、无低对比文字。
- 真实 API 数据不变：overview 82/HIGH/blocked=true；task_1003 阻断；wallet 3300。

---

## 7. 验收标准

1. **叙事完整**：五个模块都能用「主线 → 副本 → 打怪 → 裁判 → 掉落 → 升级」串起来。
2. **术语一致**：同一概念全局只有一个游戏化叫法（见 §4）。
3. **视觉统一**：所有页面使用同一套深色面板、金色赏金、青色科技、红色大Boss。
4. **信息不丢**：状态、风险、合规、资金、责任边界仍可读；游戏化只改包装。
5. **工程不破**：类型、构建、API、持久化全部保持原状。

---

## 8. 风险与对策

| 风险 | 对策 |
|---|---|
| 调色板反转导致对比度不足 | 每个路由截图走查；次要文字不低于 `ink-muted` |
| 游戏化削弱合规严肃性 | 标题游戏化，正文保留责任条款原文 |
| 长中文标签溢出 | 徽标允许换行；窄屏截图检查 |
| 并发改动冲突 | 每个 Agent 只改自己模块文件；主题基座先合入 |
| 现有验收证据失效 | 不改状态机与 API；只更新文案断言 |
