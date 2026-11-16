# Starting the AIxVuln Server

This guide covers a local deployment of the whole project: build the web UI, build the Go binary, start the service, verify it, and stop it.

## 1. Prerequisites

| Component | Version | Notes |
|---|---|---|
| Go | 1.24.1+ | `CGO_ENABLED=0` is required (pure-Go SQLite driver) |
| Node.js / npm | Node 18+ | Only needed to rebuild the frontend |
| Docker | Any recent version | Required for sandbox work (image build, target containers), not for serving the UI |

Check:

```bash
/usr/local/go/bin/go version
node -v && npm -v
docker info
```

If the Go module proxy is unreachable, use the mirror:

```bash
export GOPROXY=https://goproxy.cn,direct
export GOSUMDB=off
```

## 2. Build the frontend

`main.go` embeds `webui/dist`, so the frontend must exist before the Go build. Rebuild it after any UI change.

```bash
cd webui
npm ci
npm run build
cd ..
```

If `npm run build` fails with `You installed esbuild for another platform`, the checked-in `node_modules` came from macOS. `npm ci` normally fixes it; if not, run the esbuild installer explicitly:

```bash
node node_modules/esbuild/install.js
```

## 3. Build the Go binary

```bash
cd /root/PatchTrace
CGO_ENABLED=0 go build -o AIxVuln-web .
```

The binary is large (~331 MB) because the UI, Dockerfiles, and agent tools are embedded.

## 4. Start the server

```bash
AIxVuln_RESOURCE_ROOT=/root/PatchTrace/deploy/resource \
  ./AIxVuln-web -port 9999 -ip 127.0.0.1
```

- `-ip` avoids the interactive IP prompt. Use the address other machines will use to download agent tools.
- `AIxVuln_RESOURCE_ROOT` points at the extracted `resource/{tools,docs,pocs}` tree. Without it the same tree is looked up at `../deploy/resource`, `./resource`, then next to the executable.
- Set `AIXVULN_DATA_DIR` only before the first config access if you want data outside `./data`.

Useful flags:

| Flag | Default | Purpose |
|---|---|---|
| `-port` | `9999` | Listen port |
| `-ip` | prompt | Public IP used in agent-tool download URLs |
| `-auto-mode` | off | `full` auto-approves host commands |
| `-full-auto` | false | Shortcut for `-auto-mode full` |
| `-web-mode` | `audit` | Web group mode: `audit` or `comprehensive` |
| `-mcp-config` | empty | Path to a file-backed MCP config |

Run it in the background for a long-lived deployment:

```bash
nohup env AIxVuln_RESOURCE_ROOT=/root/PatchTrace/deploy/resource \
  ./AIxVuln-web -port 9999 -ip 127.0.0.1 > /tmp/aixvuln-web.log 2>&1 &
```

## 5. First-run setup

1. Open `http://127.0.0.1:9999`.
2. The wizard asks for the first admin account (any username, password ≥ 6 characters). There is no default account.
3. In **设置 / Settings**, fill `main_setting.BASE_URL`, `main_setting.OPENAI_API_KEY`, `main_setting.MODEL`.
4. Docker images `aisandbox` and `java_env` must be built before projects can run:
   `bash dockerfile/build.sh` (requires a running Docker daemon).

The setup API can also be driven directly:

```bash
curl -s http://127.0.0.1:9999/init_status
curl -s -X POST http://127.0.0.1:9999/init \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"change-me"}'
```

## 6. Verify the deployment

```bash
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:9999/init_status   # 200
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:9999/              # 200
curl -s http://127.0.0.1:9999/agent_tools                                     # JSON tool list
ss -ltnp | grep 9999                                                          # listening socket
```

Login and authenticated call:

```bash
TOKEN=$(curl -s -X POST http://127.0.0.1:9999/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"change-me"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:9999/projects
```

Change the password (endpoint added for deployments without a UI entry):

```bash
curl -s -X POST http://127.0.0.1:9999/change_password \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"oldPassword":"change-me","newPassword":"new-password"}'
```

## 7. Stop / restart

```bash
pkill -f './AIxVuln-web'     # stop
# restart with the same command as step 4
```

Tokens are HMAC-signed with a process-random secret, so every restart invalidates existing sessions; log in again afterwards.

## 8. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `package AIxVuln/skill is not in std` | Missing `skill/` or `resource/` package; restore them, do not delete imports |
| `npm run build` fails on esbuild platform | Run `npm ci` and `node node_modules/esbuild/install.js` on the Linux host |
| `open .../deploy/resource/tools/manifest.json: no such file` | Extract the release tarball's `resource/` to `deploy/resource` or set `AIxVuln_RESOURCE_ROOT` |
| `init_status` reports `docker_images: false` | Docker not installed/running; install and run `bash dockerfile/build.sh` |
| Port already in use | Change `-port`, or stop the old process with `pkill -f AIxVuln-web` |
| Server exits asking for an IP | Pass `-ip <address>` for non-interactive starts |
