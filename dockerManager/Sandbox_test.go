package dockerManager

import (
	"AIxVuln/misc"
	"reflect"
	"testing"
)

func TestExtractSandboxListenerPorts(t *testing.T) {
	portMap := []PortMapping{
		{HostPort: "41002", ContainerPort: "41002", Protocol: "tcp"},
		{HostPort: "41001", ContainerPort: "41001", Protocol: "tcp"},
		{HostPort: "41003", ContainerPort: "41003", Protocol: "tcp"},
		{HostPort: "9999", ContainerPort: "9999", Protocol: "udp"},
		{HostPort: "41002", ContainerPort: "41002", Protocol: "tcp"},
	}
	got := extractSandboxListenerPorts(portMap)
	want := []string{"41001", "41002", "41003"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected listener ports: got %#v want %#v", got, want)
	}
}

func TestBuildSandboxEnvMsgOmitsContainerPrivateIP(t *testing.T) {
	oldBaseURL := misc.GetRuntimeServerBaseURL()
	misc.SetRuntimeServerBaseURL("http://62.234.99.49:9999")
	defer misc.SetRuntimeServerBaseURL(oldBaseURL)

	msg := buildSandboxEnvMsg([]string{"41003", "41001", "41002"})
	if msg.Key != "SandboxListenerInfo" {
		t.Fatalf("unexpected key: %q", msg.Key)
	}
	content, ok := msg.Content.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected content type: %T", msg.Content)
	}
	if _, exists := content["ContainerIP"]; exists {
		t.Fatalf("container private IP must not be exposed: %+v", content)
	}
	if _, exists := content["ContainerId"]; exists {
		t.Fatalf("container id must not be exposed: %+v", content)
	}
	if got, _ := content["CallbackHostIP"].(string); got != "62.234.99.49" {
		t.Fatalf("unexpected callback host: %q", got)
	}
	ports, ok := content["ListenerPorts"].([]string)
	if !ok {
		t.Fatalf("unexpected listener ports type: %T", content["ListenerPorts"])
	}
	want := []string{"41001", "41002", "41003"}
	if !reflect.DeepEqual(ports, want) {
		t.Fatalf("unexpected listener ports: got %#v want %#v", ports, want)
	}
}
