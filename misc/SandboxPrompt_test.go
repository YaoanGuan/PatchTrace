package misc

import (
	"strings"
	"testing"
)

func TestRenderSandboxPromptTemplateReplacesHostAndPorts(t *testing.T) {
	raw := "host=<serverip> ports=xxxxx,xxxxx,xxxxx"
	got := RenderSandboxPromptTemplate(raw, "192.168.1.2", []string{"40002", "40001", "40003"})
	want := "host=192.168.1.2 ports=40001,40002,40003"
	if got != want {
		t.Fatalf("unexpected rendered prompt: got %q want %q", got, want)
	}
}

func TestRenderSandboxPromptTemplateFallsBackWhenInfoMissing(t *testing.T) {
	got := BuildSandboxListenerInstruction("host=<serverip> ports=xxxxx,xxxxx,xxxxx", "", []string{"40001"})
	if got != SandboxListenerFallbackInstruction {
		t.Fatalf("unexpected fallback prompt: %q", got)
	}
}

func TestSandboxListenerFallbackInstructionWarnsAgainstContainerIP(t *testing.T) {
	if !strings.Contains(SandboxListenerFallbackInstruction, "Never use the sandbox/container private IP") {
		t.Fatalf("fallback instruction must explicitly forbid sandbox private IP usage: %q", SandboxListenerFallbackInstruction)
	}
}

func TestRenderSandboxPromptTemplateDoesNotDiscardOtherTextWhenInfoMissing(t *testing.T) {
	raw := "prefix <serverip> ports xxxxx suffix"
	got := RenderSandboxPromptTemplate(raw, "", nil)
	if got != raw {
		t.Fatalf("unexpected raw preservation: got %q want %q", got, raw)
	}
}
