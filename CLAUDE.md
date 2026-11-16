# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

AIxVuln (this working copy is named PatchTrace) is a Go web service for LLM-driven, multi-agent vulnerability discovery and verification. A "project" is internally a **lab**: a total brain dispatches work to three research groups, each with its own decision brain, persistent digital-human agent pool, and tools. Agents install/run targets inside Docker sandboxes and produce exploit ideas, verified evidence, and reports. UI text and prompts are Chinese; code identifiers are English.

## Commands

Prerequisites: Go 1.24.1+, Node/npm, and a running Docker daemon.

```bash
# Frontend must be built first — main.go embeds webui/dist
cd webui && npm install && npm run build
cd .. && CGO_ENABLED=0 go build -o AIxVuln-web .

# Run from source (default :9999)
go run .
go run . -port 8080 -ip 192.168.1.10 -auto-mode full
go run . -web-mode comprehensive
go run . -mcp-config /path/to/mcp.json

# Frontend dev server (Vite on :5173); API calls fall back to 127.0.0.1:9999
cd webui && npm run dev

# Tests
go test ./...
go test ./DecisionBrain -run TestBrainMemoryExportDebugSnapshot -v

# Release builds
./build_all.sh              # frontend + linux/amd64, writes dist/ + tar.gz
./build_release.sh          # current OS/arch, writes release/
bash dockerfile/build.sh    # build embedded aisandbox and java_env images
```

There is no lint config. Use `gofmt -w`; run `go vet ./...` once the missing packages below are restored. `CGO_ENABLED=0` is used because SQLite is the pure-Go `modernc.org/sqlite` driver.

## Current build blocker

`go build ./...` does **not** compile in this checkout: internal packages `AIxVuln/skill` and `AIxVuln/resource` are imported but the directories do not exist.

- `AIxVuln/skill` is used by `agents/base.go`, `Web/Route.go`, and `toolCalling/{ListSkills,ListSkillFiles,ReadSkill}Tool.go` plus `toolCalling/skill_markdown_summary.go`.
- `AIxVuln/resource` (aliased `resourcepack`) is used by `Web/AgentTools.go` and eight `toolCalling/` files.

The contracts are documented in `docs/2026-10-12-patch-oracle-design.md` §3.1; `toolCalling/ReadSkillTool_test.go` is the stub acceptance test for the skill package. Implement/stub these packages rather than deleting imports. `webui/dist/` and `dockerfile/` already exist and embed correctly. The release tarball contains a `resource/{tools,docs,pocs}` tree; the design expects the resource package to read `AIxVuln_RESOURCE_ROOT` (default `../deploy/resource`). `__MACOSX/` and `docs/*.pdf|pptx` are artifacts, not source.

## Architecture

### Control flow

```
main.go
  └─ Web/Server.go (gin HTTP + /ws WebSocket, token auth, project start queue)
       └─ LabManager (one per project/lab; total brain + 3 research groups)
            ├─ WebSecurityGroup  → ProjectManager → DecisionBrain → agents → tools
            ├─ BinarySecurityGroup → its own DecisionBrain → binary agents
            └─ PentestSecurityGroup → its own DecisionBrain → pentest agents
```

- `main.go` parses flags, sets runtime automation/web-research modes, embeds `webui/dist` and `dockerfile/`, then starts `Web.Server`.
- `Web/Server.go` is the only HTTP/WS surface. `/projects/*` and `/labs/*` are alias route families. It persists project snapshots via `Web/ProjectStateStore.go` (SQLite) and runs starts through a queue capped by `misc.PROJECT_TASK_MAX_CONCURRENCY`. Bearer tokens are HMAC-signed with a process-random secret, so all sessions invalidate on restart.
- `LabManager` orchestrates. The **total brain** (总脑) makes an LLM tool call (`Tool-DispatchResearchTasks`) to pick the minimum set of groups and per-group tasks. It owns team chat, shared report catalog, cross-group collaboration sessions, and host-command approvals. All groups share the lab's sandbox: binary/pentest register runtime aliases (`<labID>_binary_security` → `<labID>`) so `taskManager` resource lookups resolve to the same Docker objects.
- `ProjectManager` exists only for the web group. It copies source into the project workspace, owns the task list, `DockerManager`/`ServiceManager`/sandbox, the group `DecisionBrain`, and start/stop lifecycle.
- `DecisionBrain` (one per group) is a long-running loop: read memory → call the LLM with `Agent-*` tools plus utility tools → spawn/reuse digital humans from a per-agent-type pool with a FIFO queue → audit exploit ideas → compress memory on context pressure → persist exploit state. It stores confirmed exploit ideas/chains in `data/projects/<labID>/memory/exploits.db`.
- `agents` are persistent capability instances, not per-call objects: `StartTask(ctx)` blocks on an `AssignTask` channel and reuses memory across assignments. Capability catalogs are under `agents/web`, `agents/binary`, `agents/pentest`, plus `agents/general_catalog.go`; each `AgentDefinition` declares the tool name (`Agent-<Domain>-<Type>`), description, args schema, and constructor. Tool sets are assembled in `agents/agent_builder.go`.
- `toolCalling` defines `ToolHandler` and the central `ToolManager`: argument normalization/validation, `<aixfile>` path rewriting, target-action approval, token-budget trimming of MCP tools, and MCP profile binding per agent type (backed by `misc/MCPDB.go`).
- `llm` is the provider boundary. `llm.GetResponsesClient(section, "main_setting")` is the normal client factory; it selects Chat Completions, Responses, or Anthropic Messages based on `API_MODE` (and auto-detects `anthropic.com` base URLs), rotates `|-|`-separated keys, enforces per-key `MaxRequest` concurrency, and caches clients. `ContextManager`, `SharedContext`, and `BrainMemory` handle turns, key messages, compression, and token counting.
- `dockerManager` wraps the Docker SDK (`Sandbox`, `ServiceManager` language environments). `taskManager` holds process-global per-lab registries (sandbox, service manager, docker manager, goroutine channel) and the domain models (`Task`, `Vuln`, `ExploitIdea`, `Report`, background processes). `misc` owns SQLite config and other local DBs, paths, and output helpers. `prompts` contains all system prompts and fragments.

### Layering

`Web → LabManager → ProjectManager / DecisionBrain → agents → toolCalling → llm / dockerManager / taskManager / misc`. `prompts` and `misc` are leaves. Avoid introducing upward imports.

### Configuration and data

- Config lives in `data/AIxVuln.db`; defaults are declared in `misc/ConfigDB.go`. `AIXVULN_DATA_DIR` only overrides where that DB is opened, and only before the singleton initializes. Required keys: `main_setting.BASE_URL`, `main_setting.OPENAI_API_KEY`, `main_setting.MODEL`.
- `misc.DATA_DIR` (a config value) controls all other data paths: `data/projects/<labID>/workdir/sourceCodeDir`, `data/projects/<labID>/memory/`, `data/.reportTemplate`, `data/.avatars`.
- Agent-type config sections inherit from `main_setting`: `decision`, `general`, `ops`, `analyze`, `verifier`, `report`, `overview`, `pentest`, `src`, `android`, `reverse`, `remote_control`. Use `misc.GetConfigValueDefault(section, key, fallback)` / `GetResponsesClient(section, "main_setting")` to preserve this fallback.
- Digital humans and MCP server/profile bindings are in SQLite; MCP settings can also be file-backed via `-mcp-config`.
- Project snapshots persist through `Web/ProjectStateStore.go`; legacy `projectManager.json` is migrated on load.

## Conventions and gotchas

- **Adding a tool**: implement `toolCalling.ToolHandler`, add a factory in `agents/agent_builder.go`, and register it in the relevant tool-factory list. Do not shift existing factory indices — the design relies on slices such as `factories[:7]` and `factories[:12]`; append instead.
- **Adding an agent capability**: add an `AgentDefinition` + constructor in the right catalog, add its tool factories, and ensure digital-human profiles exist for that `Agent-*` type, because instances are pooled by agent-tool name.
- **Tool args** are validated centrally. Invalid arguments must come back to the model as tool-error JSON, not as panics.
- **Context limit handling**: on `llm.IsContextLimitError`, compress memory with `CompressIfNeeded`, then retry; existing agents cap this at two retries. Follow the `UnlockForLLM`/`LockForLLM` pattern around context reads.
- **Project-scoped runtime state** must be registered under the lab ID in `taskManager`'s global maps. Binary/pentest code relies on runtime aliases, so use the accessors rather than direct map access.
- **Approvals**: host commands and target actions can require human approval (`LabManager/HostCommandApproval.go`; target-action intent in `toolCalling`). Don't bypass approval flows; `-auto-mode full` / `-full-auto` is an explicit automation switch.
- **Prompts** belong in `prompts/`; keep prompt text out of agent/tool code where possible.
- **Frontend**: one React app (`webui/src/App.tsx`, no router, no frontend tests). API helpers are in `web_api.ts`. Rebuild `webui/dist` after UI changes or the embedded UI will be stale.
- **Tests** are offline unit tests (no real Docker or LLM); use `t.TempDir` and config overrides. The config DB is a process singleton, so set `AIXVULN_DATA_DIR` before first config access when isolating. Tests may create package-local `data/` directories.

## Planning documents

- `README.md` — user-facing setup, flags, config, deployment.
- `docs/2026-10-12-patch-oracle-design.md` — PatchOracle design: patch-diff ingestion → semantic inversion → exploit idea → sandbox validation → rule base. §3.1 documents the missing `skill`/`resource` package contracts and acceptance criteria.
- `docs/claude/*.md` — Chinese design documents for a graph-engineering refactor (three graphs + worker pool). These are proposals; the current tree is still the hub-and-spoke implementation.
