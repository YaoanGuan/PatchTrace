package dockerManager

import (
	"AIxVuln/llm"
	"AIxVuln/misc"
	"fmt"
	"net"
	"strings"
)

type Sandbox struct {
	ContainerId   string
	ContainerIp   string
	ListenerPorts []string
	dm            *DockerManager
	WorkDir       string
	SourceCodeDir string
	sandboxEnvMsg llm.EnvMessageX
}

const sandboxListenerPortCount = 3

func buildSandboxEnvMsg(listenerPorts []string) llm.EnvMessageX {
	return llm.EnvMessageX{
		Key: "SandboxListenerInfo",
		Content: map[string]interface{}{
			"CallbackHostIP": misc.GetRuntimeServerHost(),
			"ListenerPorts":  misc.NormalizeSandboxListenerPorts(listenerPorts),
			"Note":           "Use CallbackHostIP with one assigned ListenerPort when you need a sandbox listener. Never use sandbox/container private IPs as reverse-shell callback targets.",
		},
		AppendEnv: false,
	}
}

func extractSandboxListenerPorts(portMap []PortMapping) []string {
	ports := make([]string, 0, len(portMap))
	for _, item := range portMap {
		if strings.TrimSpace(item.Protocol) != "" && !strings.EqualFold(item.Protocol, "tcp") {
			continue
		}
		if strings.TrimSpace(item.HostPort) == "" {
			continue
		}
		ports = append(ports, item.HostPort)
	}
	return misc.NormalizeSandboxListenerPorts(ports)
}

func reserveSandboxListenerPorts(count int) ([]string, error) {
	if count <= 0 {
		return nil, fmt.Errorf("invalid listener port count: %d", count)
	}
	listeners := make([]net.Listener, 0, count)
	ports := make([]string, 0, count)
	cleanup := func() {
		for _, l := range listeners {
			_ = l.Close()
		}
	}
	for i := 0; i < count; i++ {
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			cleanup()
			return nil, err
		}
		tcpAddr, ok := l.Addr().(*net.TCPAddr)
		if !ok || tcpAddr.Port <= 0 {
			_ = l.Close()
			cleanup()
			return nil, fmt.Errorf("invalid listener addr: %v", l.Addr())
		}
		listeners = append(listeners, l)
		ports = append(ports, fmt.Sprintf("%d", tcpAddr.Port))
	}
	cleanup()
	return misc.NormalizeSandboxListenerPorts(ports), nil
}

func isPortConflictError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "port is already allocated") ||
		strings.Contains(msg, "address already in use") ||
		strings.Contains(msg, "bind for 0.0.0.0:")
}

func NewSandboxFromExisting(dm *DockerManager, containerID, containerIP, workDir string, sourceCodeDir string, listenerPorts []string) *Sandbox {
	s := &Sandbox{
		ContainerId:   containerID,
		ContainerIp:   containerIP,
		ListenerPorts: misc.NormalizeSandboxListenerPorts(listenerPorts),
		dm:            dm,
		WorkDir:       workDir,
		SourceCodeDir: sourceCodeDir,
	}
	s.sandboxEnvMsg = buildSandboxEnvMsg(s.ListenerPorts)
	return s
}

func NewSandbox(dm *DockerManager, workDir string, sourceCodeDir string) (*Sandbox, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		listenerPorts, err := reserveSandboxListenerPorts(sandboxListenerPortCount)
		if err != nil {
			return nil, err
		}
		portBindings := make(map[string]string, len(listenerPorts))
		for _, port := range listenerPorts {
			portBindings[port] = port
		}
		r, err := dm.Run("aisandbox", nil, 10, SetVolume(workDir, misc.SandboxWorkDir), SetWorkingDir(misc.SandboxWorkDir), SetPorts(portBindings))
		if err != nil {
			lastErr = err
			if isPortConflictError(err) {
				continue
			}
			return nil, err
		}
		actualPorts := extractSandboxListenerPorts(r.PortMap)
		if len(actualPorts) == 0 {
			actualPorts = listenerPorts
		}
		s := &Sandbox{
			ContainerId:   r.ContainerID,
			ContainerIp:   r.IPAddress,
			ListenerPorts: actualPorts,
			dm:            dm,
			WorkDir:       workDir,
			SourceCodeDir: sourceCodeDir,
		}
		s.sandboxEnvMsg = buildSandboxEnvMsg(s.ListenerPorts)
		return s, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failed to allocate sandbox listener ports")
	}
	return nil, lastErr
}

func (s *Sandbox) GetSandboxEnvMsg() *llm.EnvMessageX {
	return &s.sandboxEnvMsg
}

func (s *Sandbox) GetListenerPorts() []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s.ListenerPorts...)
}

func (s *Sandbox) RunCommand(command []string, timeOutSecond int16) (string, error) {
	r, err := s.dm.DockerExec(s.ContainerId, command, timeOutSecond)
	if err != nil {
		return "", err
	}
	return r, nil
}
