package taskManager

type TaskActor struct {
	AgentToolName   string `json:"agent_tool_name,omitempty"`
	AgentName       string `json:"agent_name,omitempty"`
	DigitalHumanID  string `json:"digital_human_id,omitempty"`
	PersonaName     string `json:"persona_name,omitempty"`
	AvatarFile      string `json:"avatar_file,omitempty"`
	GroupID         string `json:"group_id,omitempty"`
	GroupName       string `json:"group_name,omitempty"`
	RequesterSource string `json:"requester_source,omitempty"`
}

const (
	DecisionApprovalKindHostCommand  = "host_command"
	DecisionApprovalKindTargetAction = "target_action"
	DecisionApprovalKindAbandonIdea  = "abandon_idea"
)

type DecisionApprovalRequest struct {
	Kind             string `json:"kind"`
	Title            string `json:"title"`
	Target           string `json:"target,omitempty"`
	Action           string `json:"action"`
	Reason           string `json:"reason"`
	Impact           string `json:"impact,omitempty"`
	SaferAlternative string `json:"safer_alternative,omitempty"`
}

type DecisionApprovalResult struct {
	RequestID    string `json:"request_id,omitempty"`
	Status       string `json:"status,omitempty"`
	Approved     bool   `json:"approved,omitempty"`
	Kind         string `json:"kind,omitempty"`
	Title        string `json:"title,omitempty"`
	Target       string `json:"target,omitempty"`
	Action       string `json:"action,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Impact       string `json:"impact,omitempty"`
	DecisionNote string `json:"decision_note,omitempty"`
	Error        string `json:"error,omitempty"`
}

type HostCommandRequest struct {
	Command        string `json:"command"`
	Reason         string `json:"reason"`
	Workdir        string `json:"workdir,omitempty"`
	TimeoutSec     int    `json:"timeout_sec,omitempty"`
	MaxOutputChars int    `json:"max_output_chars,omitempty"`
	Background     bool   `json:"background,omitempty"`
	WaitMs         int    `json:"wait_ms,omitempty"`
}

type HostCommandResult struct {
	RequestID         string `json:"request_id,omitempty"`
	Status            string `json:"status,omitempty"`
	Approved          bool   `json:"approved,omitempty"`
	Command           string `json:"command,omitempty"`
	Workdir           string `json:"workdir,omitempty"`
	Output            string `json:"output,omitempty"`
	Error             string `json:"error,omitempty"`
	DecisionNote      string `json:"decision_note,omitempty"`
	ExitCode          int    `json:"exit_code,omitempty"`
	OutputTruncated   bool   `json:"output_truncated,omitempty"`
	TimedOut          bool   `json:"timed_out,omitempty"`
	Background        bool   `json:"background,omitempty"`
	SessionID         string `json:"session_id,omitempty"`
	Source            string `json:"source,omitempty"`
	PID               int    `json:"pid,omitempty"`
	StartedAt         string `json:"started_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
	EndedAt           string `json:"ended_at,omitempty"`
	Running           bool   `json:"running,omitempty"`
	RunningForSec     int64  `json:"running_for_sec,omitempty"`
	Stdout            string `json:"stdout,omitempty"`
	Stderr            string `json:"stderr,omitempty"`
	StdoutOffset      int64  `json:"stdout_offset,omitempty"`
	StderrOffset      int64  `json:"stderr_offset,omitempty"`
	StdoutTotal       int64  `json:"stdout_total,omitempty"`
	StderrTotal       int64  `json:"stderr_total,omitempty"`
	WaitEvent         string `json:"wait_event,omitempty"`
	HasNewOutput      bool   `json:"has_new_output,omitempty"`
	TerminatedBy      string `json:"terminated_by,omitempty"`
	TerminationReason string `json:"termination_reason,omitempty"`
}
