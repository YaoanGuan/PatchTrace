# PatchTrace 前端迭代实施计划（front_task.md）

> 对应需求：`docs/plans/front_requirement.md`
> 对应后端计划：`docs/plans/task.md` Task 1–11
> 执行顺序：F1 → F2 → F3 → F4 → F5 → F6 → F7，逐步提交、逐步验证
> 日期：2026-11-17

---

## 0. 前一轮前端错误复盘（执行前必读）

上一轮前端迭代只交付了 `#/graph` 图协议面板，却**移除了旧功能入口**：

- 首页不再能创建实验室（压缩包 / Git / 链接 / 空目录四种来源）；
- 实验室启动、停止、删除、进入详情入口不可见；
- 沙箱自动创建与执行的旧链路失去首页起点；
- 数字人、报告模板、MCP、Skills 等入口在部分视图被隐藏。

**本计划的第一约束**：所有改动必须“只增不减”。新增图驱动首页时，
`创建实验室`、实验室列表操作、顶栏旧导航必须原样保留。F4 的验收专门检查这一点。

---

## Global Constraints

- 不新增 npm 依赖；只使用现有 `components/ui/*`、`App.css`、Tailwind 工具类。
- 不修改后端路由、图协议字段与旧 `/projects/*` 调用。
- 不删除旧视图分支（`home/detail/settings/mcp/skills/digital_humans/report_templates/graph`）。
- 标题与 slogan 使用精确文案：
  - `ParchTrace | 寻迹 - Autonomous Penetration System`
  - `Trace the Patch, Uncover the Flaw.`
- 首页数据只读；写操作仍走溯源图面板与旧实验室入口。
- 每个 Task 结束必须能 `npx tsc --noEmit`；最终 `npm run build` 与 `go build ./...` 通过。

---

## 文件边界

```
Create:
  webui/src/GraphOverview.tsx        # 首页图驱动总览（只读聚合）
  docs/plans/front_requirement.md
  docs/plans/front_task.md
  docs/plans/front_todo.md

Modify:
  webui/index.html                   # 浏览器标题
  webui/src/App.tsx                  # 顶栏品牌、首页集成、登录/初始化文案
  webui/src/App.css                  # 品牌与首页模块样式
  webui/src/GraphPanel.tsx           # 仅补充“迭代能力”说明条（可选）

Not modified:
  Web/*, graphserver/*, graph/*, dispatch/*   # 后端零改动
```

---

### Task F1: 文档与组件盘点（R1–R16 对齐）

**Files:**
- Create: `docs/plans/front_requirement.md`
- Create: `docs/plans/front_task.md`
- Create: `docs/plans/front_todo.md`

**Steps:**
- [ ] **Step 1**: 写三份 front 文档，全部包含 §0“前一轮移除旧入口”复盘。
- [ ] **Step 2**: 在 `front_requirement.md` §2 建立 R/Task → 前端落点矩阵。
- [ ] **Step 3**: 用 `grep` 盘点现有前端入口，确认旧按钮与导航的现状：
  ```bash
  grep -n "创建实验室\|压缩包文件\|Git 仓库\|压缩包链接\|空工作目录" webui/src/App.tsx
  grep -n "digital_humans\|report_templates\|#/graph\|#/mcp\|#/skills\|#/settings" webui/src/App.tsx | head
  ```
- [ ] **Step 4**: 提交文档。
  ```bash
  git add docs/plans/front_*.md
  git commit -m "docs(front): plan graph-first homepage without dropping legacy entries"
  ```

---

### Task F2: 品牌标题与 slogan（R13 产品面）

**Files:**
- Modify: `webui/index.html`
- Modify: `webui/src/App.tsx`（加载态、初始化、登录卡片、顶栏）
- Modify: `webui/src/App.css`

**Steps:**
- [ ] **Step 1**: `webui/index.html` 的 `<title>` 改为
  `ParchTrace | 寻迹 - Autonomous Penetration System`。
- [ ] **Step 2**: 顶栏 `.aix-header` 左侧改为品牌块：
  ```tsx
  <div className="aix-brand">
      <div className="aix-title">ParchTrace | 寻迹 - Autonomous Penetration System</div>
      <div className="aix-slogan">Trace the Patch, Uncover the Flaw.</div>
  </div>
  ```
- [ ] **Step 3**: 加载态、初始化设置、登录卡片中的 `AIxVuln` 文案改为 `ParchTrace | 寻迹`；
  登录卡片追加 slogan 一行。
- [ ] **Step 4**: 在 `App.css` 增加 `.aix-brand`、`.aix-slogan` 样式（渐变文字、字距、响应式字号）。
- [ ] **Step 5**: 验证：
  ```bash
  grep -n "ParchTrace | 寻迹 - Autonomous Penetration System" webui/index.html webui/src/App.tsx
  grep -n "Trace the Patch, Uncover the Flaw." webui/src/App.tsx
  cd webui && npx tsc --noEmit
  ```

---

### Task F3: 首页图驱动总览组件（R3–R12, R15）

**Files:**
- Create: `webui/src/GraphOverview.tsx`

**Interfaces（组件对外）:**
```ts
type Props = {
    baseURL: string;
    authHeader: string;
    labTotal: number;
    labRunning: number;
    onOpenGraph: () => void;
    onOpenLabs: () => void;
};
```

**数据来源（只读）:**
- `GET /api/graph/projects` → 项目列表
- `GET /api/graph/projects/:id/snapshot` → 每个项目（最多 5 个）的 nodes/edges/project

**聚合指标:**
- 项目数、active/completed/stopped 数
- facts / intents / hints / edges 总数
- 按 `type` 计数：`patch_profile`、`exploit_idea`、`constraint`、`observation`、`verdict`、`rule_hit`、`finding`
- open intents、claimed intents、持有 reason lease 的项目数

**Steps:**
- [ ] **Step 1**: 实现 `requestJSON`，兼容裸对象与 `{success,result}` 两种响应；401/403 显示“图 API 不可用”。
- [ ] **Step 2**: 实现项目加载 + 首批快照并行聚合；加载失败只降级该模块，不影响旧实验室区。
- [ ] **Step 3**: 渲染 §3 的指标条与 01–07 模块卡：
  - 每卡含编号、标题、R 编号、实时指标、说明、`打开溯源图` 按钮。
  - R14 只展示评测口径（反演成功率 / 试错轮次 / 单位 finding token 成本），不展示伪造数据。
- [ ] **Step 4**: 渲染“图项目速览”：标题、状态、bootstrap、origin 文件名、reason worker；
  点击项目卡片进入溯源图。
- [ ] **Step 5**: 空态：无项目时提示“新建图项目并填写补丁 diff 路径”，按钮进入溯源图；
  API 不可用时提示 `misc.graph_lane_enabled=true` 并保留旧实验室入口。
- [ ] **Step 6**: 验证：
  ```bash
  cd webui && npx tsc --noEmit
  ```

---

### Task F4: 首页集成与旧入口保留（强制验收）

**Files:**
- Modify: `webui/src/App.tsx`（仅 `view === 'home'` 分支与相关 props）

**Steps:**
- [ ] **Step 1**: 在 `view === 'home'` 分支顶部渲染 `<GraphOverview ... />`。
- [ ] **Step 2**: 在其下方加入分区标题
  `实验室与沙箱（保留入口）`，说明“旧车道与图车道并存；创建/启动后由后端自动初始化 sandbox”。
- [ ] **Step 3**: **原样保留**原实验室列表 Card：
  - `启动`、`停止`、`进入实验室`、`删除`
  - 列表空态与状态徽标
- [ ] **Step 4**: **原样保留**原创建实验室 Card：
  - 输入：实验室名称、总任务
  - 四种来源按钮与对应输入控件
  - 创建按钮动态文案
  - （仅调整占位文案，不删除控件）
- [ ] **Step 5**: 顶栏导航保持六项：数字人、报告模板、溯源图、MCP、Skills、设置。
- [ ] **Step 6**: 验证旧入口：
  ```bash
  grep -n "创建实验室\|压缩包文件\|Git 仓库\|压缩包链接\|空工作目录" webui/src/App.tsx
  grep -n "进入实验室\|启动实验室\|停止实验室\|删除实验室" webui/src/App.tsx
  ```

---

### Task F5: 溯源图面板能力说明补齐（R3–R13）

**Files:**
- Modify: `webui/src/GraphPanel.tsx`（仅新增说明，不改协议逻辑）

**Steps:**
- [ ] **Step 1**: 在面板副标题下方增加一行能力图例：
  `补丁反演 R7 · 推理 R8 · 沙箱 R9 · 判决 R10 · 规则 R11 · 人类意图 R12`。
- [ ] **Step 2**: 事实徽标已按类型着色，确认 `patch_profile/constraint/observation/verdict/rule_hit/finding`
  均有颜色映射；无映射的类型走默认样式。
- [ ] **Step 3**: 验证：
  ```bash
  cd webui && npx tsc --noEmit
  ```

---

### Task F6: 构建、测试与真实页面验证

**Steps:**
- [ ] **Step 1**: 类型检查：`cd webui && npx tsc --noEmit` → exit 0。
- [ ] **Step 2**: 前端构建：`cd webui && npm run build` → exit 0，`webui/dist` 更新。
- [ ] **Step 3**: Go 构建：`CGO_ENABLED=0 go build -o /root/aixvuln-web-graphui .` → exit 0。
- [ ] **Step 4**: 重启服务（释放旧端口、确认 `图调度车道已启动`）。
- [ ] **Step 5**: 真实页面验证：
  - `/` 返回 200；
  - 页面标题含 `ParchTrace | 寻迹 - Autonomous Penetration System`；
  - 顶栏含 `Trace the Patch, Uncover the Flaw.`；
  - 首页含 01–07 模块与“创建实验室”四种来源按钮；
  - 截图保存到 `dist/front-home.png`（或 `/tmp` 验收目录）。
- [ ] **Step 6**: 回归：进入 `#/graph`，确认导出、写入提示、意图领取/总结、总脑租约控件仍在。

---

### Task F7: 提交与推送

- [ ] **Step 1**: `git add` 三份 front 文档、`webui/index.html`、`webui/src/App.tsx`、
  `webui/src/App.css`、`webui/src/GraphOverview.tsx`、`webui/src/GraphPanel.tsx`、`webui/dist`。
- [ ] **Step 2**: `git commit -m "feat(front): graph-first homepage with legacy lab entries retained"`。
- [ ] **Step 3**: `git push origin main`（按既有约定直接覆盖 main，不走 PR）。
- [ ] **Step 4**: 回写 `front_todo.md` 与 `docs/plans/todo.md`。

---

## Self-Review

1. **旧入口覆盖**：F4 Step 3/4 与“5. 旧功能保留验收口径”逐项对应，任何一项缺失都判定 F4 不通过。
2. **迭代组件覆盖**：`front_requirement.md` §2 的 R1–R16 每行都有前端落点；无接口的 R14 明确标为口径说明。
3. **数据真实性**：首页只读聚合既有 API；不添加 mock 指标，不把“未接入”写成“已完成”。
4. **可回滚**：新增组件与样式独立，首页分支可单独回退；后端零改动。
5. **验证闭环**：tsc、npm build、go build、真实截图、图面板回归五类证据缺一不可。
