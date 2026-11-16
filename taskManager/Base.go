package taskManager

import (
	"AIxVuln/dockerManager"
	"fmt"
	"strings"
)

var projects *ProjectS

type ContainerInfo struct {
	Type        string   `json:"type"` //Create OR Remove
	ContainerId string   `json:"containerId"`
	ContainerIP string   `json:"containerIP"`
	Image       string   `json:"image"`
	WebPort     []string `json:"webPort"`
}

type ProjectInfo struct {
	ProjectID           string                 `json:"project_id,omitempty"`
	ProjectUUID         string                 `json:"project_uuid,omitempty"`
	ProjectName         string                 `json:"projectName"`
	WorkDir             string                 `json:"work_dir,omitempty"`
	SourceCodeDir       string                 `json:"source_code_dir"`
	SkipProjectOverview bool                   `json:"skip_project_overview,omitempty"`
	TaskContent         string                 `json:"taskContent"`
	SandboxID           string                 `json:"sandbox_container_id,omitempty"`
	StartTime           string                 `json:"start_time"`
	EndTime             string                 `json:"end_time"`
	LabStatus           string                 `json:"lab_status,omitempty"`
	LabBrain            LabBrainSnapshot       `json:"lab_brain,omitempty"`
	GroupStates         map[string]GroupState  `json:"group_states,omitempty"`
	ContainerList       []ContainerInfo        `json:"containerList"`
	VulnList            []Vuln                 `json:"vuln_list"`
	ExploitIdeas        []*ExploitIdea         `json:"exploit_ideas,omitempty"`
	ExploitChains       []*ExploitChain        `json:"exploit_chains,omitempty"`
	TokenUsage          TokenUsageSnapshot     `json:"token_usage,omitempty"`
	EventList           []string               `json:"event_list"`
	EnvInfo             map[string]interface{} `json:"envInfo"`
	ProjectDir          string                 `json:"projectDir"`
	ReportList          map[string]string      `json:"report_list"`
	LabReportCatalog    []ReportMeta           `json:"lab_report_catalog,omitempty"`
}

type LabBrainSnapshot struct {
	Status             string   `json:"status,omitempty"`
	LastDispatch       string   `json:"last_dispatch,omitempty"`
	LastDispatchGroups []string `json:"last_dispatch_groups,omitempty"`
	LastDispatchReason string   `json:"last_dispatch_reason,omitempty"`
	ProjectOverview    string   `json:"project_overview,omitempty"`
}

type GroupState struct {
	GroupID       string                 `json:"group_id"`
	GroupKind     string                 `json:"group_kind,omitempty"`
	Status        string                 `json:"status,omitempty"`
	BrainFinished bool                   `json:"brain_finished,omitempty"`
	LastTask      string                 `json:"last_task,omitempty"`
	LastSummary   string                 `json:"last_summary,omitempty"`
	ContainerList []ContainerInfo        `json:"container_list,omitempty"`
	VulnList      []Vuln                 `json:"vuln_list,omitempty"`
	ExploitIdeas  []*ExploitIdea         `json:"exploit_ideas,omitempty"`
	ExploitChains []*ExploitChain        `json:"exploit_chains,omitempty"`
	TokenUsage    TokenUsageSnapshot     `json:"token_usage,omitempty"`
	EventList     []string               `json:"event_list,omitempty"`
	EnvInfo       map[string]interface{} `json:"env_info,omitempty"`
	ReportCatalog []ReportMeta           `json:"report_catalog,omitempty"`
}

type AgentTokenUsageSnapshot struct {
	Label            string `json:"label"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	TotalTokens      int64  `json:"total_tokens"`
}

type TokenUsageSnapshot struct {
	PromptTokens     int64                     `json:"prompt_tokens"`
	CompletionTokens int64                     `json:"completion_tokens"`
	TotalTokens      int64                     `json:"total_tokens"`
	Agents           []AgentTokenUsageSnapshot `json:"agents,omitempty"`
}

type ProjectS struct {
	sandboxList    map[string]*dockerManager.Sandbox
	serviceManager map[string]*dockerManager.ServiceManager
	dockerManager  map[string]*dockerManager.DockerManager
	goroutineChan  map[string]chan func()
	runtimeAlias   map[string]string
}

func NewProjectS() *ProjectS {
	return &ProjectS{
		sandboxList:    make(map[string]*dockerManager.Sandbox),
		serviceManager: make(map[string]*dockerManager.ServiceManager),
		dockerManager:  make(map[string]*dockerManager.DockerManager),
		goroutineChan:  make(map[string]chan func()),
		runtimeAlias:   make(map[string]string),
	}
}

func (p *ProjectS) resolveRuntimeProject(projectName string) string {
	current := strings.TrimSpace(projectName)
	if current == "" {
		return projectName
	}
	seen := make(map[string]struct{})
	for {
		next, ok := p.runtimeAlias[current]
		if !ok {
			return current
		}
		next = strings.TrimSpace(next)
		if next == "" || next == current {
			return current
		}
		if _, exists := seen[current]; exists {
			return current
		}
		seen[current] = struct{}{}
		current = next
	}
}

func (p *ProjectS) SetRuntimeAlias(projectName string, runtimeProject string) {
	projectName = strings.TrimSpace(projectName)
	runtimeProject = strings.TrimSpace(runtimeProject)
	if projectName == "" {
		return
	}
	if runtimeProject == "" || runtimeProject == projectName {
		delete(p.runtimeAlias, projectName)
		return
	}
	p.runtimeAlias[projectName] = runtimeProject
}

func (p *ProjectS) SetGoroutineChan(projectName string, fc chan func()) {
	p.goroutineChan[projectName] = fc
}

func (p *ProjectS) SetSandbox(projectName string, sandbox *dockerManager.Sandbox) {
	p.sandboxList[projectName] = sandbox
}
func (p *ProjectS) GetSandbox(projectName string) (*dockerManager.Sandbox, error) {
	s, ok := p.sandboxList[p.resolveRuntimeProject(projectName)]
	if !ok {
		return nil, fmt.Errorf("project %s not found", projectName)
	}
	return s, nil
}

func (p *ProjectS) RemoveSandbox(projectName string) {
	delete(p.sandboxList, projectName)
}

func (p *ProjectS) SetServiceManager(projectName string, sm *dockerManager.ServiceManager) {
	p.serviceManager[projectName] = sm
}
func (p *ProjectS) GetServiceManager(projectName string) (*dockerManager.ServiceManager, error) {
	s, ok := p.serviceManager[p.resolveRuntimeProject(projectName)]
	if !ok {
		return nil, fmt.Errorf("project %s not found", projectName)
	}
	return s, nil
}

func (p *ProjectS) SetDockerManager(projectName string, dm *dockerManager.DockerManager) {
	p.dockerManager[projectName] = dm
}
func (p *ProjectS) GetDockerManager(projectName string) (*dockerManager.DockerManager, error) {
	s, ok := p.dockerManager[projectName]
	if !ok {
		return nil, fmt.Errorf("project %s not found", projectName)
	}
	return s, nil
}

func init() {
	projects = NewProjectS()
}

func SetSandbox(projectName string, sandbox *dockerManager.Sandbox) {
	projects.sandboxList[projectName] = sandbox
}
func SetServiceManager(projectName string, sm *dockerManager.ServiceManager) {
	projects.serviceManager[projectName] = sm
}
func SetDockerManager(projectName string, dm *dockerManager.DockerManager) {
	projects.dockerManager[projectName] = dm
}
func GetSandbox(projectName string) (*dockerManager.Sandbox, error) {
	return projects.GetSandbox(projectName)
}
func RemoveSandbox(projectName string) {
	projects.RemoveSandbox(projectName)
}
func GetServiceManager(projectName string) (*dockerManager.ServiceManager, error) {
	return projects.GetServiceManager(projectName)
}
func GetDockerManager(projectName string) (*dockerManager.DockerManager, error) {
	return projects.GetDockerManager(projectName)
}
func SetRuntimeAlias(projectName string, runtimeProject string) {
	projects.SetRuntimeAlias(projectName, runtimeProject)
}
func ResolveRuntimeProjectName(projectName string) string {
	return strings.TrimSpace(projects.resolveRuntimeProject(projectName))
}
func SetGoroutineChan(projectName string, fc chan func()) {
	projects.goroutineChan[projectName] = fc
}
func (p *ProjectS) GetGoroutineChan(projectName string) chan func() {
	return p.goroutineChan[p.resolveRuntimeProject(projectName)]
}
func GetGoroutineChan(projectName string) chan func() {
	return projects.GetGoroutineChan(projectName)
}
