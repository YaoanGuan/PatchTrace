package taskManager

import (
	"AIxVuln/dockerManager"
	"testing"
)

func TestProjectRuntimeAliasResolvesExecutionResources(t *testing.T) {
	p := NewProjectS()
	sm := &dockerManager.ServiceManager{}
	sb := &dockerManager.Sandbox{ContainerId: "sandbox-1"}
	ch := make(chan func(), 1)

	p.SetServiceManager("lab", sm)
	p.SetSandbox("lab", sb)
	p.SetGoroutineChan("lab", ch)
	p.SetRuntimeAlias("lab_binary_security", "lab")

	gotSM, err := p.GetServiceManager("lab_binary_security")
	if err != nil {
		t.Fatalf("GetServiceManager returned error: %v", err)
	}
	if gotSM != sm {
		t.Fatalf("service manager mismatch: got %p want %p", gotSM, sm)
	}

	gotSB, err := p.GetSandbox("lab_binary_security")
	if err != nil {
		t.Fatalf("GetSandbox returned error: %v", err)
	}
	if gotSB != sb {
		t.Fatalf("sandbox mismatch: got %p want %p", gotSB, sb)
	}

	if gotCh := p.GetGoroutineChan("lab_binary_security"); gotCh != ch {
		t.Fatalf("goroutine chan mismatch: got %p want %p", gotCh, ch)
	}
}

func TestProjectRuntimeAliasSupportsChains(t *testing.T) {
	p := NewProjectS()
	sm := &dockerManager.ServiceManager{}

	p.SetServiceManager("lab", sm)
	p.SetRuntimeAlias("group_a", "group_b")
	p.SetRuntimeAlias("group_b", "lab")

	gotSM, err := p.GetServiceManager("group_a")
	if err != nil {
		t.Fatalf("GetServiceManager returned error: %v", err)
	}
	if gotSM != sm {
		t.Fatalf("service manager mismatch through chain: got %p want %p", gotSM, sm)
	}
}
