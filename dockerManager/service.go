package dockerManager

import (
	"AIxVuln/misc"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ServiceManager struct {
	dm             *DockerManager
	workDirPath    string
	sourceCodePath string
}

func NewServiceManager(workDirPath string, sourceCodePath string, dm *DockerManager) *ServiceManager {
	if !strings.HasPrefix(workDirPath, "/") {
		absWorkDirPath, err := filepath.Abs(workDirPath)
		if err != nil {
			misc.PanicWithStack("计算工作目录绝对路径失败: %v", err)
		}
		workDirPath = absWorkDirPath
	}
	if !strings.HasPrefix(sourceCodePath, "/") {
		absSourceCodePath, err := filepath.Abs(sourceCodePath)
		if err != nil {
			misc.PanicWithStack("计算源码目录绝对路径失败: %v", err)
		}
		sourceCodePath = absSourceCodePath
	}
	return &ServiceManager{dm: dm, workDirPath: workDirPath, sourceCodePath: sourceCodePath}
}

func (sm *ServiceManager) GetDockerManager() *DockerManager {
	return sm.dm
}
func (sm *ServiceManager) GetSourceCodePath() string {
	return sm.sourceCodePath
}

func (sm *ServiceManager) GetWorkDirPath() string {
	return sm.workDirPath
}

func (sm *ServiceManager) StartPhpEnv(version string, runPort string) (*RunResult, error) {
	imageName := "devwithlando/php:"
	v, _ := strconv.ParseFloat(version, 2)
	if v < 7 {
		imageName += version + "-apache-2"
	} else {
		imageName += version + "-apache-6"
	}
	absPath, err := filepath.Abs(sm.workDirPath)
	if err != nil {
		return nil, err
	}
	mounts := make(map[string]string)
	mounts[absPath] = misc.SandboxWorkDir
	r, err := sm.dm.Run(imageName, []string{"sh", "-lc", "cp -r " + misc.SandboxSourceDir + " /var/www/html && chown www-data:www-data -R /var/www/html && chmod 777 -R /var/www/html && rm -f /var/log/apache2/* && a2enmod rewrite && apache2ctl start && tail -f /dev/null"}, 600, SetVolumes(mounts), SetPort("", runPort), SetWorkingDir(misc.SandboxSourceDir))
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (sm *ServiceManager) StartJavaEnv(runPort string) (*RunResult, error) {
	imageName := "java_env"
	absPath, err := filepath.Abs(sm.workDirPath)
	if err != nil {
		return nil, err
	}
	absPath1, err := filepath.Abs("./data/.m2")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absPath1, 0755); err != nil {
		return nil, fmt.Errorf("创建 .m2 目录失败: %w", err)
	}
	mountMap := make(map[string]string)
	mountMap[absPath] = misc.SandboxWorkDir
	mountMap[absPath1] = "/root/.m2"
	r, err := sm.dm.Run(imageName, []string{"tail", "-f", "/dev/null"}, 600, SetVolumes(mountMap), SetPort("", runPort), SetWorkingDir(misc.SandboxSourceDir))
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (sm *ServiceManager) StartPythonEnv(runPort string, version string) (*RunResult, error) {
	imageName := "python:" + version + "-slim"
	absPath, err := filepath.Abs(sm.workDirPath)
	if err != nil {
		return nil, err
	}
	r, err := sm.dm.Run(imageName, []string{"tail", "-f", "/dev/null"}, 600, SetVolume(absPath, misc.SandboxWorkDir), SetPort("", runPort), SetWorkingDir(misc.SandboxSourceDir))
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (sm *ServiceManager) StartGolangEnv(runPort string, version string) (*RunResult, error) {
	imageName := "cimg/go:" + version
	absPath, err := filepath.Abs(sm.workDirPath)
	if err != nil {
		return nil, err
	}
	r, err := sm.dm.Run(imageName, []string{"sh", "-c", "go env -w GOPROXY=https://goproxy.cn,direct && tail -f /dev/null"}, 600, SetVolume(absPath, misc.SandboxWorkDir), SetPort("", runPort), SetWorkingDir(misc.SandboxSourceDir))
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (sm *ServiceManager) StartNodeEnv(runPort string, version string) (*RunResult, error) {
	imageName := "node:" + version
	absPath, err := filepath.Abs(sm.workDirPath)
	absPath1, err := filepath.Abs("./data/.npm")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absPath1, 0755); err != nil {
		return nil, fmt.Errorf("创建 .npm 目录失败: %w", err)
	}
	mountMap := make(map[string]string)
	mountMap[absPath] = misc.SandboxWorkDir
	mountMap[absPath1] = "/root/.npm"
	r, err := sm.dm.Run(imageName, []string{"tail", "-f", "/dev/null"}, 600, SetVolumes(mountMap), SetPort("", runPort), SetWorkingDir(misc.SandboxSourceDir))
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (sm *ServiceManager) StartMysql(version string, rootPassword string, initSqlDir string) (*RunResult, error) {
	imageName := "mysql:" + version
	if rootPassword == "" {
		rootPassword = "root"
	}
	var absPath string
	var r *RunResult
	var err error
	if len(initSqlDir) > 0 {
		absPath, err = filepath.Abs(sm.sourceCodePath + "/" + initSqlDir)
		if err != nil {
			return nil, err
		}
		r, err = sm.dm.Run(imageName, []string{}, 600, SetVolume(absPath, "/docker-entrypoint-initdb.d/"), SetEnv("MYSQL_ROOT_PASSWORD", rootPassword))
	} else {
		r, err = sm.dm.Run(imageName, []string{}, 600, SetEnv("MYSQL_ROOT_PASSWORD", rootPassword))
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (sm *ServiceManager) StartRedis(version string, rootPassword string) (*RunResult, error) {
	imageName := "redis:" + version
	if rootPassword == "" {
		return nil, fmt.Errorf("root password is empty")
	}
	var r *RunResult
	var err error
	if len(rootPassword) > 0 {
		r, err = sm.dm.Run(imageName, []string{"redis-server", "--bind", "0.0.0.0", "--requirepass", rootPassword}, 600)
	} else {
		r, err = sm.dm.Run(imageName, []string{"redis-server", "--bind", "0.0.0.0"}, 600)
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (sm *ServiceManager) StartDockerEnv(image string, command []string, timeout int16, env map[string]string, webPort string) (*RunResult, error) {
	var err error
	var out *RunResult

	if webPort == "" {
		out, err = sm.dm.Run(image, command, timeout, SetEnvs(env), SetVolume(sm.workDirPath, misc.SandboxWorkDir), SetWorkingDir(misc.SandboxSourceDir))
	} else {
		out, err = sm.dm.Run(image, command, timeout, SetEnvs(env), SetVolume(sm.workDirPath, misc.SandboxWorkDir), SetPort("", webPort), SetWorkingDir(misc.SandboxSourceDir))
	}
	return out, err
}
