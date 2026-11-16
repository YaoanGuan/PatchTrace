package misc

import (
	"bufio"
	"strings"
	"testing"
)

func TestSelectStartupIPByIndex(t *testing.T) {
	choices := []LocalIPChoice{
		{Interface: "en0", IP: "192.168.1.5"},
		{Interface: "utun3", IP: "10.0.0.8"},
	}
	ip, ok := selectStartupIP(bufio.NewReader(strings.NewReader("")), "2", choices)
	if !ok {
		t.Fatal("expected numeric selection to succeed")
	}
	if ip != "10.0.0.8" {
		t.Fatalf("expected selected ip to match choice, got %q", ip)
	}
}

func TestSelectStartupIPByDirectInput(t *testing.T) {
	ip, ok := selectStartupIP(bufio.NewReader(strings.NewReader("")), "172.16.0.9", nil)
	if !ok {
		t.Fatal("expected direct ip input to succeed")
	}
	if ip != "172.16.0.9" {
		t.Fatalf("unexpected direct ip: %q", ip)
	}
}

func TestSelectStartupIPByCustomInput(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("192.168.50.10\n"))
	ip, ok := selectStartupIP(reader, "0", []LocalIPChoice{{Interface: "en0", IP: "192.168.1.5"}})
	if !ok {
		t.Fatal("expected custom ip flow to succeed")
	}
	if ip != "192.168.50.10" {
		t.Fatalf("unexpected custom ip: %q", ip)
	}
}

func TestShouldBypassStartupIPPromptDisabled(t *testing.T) {
	if shouldBypassStartupIPPrompt() {
		t.Fatal("startup IP prompt bypass should stay disabled in web-only mode")
	}
}
