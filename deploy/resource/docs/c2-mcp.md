# RTunnelServer MCP Tools 使用说明

本文面向通过 MCP 调用 RTunnelServer 的操作者、自动化 Agent 和上层编排系统。

目标不是重复 `tools/list` 的字段定义，而是回答 4 个更实际的问题：

1. 什么时候该调用哪个 tool。
2. tool 之间的推荐顺序是什么。
3. 哪些参数最容易传错。
4. 高影响操作应该怎么做决策，尤其是 `configure_port_forward` 里的监听端选择。

---

## 1. MCP 定位

RTunnelServer MCP 是一个基于 HTTP POST JSON-RPC 的 MCP 接口，不是 SSE 会话型 MCP。

它更适合以下场景：

- 无状态的 MCP Client 直接按需调用。
- 自动化 Agent 按步骤发起 `tools/call`。
- 外部编排器把 RTunnel 当作一个“接入 + 控制 + 文件 + 转发 + 插件”后端。

它不适合以下预期：

- 长连接会话流式工具调用。
- 靠 MCP 本身保存复杂会话状态。
- 假设服务端会帮你自动完成“生成客户端后投放并上线”的全链路闭环。

---

## 2. 接入方式

### 2.1 入口

- `GET /mcp`
  读取 MCP 入口说明、认证方式、支持的方法。
- `POST /mcp`
  发送 JSON-RPC 2.0 请求，实际执行 MCP 方法。

### 2.2 认证

`/mcp` 支持以下任一认证方式：

- Cookie：`p=<secret>`
- Header：`Authorization: Bearer <secret>`
- Header：`x-rtunnel-secret: <secret>`
- Query：`/mcp?p=<secret>`

它与 Web 管理面板使用同一个 secret。

### 2.3 当前支持的方法

- `initialize`
- `tools/list`
- `tools/call`
- `prompts/list`
- `prompts/get`
- `ping`

兼容但基本无业务含义的方法：

- `notifications/initialized`
- `resources/list`

### 2.4 最小请求模板

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list_agents",
    "arguments": {}
  }
}
```

---

## 3. 返回格式怎么读

### 3.1 成功返回

所有 tool 成功时，都会返回类似结构：

```json
{
  "content": [
    {
      "type": "text",
      "text": "{ ... pretty json ... }"
    }
  ],
  "structuredContent": {
    "...": "..."
  }
}
```

对自动化系统来说，优先读 `structuredContent`。

`content[0].text` 更适合给人看，`structuredContent` 更适合程序判断。

### 3.2 失败返回

tool 失败时，返回结构仍然在 `result` 里，但会带：

```json
{
  "isError": true,
  "structuredContent": {
    "...": "..."
  }
}
```

处理建议：

- 先判断 `isError`。
- 再读取 `structuredContent`。
- 不要只解析 `content[0].text`。

常见两类失败：

- JSON-RPC 层失败：例如方法名错、参数结构不对。
- tool 层失败：例如 `Agent not found`、路径不存在、端口不是数字、平台架构没有对应客户端。

---

## 4. 推荐总流程

如果你的目标是“接入目标机器并继续控制”，推荐固定使用下面这套顺序：

1. `get_client_info`
   先读取当前允许的 `protocol / os / arch / crypto`，不要硬编码。
2. `generate_client`
   生成客户端元数据、下载地址和下载 code。
3. 将客户端投放到目标机器并运行。
4. 轮询 `list_agents`
   确认目标机器已经上线，并拿到 `dev_id`。
5. 拿到 `dev_id` 后，再进入控制阶段：
   - 命令执行：`exec_command`
   - 网络转发：`configure_port_forward`
   - 文件管理：`list_files`、`read_file`、`write_file`、`upload_file` 等
   - 插件：先 `list_plugins`，再 `run_plugin`
6. 需要做节点包/子节点切换时，再使用：
   - `add_package_node`
   - `switch_package_node`

不要反过来做：

- 不要在没有 `dev_id` 时直接调用控制类工具。
- 不要跳过 `get_client_info` 直接写死 `os/arch`。
- 不要继续向 `generate_client` 传 `dns` 或 `oss`。

---

## 5. Tools 总览

### 5.1 上线与接入类

| Tool | 作用 | 典型使用时机 |
| --- | --- | --- |
| `get_client_info` | 查询当前支持的客户端生成参数 | 准备生成新客户端前 |
| `generate_client` | 生成客户端下载信息 | 明确目标平台后 |
| `list_client_links` | 回看本次运行期生成过的客户端记录 | 回查 code / URL 时 |
| `list_download_nodes` | 查看当前中继/下载节点列表 | 判断下载链路时 |
| `add_agent_from_webio` | 让服务端主动挂接 WebIO 入口 | 需要通过 WebIO 拉起目标时 |

### 5.2 基础控制类

| Tool | 作用 | 典型使用时机 |
| --- | --- | --- |
| `list_agents` | 列出当前在线 Agent | 几乎所有动作前 |
| `exec_command` | 执行一次性命令 | 探测、执行、拉起进程 |
| `configure_port_forward` | 建立 TCP 转发或 SOCKS5 | 内网访问、代理、横向 |
| `disconnect_agent` | 强制下线 Agent | 清理连接、切换、终止 |

### 5.3 文件管理类

| Tool | 作用 | 典型使用时机 |
| --- | --- | --- |
| `list_files` | 列目录 | 文件操作前先确认路径 |
| `read_file` | 读单个文件 | 分析配置、读取数据 |
| `write_file` | 写文件 | 覆盖配置、写脚本 |
| `upload_file` | 上传任意文件 | 二进制、压缩包、工具投放 |
| `create_folder` | 建目录 | 上传或解压前准备路径 |
| `create_file` | 建空文件 | 占位或触发存在性检查 |
| `zip_path` | 打包路径 | 批量归档、准备回传 |
| `unzip_path` | 解压 zip | 部署工具、展开资源 |
| `copy_path` | 复制文件/目录 | 备份、生成副本 |
| `rename_path` | 重命名/移动 | 替换文件、切换目录 |
| `delete_path` | 删除文件/目录 | 清理或破坏性操作 |
| `download_url_to_path` | 让远端主动下载 URL 到路径 | 目标能出网时的快速投放 |

### 5.4 上下文与拓扑类

| Tool | 作用 | 典型使用时机 |
| --- | --- | --- |
| `get_topology` | 读取当前拓扑图 | 做关系判断、图谱展示 |
| `get_device_notes` | 读取设备备注 | 自动化补上下文 |
| `update_device_note` | 更新设备备注 | 标注用途、角色、风险 |

### 5.5 插件类

| Tool | 作用 | 典型使用时机 |
| --- | --- | --- |
| `list_plugins` | 查看当前插件列表 | 跑插件前先确认名字 |
| `run_plugin` | 在指定 Agent 上运行插件 | 有明确插件目标时 |

### 5.6 节点包类

| Tool | 作用 | 典型使用时机 |
| --- | --- | --- |
| `add_package_node` | 上传某个客户端包到指定 Agent | 准备让其具备新节点包 |
| `switch_package_node` | 切换到已上传过的节点包 | 已确认切换影响后 |

---

## 6. 逐类使用说明

## 6.1 `list_agents`

这是最基础的入口 tool。

你应当在以下动作前优先调用它：

- 执行命令前，确认 `dev_id` 还在线。
- 端口转发前，确认目标 Agent 还在线。
- 文件操作前，确认没有选错机器。
- 插件运行前，确认目标 Agent 没有掉线。

典型返回字段通常包括：

- `dev_id`
- `os`
- `inner_ips`
- `port_maps`
- `tunnel_type`

实战建议：

- 自动化流程里把它当作“会话有效性检查”。
- 看到新机器上线后，立刻记录 `dev_id`，后续一切控制都基于它。

---

## 6.2 `get_client_info`

用途：读取当前服务端真正支持的客户端生成参数。

你应该把它当作运行期真相源，而不是把平台和架构硬编码在上层系统里。

当前返回重点：

- `protocols`
- `os`
- `crypto`
- `tcp_addrs`
- `server_ip`
- `download_base_url`

关键点：

- 当前 MCP 只允许 `tcp` 和 `webio`。
- `os/arch` 是按服务端当前内置客户端二进制动态算出来的。
- `download_base_url` 基于服务端启动时配置的 `server_ip`，不是你访问 MCP 时的 Host 头。

适用场景：

- 生成新客户端前做参数发现。
- 自动化平台为用户展示下拉框时读取可选项。
- 排查“为什么这个平台架构生成不了”。

---

## 6.3 `generate_client`

用途：生成一个新的客户端下载项，并返回下载地址和明文连接 URL。

关键参数：

- `protocol`
  当前只能是 `tcp` 或 `webio`。
- `host_port`
  服务端对外地址，`tcp` 未写端口时会自动补 `:63781`。
- `crypto`
  应以 `get_client_info` 返回为准。
- `os`
  目标客户端操作系统。
- `arch`
  目标客户端架构。
- `startup_password`
  默认建议留空，只有用户明确要求时再设置。

关键行为：

- 会返回 `code`。
- 会返回 `download_url`。
- 会返回明文连接串 `url`。
- 会把该记录保存在服务端内存里，供后续 `/d/{code}` 下载和 `list_client_links` 回查。

重要限制：

- 只支持当前内置的客户端二进制。
- 不再支持 `dns`、`oss`。
- `arch` 支持别名，例如 `x64 -> amd64`、`arm64 -> aarch64`。

什么时候用 `tcp`：

- 目标可以稳定回连你的服务端地址。
- 你希望保持最直接的接入路径。

什么时候用 `webio`：

- 目标环境不方便直接向你的服务端建立稳定连接。
- 你计划借助目标已有 Web 服务做自研 WebIO 入口。
- 你准备后续配合 `add_agent_from_webio` 完成上线。

不建议的做法：

- 把 `startup_password` 当默认必填项。
- 使用未在 `get_client_info` 中出现的 `os/arch`。
- 假设 `download_url` 会跟着当前反代域名自动变化。

---

## 6.4 `list_client_links`

用途：列出当前运行期已经生成过的客户端记录。

适合：

- 回查某个 `code` 对应的平台、协议和 URL。
- 自动化流程在丢失上下文后重新找回记录。

注意：

- 这是运行期内存数据。
- 不要假设它在服务重启后仍然存在。

---

## 6.5 `list_download_nodes`

用途：查看当前维护的中继/下载节点列表。

适合：

- 判断当前是否已有可用下载节点。
- 诊断下载链路、投递链路或中继状态。

它更偏“观察型”工具，不是直接执行动作的工具。

---

## 6.6 `add_agent_from_webio`

用途：让服务端主动连接一个 WebIO 入口，并等待 Agent 通过它上线。

关键参数：

- `url`
  WebIO 入口地址。
- `password`
  WebIO 鉴权密码，会作为 `Rauth` Cookie 使用。
- `keep_time`
  与轮询/保持相关的时间参数。

重要认知：

- 它是异步提交型 tool。
- 返回“成功”只表示任务已下发，不代表目标已经上线。
- 目标是否真的上线，仍然要靠 `list_agents` 去轮询确认。

典型场景：

- 目标机器本身不方便直接回连，但目标环境已有一个可挂载 WebIO 的 Web 服务。
- 你已经生成了 `webio` 协议客户端，并把客户端及 WebIO 桥接逻辑部署到目标侧。

实战建议：

- 调用它后，紧接着进入 `list_agents` 轮询，而不是立即执行控制类操作。

---

## 6.7 `exec_command`

用途：在指定 Agent 上执行一次性命令。

最适合做这些事：

- 信息探测：系统版本、网卡、端口、进程、权限、域信息。
- 短平快动作：创建用户、启动单个进程、写注册表、修改单项配置。
- 配合文件操作拉起工具：先上传，再执行。

不适合做这些事：

- 强交互命令。
- 永不退出的前台命令。
- 输出极大的命令。
- 你无法承受后果的破坏性命令。

使用建议：

- 先根据 `os` 选对命令语法。
- Windows 和 Linux/macOS 语法完全不同。
- 若命令可能持续运行，优先考虑后台化。

高风险原因：

- 会直接在目标主机上执行命令。
- 返回内容可能不完整，但副作用已经发生。

---

## 6.8 `configure_port_forward`

这是最需要单独理解的 tool，也是最容易因为监听位置选错而“看起来调用成功，但实际用不了”的 tool。

它支持两种能力：

- `forward`
  固定 TCP 转发。
- `socks5`
  启动 SOCKS5 代理。

### 6.8.1 参数解释

- `dev_id`
  目标 Agent。
- `listen_location`
  监听位置，只应传 `server` 或 `node`。
- `listen_port`
  监听端口，虽然是字符串，但必须能解析成数字。
- `forward_type`
  `forward` 或 `socks5`。
- `target_address`
  仅 `forward` 必填，格式一般是 `host:port`。
- `description`
  备注说明，便于后续识别。
- `password`
  仅 `socks5` 有意义。

### 6.8.2 最重要的理解：`target_address` 是按 Agent 视角去访问

无论你选 `server` 还是 `node` 监听，真正去连接 `target_address` 的都是目标 Agent 所在一侧的能力链路。

因此：

- `target_address` 通常应该写目标内网可达地址。
- 很多时候应写 `127.0.0.1:端口`、`10.x.x.x:端口`、`192.168.x.x:端口` 之类从 Agent 视角可达的地址。
- 不应该默认按 RTunnelServer 自己的网络视角去写。

### 6.8.3 `listen_location=server` 何时选

`server` 表示监听端开在 RTunnelServer 所在机器。

适合这些场景：

- 你自己能访问 RTunnelServer，但不能直接访问目标内网。
- 你希望把目标内网服务“投影”到服务端上，再从服务端访问。
- 你想把 SOCKS5 代理开在服务端，方便本地浏览器、代理工具或其他上层系统直接连服务端。

这是最常见、最稳妥的选择。

典型理解：

- 服务端负责“对外暴露监听口”。
- Agent 负责“把服务端收到的流量带到目标地址”。

典型例子 1：把目标机器本地 Web 暴露到服务端

```json
{
  "dev_id": "target-dev-id",
  "listen_location": "server",
  "listen_port": "8080",
  "forward_type": "forward",
  "target_address": "127.0.0.1:80",
  "description": "目标本地Web"
}
```

效果：

- 访问 `http://<RTunnelServer>:8080`
- 实际打到目标机器的 `127.0.0.1:80`

典型例子 2：在服务端开 SOCKS5

```json
{
  "dev_id": "target-dev-id",
  "listen_location": "server",
  "listen_port": "1080",
  "forward_type": "socks5",
  "password": "abc123"
}
```

效果：

- 代理客户端连 `RTunnelServer:1080`
- 流量再通过该 Agent 走目标网络

特别有效的情况：

- 客户端在目标内网，操作者本地无法直接访问目标网段。
- 需要把目标的本地服务或内网服务拉到你能控制的服务端出口。

### 6.8.4 `listen_location=node` 何时选

`node` 表示监听端开在目标 Agent 所在机器。

适合这些场景：

- 你希望目标主机自己在本地或内网侧暴露一个端口。
- 会有同网段其他机器去访问这个监听口。
- 你打算把这台主机当作内网中的跳板，而不是把口暴露回服务端。

典型理解：

- Agent 自己在它所在机器开端口。
- 其他能到达这台 Agent 主机的人或主机，再通过这个端口转进目标地址。

典型例子：让目标内网其他机器通过该 Agent 访问一台数据库

```json
{
  "dev_id": "target-dev-id",
  "listen_location": "node",
  "listen_port": "15432",
  "forward_type": "forward",
  "target_address": "10.10.20.15:5432",
  "description": "内网DB中继"
}
```

效果：

- 同内网其他机器访问 `<目标Agent内网IP>:15432`
- 实际流量被转到 `10.10.20.15:5432`

什么时候不要选 `node`：

- 你自己根本访问不到这台 Agent 主机的监听口。
- 目标主机没有对你或其他使用方开放入站路径。
- 你真正需要的是“把服务拉回 RTunnelServer”，这时应该选 `server`。

### 6.8.5 `forward` 和 `socks5` 怎么选

选 `forward`：

- 你只需要访问一个固定服务。
- 目标明确，例如 `127.0.0.1:3389`、`10.0.0.5:445`、`127.0.0.1:3306`。
- 你想减少额外代理层的复杂度。

选 `socks5`：

- 你还不确定后面要访问哪些内网地址。
- 你要做浏览器代理、扫描、临时横向、代理链接入。
- 你希望把一个通用出网/代理能力挂起来，而不是单服务映射。

### 6.8.6 关于 SOCKS5 密码

当前代码里，若 `password` 不传，Agent 侧默认密码是 `Rtunnel`。

但上层说明文字里曾出现过 `RTunnel` 的写法。

因此最稳妥的做法是：

- 不要依赖默认值。
- 永远显式传入你想要的 `password`。

### 6.8.7 端口转发选择速查

| 目标 | 推荐监听位置 | 原因 |
| --- | --- | --- |
| 让你自己从服务端访问目标内网服务 | `server` | 你能到服务端，不能到目标内网 |
| 在服务端开 SOCKS5 供本地代理工具用 | `server` | 操作面集中在服务端 |
| 让目标内网其他机器访问某个转发口 | `node` | 监听口要暴露在目标内网侧 |
| 你不确定后续会访问哪些地址 | `server + socks5` | 通用性最好 |
| 你只想暴露一个明确端口 | `forward` | 简单直接 |

---

## 6.9 `disconnect_agent`

用途：强制让指定 Agent 下线。

适合：

- 清理异常连接。
- 在切换节点包、重置状态、终止会话前断开。

风险：

- 会立即失去当前控制通道。
- 后续必须等待其重新上线。

一般不应把它当作日常操作。

---

## 6.10 文件类 tools

这些 tool 组合起来就是远程文件管理能力。

推荐使用顺序：

1. `list_files`
2. `read_file`
3. `copy_path`
4. 再做 `write_file` / `rename_path` / `delete_path` / `unzip_path`

### `list_files`

先看目录现状，避免路径猜错。

### `read_file`

特点：

- 一定返回 `content_base64`
- 若恰好是 UTF-8 文本，还会附带 `text`

最适合：

- 读配置文件
- 读脚本
- 读小型文本资料
- 拉取二进制后交给上层解码

### `write_file`

用途：覆盖写入。

关键点：

- `encoding=text`
  传 UTF-8 文本。
- `encoding=base64`
  传原始字节的 Base64。

风险：

- 默认应视为覆盖原文件。
- 写错路径或编码，破坏会立刻发生。

### `upload_file`

用途：上传任意文件。

更适合：

- 二进制文件
- 压缩包
- 工具程序
- 非 UTF-8 文本

与 `write_file` 的区别：

- `write_file` 更像“写内容到路径”
- `upload_file` 更像“上传一个文件对象”

### `create_folder` / `create_file`

适合在正式上传、解压、部署前先准备路径。

### `zip_path`

适合：

- 批量归档目录
- 回传资料前先打包

### `unzip_path`

风险高于看起来：

- 可能一次性写入很多文件
- 可能覆盖既有目录内容

### `copy_path`

建议把它当作很多高危文件操作前的“快速备份”手段。

### `rename_path`

适合做：

- 原子式替换
- 版本切换
- 目录迁移

### `delete_path`

高危操作。

建议固定流程：

1. `list_files`
2. `read_file` 或 `copy_path`
3. 再执行删除

### `download_url_to_path`

用途：让目标 Agent 主动下载 URL 到指定路径。

只有在以下条件都满足时才推荐：

- 目标机器能出网
- 目标机器能访问该 URL
- 你信任下载来源

它与 `upload_file` 的差异：

- `upload_file` 是你把文件推过去
- `download_url_to_path` 是目标自己拉

如果目标不能出网，就不要用它。

---

## 6.11 `get_topology` / `get_device_notes` / `update_device_note`

这三类 tool 本身不直接打控制链路，但对自动化很有帮助。

### `get_topology`

适合：

- 做节点关系判断
- 给上层 UI/Agent 读图
- 分析当前跳板结构

注意：

- 它是服务端当前保存/缓存的拓扑，不等于实时扫描结果。

### `get_device_notes`

适合：

- 读取已有人工标注
- 在自动化流程开始前补上下文

### `update_device_note`

强烈建议在以下时机更新：

- 识别出主机角色后
- 发现关键路径后
- 确认这台主机是 DB / JumpServer / 域控 / Web / 文件服务器后

好的备注示例：

- `财务网段跳板，192.168.20.0/24 可达`
- `Windows Web 服务器，IIS，疑似业务入口`
- `DB 节点，本地 3306 开放，配置位于 D:\\app\\conf`

---

## 6.12 `list_plugins` / `run_plugin`

先 `list_plugins`，后 `run_plugin`，不要跳过第一步。

原因：

- 插件是否存在取决于当前服务端加载了什么。
- 插件的参数顺序是插件自己定义的，不是 MCP 统一定义的。

`run_plugin` 的风险高于普通工具：

- 插件可能执行命令
- 可能改文件
- 可能建连接
- 可能改系统配置

适合的使用方式：

1. `list_plugins`
2. 明确插件名
3. 明确参数顺序
4. 再 `run_plugin`

---

## 6.13 `add_package_node` / `switch_package_node`

这两个 tool 用于节点包/子节点相关操作，应视为高影响能力。

### `add_package_node`

实际作用：

- 先根据之前 `generate_client` 得到的 `code`
- 取出对应平台的客户端包
- 把包上传到指定 Agent

更稳妥的理解是“上传节点包”，不要默认理解成“一次调用就自动切换完成”。

### `switch_package_node`

实际作用：

- 让目标 Agent 切换到某个先前已经上传过的 `code`

推荐顺序：

1. `generate_client`
2. `add_package_node`
3. 确认影响
4. `switch_package_node`

风险：

- 会改变节点角色、连接方式或运行形态
- 切换后行为可能与当前会话预期不同

---

## 7. 常见操作配方

## 7.1 生成新客户端并等待上线

1. `get_client_info`
2. `generate_client`
3. 投放并运行客户端
4. 轮询 `list_agents`
5. 找到新 `dev_id`
6. 再进入控制阶段

## 7.2 暴露目标本地服务到服务端

目标：访问目标机器上的 `127.0.0.1:80`

调用：

```json
{
  "name": "configure_port_forward",
  "arguments": {
    "dev_id": "target-dev-id",
    "listen_location": "server",
    "listen_port": "8080",
    "forward_type": "forward",
    "target_address": "127.0.0.1:80",
    "description": "目标本地Web"
  }
}
```

## 7.3 在服务端开一个通用 SOCKS5

目标：给浏览器、代理链、扫描器使用

调用：

```json
{
  "name": "configure_port_forward",
  "arguments": {
    "dev_id": "target-dev-id",
    "listen_location": "server",
    "listen_port": "1080",
    "forward_type": "socks5",
    "password": "abc123"
  }
}
```

## 7.4 上传工具并执行

1. `create_folder`
2. `upload_file`
3. `list_files`
4. `exec_command`

如果是纯文本脚本，可以考虑 `write_file + exec_command`。

## 7.5 先备份再改配置

1. `read_file`
2. `copy_path`
3. `write_file`
4. 必要时 `read_file` 再验一次

---

## 8. 最容易踩坑的地方

### 8.1 没拿到 `dev_id` 就开始控制

所有 Agent 控制类 tool 都建立在 `dev_id` 上。

如果目标还没上线，先做的是：

- `generate_client`
- 投放运行
- `list_agents`

### 8.2 把 `target_address` 按服务端视角写

端口转发里，`target_address` 应优先按 Agent 视角理解。

最常见的正确写法是：

- `127.0.0.1:80`
- `127.0.0.1:3389`
- `10.x.x.x:445`

### 8.3 监听位置选错

如果你要“把服务拉回 RTunnelServer 供自己访问”，通常选 `server`。

如果你要“让目标内网里的人访问目标机器上的监听口”，通常选 `node`。

### 8.4 依赖默认 SOCKS5 密码

不要依赖。

显式传 `password`。

### 8.5 把 `write_file` 当追加写

当前应把它按覆盖写理解。

### 8.6 让 `download_url_to_path` 去下载一个目标根本访问不到的地址

这是远端主动下载，不是服务端代下。

### 8.7 认为 `add_agent_from_webio` 成功就等于目标已经上线

它只是“任务已下发”。

真正上线仍要看 `list_agents`。

### 8.8 认为 `list_client_links` 会跨服务重启保留

不要这样假设。

---

## 9. 内置 Prompt

除了 tools 之外，当前还有一个很实用的内置 prompt：

- `operator_workflow`

建议 MCP Client / 自动化 Agent 在接入时这样用：

1. `prompts/list`
2. `prompts/get`，读取 `operator_workflow`

这个 prompt 的作用是把 RTunnel 的标准操作顺序固化给上层 Agent：

- 先生成客户端
- 再等待上线
- 拿到 `dev_id` 后再控制
- 高危动作前再次确认目标、路径、端口和影响

如果你要给大模型 Agent 接 RTunnel，这一步很值得保留。

---

## 10. 对上层自动化系统的建议

如果你在写自己的 MCP Client、编排器或 AI Agent，建议固定遵守下面这些工程约束：

1. 每次高危操作前，重新读取并确认 `dev_id`。
2. 文件操作前，先 `list_files` 或 `read_file`。
3. 删除、覆盖、切换、断连前，先做备份或显式确认。
4. 自动化逻辑统一读 `structuredContent`，不要只读文本。
5. 所有网络动作都显式记录：
   - 监听位置
   - 监听端口
   - 目标地址
   - 密码
   - 备注说明
6. 生成客户端时，不要写死 `os/arch/crypto`，一律先 `get_client_info`。

按这个思路用，MCP 会比较稳定；反过来如果跳步骤、猜参数、依赖默认值，就很容易出现“工具返回成功，但链路实际不可用”的问题。
