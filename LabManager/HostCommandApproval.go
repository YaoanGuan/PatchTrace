package LabManager

import (
	"AIxVuln/misc"
	"AIxVuln/taskManager"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultHostCommandTimeoutSec = 300
	defaultHostCommandMaxOutput  = 16384
	maxDecisionHistoryItems      = 20
)

type hostCommandApprovalRecord struct {
	ID                string `json:"id"`
	Kind              string `json:"kind,omitempty"`
	Title             string `json:"title,omitempty"`
	Target            string `json:"target,omitempty"`
	Action            string `json:"action,omitempty"`
	Impact            string `json:"impact,omitempty"`
	SaferAlternative  string `json:"safer_alternative,omitempty"`
	Status            string `json:"status"`
	Command           string `json:"command"`
	Reason            string `json:"reason"`
	Background        bool   `json:"background,omitempty"`
	SessionID         string `json:"session_id,omitempty"`
	Source            string `json:"source,omitempty"`
	Workdir           string `json:"workdir,omitempty"`
	TimeoutSec        int    `json:"timeout_sec,omitempty"`
	MaxOutputChars    int    `json:"max_output_chars,omitempty"`
	CreatedAt         string `json:"created_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
	DecisionAt        string `json:"decision_at,omitempty"`
	DecisionNote      string `json:"decision_note,omitempty"`
	Reviewer          string `json:"reviewer,omitempty"`
	GroupID           string `json:"group_id,omitempty"`
	GroupName         string `json:"group_name,omitempty"`
	RequesterSource   string `json:"requester_source,omitempty"`
	AgentToolName     string `json:"agent_tool_name,omitempty"`
	DigitalHumanID    string `json:"digital_human_id,omitempty"`
	PersonaName       string `json:"persona_name,omitempty"`
	AvatarFile        string `json:"avatar_file,omitempty"`
	PID               int    `json:"pid,omitempty"`
	StartedAt         string `json:"started_at,omitempty"`
	EndedAt           string `json:"ended_at,omitempty"`
	Running           bool   `json:"running,omitempty"`
	RunningForSec     int64  `json:"running_for_sec,omitempty"`
	ExitCode          int    `json:"exit_code,omitempty"`
	OutputPreview     string `json:"output_preview,omitempty"`
	Error             string `json:"error,omitempty"`
	OutputTruncated   bool   `json:"output_truncated,omitempty"`
	TimedOut          bool   `json:"timed_out,omitempty"`
	TerminatedBy      string `json:"terminated_by,omitempty"`
	TerminationReason string `json:"termination_reason,omitempty"`
}

type hostCommandApprovalDecision struct {
	approved bool
	note     string
	reviewer string
}

type hostCommandApprovalState struct {
	record     hostCommandApprovalRecord
	decisionCh chan hostCommandApprovalDecision
}

type hostCommandApprovalManager struct {
	mu               sync.RWMutex
	projectName      string
	getWorkDir       func() string
	getSourceCodeDir func() string
	getProjectDir    func() string
	notify           func()
	items            map[string]*hostCommandApprovalState
	order            []string
}

func newHostCommandApprovalManager(projectName string, getWorkDir func() string, getSourceCodeDir func() string, getProjectDir func() string, notify func()) *hostCommandApprovalManager {
	return &hostCommandApprovalManager{
		projectName:      strings.TrimSpace(projectName),
		getWorkDir:       getWorkDir,
		getSourceCodeDir: getSourceCodeDir,
		getProjectDir:    getProjectDir,
		notify:           notify,
		items:            make(map[string]*hostCommandApprovalState),
		order:            make([]string, 0),
	}
}

func (m *hostCommandApprovalManager) Request(ctx context.Context, actor taskManager.TaskActor, req taskManager.HostCommandRequest) taskManager.HostCommandResult {
	command := strings.TrimSpace(req.Command)
	reason := strings.TrimSpace(req.Reason)
	if command == "" {
		return taskManager.HostCommandResult{Status: "failed", Error: "command is required"}
	}
	if reason == "" {
		return taskManager.HostCommandResult{Status: "failed", Error: "reason is required"}
	}
	timeoutSec := req.TimeoutSec
	if timeoutSec <= 0 && !req.Background {
		timeoutSec = defaultHostCommandTimeoutSec
	}
	if timeoutSec > 3600 {
		timeoutSec = 3600
	}
	maxOutputChars := req.MaxOutputChars
	if maxOutputChars <= 0 {
		maxOutputChars = defaultHostCommandMaxOutput
	}
	if maxOutputChars > 131072 {
		maxOutputChars = 131072
	}
	workdir, err := m.resolveWorkdir()
	if err != nil {
		return taskManager.HostCommandResult{
			Status:   "failed",
			Command:  command,
			Workdir:  "",
			Error:    err.Error(),
			Approved: false,
		}
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	id := uuid.New().String()
	autoDecision := misc.ResolveAutomationDecision(misc.AutomationScopeHostCommandApproval)
	initialStatus := "pending_approval"
	decisionAt := ""
	decisionNote := ""
	reviewer := ""
	if autoDecision.Enabled {
		initialStatus = "approved"
		decisionAt = now
		decisionNote = autoDecision.Note
		reviewer = autoDecision.Reviewer
	}
	state := &hostCommandApprovalState{
		record: hostCommandApprovalRecord{
			ID:              id,
			Kind:            taskManager.DecisionApprovalKindHostCommand,
			Title:           "宿主机命令执行",
			Action:          command,
			Impact:          "将在宿主机项目工作目录中启动后台命令",
			Status:          initialStatus,
			Command:         command,
			Reason:          reason,
			Background:      req.Background,
			Workdir:         workdir,
			TimeoutSec:      timeoutSec,
			MaxOutputChars:  maxOutputChars,
			CreatedAt:       now,
			UpdatedAt:       now,
			DecisionAt:      decisionAt,
			DecisionNote:    decisionNote,
			Reviewer:        reviewer,
			GroupID:         strings.TrimSpace(actor.GroupID),
			GroupName:       strings.TrimSpace(actor.GroupName),
			RequesterSource: strings.TrimSpace(actor.RequesterSource),
			AgentToolName:   strings.TrimSpace(actor.AgentToolName),
			DigitalHumanID:  strings.TrimSpace(actor.DigitalHumanID),
			PersonaName:     strings.TrimSpace(actor.PersonaName),
			AvatarFile:      strings.TrimSpace(actor.AvatarFile),
		},
		decisionCh: make(chan hostCommandApprovalDecision, 1),
	}

	m.mu.Lock()
	m.items[id] = state
	m.order = append(m.order, id)
	m.compactHistoryLocked()
	m.mu.Unlock()
	m.emitUpdate()

	if !autoDecision.Enabled {
		select {
		case decision := <-state.decisionCh:
			if !decision.approved {
				return taskManager.HostCommandResult{
					RequestID:    id,
					Status:       "rejected",
					Approved:     false,
					Command:      command,
					Workdir:      workdir,
					DecisionNote: decision.note,
					Error:        firstNonEmpty(decision.note, "host command request rejected by user"),
				}
			}
		case <-ctx.Done():
			m.markCancelled(id, firstNonEmpty(contextErrString(ctx), "host command request cancelled"))
			return taskManager.HostCommandResult{
				RequestID: id,
				Status:    "cancelled",
				Approved:  false,
				Command:   command,
				Workdir:   workdir,
				Error:     firstNonEmpty(contextErrString(ctx), "host command request cancelled"),
			}
		}
	}

	if req.Background {
		result := m.executeApprovedBackgroundCommand(id, actor, req, command, workdir)
		result.DecisionNote = firstNonEmpty(result.DecisionNote, decisionNote)
		m.finish(id, result)
		if result.Background && result.SessionID != "" && result.Running {
			go m.syncBackgroundSession(id, result.SessionID)
		}
		return result
	}

	m.markRunning(id)
	result := m.executeApprovedCommand(ctx, id, command, workdir, timeoutSec, maxOutputChars)
	result.DecisionNote = firstNonEmpty(result.DecisionNote, decisionNote)
	m.finish(id, result)
	return result
}

func (m *hostCommandApprovalManager) RequestDecision(ctx context.Context, actor taskManager.TaskActor, req taskManager.DecisionApprovalRequest) taskManager.DecisionApprovalResult {
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = taskManager.DecisionApprovalKindTargetAction
	}
	title := strings.TrimSpace(req.Title)
	target := strings.TrimSpace(req.Target)
	action := strings.TrimSpace(req.Action)
	reason := strings.TrimSpace(req.Reason)
	impact := strings.TrimSpace(req.Impact)
	saferAlternative := strings.TrimSpace(req.SaferAlternative)
	if title == "" {
		title = "待确认操作"
	}
	if action == "" {
		return taskManager.DecisionApprovalResult{Status: "failed", Kind: kind, Title: title, Target: target, Error: "action is required"}
	}
	if reason == "" {
		return taskManager.DecisionApprovalResult{Status: "failed", Kind: kind, Title: title, Target: target, Action: action, Error: "reason is required"}
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	id := uuid.New().String()
	autoScope := misc.AutomationScopeDecisionApproval
	if kind == taskManager.DecisionApprovalKindAbandonIdea {
		autoScope = misc.AutomationScopeAbandonIdeaDecision
	}
	autoDecision := misc.ResolveAutomationDecision(autoScope)
	initialStatus := "pending_approval"
	decisionAt := ""
	decisionNote := ""
	reviewer := ""
	if autoDecision.Enabled {
		initialStatus = "approved"
		decisionAt = now
		decisionNote = autoDecision.Note
		reviewer = autoDecision.Reviewer
	}
	state := &hostCommandApprovalState{
		record: hostCommandApprovalRecord{
			ID:               id,
			Kind:             kind,
			Title:            title,
			Target:           target,
			Action:           action,
			Reason:           reason,
			Impact:           impact,
			SaferAlternative: saferAlternative,
			Status:           initialStatus,
			CreatedAt:        now,
			UpdatedAt:        now,
			DecisionAt:       decisionAt,
			DecisionNote:     decisionNote,
			Reviewer:         reviewer,
			GroupID:          strings.TrimSpace(actor.GroupID),
			GroupName:        strings.TrimSpace(actor.GroupName),
			RequesterSource:  strings.TrimSpace(actor.RequesterSource),
			AgentToolName:    strings.TrimSpace(actor.AgentToolName),
			DigitalHumanID:   strings.TrimSpace(actor.DigitalHumanID),
			PersonaName:      strings.TrimSpace(actor.PersonaName),
			AvatarFile:       strings.TrimSpace(actor.AvatarFile),
		},
		decisionCh: make(chan hostCommandApprovalDecision, 1),
	}

	m.mu.Lock()
	m.items[id] = state
	m.order = append(m.order, id)
	m.compactHistoryLocked()
	m.mu.Unlock()
	m.emitUpdate()

	if autoDecision.Enabled {
		return taskManager.DecisionApprovalResult{RequestID: id, Status: "approved", Approved: true, Kind: kind, Title: title, Target: target, Action: action, Reason: reason, Impact: impact, DecisionNote: decisionNote}
	}

	select {
	case decision := <-state.decisionCh:
		if !decision.approved {
			return taskManager.DecisionApprovalResult{RequestID: id, Status: "rejected", Approved: false, Kind: kind, Title: title, Target: target, Action: action, Reason: reason, Impact: impact, DecisionNote: decision.note, Error: firstNonEmpty(decision.note, "decision request rejected by user")}
		}
		return taskManager.DecisionApprovalResult{RequestID: id, Status: "approved", Approved: true, Kind: kind, Title: title, Target: target, Action: action, Reason: reason, Impact: impact, DecisionNote: decision.note}
	case <-ctx.Done():
		m.markCancelled(id, firstNonEmpty(contextErrString(ctx), "decision request cancelled"))
		return taskManager.DecisionApprovalResult{RequestID: id, Status: "cancelled", Approved: false, Kind: kind, Title: title, Target: target, Action: action, Reason: reason, Impact: impact, Error: firstNonEmpty(contextErrString(ctx), "decision request cancelled")}
	}
}

func (m *hostCommandApprovalManager) Decide(id string, approved bool, note string, reviewer string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("approval id is required")
	}
	note = strings.TrimSpace(note)
	reviewer = strings.TrimSpace(reviewer)
	if reviewer == "" {
		reviewer = "用户"
	}
	var decisionCh chan hostCommandApprovalDecision
	m.mu.Lock()
	state, ok := m.items[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("approval request not found")
	}
	if state.record.Status != "pending_approval" {
		current := state.record.Status
		m.mu.Unlock()
		return fmt.Errorf("approval request is already %s", current)
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	if approved {
		state.record.Status = "approved"
	} else {
		state.record.Status = "rejected"
		state.record.Error = firstNonEmpty(note, "host command request rejected by user")
	}
	state.record.DecisionAt = now
	state.record.DecisionNote = note
	state.record.Reviewer = reviewer
	state.record.UpdatedAt = now
	m.compactHistoryLocked()
	decisionCh = state.decisionCh
	m.mu.Unlock()
	m.emitUpdate()
	decisionCh <- hostCommandApprovalDecision{approved: approved, note: note, reviewer: reviewer}
	return nil
}

func (m *hostCommandApprovalManager) Snapshot() []hostCommandApprovalRecord {
	m.mu.RLock()
	out := m.windowedRecordsLocked()
	m.mu.RUnlock()
	sortApprovalRecordsForDisplay(out)
	return out
}

func (m *hostCommandApprovalManager) markCancelled(id string, reason string) {
	m.mu.Lock()
	state, ok := m.items[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	if isApprovalTerminal(state.record.Status) {
		m.mu.Unlock()
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	state.record.Status = "cancelled"
	state.record.Error = strings.TrimSpace(reason)
	state.record.UpdatedAt = now
	m.compactHistoryLocked()
	m.mu.Unlock()
	m.emitUpdate()
}

func (m *hostCommandApprovalManager) markRunning(id string) {
	m.mu.Lock()
	state, ok := m.items[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	state.record.Status = "running"
	state.record.UpdatedAt = now
	m.mu.Unlock()
	m.emitUpdate()
}

func (m *hostCommandApprovalManager) finish(id string, result taskManager.HostCommandResult) {
	m.mu.Lock()
	state, ok := m.items[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	state.record.Status = strings.TrimSpace(result.Status)
	state.record.Background = result.Background
	state.record.SessionID = strings.TrimSpace(result.SessionID)
	state.record.Source = strings.TrimSpace(result.Source)
	state.record.UpdatedAt = now
	state.record.PID = result.PID
	state.record.StartedAt = strings.TrimSpace(result.StartedAt)
	state.record.EndedAt = strings.TrimSpace(result.EndedAt)
	state.record.Running = result.Running
	state.record.RunningForSec = result.RunningForSec
	state.record.ExitCode = result.ExitCode
	state.record.Error = strings.TrimSpace(result.Error)
	state.record.OutputPreview = firstNonEmpty(result.Output, buildBackgroundOutputPreview(result.Stdout, result.Stderr))
	state.record.OutputTruncated = result.OutputTruncated
	state.record.TimedOut = result.TimedOut
	state.record.TerminatedBy = strings.TrimSpace(result.TerminatedBy)
	state.record.TerminationReason = strings.TrimSpace(result.TerminationReason)
	m.compactHistoryLocked()
	m.mu.Unlock()
	m.emitUpdate()
}

func (m *hostCommandApprovalManager) executeApprovedBackgroundCommand(requestID string, actor taskManager.TaskActor, req taskManager.HostCommandRequest, command string, workdir string) taskManager.HostCommandResult {
	waitMs := req.WaitMs
	if waitMs < 0 {
		waitMs = 0
	}
	snapshot, err := taskManager.StartHostBackgroundProcess(taskManager.BackgroundProcessStartOptions{
		ProjectName: m.projectName,
		OwnerTaskID: "",
		Actor:       actor,
		Command:     command,
		Reason:      req.Reason,
		Workdir:     workdir,
		TimeoutSec:  req.TimeoutSec,
		RequestID:   requestID,
	})
	if err != nil {
		return taskManager.HostCommandResult{
			RequestID: requestID,
			Status:    "failed",
			Approved:  true,
			Command:   command,
			Workdir:   workdir,
			Error:     err.Error(),
		}
	}
	readResult, err := taskManager.ReadBackgroundProcessUntilExitOrTimeout(m.projectName, snapshot.SessionID, "", 0, 0, req.MaxOutputChars, waitMs)
	if err != nil {
		return backgroundResultFromSession(taskManager.BackgroundProcessReadResult{Session: snapshot}, requestID, command, workdir)
	}
	return backgroundResultFromSession(readResult, requestID, command, workdir)
}

func (m *hostCommandApprovalManager) syncBackgroundSession(requestID string, sessionID string) {
	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot, err := taskManager.GetBackgroundProcessSnapshot(m.projectName, sessionID, "")
		if err != nil {
			return
		}
		readResult, _ := taskManager.ReadBackgroundProcess(m.projectName, sessionID, "", 0, 0, 4096, 0)
		result := backgroundResultFromSession(readResult, requestID, snapshot.Command, snapshot.Workdir)
		if result.Status == "" {
			result = taskManager.HostCommandResult{
				RequestID:         requestID,
				Status:            snapshot.Status,
				Approved:          true,
				Background:        true,
				SessionID:         snapshot.SessionID,
				Source:            snapshot.Source,
				Command:           snapshot.Command,
				Workdir:           snapshot.Workdir,
				PID:               snapshot.PID,
				StartedAt:         snapshot.StartedAt,
				UpdatedAt:         snapshot.UpdatedAt,
				EndedAt:           snapshot.EndedAt,
				Running:           snapshot.Running,
				RunningForSec:     snapshot.RunningForSec,
				ExitCode:          snapshot.ExitCode,
				TimedOut:          snapshot.TimedOut,
				TerminatedBy:      snapshot.TerminatedBy,
				TerminationReason: snapshot.TerminationReason,
				Error:             snapshot.Error,
				Output:            snapshot.OutputPreview,
			}
		}
		m.finish(requestID, result)
		if !snapshot.Running {
			return
		}
		<-ticker.C
	}
}

func (m *hostCommandApprovalManager) executeApprovedCommand(ctx context.Context, requestID string, command string, workdir string, timeoutSec int, maxOutputChars int) taskManager.HostCommandResult {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, "sh", "-lc", command)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(),
		"AIX_WORK_DIR="+m.safeWorkDir(),
		"AIX_SOURCE_DIR="+m.safeSourceCodeDir(),
		"AIX_PROJECT_DIR="+m.safeProjectDir(),
		"AIX_PROJECT_NAME="+m.projectName,
		"AIX_HOST_COMMAND_REQUEST_ID="+requestID,
	)
	output, err := cmd.CombinedOutput()
	text, truncated := truncateOutput(string(output), maxOutputChars)
	result := taskManager.HostCommandResult{
		RequestID:       requestID,
		Command:         command,
		Workdir:         workdir,
		Output:          text,
		Approved:        true,
		OutputTruncated: truncated,
		Status:          "completed",
	}
	if timeoutCtx.Err() == context.DeadlineExceeded {
		result.Status = "cancelled"
		result.TimedOut = true
		result.Error = fmt.Sprintf("host command timed out after %ds", timeoutSec)
		return result
	}
	if timeoutCtx.Err() == context.Canceled {
		result.Status = "cancelled"
		result.Error = "host command cancelled"
		return result
	}
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Error = fmt.Sprintf("host command exited with code %d", result.ExitCode)
		}
		return result
	}
	return result
}

func backgroundResultFromSession(readResult taskManager.BackgroundProcessReadResult, requestID string, command string, workdir string) taskManager.HostCommandResult {
	session := readResult.Session
	if strings.TrimSpace(command) == "" {
		command = session.Command
	}
	if strings.TrimSpace(workdir) == "" {
		workdir = session.Workdir
	}
	status := strings.TrimSpace(session.Status)
	if status == "" {
		status = "running"
	}
	return taskManager.HostCommandResult{
		RequestID:         requestID,
		Status:            status,
		Approved:          true,
		Command:           command,
		Workdir:           workdir,
		ExitCode:          session.ExitCode,
		Error:             strings.TrimSpace(session.Error),
		TimedOut:          session.TimedOut,
		Background:        true,
		SessionID:         session.SessionID,
		Source:            session.Source,
		PID:               session.PID,
		StartedAt:         session.StartedAt,
		UpdatedAt:         session.UpdatedAt,
		EndedAt:           session.EndedAt,
		Running:           session.Running,
		RunningForSec:     session.RunningForSec,
		Stdout:            readResult.Stdout,
		Stderr:            readResult.Stderr,
		StdoutOffset:      readResult.StdoutOffset,
		StderrOffset:      readResult.StderrOffset,
		StdoutTotal:       readResult.StdoutTotal,
		StderrTotal:       readResult.StderrTotal,
		WaitEvent:         readResult.WaitEvent,
		HasNewOutput:      readResult.HasNewOutput,
		TerminatedBy:      session.TerminatedBy,
		TerminationReason: session.TerminationReason,
		Output:            firstNonEmpty(buildBackgroundOutputPreview(readResult.Stdout, readResult.Stderr), session.OutputPreview),
	}
}

func buildBackgroundOutputPreview(stdout string, stderr string) string {
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	switch {
	case stdout != "" && stderr != "":
		return stdout + "\n[stderr]\n" + stderr
	case stdout != "":
		return stdout
	default:
		return stderr
	}
}

func (m *hostCommandApprovalManager) resolveWorkdir() (string, error) {
	workDir := strings.TrimSpace(m.safeWorkDir())
	if workDir == "" {
		return "", fmt.Errorf("host work directory is empty")
	}
	return filepath.Clean(workDir), nil
}

func (m *hostCommandApprovalManager) safeWorkDir() string {
	if m.getWorkDir == nil {
		return ""
	}
	return strings.TrimSpace(m.getWorkDir())
}

func (m *hostCommandApprovalManager) safeSourceCodeDir() string {
	if m.getSourceCodeDir == nil {
		return ""
	}
	return strings.TrimSpace(m.getSourceCodeDir())
}

func (m *hostCommandApprovalManager) safeProjectDir() string {
	if m.getProjectDir == nil {
		return ""
	}
	return strings.TrimSpace(m.getProjectDir())
}

func (m *hostCommandApprovalManager) compactHistoryLocked() {
	terminalCount := 0
	for _, id := range m.order {
		state, ok := m.items[id]
		if ok && isApprovalHistoryRecord(state.record) {
			terminalCount++
		}
	}
	if terminalCount <= maxDecisionHistoryItems {
		return
	}
	removeCount := terminalCount - maxDecisionHistoryItems
	next := make([]string, 0, len(m.order)-removeCount)
	for _, id := range m.order {
		state, ok := m.items[id]
		if !ok {
			continue
		}
		if removeCount > 0 && isApprovalHistoryRecord(state.record) {
			delete(m.items, id)
			removeCount--
			continue
		}
		next = append(next, id)
	}
	m.order = next
}

func (m *hostCommandApprovalManager) windowedRecordsLocked() []hostCommandApprovalRecord {
	active := make([]hostCommandApprovalRecord, 0)
	history := make([]hostCommandApprovalRecord, 0, maxDecisionHistoryItems)
	for i := len(m.order) - 1; i >= 0; i-- {
		state, ok := m.items[m.order[i]]
		if !ok {
			continue
		}
		record := state.record
		if isApprovalHistoryRecord(record) {
			if len(history) < maxDecisionHistoryItems {
				history = append(history, record)
			}
			continue
		}
		active = append(active, record)
	}
	out := make([]hostCommandApprovalRecord, 0, len(active)+len(history))
	out = append(out, active...)
	out = append(out, history...)
	return out
}

func (m *hostCommandApprovalManager) emitUpdate() {
	if m == nil || m.notify == nil {
		return
	}
	m.notify()
}

func sortApprovalRecordsForDisplay(records []hostCommandApprovalRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		pi := approvalStatusPriority(records[i].Status)
		pj := approvalStatusPriority(records[j].Status)
		if pi != pj {
			return pi < pj
		}
		if records[i].UpdatedAt == records[j].UpdatedAt {
			return records[i].CreatedAt > records[j].CreatedAt
		}
		return records[i].UpdatedAt > records[j].UpdatedAt
	})
}

func approvalStatusPriority(status string) int {
	switch strings.TrimSpace(status) {
	case "pending_approval":
		return 0
	case "approved", "starting", "running", "terminating":
		return 1
	default:
		return 2
	}
}

func isApprovalHistoryRecord(record hostCommandApprovalRecord) bool {
	status := strings.TrimSpace(record.Status)
	if status == "approved" {
		return strings.TrimSpace(record.Kind) != taskManager.DecisionApprovalKindHostCommand
	}
	return isApprovalTerminalStatus(status)
}

func isApprovalTerminal(status string) bool {
	return isApprovalTerminalStatus(status)
}

func isApprovalTerminalStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "completed", "failed", "rejected", "cancelled", "terminated":
		return true
	default:
		return false
	}
}

func truncateOutput(raw string, maxChars int) (string, bool) {
	if maxChars <= 0 {
		return raw, false
	}
	runes := []rune(raw)
	if len(runes) <= maxChars {
		return raw, false
	}
	return string(runes[:maxChars]) + "\n...[truncated]", true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func contextErrString(ctx context.Context) string {
	if ctx == nil || ctx.Err() == nil {
		return ""
	}
	return ctx.Err().Error()
}
