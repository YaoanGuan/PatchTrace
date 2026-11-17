# Frontend Graph-First Homepage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the post-iteration PatchTrace homepage: a graph-first module walkthrough (patch → graph → reason → dispatch → sandbox → rules → completion) that keeps every legacy lab entry and all old navigation reachable.

**Architecture:** Add one read-only `GraphOverview` component that aggregates the existing `/api/graph/projects` and `/api/graph/projects/:id/snapshot` APIs and renders six metric tiles, a numbered 01–07 module sequence, an R1–R16 coverage matrix, a project list, and a link down to the legacy lane. Keep the legacy lab list and four-source creation form in `App.tsx` directly beneath it, unchanged in behavior. Rebrand the header and index title. No backend change.

**Tech Stack:** React 18 + TypeScript + Vite, existing `components/ui/*` primitives, Tailwind utility classes, `App.css`. Go 1.24 embeds `webui/dist`. Verification uses `tsc`, `npm run build`, `go build`, and a headless Chromium CDP script.

**Spec:** `docs/plans/front_requirement.md` (also `docs/plans/front_task.md`, `docs/plans/front_todo.md`)

**Baseline:** Branch commit `7dacf94`. The working tree already contains a partial implementation of Tasks 2–5. Treat each step as "verify the outcome, repair if missing"; do not duplicate cards or re-add already-present handlers. A task is complete only when its stated check passes.

## Global Constraints

- No new npm dependencies; reuse `components/ui/*`, `App.css`, and Tailwind utilities.
- Do not delete or hide any legacy view, button, or API call. The eight hash views `home/detail/settings/mcp/skills/digital_humans/report_templates/graph` stay mounted.
- Do not modify `Web/*`, `graphserver/*`, `graph/*`, `dispatch/*`, `/projects/*` routes, or graph protocol fields.
- Exact branding values:
  - Title: `ParchTrace | 寻迹 - Autonomous Penetration System` (spelling **ParchTrace**, not PatchTrace).
  - Slogan: `Trace the Patch, Uncover the Flaw.`
- Homepage is read-only: no graph writes from `GraphOverview`; writes stay in `GraphPanel` and the legacy forms.
- Never fabricate R14 eval numbers. The eval package emits NDJSON and has no HTTP endpoint; say so.
- Every completion claim needs fresh command output or a screenshot.

## Review Focus

- **Expired/invalid token (401/403):** `GraphOverview` must show "图 API 不可用" and still render the legacy lab cards below; it must not throw or blank the page.
- **Graph lane disabled:** the route returns HTML (non-JSON). The overview must not call `resp.json()` blindly.
- **Zero graph projects:** empty state must name the next action ("新建图项目…补丁 diff") and keep "前往实验室入口".
- **Partial snapshot failure:** one bad project snapshot must degrade only that project's counts; the page still renders.
- **R14 with no endpoint:** no mock pass-rate, retry, or token-cost figures may appear.
- **Long project title/origin:** truncation or wrapping must prevent horizontal overflow in the card and list.

Each is pinned by a check in Task 3.

---

### Task 1: Front iteration documents and legacy inventory

**Files:**
- Create/verify: `docs/plans/front_requirement.md`
- Create/verify: `docs/plans/front_task.md`
- Create/verify: `docs/plans/front_todo.md`
- Read: `webui/src/App.tsx`

**Interfaces:**
- Consumes: `docs/plans/requirement.md` R1–R16, `docs/plans/task.md` Task 1–11.
- Produces: the spec this plan implements; §0 of all three docs records that a prior frontend pass removed legacy entries.

- [ ] **Step 1: Assert the three documents exist and carry the retrospective**

Run:
```bash
for f in docs/plans/front_requirement.md docs/plans/front_task.md docs/plans/front_todo.md; do
  test -s "$f" && grep -q "旧" "$f" && echo "OK $f" || echo "MISSING $f"
done
```
Expected: three `OK` lines. If a file is missing, write it from `front_requirement.md` §0–§7; keep the three roles distinct (requirement / task / status).

- [ ] **Step 2: Assert the R1–R16 → frontend matrix exists**

Run:
```bash
grep -n "R1\|R16" docs/plans/front_requirement.md | head
```
Expected: rows covering R1 through R16, including R14 marked as no HTTP endpoint.

- [ ] **Step 3: Inventory legacy entries before touching code**

Run:
```bash
grep -n "创建实验室\|压缩包文件\|Git 仓库\|压缩包链接\|空工作目录" webui/src/App.tsx
grep -n "进入实验室\|启动\|停止\|删除" webui/src/App.tsx | head
grep -n "digital_humans\|report_templates\|#/graph\|#/mcp\|#/skills\|#/settings" webui/src/App.tsx | head
```
Expected: all four source labels, the lab action labels, and six nav targets are present. Record any missing item; it must be restored in Task 4.

- [ ] **Step 4: Commit the documents**

```bash
git add docs/plans/front_requirement.md docs/plans/front_task.md docs/plans/front_todo.md
git commit -m "docs(front): document graph-first iteration and legacy-entry retrofit"
```

---

### Task 2: Branding and top banner

**Files:**
- Modify: `webui/index.html`
- Modify: `webui/src/App.tsx` (loading, init, login, header)
- Modify: `webui/src/App.css`

**Interfaces:**
- Consumes: nothing.
- Produces: CSS classes `.aix-brand`, `.aix-title`, `.aix-slogan`; header markup with exact title and slogan.

- [ ] **Step 1: Assert the browser title**

Run:
```bash
grep -n "<title>ParchTrace | 寻迹 - Autonomous Penetration System</title>" webui/index.html
```
Expected: one match. If absent, replace the existing `<title>` text exactly.

- [ ] **Step 2: Assert header markup**

Run:
```bash
grep -n "ParchTrace | 寻迹 - Autonomous Penetration System" webui/src/App.tsx
grep -n "Trace the Patch, Uncover the Flaw." webui/src/App.tsx
```
Expected: the title appears in the header and the slogan appears in the header plus the login card. Loading/init brand text is `ParchTrace | 寻迹`.

- [ ] **Step 3: Assert styles produce two distinct lines**

Run:
```bash
grep -n "\.aix-brand\|\.aix-title\|\.aix-slogan" webui/src/App.css
```
Expected: `.aix-brand` (flex column), `.aix-title` (gradient/weight), `.aix-slogan` (muted, normal case), responsive overrides. The slogan must not use `text-transform: uppercase`, or a case-sensitive innerText check fails.

- [ ] **Step 4: Typecheck**

Run: `cd webui && npx tsc --noEmit`
Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
git add webui/index.html webui/src/App.tsx webui/src/App.css
git commit -m "feat(front): rebrand header to ParchTrace with slogan"
```

---

### Task 3: GraphOverview read-only aggregation

**Files:**
- Create/repair: `webui/src/GraphOverview.tsx`
- Read: `webui/src/components/ui/card.tsx`, `badge.tsx`, `button.tsx`

**Interfaces:**
- Consumes: `GET /api/graph/projects` (bare array or `{success:true,result:[...]}`), `GET /api/graph/projects/:id/snapshot` (`{project,nodes,edges}` or `{success:true,result:{...}}`).
- Produces: default export `GraphOverview(props: GraphOverviewProps): JSX.Element`.

```ts
type GraphOverviewProps = {
    baseURL: string;
    authHeader: string;
    labTotal: number;
    labRunning: number;
    onOpenGraph: () => void;
    onOpenLabs: () => void;
};
```

- [ ] **Step 1: Write the source-level acceptance checks**

Create `/tmp/front-acceptance.sh`:
```bash
#!/usr/bin/env bash
set -u
f=webui/src/GraphOverview.tsx
fail=0
check() { grep -q "$1" "$f" && echo "OK  $2" || { echo "FAIL $2"; fail=1; }; }
check "补丁摄入与反演" "module 01"
check "溯源图事实 / 意图 / 提示" "module 02"
check "推理与缺口检测" "module 03"
check "Worker 调度与租约" "module 04"
check "沙箱实证与证据判决" "module 05"
check "规则回流与评测" "module 06"
check "完成、重开与导出" "module 07"
check "R1" "R matrix R1"
check "R16" "R matrix R16"
check "无 HTTP 端点" "R14 honesty"
check "onOpenLabs" "legacy CTA"
check "Array.isArray(payload)" "bare-array handling"
grep -q "Math.random\|mock\|fake" "$f" && { echo "FAIL fabricated data"; fail=1; } || echo "OK  no fabricated data"
exit $fail
```
Run: `bash /tmp/front-acceptance.sh`
Expected before implementation: FAIL lines for any missing module. In the current tree most checks pass; record failures to fix.

- [ ] **Step 2: Implement response handling**

Implement `isJsonResponse(resp)` (content-type contains `application/json`) and an `unwrap(payload)` that returns `payload.result` only when `payload.success === true`, and `payload.Data` when present. Fetch projects with the `Authorization: authHeader` header and an `AbortController`.

Required behavior:
- non-JSON response or non-array payload → `laneUnavailable=true`, empty lists, no throw;
- `401`/`403` → `laneUnavailable=true` plus message `鉴权失败（HTTP <status>），请重新登录`;
- no `authHeader` → stop without fetching.

- [ ] **Step 3: Implement aggregation**

Fetch snapshots for at most five projects in parallel. A failed snapshot yields `{project:p,nodes:[],edges:[]}` and must not reject the batch. Derive:
`facts`, `intents`, `hints`, `edges`, `openIntents` (`state` absent or `open`), `claimedIntents`, `patchProfiles`, `exploitIdeas`, `constraints`, `observations`, `verdicts`, `ruleHits`, `findings`, `activeProjects`, `completedProjects`, `stoppedProjects`, `reasonLeases`, `bootstrapProjects`.

- [ ] **Step 4: Render the required surface**

Order: title "图驱动总览" → lane badge (图 API 可用/不可用) + project count + refresh button → six metric tiles (图项目/事实/意图/提示/证据边/实验室) → "模块演示序列" 01–07 cards with R ids, live metric text, description, click → `onOpenGraph` → `<details>` "迭代组件覆盖（R1–R16 逐项对照）" with statuses 已展示/部分展示/待接入/工程侧 → "图项目速览" (max 4, title, origin basename, bootstrap/reason/status badges, click → graph) → legacy CTA `onOpenLabs`.

Constraints:
- R14 text must state `无 HTTP 端点` and name the three metrics without values.
- Empty projects: "暂无图项目。进入溯源图新建项目，来源填写补丁 diff 绝对路径并勾选 bootstrap。"
- Use `truncate` on titles/origins and `break-words` on descriptions.

- [ ] **Step 5: Run the source-level acceptance check**

Run: `bash /tmp/front-acceptance.sh`
Expected: every line `OK`, exit 0.

- [ ] **Step 6: Typecheck**

Run: `cd webui && npx tsc --noEmit`
Expected: exit 0.

- [ ] **Step 7: Commit**

```bash
git add webui/src/GraphOverview.tsx
git commit -m "feat(front): add graph overview with R1-R16 coverage"
```

---

### Task 4: Homepage integration without removing legacy entries

**Files:**
- Modify: `webui/src/App.tsx` (`view === 'home'` branch only, plus the `GraphOverview` import)

**Interfaces:**
- Consumes: `GraphOverview` props from Task 3; existing `projects`, `startProject`, `stopProject`, `deleteProject`, `enterProject`, `projectName`, `taskContent`, `sourceType`, `gitUrl`, `fileUrl`, `fileRef`, `createProject`.
- Produces: homepage JSX where `GraphOverview` precedes `id="legacy-labs"`.

- [ ] **Step 1: Assert the integration shape**

Run:
```bash
grep -n "import GraphOverview" webui/src/App.tsx
grep -n "<GraphOverview" webui/src/App.tsx
grep -n "id=\"legacy-labs\"" webui/src/App.tsx
```
Expected: import present, exactly one `<GraphOverview`, one `id="legacy-labs"` anchor below it.

- [ ] **Step 2: Render the overview and anchor**

In the `view === 'home'` branch, render:
```tsx
<GraphOverview
    baseURL={baseURL}
    authHeader={authHeader}
    labTotal={projects.length}
    labRunning={projects.filter(p => p.isRunning || p.isStarting).length}
    onOpenGraph={() => { window.location.hash = '#/graph'; }}
    onOpenLabs={() => {
        document.getElementById('legacy-labs')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }}
/>
```
Then a heading `实验室与沙箱（保留入口）` with the note that creation/start automatically initializes the sandbox, and a badge `旧入口保留 · 4 种源码来源`.

- [ ] **Step 3: Keep the legacy cards verbatim**

The lab list card must retain `启动`, `停止`, `进入实验室`, `删除`, each bound to the existing handler and guarded by the existing `btnLoading` keys. The creation card must retain the name input, task textarea, four source buttons (`压缩包文件`, `Git 仓库`, `压缩包链接`, `空工作目录`), the per-source inputs, and the dynamic create-button label. Copy may change; controls and handlers may not.

- [ ] **Step 4: Assert legacy entries**

Run:
```bash
grep -c "压缩包文件" webui/src/App.tsx
grep -c "Git 仓库" webui/src/App.tsx
grep -c "压缩包链接" webui/src/App.tsx
grep -c "空工作目录" webui/src/App.tsx
grep -c "进入实验室" webui/src/App.tsx
```
Expected: each count ≥ 1.

- [ ] **Step 5: Assert nav and views are intact**

Run:
```bash
grep -n "'home' | 'detail' | 'settings' | 'mcp' | 'skills' | 'digital_humans' | 'report_templates' | 'graph'" webui/src/App.tsx
grep -n "view === 'graph'" webui/src/App.tsx
```
Expected: the eight-value union and the graph branch both present.

- [ ] **Step 6: Typecheck**

Run: `cd webui && npx tsc --noEmit`
Expected: exit 0.

- [ ] **Step 7: Commit**

```bash
git add webui/src/App.tsx
git commit -m "feat(front): mount graph overview above preserved lab lane"
```

---

### Task 5: GraphPanel semantic labels and capability legend

**Files:**
- Modify: `webui/src/GraphPanel.tsx`

**Interfaces:**
- Consumes: protocol enums `fact.type`, `intent.state`, `intent.type`, `edge.kind`.
- Produces: helpers `factTypeLabel(type?: string): string`, `edgeKindLabel(kind?: string): string`, `intentTypeLabel(type?: string): string`, `nodeStateLabel(state?: string): string`; badge variants for `constraint` (secondary), `rule_hit` (success), `finding` (destructive).

- [ ] **Step 1: Assert label maps exist**

Run:
```bash
grep -n "FACT_TYPE_LABELS\|EDGE_KIND_LABELS\|INTENT_TYPE_LABELS\|NODE_STATE_LABELS" webui/src/GraphPanel.tsx
grep -n "function factTypeLabel\|function edgeKindLabel\|function intentTypeLabel\|function nodeStateLabel" webui/src/GraphPanel.tsx
```
Expected: four maps and four functions. Map values exactly:
`patch_profile→补丁档案`, `exploit_idea→利用思路`, `constraint→约束`, `observation→验证观测`, `verdict→判决`, `rule_hit→规则命中`, `finding→发现`; edges `derive→推导`, `support→支持`, `refute→反证`, `block→阻塞`, `produce→产出`; states `open→待领取`, `claimed→已领取`, `running→执行中`, `resolved/concluded→已结论`, `refuted→已反驳`.

- [ ] **Step 2: Assert the render sites use them**

Run:
```bash
grep -n "factTypeLabel(" webui/src/GraphPanel.tsx
grep -n "edgeKindLabel(" webui/src/GraphPanel.tsx
grep -n "intentTypeLabel(" webui/src/GraphPanel.tsx
grep -n "nodeStateLabel(" webui/src/GraphPanel.tsx
```
Expected: at least four call sites (fact picker, snapshot fact badge, fact list, intent list, edge list). Each badge keeps the raw enum in `title={...}` for copyability.

- [ ] **Step 3: Assert the capability legend**

Run:
```bash
grep -n "迭代能力：补丁反演 R7" webui/src/GraphPanel.tsx
```
Expected: one match near the panel subtitle. If missing, add the line without changing protocol logic.

- [ ] **Step 4: Typecheck**

Run: `cd webui && npx tsc --noEmit`
Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
git add webui/src/GraphPanel.tsx
git commit -m "fix(front): restore semantic labels in graph panel"
```

---

### Task 6: Build and runtime verification

**Files:**
- Build output: `webui/dist/*` (generated, embedded)
- No source edits.

**Interfaces:**
- Consumes: Tasks 2–5.
- Produces: a running binary with the new UI and fresh evidence.

- [ ] **Step 1: Typecheck**

Run: `cd webui && npx tsc --noEmit`
Expected: exit 0.

- [ ] **Step 2: Build the frontend**

Run: `cd webui && npm run build`
Expected: exit 0; `dist/index.html` and new hashed JS/CSS emitted.

- [ ] **Step 3: Build the Go binary**

Run: `cd /root/PatchTrace && CGO_ENABLED=0 go build -o /root/aixvuln-web-graphui .`
Expected: exit 0.

- [ ] **Step 4: Restart and smoke-test the service**

Run:
```bash
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:9999/
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:9999/api/graph/projects
```
Expected: `/` → `200`; unauthenticated graph API → `401`. Do not print tokens or API keys.

- [ ] **Step 5: Run the browser acceptance check**

Obtain a fresh Bearer token for the restarted process and write it to a mode-600 file without echoing it:
```bash
umask 077
curl -s -X POST http://127.0.0.1:9999/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"'"$AIX_USER"'","password":"'"$AIX_PASS"'"}' \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])' > /tmp/r13/verify-token
test -s /tmp/r13/verify-token && echo token-ready
```
Expected: `token-ready`; the token itself is never printed.
```bash
BASE=http://127.0.0.1:9999 TOKEN_FILE=/tmp/r13/verify-token python3 /tmp/r13/front_shot.py /tmp/r13/front-home.png
BASE=http://127.0.0.1:9999 TOKEN_FILE=/tmp/r13/verify-token python3 /tmp/r13/graph_check.py
```
Expected assertions: title true, slogan true, `模块演示序列` true, `实验室与沙箱（保留入口）` true, four source labels true, `启动` true, `graph_nav` true, module 01 and 07 true; graph panel `legend`, `export_yaml`, `export_timeline`, `hint`, `intent`, `reason` true.

- [ ] **Step 6: Inspect the screenshot**

Open `/tmp/r13/front-home.png` and confirm: two header lines, metric tiles, 01–07 cards, R matrix collapsed, project list, legacy section with four buttons. A blank or login-only frame is a failure.

- [ ] **Step 7: Commit only generated dist plus sources if dist is tracked**

Run:
```bash
git status --porcelain webui/dist webui/src webui/index.html docs/plans docs/superpowers
```
Expected: only intended files. Do not add unrelated marketplace changes.

---

### Task 7: Evidence write-back and push

**Files:**
- Modify: `docs/plans/front_todo.md`
- Modify: `docs/plans/todo.md`

**Interfaces:**
- Consumes: Task 6 outputs and screenshot paths.
- Produces: status table F1–F7 with real evidence and a push to `origin/main`.

- [ ] **Step 1: Fill the evidence table**

For F1–F7 record the exact command and result: `tsc exit 0`, `npm run build exit 0`, `go build exit 0`, HTTP codes, screenshot path, and the graph-panel check names. Do not write "通过" without the adjacent evidence.

- [ ] **Step 2: Note the supersession**

Add one line to `front_todo.md` §0: this round supersedes the previous front docs and restores the legacy entries that the earlier pass removed.

- [ ] **Step 3: Add the frontend entry to `docs/plans/todo.md`**

Record date, changed files, verification evidence, and the remaining known gap: R14 has no HTTP endpoint; Finding report is still deferred.

- [ ] **Step 4: Commit**

```bash
git add docs/plans/front_todo.md docs/plans/todo.md
git commit -m "docs(front): record homepage iteration evidence"
```

- [ ] **Step 5: Push directly to main**

```bash
git push origin HEAD:main
```
Expected: push accepted. The standing instruction is to overwrite `main` directly without a PR. Only push the commits from Tasks 1–7; unrelated marketplace work stays uncommitted.

- [ ] **Step 6: Confirm service health after push**

Run: `curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:9999/`
Expected: `200`. Note that a restart invalidates Bearer tokens; the user must log in again.

---

## Self-Review

**Spec coverage:** `front_requirement.md` §1 goals map to Tasks 2–6; §2 R1–R16 matrix is rendered in Task 3 Step 4 and asserted in Step 1; §3 IA order is Task 3 Step 4 plus Task 4; §4 exact copy is Task 2; §5 legacy retention is Task 4 Steps 3–5; §6 F-A1–F-A8 are split across Tasks 2, 4, 5, 6; §7 constraints are repeated in Global Constraints.

**Step scan:** Each step has one command or one concrete code outcome. No "handle edge cases" placeholders. Where code already exists, the step is a verification with a repair instruction rather than a transcription.

**Type consistency:** `GraphOverviewProps` in Task 3 matches the call site in Task 4. Label helper names in Task 5 match their call sites. CSS class names in Task 2 match `App.css`.

**Review Focus:** 401/non-JSON/zero-project/partial-snapshot/R14/long-text each have an explicit required behavior in Task 3 Step 2–4; runtime confirmation is Task 6 Step 5.

**Proportion:** The plan is longer than the diff it governs because the frontend has no test runner; assertions and browser checks replace unit tests.
