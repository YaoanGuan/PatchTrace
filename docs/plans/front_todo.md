# PatchTrace 前端迭代进度（front_todo.md）

> 配套：`front_requirement.md`（需求）、`front_task.md`（计划）
> 更新规则：每完成一步立即回写；没有命令输出或截图就不打勾。
> 当前轮次：F1–F7 全部完成 · 2026-11-17

---

## 0. 下一步动作（只做这一件）

> **本轮前端迭代已交付**：前端代码最终提交 `e008d0f` 已推送 `origin/main`；9999 已用新二进制重启，首页与溯源图终验通过。
> 下一轮唯一动作：按 `docs/runbooks/cve-2025-3248.md` §2–§5，在有 Docker + LLM 配置的环境跑 CVE-2025-3248 实弹验收。

### 0.1 前一轮前端错误复盘（必须保留在文档中）

上一轮前端迭代只新增 `#/graph` 图协议面板，**移除了旧功能入口**：

- 首页无法创建实验室（压缩包 / Git / 链接 / 空目录四种来源）；
- 实验室启动、停止、删除、进入详情入口不可见；
- 沙箱自动创建与执行的旧链路失去首页起点；
- 数字人、报告模板、MCP、Skills 等导航在部分视图被隐藏。

**纠偏原则**：本轮只增不减。图驱动首页与旧实验室闭环必须并存；
F4 的验收就是逐项检查旧入口是否仍在。

---

## 1. 状态总览

| # | Task | 需求 | 状态 | 证据 |
|---|---|---|---|---|
| F1 | 三份 front 文档与入口盘点 | R1–R16 对齐 | 完成 | 3 份文档共 523 行；`grep` 四来源与操作入口均命中 |
| F2 | 品牌标题与 slogan | R13 | 完成 | `index.html` 1 处、`App.tsx` 标题 1 处、slogan 3 处 |
| F3 | 首页图驱动总览组件 | R3–R12, R15 | 完成 | `GraphOverview.tsx`；01–07 模块 + R1–R16 矩阵；`tsc` exit 0 |
| F4 | 首页集成 + 旧入口保留 | 旧能力红线 | 完成 | `<GraphOverview>` 下方保留 `#legacy-labs`；四个 `*Project` handler 均在 |
| F5 | 图面板能力说明 | R3–R13 | 完成 | 语义标签 10 处调用 + 能力图例 1 处；`tsc` exit 0 |
| F6 | 构建、重启与真实页面验证 | 验收 | 完成 | `tsc`/`npm run build`/`go build` exit 0；9999 重启后 `/` 200、图 API 401；首页 21/22（`enter_lab` 因隔离实例无实验室）、图面板 8/8；排序/空态/降级/归一化 7/7 单测 |
| F7 | 提交与推送 | 交付 | 完成 | `f94a941` + `5810293` + `e008d0f`；`git push origin HEAD:main` exit 0 |

状态取值：`未开始` / `进行中` / `阻塞` / `完成`。

---

## 2. F1 · 文档与盘点

- [x] **F1.1** 创建 `docs/plans/front_requirement.md`
  - 内容：§0 错误复盘、R/Task → 前端矩阵、首页信息架构、精确文案、验收表。
- [x] **F1.2** 创建 `docs/plans/front_task.md`
  - 内容：F1–F7 步骤、文件边界、全局约束、Self-Review。
- [x] **F1.3** 创建 `docs/plans/front_todo.md`（本文件）
- [x] **F1.4** 盘点旧入口现状
  - 命令：`grep -n "创建实验室\|压缩包文件\|Git 仓库\|压缩包链接\|空工作目录" webui/src/App.tsx`
  - 证据（2026-11-17）：命中 `4437 创建实验室`、`4455 四种来源按钮`、`4471 Git 地址输入`、`4490 动态创建按钮`；实验室操作 handler 位于 `4418–4421`。

---

## 3. F2 · 品牌

- [x] **F2.1** `webui/index.html` 标题
  - 期望：`ParchTrace | 寻迹 - Autonomous Penetration System`
- [x] **F2.2** 顶栏标题 + slogan
  - 期望：`Trace the Patch, Uncover the Flaw.`
- [x] **F2.3** 登录 / 初始化 / 加载态品牌名更新
- [x] **F2.4** 样式 `.aix-brand` / `.aix-slogan`
- 验收命令：
  ```bash
  grep -n "ParchTrace" webui/index.html webui/src/App.tsx
  grep -n "Trace the Patch, Uncover the Flaw." webui/src/App.tsx
  ```
  - 证据（2026-11-17）：`index.html` 命中 1 处；`App.tsx` 标题 1 处、slogan 3 处（登录、初始化、顶栏）；`App.css` 含 `.aix-brand`/`.aix-title`/`.aix-slogan` 及响应式覆盖。

---

## 4. F3 · 首页图驱动总览

- [x] **F3.1** `GraphOverview.tsx` 请求与响应归一化（兼容 `{success,result}`）
- [x] **F3.2** 指标聚合：facts / intents / hints / edges / 类型计数 / open intents / reason lease
- [x] **F3.3** 01–07 模块卡（补丁、图、推理、调度、沙箱判决、规则评测、完成导出）
- [x] **F3.4** 图项目速览（状态、bootstrap、origin、reason worker）
- [x] **F3.5** 空态与 API 不可用降级（不遮挡旧实验室入口）
- 追加：R1–R16 覆盖矩阵（折叠）、`onOpenLabs` 旧入口 CTA、R14 明确“无 HTTP 端点”。
- 验收命令：
  ```bash
  cd webui && npx tsc --noEmit
  ```
  - 证据（2026-11-17）：`tsc --noEmit` exit 0；`GraphOverview.tsx` 命中 01–07 标题 3/3 组、R 矩阵与 `onOpenLabs`；无 mock/随机指标。

---

## 5. F4 · 首页集成与旧入口保留（红线）

- [x] **F4.1** `view === 'home'` 渲染 `GraphOverview`
- [x] **F4.2** 分区标题“实验室与沙箱（保留入口）”
- [x] **F4.3** 实验室列表保留：启动 / 停止 / 进入实验室 / 删除
- [x] **F4.4** 创建实验室保留：名称、总任务、四种来源、创建按钮
- [x] **F4.5** 顶栏六项导航保留：数字人 / 报告模板 / 溯源图 / MCP / Skills / 设置
- 验收命令：
  ```bash
  grep -n "创建实验室\|压缩包文件\|Git 仓库\|压缩包链接\|空工作目录" webui/src/App.tsx
  grep -n "startProject(p.projectID\|stopProject(p.projectID\|deleteProject(p.projectID\|enterProject(p.projectID" webui/src/App.tsx
  ```
  - 证据（2026-11-17）：`<GraphOverview>` 位于 `4375`，`#legacy-labs` 位于 `4386`；四个 lab handler 位于 `4409–4421`；四种来源按钮位于 `4455`；八视图 union 与 `view === 'graph'` 分支均存在。

---

## 6. F5 · 图面板能力说明

- [x] **F5.1** 增加能力图例：补丁反演 R7 · 推理 R8 · 沙箱 R9 · 判决 R10 · 规则 R11 · 人类意图 R12
- [x] **F5.2** 事实类型徽标颜色覆盖检查
- 追加修复：恢复 `FACT_TYPE_LABELS`/`EDGE_KIND_LABELS`/`INTENT_TYPE_LABELS`/`NODE_STATE_LABELS` 及四个 `*Label()` 函数，徽标显示中文、`title` 保留原始枚举。
- 验收命令：`cd webui && npx tsc --noEmit`
  - 证据（2026-11-17）：四个标签函数共 10 处调用；徽标映射含 `patch_profile/exploit_idea/constraint/observation/verdict/rule_hit/finding`；能力图例命中 1 处；`tsc` exit 0。

---

## 7. F6 · 构建与真实页面验证

- [x] **F6.1** `npx tsc --noEmit` → exit 0
  - 证据：2026-11-17 执行，`tsc_exit=0`。
- [x] **F6.2** `npm run build` → exit 0
  - 证据：2026-11-17 执行，`npm_exit=0`；产出 `dist/assets/index.1c64b74b.js`、`dist/assets/index.3028854b.css`。
- [x] **F6.3** `CGO_ENABLED=0 go build ./...` → exit 0
  - 证据：2026-11-17 执行 `CGO_ENABLED=0 go build -o /tmp/aixvuln-push-check .`，`go_build_exit=0`。
- [x] **F6.4** 重启服务，日志含 `图调度车道已启动`
  - 证据（2026-11-17）：新二进制替换后重启 9999，日志 `图调度车道已启动，配置: dispatch.yaml`；`/` → 200，未授权 `/api/graph/projects` → 401；服务页面实际加载的 bundle 含 `迭代组件覆盖` / `实验室与沙箱（保留入口）` / `补丁摄入与反演`。
- [x] **F6.5** 页面验证：标题、slogan、01–07 模块、创建实验室四种来源
  - 截图：`/tmp/r13/front-home-final.png`
  - 证据（2026-11-17，隔离 9997 实例、同一二进制）：`title/slogan/modules/legacy_heading/create_lab/src_file/src_git/src_url/src_empty/start_lab/graph_nav` 全为 True；`module_01/module_07` True。`enter_lab` 为 False 是因为隔离实例无实验室；`App.tsx` 中 `enterProject(p.projectID)` 绑定仍在。
- [x] **F6.6** 图面板回归：导出、提示、意图领取/总结、总脑租约
  - 证据（2026-11-17）：`panel/legend/export_yaml/export_timeline/hint/intent/reason/capability/projects_loaded` 全部 True；快照显示 52 事实 / 48 意图 / 2 提示 / 106 边，导出与租约控件可见。
- [x] **F6.7** 完成态收口：最新项目排序、空态/降级提示、元素归一化、状态计数
  - 修复：`recentProjects` 按 `created_at` 倒序取数（快照 5 个、速览 4 个）；`normalizeProjects` 丢弃非法元素并清洗字段；加载中显示“加载中…”；401/403 或非 JSON 显示 `misc.graph_lane_enabled=true` 提示且不显示空态；图项目指标显示运行中/已完成/已停止；长 origin 换行。
  - 证据（2026-11-17）：`graphOrder.test.mjs` 7/7 通过（排序 5 + 归一化 2）；`tsc` exit 0；隔离实例 seed 6 个项目后 `quick_list_order = probe-6, probe-5, probe-4, probe-3`，`newest_first/quick_list_total/status_counts/lane_ok/lane_hint_hidden/empty_hidden/no_page_overflow/module_no_overflow` 均 True；首页 21/22，仅 `enter_lab` 因隔离库无实验室为 False（源码绑定保留）。

---

## 8. F7 · 提交与推送

- [x] **F7.1** `git add` 前端文件与三份文档
- [x] **F7.2** `git commit`
- [x] **F7.3** `git push origin main`
- [x] **F7.4** 回写本文件与 `docs/plans/todo.md`
  - 证据（2026-11-17）：提交 `f94a941`（基于 `origin/main` 的 `6d918d2`，仅含 9 个前端/文档文件）；`git push origin HEAD:main` 输出 `6d918d2..f94a941  HEAD -> main`，`push_exit=0`。后续修正：`5810293`（断言计数）、`e008d0f`（首页按 `created_at` 选取最新项目，附排序测试）；前端代码最终提交 `e008d0f` 已推送 `origin/main`。

---

## 9. 阻塞

| 日期 | Task | 现象 | 需要谁决策 | 状态 |
|---|---|---|---|---|
| — | — | 无 | — | — |
