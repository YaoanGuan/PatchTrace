# PatchTrace 前端迭代待办与进度（front_todo.md）

> 配套：`docs/plans/front_requirement.md`（F1–F8）、`docs/plans/front_task.md`（F1–F7）
> 更新规则：每完成一步立即回写；没有命令输出/截图断言就不打勾。
> 当前轮次：第 1 轮 · 2026-11-17

---

## 0. 下一步动作（只做这一件）

> **前端对齐已完成 F1–F7**：首页图驱动范式、实时图项目总览、6 步演示路径、R1–R16 组件矩阵、旧车道降级、品牌文案、图节点/边中文语义标签。
> **证据**：`npx tsc --noEmit` → exit 0；`npm run build` → exit 0（`dist/assets/index.897af318.js`）；`CGO_ENABLED=0 go build ./...` → exit 0；隔离实例截图断言全部 true（含“刷新图状态”按钮）。
> **下一动作**：由后端 owner 实现 F8 的四个只读端点（`/api/graph/meta`、`/projects/:id/rules`、`/projects/:id/findings`、`/api/eval/reports`），前端再把 M3/M6/M9 从“部分展示/待接入”升级为“已展示”。

---

## 1. 进度总览

| # | 任务 | 需求 | 状态 | 证据 |
|---|---|---|---|---|
| F1 | 首页范式切换 | F1/F2/F5 | 完成 | `App.tsx`；截图 `/tmp/r13/home-final2.png`；断言 `new_title/demo_path/module_matrix/live_counts/project_title/legacy_demoted` 全 true；`refresh_button=true` |
| F2 | 演示路径与组件矩阵 | F3/F4 | 完成 | 首页文字包含“图驱动演示路径”“迭代组件 → 前端展示”；M1–M9 渲染 |
| F3 | 图节点/边语义标签 | F7 | 完成 | `GraphPanel.tsx`；CDP 断言 `fact_label/state_label/edge_label/raw_hidden` 全 true；截图 `/tmp/r13/graph-labels.png` |
| F4 | 品牌与文案 | F6 | 完成 | 服务返回 `<title>PatchTrace</title>`；顶栏“PatchTrace · 补丁反演与溯源图推理” |
| F5 | 构建与嵌入 | — | 完成 | `TSC_EXIT=0`、`BUILD_EXIT=0`、`GO_BUILD_EXIT=0`、`GO_BUILD_ALL_EXIT=0`；重启后车道日志 `图调度车道已启动，配置: dispatch.yaml` |
| F6 | 隔离实例 UI 验收 | — | 完成 | 独立数据目录 + 9998 端口；`POST /init`；写入真实项目/Hint/意图/结论；截图断言通过 |
| F7 | 后端缺口端点 | F8 | 未开始 | 端点未实现，前端保持“部分展示/待接入” |
| F8 | 旧车道回归 | F5 | 完成（功能保留） | 兼容车道默认折叠，展开后列表/创建表单代码未改，仅条件渲染 |

状态取值：`未开始` / `进行中` / `阻塞` / `完成`。

---

## 2. 验收证据明细

### 2.1 类型检查与构建

```text
$ cd webui && npx tsc --noEmit
TSC_EXIT=0

$ npm run build
BUILD_EXIT=0
dist/assets/index.897af318.js   542.21 KiB / gzip: 153.70 KiB

$ CGO_ENABLED=0 go build ./...
GO_BUILD_ALL_EXIT=0

$ go test ./graphserver/ -count=1
ok  AIxVuln/graphserver  0.087s
```

### 2.2 主服务冒烟

```text
$ curl -s -o /dev/null -w 'index %{http_code}\n' http://127.0.0.1:9999/
index 200

$ curl -s -o /dev/null -w 'graph-unauth %{http_code}\n' http://127.0.0.1:9999/api/graph/projects
graph-unauth 401

$ curl -s http://127.0.0.1:9999/ | grep -o '<title>[^<]*</title>'
<title>PatchTrace</title>

$ grep 'graph-lane' <服务日志> | tail -1
[2026-11-17 01:41:25][*][graph-lane]: 图调度车道已启动，配置: dispatch.yaml
```

### 2.3 首页截图断言（隔离实例 9998）

Chromium CDP 注入 token，跳过 Docker 引导，读取 `document.body.innerText`：

```json
{"new_title": true, "demo_path": true, "module_matrix": true,
 "live_counts": true, "project_title": true, "legacy_demoted": true}
```

页面可见：`图 API 可用`、`2 个图项目`、`最近图项目：Langflow CVE-2025-3248`、10 个计数卡片、6 步演示路径、M1–M9 矩阵、`刷新图状态` 按钮、折叠的兼容车道。
截图：`/tmp/r13/home-final2.png`。

### 2.4 图面板语义标签断言

```json
{"graph_title": true, "fact_label": true, "state_label": true,
 "edge_label": true, "raw_hidden": true}
```

页面可见“项目起点 / 验证观测 / 待领取 / 已结论 / 产出”等中文标签；原始枚举保留在 `title`。
截图：`/tmp/r13/graph-labels.png`。

---

## 3. F1–F7 动作清单

- [x] **F1.1** 新增 `GraphHomeState/GraphHomeCounts/GraphHomeProject` 与空态工厂
  - 证据：`webui/src/App.tsx` 类型定义；`tsc` exit 0
- [x] **F1.2** 首页拉取 `/api/graph/projects` + 最新项目 snapshot
  - 证据：隔离实例显示 `图 API 可用`、真实项目与计数
  - 修复：初版误加 `!baseURL` 守卫导致非绝对 base 时永不请求；已移除并用相对路径
  - 修复：协议不保证列表顺序；已按 `created_at` 倒序取最新
- [x] **F1.3** 首屏图驱动 hero + 10 个计数卡片 + 时间线折叠区 + “刷新图状态”按钮
  - 证据：截图 `/tmp/r13/home-final2.png`；断言 `refresh_button=true`
- [x] **F1.4** 旧实验室入口降级为默认折叠的兼容车道
  - 证据：首页首屏无实验室列表；展开按钮“展开兼容入口”
- [x] **F2.1** 6 步演示路径（R7/R8/R3-R4/R9/R10/R15）
  - 证据：首页文字与截图
- [x] **F2.2** M1–M9 组件矩阵（R3–R15）
  - 证据：首页文字“迭代组件 → 前端展示”
- [x] **F3.1** Fact/Edge/Intent/State 中文标签映射与回退
  - 证据：`GraphPanel.tsx`；CDP 断言
- [x] **F3.2** 应用到事实徽标、FactPicker、溯源链、意图、边
  - 证据：截图 `/tmp/r13/graph-labels.png`
- [x] **F4.1** `index.html` 标题、顶栏标题、旧车道副标题/占位文案
  - 证据：`<title>PatchTrace</title>`；截图
- [x] **F5.1** 前端构建、Go 构建、重启、健康检查
  - 证据：2.1/2.2 命令输出

---

## 4. 遗留与缺口（不做假数据）

| 缺口 | 现状 | 需要 |
|---|---|---|
| 调度器运行态 | 首页仅显示“图 API 可用”；真实 `图调度车道已启动` 只在服务日志 | `GET /api/graph/meta` |
| 规则库版本/置信度 | `rule_hit` 事实可读；版本、hit/confirm 计数无端点 | `GET /api/graph/projects/:id/rules` |
| Finding 报告 | 事实列表可见 `finding` 类型；无独立报告视图 | `GET /api/graph/projects/:id/findings` |
| patcheval 指标 | 首页 M9 标注“待接入” | `GET /api/eval/reports` 或固定产物路径 |
| 图库隔离 | `graph.db` 路径由 `misc.DATA_DIR` 决定，隔离实例与主实例共享图数据 | meta 暴露 `graph_db_path`，或测试指定独立 DATA_DIR |

> 上述缺口已在 `front_requirement.md` F8 与 `front_task.md` Task F7 登记；未用 mock 填充。

---

## 5. 当前阻塞

| 日期 | 任务 | 现象 | 需要谁决策 | 状态 |
|---|---|---|---|---|
| 2026-11-17 | F7/F8 | 四个只读端点未实现 | 后端 owner 确认端点字段与路由 | 待开始 |

---

## 6. 状态回写规则

1. 只在有命令输出或截图断言时打勾。
2. 状态变化同步更新 §1 总览。
3. 新增前端需求先写 `front_requirement.md` §7，再改代码。
4. 每轮结束留下：`已完成 / 证据 / 下一轮唯一动作`。
5. 同一 Task 连续两轮无进展 → 记阻塞，换实现路径。
