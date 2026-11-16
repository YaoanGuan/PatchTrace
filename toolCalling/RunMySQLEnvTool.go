package toolCalling

import (
	"AIxVuln/misc"
	"AIxVuln/taskManager"
)

type RunMySQLEnvTool struct {
	task *taskManager.Task
}

func NewMySQLEnvTool(task *taskManager.Task) *RunMySQLEnvTool {
	return &RunMySQLEnvTool{task: task}
}

func (h *RunMySQLEnvTool) Name() string {
	return "RunMySQLEnvTool"
}
func (h *RunMySQLEnvTool) Description() string {
	return "Start a MySQL environment, choose the version, and return the container-related information."
}
func (h *RunMySQLEnvTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"version": map[string]interface{}{
				"type":        "string",
				"description": "Specify a MySQL version number, only one decimal point is accepted.(required)",
			},
			"rootPassword": map[string]interface{}{
				"type":        "string",
				"description": "Specify the root password for MySQL.(required)",
			},
			"initSqlDir": map[string]interface{}{
				"type":        "string",
				"description": "If you need to automatically execute some .sql files after MySQL starts running, place them in a clean directory under `/workdir/sourceCodeDir` (legacy `/sourceCodeDir` prefixes are also accepted) and then specify that directory. For example, if the .sql files are in `/workdir/sourceCodeDir/initSql`, then specify `/initSql`.(optional)",
			},
		},
	}
}

func (h *RunMySQLEnvTool) Execute(parameters map[string]interface{}) string {
	versionTemp := parameters["version"]
	if versionTemp == nil {
		return Fail("Missing 'version' parameter")
	}
	version := versionTemp.(string)
	rootPasswordTemp := parameters["rootPassword"]
	if rootPasswordTemp == nil {
		return Fail("Missing 'rootPassword' parameter")
	}
	rootPassword := rootPasswordTemp.(string)
	initSqlDirTemp := parameters["initSqlDir"]
	var initSqlDir string
	if initSqlDirTemp != nil {
		initSqlDir = initSqlDirTemp.(string)
	}
	initSqlDir = misc.NormalizeVisibleSourcePath(initSqlDir)
	out, err := h.task.GetSm().StartMysql(version, rootPassword, initSqlDir)
	if err != nil {
		return Fail(err.Error())
	}
	msg := make(map[string]string)
	msg["MySQLPort"] = "3306"
	msg["rootPassword"] = rootPassword
	msg["version"] = version
	msg["ContainerID"] = out.ContainerID
	msg["ContainerIP"] = out.IPAddress
	h.task.AddKeyMessage("RunMySQLEnvTool", msg, false)
	return Success(out)
}
