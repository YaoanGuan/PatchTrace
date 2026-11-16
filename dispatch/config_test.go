package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const validConfig = `server: http://127.0.0.1:9999
runtime:
  max_workers: 4
  max_running_projects: 2
  max_project_workers: 3
  interval: 5s
  healthcheck_timeout: 30s
  prompt_group: default
tasks:
  bootstrap:
    timeout: 20m
    conclude_timeout: 5m
  reason:
    timeout: 10m
  explore:
    timeout: 15m
    conclude_timeout: 5m
`

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dispatch.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadConfigParsesRequiredKeys(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, validConfig))
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.ServerURL != "http://127.0.0.1:9999" {
		t.Fatalf("unexpected server: %q", cfg.ServerURL)
	}
	if cfg.MaxWorkers != 4 || cfg.MaxRunningProjects != 2 || cfg.MaxProjectWorkers != 3 {
		t.Fatalf("unexpected concurrency limits: %+v", cfg)
	}
	if cfg.Interval != 5*time.Second {
		t.Fatalf("unexpected interval: %v", cfg.Interval)
	}
	if cfg.HealthcheckTimeout != 30*time.Second {
		t.Fatalf("unexpected healthcheck timeout: %v", cfg.HealthcheckTimeout)
	}
	if cfg.PromptGroup != "default" {
		t.Fatalf("unexpected prompt group: %q", cfg.PromptGroup)
	}
	if cfg.BootstrapTimeout != 20*time.Minute || cfg.BootstrapConcludeTimeout != 5*time.Minute {
		t.Fatalf("unexpected bootstrap timeouts: %v / %v", cfg.BootstrapTimeout, cfg.BootstrapConcludeTimeout)
	}
	if cfg.ReasonTimeout != 10*time.Minute {
		t.Fatalf("unexpected reason timeout: %v", cfg.ReasonTimeout)
	}
	if cfg.ExploreTimeout != 15*time.Minute || cfg.ExploreConcludeTimeout != 5*time.Minute {
		t.Fatalf("unexpected explore timeouts: %v / %v", cfg.ExploreTimeout, cfg.ExploreConcludeTimeout)
	}
}

func TestConfigRequiresTimeouts(t *testing.T) {
	body := strings.Replace(validConfig,
		"  explore:\n    timeout: 15m\n    conclude_timeout: 5m\n",
		"  explore:\n    conclude_timeout: 5m\n", 1)
	if body == validConfig {
		t.Fatal("test fixture did not remove explore.timeout")
	}

	_, err := LoadConfig(writeConfig(t, body))
	if err == nil {
		t.Fatal("expected missing explore.timeout to be rejected")
	}
	if !strings.Contains(err.Error(), "tasks.explore.timeout") {
		t.Fatalf("expected error to name the missing key, got: %v", err)
	}
}

func TestConfigRequiresServerAndRuntimeKeys(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(string) string
		key    string
	}{
		{
			name: "missing server",
			mutate: func(s string) string {
				return strings.Replace(s, "server: http://127.0.0.1:9999\n", "", 1)
			},
			key: "server",
		},
		{
			name: "missing interval",
			mutate: func(s string) string {
				return strings.Replace(s, "  interval: 5s\n", "", 1)
			},
			key: "runtime.interval",
		},
		{
			name: "missing max workers",
			mutate: func(s string) string {
				return strings.Replace(s, "  max_workers: 4\n", "", 1)
			},
			key: "runtime.max_workers",
		},
		{
			name: "missing bootstrap timeout",
			mutate: func(s string) string {
				return strings.Replace(s, "    timeout: 20m\n    conclude_timeout: 5m\n", "    conclude_timeout: 5m\n", 1)
			},
			key: "tasks.bootstrap.timeout",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.mutate(validConfig)
			if body == validConfig {
				t.Fatal("test fixture mutation was a no-op")
			}
			_, err := LoadConfig(writeConfig(t, body))
			if err == nil {
				t.Fatalf("expected missing %s to be rejected", tc.key)
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("expected error to name %s, got: %v", tc.key, err)
			}
		})
	}
}

func TestConfigRejectsNonPositiveValues(t *testing.T) {
	body := strings.Replace(validConfig, "  interval: 5s\n", "  interval: 0s\n", 1)
	_, err := LoadConfig(writeConfig(t, body))
	if err == nil {
		t.Fatal("expected a zero interval to be rejected")
	}
	if !strings.Contains(err.Error(), "runtime.interval") {
		t.Fatalf("expected error to name runtime.interval, got: %v", err)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "absent.yaml"))
	if err == nil {
		t.Fatal("expected a missing config file to be rejected")
	}
}
