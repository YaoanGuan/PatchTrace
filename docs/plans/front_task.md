# PatchTrace 前端迭代实施计划（front_task.md）

> 配套需求：`docs/plans/front_requirement.md`（F1–F8）
> 技术栈：React 18 + TypeScript + Vite 5 + TailwindCSS 3 + 既有 UI 组件；无新增依赖
> 影响文件：`webui/src/App.tsx`、`webui/src/GraphPanel.tsx`、`webui/index.html`、`webui/dist/`
> 状态：F1–F7 已完成，F8 待后端端点

---

## Global Constraints

- 不新增 npm 依赖，不引入图表库/路由库。
- 不修改 `/api/graph` 协议字段；前端读写沿用 `graphserver/server.go` 现有契约。
- 不删除旧总脑/研究组页面，只将其降级为“兼容车道”。
- 首页所有数据必须来自真实 API；不可用时显示明确空态，禁止 mock 冒充。
- 中文界面、英文标识；枚举原始值保留在 `title` 属性。
- 改完 `webui/` 必须重新 `npm run build`，并重新 `go build`，否则嵌入 UI 过期。
- 验收证据写入 `docs/plans/front_todo.md`；没有证据不标记完成。

---

## 文件结构（本迭代）

```
webui/
  index.html                  # 页面标题 PatchTrace
  src/
    App.tsx                   # 首页图驱动范式、实时总览、演示路径、组件矩阵、兼容车道降级
    GraphPanel.tsx            # 事实/边/意图语义标签
  dist/                       # npm run build 产物（被 go:embed 嵌入）
docs/plans/
  front_requirement.md
  front_task.md
  front_todo.md
```

---

### Task F1: 首页范式切换（对应 F1/F2/F5）

**Files:**
- Modify: `webui/src/App.tsx`

**Interfaces:**
- Consumes: `baseURL`、`authHeader`（既有）、`/api/graph/projects`、`/api/graph/projects/:id/snapshot`
- Produces:
  - `GraphHomeState`、`GraphHomeCounts`、`GraphHomeProject` 类型
  - `emptyGraphHome()`、`emptyGraphCounts()`
  - 首页首屏：图驱动 hero + 实时计数 + 最新项目 + 时间线折叠区

- [x] **Step 1: 定义首页状态与计数模型**

  `GraphHomeCounts` 覆盖 facts/intents/hints/edges/evidenceEdges/patchProfiles/exploitIdeas/constraints/observations/verdicts/ruleHits/findings。

- [x] **Step 2: 拉取图项目与最新快照**

  在 `view === 'home' && authHeader && initChecked` 时请求：
  - `GET {baseURL}/api/graph/projects`
  - 取按 `created_at` 倒序的最新项目，请求 `GET .../snapshot`

  非 JSON 响应 → `图 API 未挂载`；`401/403` 不静默吞掉。

- [x] **Step 3: 重写首页首屏**

  首屏包含：`图驱动范式` 徽标、图 API 状态、项目数、标题“补丁反演 · 溯源图推理 · 沙箱实证”、10 个计数卡片、“进入溯源图”按钮。

- [x] **Step 4: 旧车道降级**

  实验室列表与创建表单移入 `legacyLaneOpen` 条件块，默认折叠；创建表单占位改为兼容车道说明。

- [x] **Step 5: 验证**

  `npx tsc --noEmit` → exit 0；`npm run build` → exit 0；隔离实例截图断言通过。

---

### Task F2: 演示路径与组件矩阵（对应 F3/F4）

**Files:**
- Modify: `webui/src/App.tsx`

**Interfaces:**
- Produces: `GRAPH_DEMO_STEPS`（6 项）、`GRAPH_MODULES`（M1–M9）常量与渲染卡片

- [x] **Step 1: 定义 6 步演示路径**

  导入补丁 → 读图找缺口 → 认领意图 → 沙箱实证 → 证据判决 → 导出与回放；每步带 R* 与入口按钮。

- [x] **Step 2: 定义组件矩阵**

  M1–M9 映射 R3–R15，状态限定为“已展示/部分展示/待接入”，并写明缺口。

- [x] **Step 3: 渲染**

  两栏/三栏响应式网格；所有入口指向 `#/graph`。

- [x] **Step 4: 验证**

  截图中 `document.body.innerText` 同时包含“图驱动演示路径”和“迭代组件 → 前端展示”。

---

### Task F3: 图节点/边语义标签（对应 F7）

**Files:**
- Modify: `webui/src/GraphPanel.tsx`

**Interfaces:**
- Produces: `factTypeLabel()`、`edgeKindLabel()`、`intentTypeLabel()`、`nodeStateLabel()`
- 覆盖事实 8 类、边 5 类、意图状态 6 类；未知值回退原始字符串。

- [x] **Step 1: 增加标签映射与回退函数**
- [x] **Step 2: 应用到事实徽标、FactPicker、溯源链、意图状态/类型、边列表**
- [x] **Step 3: 原始枚举保留在 `title`，便于复制与排障**
- [x] **Step 4: 验证**：`tsc` 与 `npm run build` 通过；截图/文字包含中文标签。

---

### Task F4: 品牌与文案（对应 F6）

**Files:**
- Modify: `webui/index.html`、`webui/src/App.tsx`

- [x] `index.html` `<title>` → `PatchTrace`
- [x] 顶栏标题 → `PatchTrace · 补丁反演与溯源图推理`
- [x] 实验室详情副标题 → `兼容车道：旧总脑调度与研究组协同面板`
- [x] 创建表单占位与列表说明去除旧主编排默认叙事

---

### Task F5: 构建与嵌入（发布步骤）

- [x] `cd webui && npx tsc --noEmit`
- [x] `cd webui && npm run build`（更新 `webui/dist`）
- [x] `CGO_ENABLED=0 go build -o /root/aixvuln-web-graphui .`
- [x] 重启服务，确认启动日志包含 `图调度车道已启动，配置: dispatch.yaml`
- [x] `curl` 首页 200、未认证 `/api/graph/projects` 401

---

### Task F6: 隔离实例 UI 验收

**Steps:**

1. 独立 `AIXVULN_DATA_DIR` + `-port 9998 -ip <eth0-ip>` 启动第二个实例。
2. `POST /init` 创建临时账号，设置 `misc.graph_lane_enabled=true` 并重启。
3. 通过 API 写入演示项目、Hint、意图领取/总结，生成真实节点与时间线。
4. Chromium CDP 注入 token，打开首页，跳过 Docker 引导，截图。
5. 断言（`document.body.innerText`）：
   - `补丁反演 · 溯源图推理 · 沙箱实证`
   - `图 API 可用`
   - `图驱动演示路径`
   - `迭代组件 → 前端展示`
   - `兼容车道（旧总脑 / 研究组）`
   - 演示项目标题
6. 证据写入 `front_todo.md`；截图保管在 `/tmp/r13/home-new*.png`（不作为仓库产物）。

> 注意：图库路径由 `misc.DATA_DIR` 决定，隔离实例与主实例可能共享 `data/patchtrace_graph.db`；验收前确认项目来源，避免把主实例真实项目误当测试数据。

---

### Task F7: 后端缺口端点（对应 F8，待做）

**目标接口（建议）：**

```go
// graphserver/server.go
g.GET("/meta", s.meta)                                  // {mode, config_path, dispatcher_running}
g.GET("/projects/:id/rules", s.rules)                   // [{rule_id, version, confidence, hit_count, confirm_count}]
g.GET("/projects/:id/findings", s.findings)             // [{id, content, provenance}]，仅返回带证据路径的 finding
```

```go
// eval 或独立路由
g.GET("/api/eval/reports", s.evalReports)               // patcheval Metrics JSON
```

**前端落点：**
- 首页车道徽标读取 `meta.mode/dispatcher_running`，M3 从“部分展示”升级为“已展示”。
- 图面板新增“规则”区与“Finding”区；M6/M8 补全。
- 首页 M9 读取评测指标，展示反演成功率、平均试错轮次、单位 Finding token 成本。

**DoD：**
- 每个端点有 `httptest` 覆盖 200/401。
- 前端无 mock；不可用时保持“待接入”。
- `go test ./graphserver/ -race` 与 `npx tsc --noEmit` 通过。

---

## 验收命令汇总

```bash
cd /root/PatchTrace/webui
npx tsc --noEmit
npm run build
cd /root/PatchTrace
CGO_ENABLED=0 go build ./...
go test ./graphserver/ -count=1
curl -s -o /dev/null -w 'index %{http_code}\n' http://127.0.0.1:9999/
curl -s -o /dev/null -w 'graph %{http_code}\n' http://127.0.0.1:9999/api/graph/projects
```

期望：`TSC_EXIT=0`、`BUILD_EXIT=0`、`go build` exit 0、`ok AIxVuln/graphserver`、`index 200`、`graph 401`。

---

## 风险与回滚

| 风险 | 应对 |
|---|---|
| 首页聚合请求失败导致空态 | 展示明确错误，不回退到旧叙事、不显示假计数 |
| 图库与旧库路径耦合，隔离不彻底 | 验收记录项目 id；F8 可增加 `graph_db_path` 到 meta |
| 嵌入 UI 过期 | 每次改 `webui/` 后强制 build + go build + 重启 |
| 旧车道功能回归 | 兼容车道保留原组件与调用，仅条件渲染；展开后逐项回归 |
| 列表顺序不稳定 | 前端按 `created_at` 倒序，快照取最新项目 |
