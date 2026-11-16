// Package dispatch implements the Cairn-style scheduler: it reads the
// provenance graph, decides which task a project needs next, selects a healthy
// worker for that task type, and owns all writes back to the graph protocol.
package dispatch

import (
	"fmt"
	"os"
	"strings"
	"time"

	yaml "github.com/goccy/go-yaml"
)

// Config is the validated form of dispatch.yaml.
//
// Required keys (LoadConfig rejects the file when any is absent or
// non-positive):
//
//	server                          — graph protocol base URL
//	runtime.max_workers             — total concurrent task cap
//	runtime.max_running_projects    — active project cap
//	runtime.max_project_workers     — per-project concurrent task cap
//	runtime.interval                — scheduler tick / heartbeat period
//	runtime.healthcheck_timeout     — worker health probe timeout
//	runtime.prompt_group            — prompt directory name
//	tasks.bootstrap.timeout
//	tasks.bootstrap.conclude_timeout
//	tasks.reason.timeout
//	tasks.explore.timeout
//	tasks.explore.conclude_timeout
type Config struct {
	ServerURL string

	MaxWorkers         int
	MaxRunningProjects int
	MaxProjectWorkers  int

	Interval           time.Duration
	HealthcheckTimeout time.Duration

	BootstrapTimeout         time.Duration
	BootstrapConcludeTimeout time.Duration
	ReasonTimeout            time.Duration
	ExploreTimeout           time.Duration
	ExploreConcludeTimeout   time.Duration

	PromptGroup string
}

// rawConfig mirrors dispatch.yaml. Pointer fields distinguish "key absent"
// from "key present with the zero value" so LoadConfig can name the missing
// key instead of silently accepting a zero timeout or limit.
type rawConfig struct {
	Server  string     `yaml:"server"`
	Runtime rawRuntime `yaml:"runtime"`
	Tasks   rawTasks   `yaml:"tasks"`
}

type rawRuntime struct {
	MaxWorkers         *int           `yaml:"max_workers"`
	MaxRunningProjects *int           `yaml:"max_running_projects"`
	MaxProjectWorkers  *int           `yaml:"max_project_workers"`
	Interval           *time.Duration `yaml:"interval"`
	HealthcheckTimeout *time.Duration `yaml:"healthcheck_timeout"`
	PromptGroup        string         `yaml:"prompt_group"`
}

type rawTasks struct {
	Bootstrap rawStage `yaml:"bootstrap"`
	Reason    rawStage `yaml:"reason"`
	Explore   rawStage `yaml:"explore"`
}

type rawStage struct {
	Timeout         *time.Duration `yaml:"timeout"`
	ConcludeTimeout *time.Duration `yaml:"conclude_timeout"`
}

// LoadConfig reads and validates the dispatch configuration at path.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("dispatch: read config %s: %w", path, err)
	}
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("dispatch: parse config %s: %w", path, err)
	}
	return buildConfig(raw)
}

func buildConfig(raw rawConfig) (Config, error) {
	serverURL := strings.TrimSpace(raw.Server)
	if serverURL == "" {
		return Config{}, configKeyError("server")
	}
	promptGroup := strings.TrimSpace(raw.Runtime.PromptGroup)
	if promptGroup == "" {
		return Config{}, configKeyError("runtime.prompt_group")
	}

	var cfg Config
	cfg.ServerURL = serverURL
	cfg.PromptGroup = promptGroup

	var err error
	if cfg.MaxWorkers, err = requiredPositiveInt("runtime.max_workers", raw.Runtime.MaxWorkers); err != nil {
		return Config{}, err
	}
	if cfg.MaxRunningProjects, err = requiredPositiveInt("runtime.max_running_projects", raw.Runtime.MaxRunningProjects); err != nil {
		return Config{}, err
	}
	if cfg.MaxProjectWorkers, err = requiredPositiveInt("runtime.max_project_workers", raw.Runtime.MaxProjectWorkers); err != nil {
		return Config{}, err
	}
	if cfg.Interval, err = requiredPositiveDuration("runtime.interval", raw.Runtime.Interval); err != nil {
		return Config{}, err
	}
	if cfg.HealthcheckTimeout, err = requiredPositiveDuration("runtime.healthcheck_timeout", raw.Runtime.HealthcheckTimeout); err != nil {
		return Config{}, err
	}
	if cfg.BootstrapTimeout, err = requiredPositiveDuration("tasks.bootstrap.timeout", raw.Tasks.Bootstrap.Timeout); err != nil {
		return Config{}, err
	}
	if cfg.BootstrapConcludeTimeout, err = requiredPositiveDuration("tasks.bootstrap.conclude_timeout", raw.Tasks.Bootstrap.ConcludeTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ReasonTimeout, err = requiredPositiveDuration("tasks.reason.timeout", raw.Tasks.Reason.Timeout); err != nil {
		return Config{}, err
	}
	if cfg.ExploreTimeout, err = requiredPositiveDuration("tasks.explore.timeout", raw.Tasks.Explore.Timeout); err != nil {
		return Config{}, err
	}
	if cfg.ExploreConcludeTimeout, err = requiredPositiveDuration("tasks.explore.conclude_timeout", raw.Tasks.Explore.ConcludeTimeout); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func configKeyError(key string) error {
	return fmt.Errorf("dispatch: config: %s is required", key)
}

func requiredPositiveInt(key string, v *int) (int, error) {
	if v == nil {
		return 0, configKeyError(key)
	}
	if *v <= 0 {
		return 0, fmt.Errorf("dispatch: config: %s must be positive", key)
	}
	return *v, nil
}

func requiredPositiveDuration(key string, v *time.Duration) (time.Duration, error) {
	if v == nil {
		return 0, configKeyError(key)
	}
	if *v <= 0 {
		return 0, fmt.Errorf("dispatch: config: %s must be positive", key)
	}
	return *v, nil
}
