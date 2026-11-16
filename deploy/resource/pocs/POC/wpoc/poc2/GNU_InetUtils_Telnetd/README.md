# GNU InetUtils Telnetd 漏洞 POC

## 系统简介

GNU InetUtils 是 GNU 项目提供的一套基础网络工具，包含 telnet、telnetd、ftp、ftpd 等组件。

---

## CVE-2023-40303 — 权限提升（未检查返回值）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2023-40303 |
| CVSS | 7.8 (HIGH) |
| 漏洞类型 | 权限提升（CWE-252: Unchecked Return Value） |
| 攻击向量 | 本地（Local） |
| 影响版本 | GNU inetutils 通过 2.4 版（含） |
| 修复版本 | 2.5 |
| 影响组件 | ftpd, rcp, rlogin, rsh, rshd, uucpd |

### 漏洞原理

`set*id()` 系列系统调用（`setuid`, `setgid`, `seteuid` 等）的返回值未被检查。当进程尝试降权（drop privileges）时，如果 `set*id()` 调用失败，程序不会终止或处理错误，而是继续以高权限运行。

攻击者可通过触发 `set*id()` 失败的条件，维持意外的高权限访问。

### 利用条件

- 本地用户权限（低权限进程）
- 目标系统使用 GNU inetutils <= 2.4

### 概念验证

由于这是一个本地权限提升漏洞，POC 需要本地代码执行权限：

```bash
# 检查是否安装易受攻击版本
telnetd --version | grep inetutils

# 验证版本
dpkg -l inetutils-telnetd 2>/dev/null || rpm -qi inetutils 2>/dev/null
```

---

## CVE-2026-32746 / GNU InetUtils Telnetd — LINEMODE SLC 预认证 RCE

> **注意**: 这是 2026 年初发现的新漏洞，涉及 32 年历史的 Bug。

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2026-32746（新） |
| 漏洞类型 | 预认证远程代码执行（Pre-Auth RCE） |
| 攻击向量 | 远程网络（无需认证） |
| 根本原因 | LINEMODE SLC 处理器中的缓冲区溢出（Buffer Overflow） |
| 历史 | 该 Bug 存在超过 32 年 |

### 漏洞原理

Telnetd 的 LINEMODE SLC（Special Line Characters）处理程序中存在缓冲区溢出漏洞。攻击者在 Telnet 握手阶段发送精心构造的 LINEMODE SLC 数据，无需认证即可触发溢出，实现远程代码执行。

### 影响范围

- GNU InetUtils telnetd（多个版本）
- 工业控制系统（ICS）和 OT 系统中使用 telnetd 的设备
- 嵌入式设备上的 telnet 服务

### 参考资料

- [NVD CVE-2023-40303](https://nvd.nist.gov/vuln/detail/CVE-2023-40303)
- [WatchTowr Labs 分析（CVE-2026-32746）](https://labs.watchtowr.com/a-32-year-old-bug-walks-into-a-telnet-server-gnu-inetutils-telnetd-cve-2026-32746/)
- [Dream Security 公告](https://dreamgroup.com/vulnerability-advisory-pre-auth-remote-code-execution-via-buffer-overflow-in-telnetd-linemode-slc-handler/)
- [Industrial Cyber 报道](https://industrialcyber.co/threats-attacks/dream-security-flags-critical-rce-vulnerability-in-gnu-inetutils-telnetd-exposing-ics-and-ot-systems/)
- [runZero 博客](https://www.runzero.com/blog/telnetd-rootf/)
- [SC Media 报道](https://www.scworld.com/brief/widespread-gnu-inetutils-telnetd-compromise-likely-amid-poc-exploit-release)
