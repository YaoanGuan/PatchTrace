# fscan 参数整理

> 本文件根据当前目录中的本地导出参数页整理，并参考上游 `shadow1ng/fscan` 的 `Common/Flag.go` 补齐当前页面未展开的参数。
>
> 当前导出页停留在“主机扫描”视图，所以 `Web` 相关和部分高级参数在页面里没有完整展开；这类参数已单独标注为“源码补齐”。

## 1. 基础用法

```bash
./fscan_amd64 -h 192.168.1.10
./fscan_amd64 -h 192.168.1.0/24 -p 80,443,8080 -m ftp,ssh,mysql
./fscan_amd64 -u http://target.example -cookie "PHPSESSID=xxxx"
./fscan_amd64 -local
```

## 2. 扫描模式

| 模式 | 参数 | 用法 |
| --- | --- | --- |
| 主机扫描 | `-h` | 扫描主机、端口、服务、弱口令、漏洞等 |
| Web 扫描 | `-u` | 扫描单个 URL |
| Web 扫描文件 | `-uf` | 从文件批量读取 URL |
| 本地扫描 | `-local` | 扫描本机环境 |

示例：

```bash
./fscan_amd64 -h 192.168.1.1/24
./fscan_amd64 -u https://demo.test
./fscan_amd64 -uf urls.txt
./fscan_amd64 -local
```

## 3. 页面明确展示的参数

### 3.1 目标配置参数

| 参数 | 说明 | 示例 |
| --- | --- | --- |
| `-h` | 目标地址，支持 IP、域名、IP 段 | `-h 192.168.1.10` |
| `-hf` | 从文件读取主机列表 | `-hf hosts.txt` |
| `-eh` | 排除主机，逗号分隔 | `-eh 192.168.1.1,192.168.1.254` |
| `-ehf` | 从文件读取排除主机列表 | `-ehf exclude_hosts.txt` |
| `-p` | 指定扫描端口 | `-p 21,22,80,443,8080` |
| `-pf` | 从文件读取端口列表 | `-pf ports.txt` |
| `-ep` | 排除端口 | `-ep 22,3389` |

页面中的端口预设：

- 主要端口
- 服务端口
- 数据库
- Web 服务
- 全端口
- 自定义

### 3.2 扫描控制参数

| 参数 | 说明 | 示例 |
| --- | --- | --- |
| `-m` | 选择扫描插件，支持逗号分隔；页面支持全选、清空、搜索、分类选择 | `-m ftp,ssh,mysql` |
| `-t` | 扫描线程数，页面范围 `1-2000` | `-t 600` |
| `-time` | 单次连接超时，页面范围 `1-30` 秒 | `-time 3` |
| `-mt` | 模块线程数，页面范围 `1-100` | `-mt 10` |
| `-gt` | 全局超时，页面范围 `60-600` 秒 | `-gt 180` |
| `-retry` | 最大重试次数，页面范围 `1-10` | `-retry 3` |
| `-rate` | 发包速率，`0` 表示不限 | `-rate 0` |
| `-maxpkts` | 最大总发包数，`0` 表示不限 | `-maxpkts 0` |
| `-np` | 禁用存活探测，对所有目标直接扫描 | `-np` |
| `-ping` | 仅做存活探测，只输出存活主机，不进行端口扫描 | `-ping` |
| `-nobr` | 禁用所有暴力破解功能 | `-nobr` |
| `-noredis` | 禁用 Redis 未授权访问检测 | `-noredis` |

### 3.3 标准认证参数

| 参数 | 说明 | 示例 |
| --- | --- | --- |
| `-user` | 默认用户名 | `-user admin` |
| `-pwd` | 默认密码 | `-pwd admin@123` |
| `-usera` | 追加用户名，逗号分隔 | `-usera root,oracle` |
| `-pwda` | 追加密码，逗号分隔 | `-pwda 123456,admin123` |

### 3.4 字典文件路径

| 参数 | 说明 | 示例 |
| --- | --- | --- |
| `-userf` | 用户名字典文件 | `-userf users.txt` |
| `-pwdf` | 密码字典文件 | `-pwdf passwords.txt` |
| `-upf` | 用户名密码对文件，`用户名:密码` 格式 | `-upf creds.txt` |

### 3.5 Hash 认证参数

页面标注适用模块：`smb`、`smb2`、`rdp`、`ldap`、`mssql`

| 参数 | 说明 | 示例 |
| --- | --- | --- |
| `-hash` | 指定单个 Hash 值认证 | `-hash aad3b435b51404eeaad3b435b51404ee:31d6cfe0d16ae931b73c59d7e0c089c0` |
| `-hashf` | 从文件读取 Hash 字典 | `-hashf hashes.txt` |

### 3.6 高级认证参数

页面标注适用模块：`smb`、`smb2`、`ldap`、`mssql`、`ssh`

| 参数 | 说明 | 示例 |
| --- | --- | --- |
| `-domain` | SMB/域环境扫描使用的域名 | `-domain corp.local` |
| `-sshkey` | SSH 私钥文件路径 | `-sshkey ~/.ssh/id_rsa` |

### 3.7 输出与显示控制参数

| 参数 | 说明 | 示例 |
| --- | --- | --- |
| `-o` | 输出文件，页面默认 `result.txt` | `-o result.txt` |
| `-f` | 输出格式，页面支持 `txt` / `json` / `csv` | `-f json` |
| `-log` | 日志级别，页面选项为：调试、全部、信息、成功、错误 | `-log SUCCESS` |
| `-no` | 禁用结果保存，不写入文件 | `-no` |
| `-silent` | 静默模式，不输出 Banner 和提示信息 | `-silent` |
| `-nocolor` | 禁用彩色输出 | `-nocolor` |
| `-pg` | 进度条显示开关；页面对应“禁用进度条” | `-pg=false` |

说明：

- 页面用中文显示日志级别：`调试 / 全部 / 信息 / 成功 / 错误`
- 对应命令行常见写法可理解为：`DEBUG / ALL / INFO / SUCCESS / ERROR`

## 4. `-m` 插件列表

`-m` 支持手动输入、搜索、全选/清空，也支持逗号分隔的多模块组合。

### 4.1 网络服务

`ftp`、`ssh`、`telnet`、`findnet`、`netbios`、`smb`、`smb2`、`smbghost`、`smbinfo`、`ldap`、`smtp`、`snmp`、`rsync`、`rdp`、`vnc`

### 4.2 数据库服务

`mssql`、`oracle`、`mysql`、`postgresql`、`redis`、`memcached`、`mongodb`、`cassandra`、`neo4j`、`elasticsearch`

### 4.3 消息队列

`rabbitmq`、`kafka`、`activemq`

### 4.4 漏洞检测

`ms17010`

示例：

```bash
./fscan_amd64 -h 192.168.1.0/24 -m smb,smb2,smbghost,ms17010
./fscan_amd64 -h 10.10.10.0/24 -m mysql,redis,mongodb
```

## 5. 源码补齐但当前导出页未完整展开的参数

下面这些参数在上游 `Common/Flag.go` 中存在，但当前本地导出页没有全部展开中文说明，以下描述按参数命名和 fscan 用途补齐。

### 5.1 Web / HTTP 相关

| 参数 | 说明 | 示例 |
| --- | --- | --- |
| `-u` | 扫描单个 URL | `-u http://target` |
| `-uf` | 从文件读取 URL 列表 | `-uf urls.txt` |
| `-cookie` | 自定义 Cookie | `-cookie "token=abc"` |
| `-proxy` | HTTP 代理 | `-proxy http://127.0.0.1:8080` |
| `-socks5` | SOCKS5 代理 | `-socks5 127.0.0.1:1080` |
| `-wt` | Web 请求超时 | `-wt 10` |

### 5.2 POC / 漏洞检测相关

| 参数 | 说明 | 示例 |
| --- | --- | --- |
| `-nopoc` | 禁用 POC 检测 | `-nopoc` |
| `-pocpath` | 自定义 POC 路径 | `-pocpath ./pocs` |
| `-pocname` | 只跑指定 POC 名称 | `-pocname shiro` |
| `-full` | 开启更完整的 POC/完整检测模式 | `-full` |
| `-num` | POC 并发数量 | `-num 20` |
| `-dns` | 开启 DNSLog 相关验证 | `-dns` |
| `-fingerprint` | 指纹识别模式/增强指纹检测 | `-fingerprint` |

### 5.3 Redis 利用相关

| 参数 | 说明 | 示例 |
| --- | --- | --- |
| `-rf` | Redis 文件利用相关参数 | `-rf id_rsa.pub` |
| `-rs` | Redis Shell/命令执行相关参数 | `-rs "whoami"` |
| `-rwp` | Redis 写入路径 | `-rwp /root/.ssh/` |
| `-rwc` | Redis 写入内容 | `-rwc "ssh-rsa AAAA..."` |
| `-rwf` | Redis 写入内容文件 | `-rwf payload.txt` |

说明：这些参数属于高级利用选项，当前导出页只明确展示了 `-noredis`，其余写入/执行类参数为源码中的扩展项。

### 5.4 其它高级参数

| 参数 | 说明 | 示例 |
| --- | --- | --- |
| `-top` | 存活探测使用的 Top 端口数量 | `-top 10` |
| `-sp` | 显示扫描计划 | `-sp` |
| `-slow` | 慢速日志输出 | `-slow` |
| `-sc` | Shellcode/利用动作相关参数 | `-sc add` |
| `-lang` | 输出语言，源码默认 `zh` | `-lang zh` |
| `-api` | 自定义 API 地址 | `-api https://api.example.com` |
| `-secret` | API 密钥或 Secret | `-secret xxxxx` |

## 6. 常用组合示例

### 6.1 主机批量扫描

```bash
./fscan_amd64 -hf hosts.txt -p 21,22,80,443,445,3306,6379 -m ftp,ssh,smb,mysql,redis -o result.txt
```

### 6.2 只做存活探测

```bash
./fscan_amd64 -h 192.168.1.0/24 -ping
```

### 6.3 跳过存活探测直接全扫

```bash
./fscan_amd64 -h 192.168.1.0/24 -np -t 800
```

### 6.4 Web 目标检测

```bash
./fscan_amd64 -u https://target.example -cookie "SESSIONID=xxxx" -wt 10
```

### 6.5 指定字典爆破

```bash
./fscan_amd64 -h 10.10.10.10 -m ssh,ftp -userf users.txt -pwdf passwords.txt
```

## 7. 生成器当前页面能确认的默认显示

- 扫描模式默认停留在 `主机扫描`
- 页面展示的端口示例为：`21,22,80,443,3306,6379,8080,8443`
- 页面展示的扫描性能为：`600` 线程、`3` 秒单次超时、`180` 秒全局超时
- 输出文件默认显示为：`result.txt`
- 输出格式页面可选：`txt`、`json`、`csv`
- 日志级别页面当前选中：`成功`
