package taskManager

import (
	"AIxVuln/dockerManager"
	"AIxVuln/misc"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/uuid"
)

const (
	backgroundProcessMaxBufferedBytes = 512 * 1024
	backgroundProcessPreviewBytes     = 4096
	backgroundProcessHistoryLimit     = 200
	backgroundProcessIDHexChars       = 10
	backgroundProcessCommandPreview   = 160
)

type BackgroundProcessSnapshot struct {
	SessionID         string    `json:"session_id"`
	ProjectName       string    `json:"project_name,omitempty"`
	Source            string    `json:"source"`
	Command           string    `json:"command"`
	Reason            string    `json:"reason,omitempty"`
	Workdir           string    `json:"workdir,omitempty"`
	Status            string    `json:"status"`
	Running           bool      `json:"running"`
	PID               int       `json:"pid,omitempty"`
	StartedAt         string    `json:"started_at,omitempty"`
	UpdatedAt         string    `json:"updated_at,omitempty"`
	EndedAt           string    `json:"ended_at,omitempty"`
	RunningForSec     int64     `json:"running_for_sec"`
	ExitCode          int       `json:"exit_code"`
	TimedOut          bool      `json:"timed_out,omitempty"`
	TerminatedBy      string    `json:"terminated_by,omitempty"`
	TerminationReason string    `json:"termination_reason,omitempty"`
	Error             string    `json:"error,omitempty"`
	OutputPreview     string    `json:"output_preview,omitempty"`
	RequestID         string    `json:"request_id,omitempty"`
	AgentToolName     string    `json:"agent_tool_name,omitempty"`
	AgentName         string    `json:"agent_name,omitempty"`
	DigitalHumanID    string    `json:"digital_human_id,omitempty"`
	PersonaName       string    `json:"persona_name,omitempty"`
	AvatarFile        string    `json:"avatar_file,omitempty"`
	GroupID           string    `json:"group_id,omitempty"`
	GroupName         string    `json:"group_name,omitempty"`
	RequesterSource   string    `json:"requester_source,omitempty"`
	CommandPreview    string    `json:"command_preview,omitempty"`
	startedAtTime     time.Time `json:"-"`
	endedAtTime       time.Time `json:"-"`
}

type BackgroundProcessReadResult struct {
	Session          BackgroundProcessSnapshot `json:"session"`
	Stdout           string                    `json:"stdout,omitempty"`
	Stderr           string                    `json:"stderr,omitempty"`
	StdoutOffset     int64                     `json:"stdout_offset"`
	StderrOffset     int64                     `json:"stderr_offset"`
	StdoutTotal      int64                     `json:"stdout_total"`
	StderrTotal      int64                     `json:"stderr_total"`
	StdoutTruncated  bool                      `json:"stdout_truncated,omitempty"`
	StderrTruncated  bool                      `json:"stderr_truncated,omitempty"`
	WaitEvent        string                    `json:"wait_event,omitempty"`
	HasNewOutput     bool                      `json:"has_new_output"`
	WaitedMs         int                       `json:"waited_ms,omitempty"`
	StateChangedOnly bool                      `json:"state_changed_only,omitempty"`
}

type BackgroundProcessStartOptions struct {
	ProjectName string
	OwnerTaskID string
	Actor       TaskActor
	Command     string
	Reason      string
	Workdir     string
	TimeoutSec  int
	RequestID   string
}

type backgroundInputHandle interface {
	WriteString(string) error
	CloseInput() error
}

type backgroundProcessStore struct {
	mu       sync.RWMutex
	projects map[string]*backgroundProjectState
}

type backgroundProjectState struct {
	sessions map[string]*backgroundProcessSession
	order    []string
	observer func([]BackgroundProcessSnapshot)
}

type backgroundProcessSession struct {
	mu          sync.Mutex
	projectName string
	ownerTaskID string
	stdout      rollingTextBuffer
	stderr      rollingTextBuffer
	waitCh      chan struct{}
	version     int64
	snapshot    BackgroundProcessSnapshot
	input       backgroundInputHandle
	timeout     *time.Timer

	hostCmd       *exec.Cmd
	sandbox       *dockerManager.Sandbox
	sandboxHandle *dockerManager.BackgroundExecHandle

	sandboxPIDPending bool
	sandboxPIDLine    []byte
}

type rollingTextBuffer struct {
	maxBytes int
	start    int64
	total    int64
	data     []byte
}

type hostProcessInput struct {
	mu     sync.Mutex
	writer io.WriteCloser
}

type sandboxProcessInput struct {
	mu     sync.Mutex
	handle *dockerManager.BackgroundExecHandle
}

var backgroundProcesses = newBackgroundProcessStore()

func newBackgroundProcessStore() *backgroundProcessStore {
	return &backgroundProcessStore{
		projects: make(map[string]*backgroundProjectState),
	}
}

func StartSandboxBackgroundProcess(opts BackgroundProcessStartOptions) (BackgroundProcessSnapshot, error) {
	projectName := resolveBackgroundProjectName(opts.ProjectName)
	command := strings.TrimSpace(opts.Command)
	if projectName == "" {
		return BackgroundProcessSnapshot{}, fmt.Errorf("project name is required")
	}
	if command == "" {
		return BackgroundProcessSnapshot{}, fmt.Errorf("command is required")
	}
	sandbox, err := GetSandbox(projectName)
	if err != nil {
		return BackgroundProcessSnapshot{}, err
	}
	workdir := strings.TrimSpace(opts.Workdir)
	if workdir == "" {
		workdir = misc.SandboxWorkDir
	}

	session := backgroundProcesses.newSession(projectName, opts, "sandbox")
	handle, err := sandbox.StartBackgroundCommand(command, workdir)
	if err != nil {
		backgroundProcesses.removeSession(projectName, session.snapshot.SessionID)
		return BackgroundProcessSnapshot{}, err
	}

	session.mu.Lock()
	session.sandbox = sandbox
	session.sandboxHandle = handle
	session.input = &sandboxProcessInput{handle: handle}
	session.sandboxPIDPending = true
	session.markUpdatedLocked()
	session.mu.Unlock()
	backgroundProcesses.emit(projectName)

	if opts.TimeoutSec > 0 {
		session.setTimeout(opts.TimeoutSec)
	}

	go session.consumeSandboxIO()
	return session.snapshotView(), nil
}

func StartHostBackgroundProcess(opts BackgroundProcessStartOptions) (BackgroundProcessSnapshot, error) {
	projectName := resolveBackgroundProjectName(opts.ProjectName)
	command := strings.TrimSpace(opts.Command)
	workdir := strings.TrimSpace(opts.Workdir)
	if projectName == "" {
		return BackgroundProcessSnapshot{}, fmt.Errorf("project name is required")
	}
	if command == "" {
		return BackgroundProcessSnapshot{}, fmt.Errorf("command is required")
	}
	if workdir == "" {
		return BackgroundProcessSnapshot{}, fmt.Errorf("workdir is required")
	}

	session := backgroundProcesses.newSession(projectName, opts, "host")
	cmd := exec.Command("sh", "-lc", command)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(),
		"AIX_WORK_DIR="+workdir,
		"AIX_SOURCE_DIR="+filepath.Join(workdir, misc.ProjectSourceDirName),
		"AIX_PROJECT_NAME="+projectName,
		"AIX_BACKGROUND_SESSION_ID="+session.snapshot.SessionID,
	)
	cmd.SysProcAttr = backgroundCommandSysProcAttr()

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		backgroundProcesses.removeSession(projectName, session.snapshot.SessionID)
		return BackgroundProcessSnapshot{}, fmt.Errorf("open stdout pipe failed: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		backgroundProcesses.removeSession(projectName, session.snapshot.SessionID)
		return BackgroundProcessSnapshot{}, fmt.Errorf("open stderr pipe failed: %w", err)
	}
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		backgroundProcesses.removeSession(projectName, session.snapshot.SessionID)
		return BackgroundProcessSnapshot{}, fmt.Errorf("open stdin pipe failed: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdinPipe.Close()
		backgroundProcesses.removeSession(projectName, session.snapshot.SessionID)
		return BackgroundProcessSnapshot{}, fmt.Errorf("start host process failed: %w", err)
	}

	session.mu.Lock()
	session.hostCmd = cmd
	session.input = &hostProcessInput{writer: stdinPipe}
	if cmd.Process != nil {
		session.snapshot.PID = cmd.Process.Pid
	}
	session.snapshot.Status = "running"
	session.snapshot.Running = true
	session.markUpdatedLocked()
	session.mu.Unlock()
	backgroundProcesses.emit(projectName)

	if opts.TimeoutSec > 0 {
		session.setTimeout(opts.TimeoutSec)
	}

	go session.consumeHostIO(stdoutPipe, stderrPipe)
	return session.snapshotView(), nil
}

func ReadBackgroundProcess(projectName string, sessionID string, ownerTaskID string, stdoutOffset int64, stderrOffset int64, limit int, waitMs int) (BackgroundProcessReadResult, error) {
	session, err := backgroundProcesses.getSession(projectName, sessionID, ownerTaskID)
	if err != nil {
		return BackgroundProcessReadResult{}, err
	}
	if limit <= 0 {
		limit = 16 * 1024
	}
	if waitMs < 0 {
		waitMs = 0
	}
	started := time.Now()
	deadline := started.Add(time.Duration(waitMs) * time.Millisecond)

	session.mu.Lock()
	initialVersion := session.version
	session.mu.Unlock()

	for {
		session.mu.Lock()
		snapshot := session.snapshotViewLocked()
		stdout, nextStdout, totalStdout, stdoutTruncated := session.stdout.ReadFrom(stdoutOffset, limit)
		stderr, nextStderr, totalStderr, stderrTruncated := session.stderr.ReadFrom(stderrOffset, limit)
		version := session.version
		waitCh := session.waitCh
		session.mu.Unlock()

		hasNewOutput := stdout != "" || stderr != ""
		if hasNewOutput {
			return BackgroundProcessReadResult{
				Session:         snapshot,
				Stdout:          stdout,
				Stderr:          stderr,
				StdoutOffset:    nextStdout,
				StderrOffset:    nextStderr,
				StdoutTotal:     totalStdout,
				StderrTotal:     totalStderr,
				StdoutTruncated: stdoutTruncated,
				StderrTruncated: stderrTruncated,
				WaitEvent:       "output",
				HasNewOutput:    true,
				WaitedMs:        int(time.Since(started).Milliseconds()),
			}, nil
		}
		if !snapshot.Running {
			return BackgroundProcessReadResult{
				Session:         snapshot,
				StdoutOffset:    nextStdout,
				StderrOffset:    nextStderr,
				StdoutTotal:     totalStdout,
				StderrTotal:     totalStderr,
				StdoutTruncated: stdoutTruncated,
				StderrTruncated: stderrTruncated,
				WaitEvent:       "exit",
				HasNewOutput:    false,
				WaitedMs:        int(time.Since(started).Milliseconds()),
			}, nil
		}
		if version != initialVersion {
			return BackgroundProcessReadResult{
				Session:          snapshot,
				StdoutOffset:     nextStdout,
				StderrOffset:     nextStderr,
				StdoutTotal:      totalStdout,
				StderrTotal:      totalStderr,
				StdoutTruncated:  stdoutTruncated,
				StderrTruncated:  stderrTruncated,
				WaitEvent:        "state",
				HasNewOutput:     false,
				WaitedMs:         int(time.Since(started).Milliseconds()),
				StateChangedOnly: true,
			}, nil
		}
		if waitMs <= 0 {
			return BackgroundProcessReadResult{
				Session:         snapshot,
				StdoutOffset:    nextStdout,
				StderrOffset:    nextStderr,
				StdoutTotal:     totalStdout,
				StderrTotal:     totalStderr,
				StdoutTruncated: stdoutTruncated,
				StderrTruncated: stderrTruncated,
				WaitEvent:       "none",
				HasNewOutput:    false,
			}, nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return BackgroundProcessReadResult{
				Session:         snapshot,
				StdoutOffset:    nextStdout,
				StderrOffset:    nextStderr,
				StdoutTotal:     totalStdout,
				StderrTotal:     totalStderr,
				StdoutTruncated: stdoutTruncated,
				StderrTruncated: stderrTruncated,
				WaitEvent:       "timeout",
				HasNewOutput:    false,
				WaitedMs:        int(time.Since(started).Milliseconds()),
			}, nil
		}

		select {
		case <-waitCh:
		case <-time.After(remaining):
			session.mu.Lock()
			snapshot = session.snapshotViewLocked()
			_, nextStdout, totalStdout, stdoutTruncated = session.stdout.ReadFrom(stdoutOffset, limit)
			_, nextStderr, totalStderr, stderrTruncated = session.stderr.ReadFrom(stderrOffset, limit)
			session.mu.Unlock()
			return BackgroundProcessReadResult{
				Session:         snapshot,
				StdoutOffset:    nextStdout,
				StderrOffset:    nextStderr,
				StdoutTotal:     totalStdout,
				StderrTotal:     totalStderr,
				StdoutTruncated: stdoutTruncated,
				StderrTruncated: stderrTruncated,
				WaitEvent:       "timeout",
				HasNewOutput:    false,
				WaitedMs:        int(time.Since(started).Milliseconds()),
			}, nil
		}
	}
}

func ReadBackgroundProcessUntilExitOrTimeout(projectName string, sessionID string, ownerTaskID string, stdoutOffset int64, stderrOffset int64, limit int, waitMs int) (BackgroundProcessReadResult, error) {
	if waitMs <= 0 {
		return ReadBackgroundProcess(projectName, sessionID, ownerTaskID, stdoutOffset, stderrOffset, limit, 0)
	}
	if limit <= 0 {
		limit = 16 * 1024
	}

	started := time.Now()
	deadline := started.Add(time.Duration(waitMs) * time.Millisecond)

	scanStdoutOffset := stdoutOffset
	scanStderrOffset := stderrOffset
	returnStdoutOffset := stdoutOffset
	returnStderrOffset := stderrOffset
	stdoutRemaining := limit
	stderrRemaining := limit

	result := BackgroundProcessReadResult{
		StdoutOffset: stdoutOffset,
		StderrOffset: stderrOffset,
	}

	for {
		remaining := int(time.Until(deadline).Milliseconds())
		if remaining < 0 {
			remaining = 0
		}

		part, err := ReadBackgroundProcess(projectName, sessionID, ownerTaskID, scanStdoutOffset, scanStderrOffset, limit, remaining)
		if err != nil {
			return BackgroundProcessReadResult{}, err
		}

		result.Session = part.Session
		result.StdoutTotal = part.StdoutTotal
		result.StderrTotal = part.StderrTotal
		result.HasNewOutput = result.HasNewOutput || part.HasNewOutput
		result.StateChangedOnly = result.StateChangedOnly || part.StateChangedOnly
		result.StdoutTruncated = result.StdoutTruncated || part.StdoutTruncated
		result.StderrTruncated = result.StderrTruncated || part.StderrTruncated

		scanStdoutOffset = part.StdoutOffset
		scanStderrOffset = part.StderrOffset

		if part.Stdout != "" && stdoutRemaining > 0 {
			chunk := part.Stdout
			if len(chunk) > stdoutRemaining {
				chunk = chunk[:stdoutRemaining]
				result.StdoutTruncated = true
			}
			result.Stdout += chunk
			returnStdoutOffset += int64(len(chunk))
			stdoutRemaining -= len(chunk)
		} else if part.Stdout != "" && stdoutRemaining <= 0 {
			result.StdoutTruncated = true
		}
		if part.Stderr != "" && stderrRemaining > 0 {
			chunk := part.Stderr
			if len(chunk) > stderrRemaining {
				chunk = chunk[:stderrRemaining]
				result.StderrTruncated = true
			}
			result.Stderr += chunk
			returnStderrOffset += int64(len(chunk))
			stderrRemaining -= len(chunk)
		} else if part.Stderr != "" && stderrRemaining <= 0 {
			result.StderrTruncated = true
		}

		result.StdoutOffset = returnStdoutOffset
		result.StderrOffset = returnStderrOffset

		switch part.WaitEvent {
		case "exit":
			result.WaitEvent = "exit"
			result.WaitedMs = int(time.Since(started).Milliseconds())
			return result, nil
		case "timeout":
			result.WaitEvent = "timeout"
			result.WaitedMs = int(time.Since(started).Milliseconds())
			return result, nil
		case "none":
			result.WaitEvent = "none"
			result.WaitedMs = int(time.Since(started).Milliseconds())
			return result, nil
		}

		if !part.Session.Running {
			result.WaitEvent = "exit"
			result.WaitedMs = int(time.Since(started).Milliseconds())
			return result, nil
		}
		if time.Now().After(deadline) {
			result.WaitEvent = "timeout"
			result.WaitedMs = int(time.Since(started).Milliseconds())
			return result, nil
		}
	}
}

func WriteBackgroundProcess(projectName string, sessionID string, ownerTaskID string, input string, closeInput bool) (BackgroundProcessSnapshot, error) {
	session, err := backgroundProcesses.getSession(projectName, sessionID, ownerTaskID)
	if err != nil {
		return BackgroundProcessSnapshot{}, err
	}
	session.mu.Lock()
	handle := session.input
	running := session.snapshot.Running
	session.mu.Unlock()

	if handle == nil {
		return BackgroundProcessSnapshot{}, fmt.Errorf("process stdin is unavailable")
	}
	if !running && strings.TrimSpace(input) == "" && !closeInput {
		return session.snapshotView(), nil
	}
	if strings.TrimSpace(input) != "" || input != "" {
		if err := handle.WriteString(input); err != nil {
			return BackgroundProcessSnapshot{}, err
		}
	}
	if closeInput {
		if err := handle.CloseInput(); err != nil {
			return BackgroundProcessSnapshot{}, err
		}
	}

	session.mu.Lock()
	session.markUpdatedLocked()
	session.mu.Unlock()
	backgroundProcesses.emit(session.projectName)
	return session.snapshotView(), nil
}

func TerminateBackgroundProcess(projectName string, sessionID string, ownerTaskID string, terminatedBy string, reason string, signal string) (BackgroundProcessSnapshot, error) {
	session, err := backgroundProcesses.getSession(projectName, sessionID, ownerTaskID)
	if err != nil {
		return BackgroundProcessSnapshot{}, err
	}

	session.mu.Lock()
	if !session.snapshot.Running {
		snapshot := session.snapshotViewLocked()
		session.mu.Unlock()
		return snapshot, nil
	}
	terminatedBy = strings.TrimSpace(terminatedBy)
	if terminatedBy == "" {
		terminatedBy = "agent"
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = fmt.Sprintf("terminated by %s", terminatedBy)
	}
	if session.snapshot.TerminatedBy == "" {
		session.snapshot.TerminatedBy = terminatedBy
	}
	if session.snapshot.TerminationReason == "" {
		session.snapshot.TerminationReason = reason
	}
	if terminatedBy == "timeout" {
		session.snapshot.TimedOut = true
	}
	session.snapshot.Status = "terminating"
	session.markUpdatedLocked()
	session.mu.Unlock()
	backgroundProcesses.emit(session.projectName)

	if session.hostCmd != nil {
		if err := signalBackgroundProcess(session.hostCmd.Process, signal); err != nil && !isIgnorableSignalError(err) {
			return BackgroundProcessSnapshot{}, err
		}
		return session.snapshotView(), nil
	}
	if session.sandbox != nil {
		pid := session.snapshotView().PID
		if pid <= 0 {
			return BackgroundProcessSnapshot{}, fmt.Errorf("sandbox process pid is not ready yet")
		}
		if err := session.sandbox.SignalBackgroundPID(pid, signal); err != nil && !isIgnorableSignalError(err) {
			return BackgroundProcessSnapshot{}, err
		}
		return session.snapshotView(), nil
	}
	return BackgroundProcessSnapshot{}, fmt.Errorf("background process backend is unavailable")
}

func GetBackgroundProcessSnapshot(projectName string, sessionID string, ownerTaskID string) (BackgroundProcessSnapshot, error) {
	session, err := backgroundProcesses.getSession(projectName, sessionID, ownerTaskID)
	if err != nil {
		return BackgroundProcessSnapshot{}, err
	}
	return session.snapshotView(), nil
}

func ListActiveBackgroundProcesses(projectName string) []BackgroundProcessSnapshot {
	return backgroundProcesses.activeSnapshots(projectName, "")
}

func ListTaskActiveBackgroundProcesses(projectName string, ownerTaskID string) []BackgroundProcessSnapshot {
	return backgroundProcesses.activeSnapshots(projectName, "")
}

func SetBackgroundProcessObserver(projectName string, observer func([]BackgroundProcessSnapshot)) {
	backgroundProcesses.setObserver(projectName, observer)
}

func CloseBackgroundProcesses(projectName string) {
	projectName = resolveBackgroundProjectName(projectName)
	for _, item := range ListActiveBackgroundProcesses(projectName) {
		_, _ = TerminateBackgroundProcess(projectName, item.SessionID, "", "system", "project stopped", "KILL")
	}
}

func resolveBackgroundProjectName(projectName string) string {
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return ""
	}
	return strings.TrimSpace(projects.resolveRuntimeProject(projectName))
}

func (s *backgroundProcessStore) getProjectState(projectName string, create bool) *backgroundProjectState {
	projectName = resolveBackgroundProjectName(projectName)
	if projectName == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.projects[projectName]
	if ok || !create {
		return state
	}
	state = &backgroundProjectState{
		sessions: make(map[string]*backgroundProcessSession),
		order:    make([]string, 0),
	}
	s.projects[projectName] = state
	return state
}

func (s *backgroundProcessStore) newSession(projectName string, opts BackgroundProcessStartOptions, source string) *backgroundProcessSession {
	projectName = resolveBackgroundProjectName(projectName)
	now := time.Now()
	sessionID := newBackgroundProcessSessionID(source)
	session := &backgroundProcessSession{
		projectName: projectName,
		ownerTaskID: strings.TrimSpace(opts.OwnerTaskID),
		stdout:      newRollingTextBuffer(backgroundProcessMaxBufferedBytes),
		stderr:      newRollingTextBuffer(backgroundProcessMaxBufferedBytes),
		waitCh:      make(chan struct{}),
		snapshot: BackgroundProcessSnapshot{
			SessionID:       sessionID,
			ProjectName:     projectName,
			Source:          source,
			Command:         strings.TrimSpace(opts.Command),
			Reason:          strings.TrimSpace(opts.Reason),
			Workdir:         strings.TrimSpace(opts.Workdir),
			Status:          "starting",
			Running:         true,
			StartedAt:       now.Format("2006-01-02 15:04:05"),
			UpdatedAt:       now.Format("2006-01-02 15:04:05"),
			RequestID:       strings.TrimSpace(opts.RequestID),
			AgentToolName:   strings.TrimSpace(opts.Actor.AgentToolName),
			AgentName:       strings.TrimSpace(opts.Actor.AgentName),
			DigitalHumanID:  strings.TrimSpace(opts.Actor.DigitalHumanID),
			PersonaName:     strings.TrimSpace(opts.Actor.PersonaName),
			AvatarFile:      strings.TrimSpace(opts.Actor.AvatarFile),
			GroupID:         strings.TrimSpace(opts.Actor.GroupID),
			GroupName:       strings.TrimSpace(opts.Actor.GroupName),
			RequesterSource: strings.TrimSpace(opts.Actor.RequesterSource),
			CommandPreview:  summarizeBackgroundCommand(strings.TrimSpace(opts.Command)),
			startedAtTime:   now,
		},
	}

	state := s.getProjectState(projectName, true)
	s.mu.Lock()
	if state != nil {
		state.sessions[sessionID] = session
		state.order = append(state.order, sessionID)
		s.compactProjectHistoryLocked(state)
	}
	s.mu.Unlock()
	s.emit(projectName)
	return session
}

func (s *backgroundProcessStore) compactProjectHistoryLocked(state *backgroundProjectState) {
	if state == nil || len(state.order) <= backgroundProcessHistoryLimit {
		return
	}
	next := make([]string, 0, len(state.order))
	removable := len(state.order) - backgroundProcessHistoryLimit
	for _, id := range state.order {
		item := state.sessions[id]
		if item == nil {
			continue
		}
		snapshot := item.snapshotView()
		if removable > 0 && !snapshot.Running {
			delete(state.sessions, id)
			removable--
			continue
		}
		next = append(next, id)
	}
	state.order = next
}

func (s *backgroundProcessStore) removeSession(projectName string, sessionID string) {
	projectName = resolveBackgroundProjectName(projectName)
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.projects[projectName]
	if state == nil {
		return
	}
	delete(state.sessions, sessionID)
	next := make([]string, 0, len(state.order))
	for _, id := range state.order {
		if id == sessionID {
			continue
		}
		next = append(next, id)
	}
	state.order = next
}

func (s *backgroundProcessStore) getSession(projectName string, sessionID string, ownerTaskID string) (*backgroundProcessSession, error) {
	projectName = resolveBackgroundProjectName(projectName)
	sessionID = strings.TrimSpace(sessionID)
	ownerTaskID = strings.TrimSpace(ownerTaskID)
	if projectName == "" {
		return nil, fmt.Errorf("project name is required")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	s.mu.RLock()
	state := s.projects[projectName]
	var session *backgroundProcessSession
	if state != nil {
		session = state.sessions[sessionID]
		if session == nil {
			var matches []*backgroundProcessSession
			matchIDs := make([]string, 0, 4)
			for id, item := range state.sessions {
				if strings.HasPrefix(id, sessionID) {
					matches = append(matches, item)
					matchIDs = append(matchIDs, id)
				}
			}
			switch len(matches) {
			case 1:
				session = matches[0]
			case 0:
			default:
				s.mu.RUnlock()
				return nil, fmt.Errorf("session_id %q is ambiguous; matches: %s", sessionID, strings.Join(matchIDs, ", "))
			}
		}
	}
	s.mu.RUnlock()
	if session == nil {
		return nil, fmt.Errorf("background process not found")
	}
	return session, nil
}

func (s *backgroundProcessStore) setObserver(projectName string, observer func([]BackgroundProcessSnapshot)) {
	projectName = resolveBackgroundProjectName(projectName)
	if projectName == "" {
		return
	}
	s.mu.Lock()
	state := s.projects[projectName]
	if state == nil {
		state = &backgroundProjectState{
			sessions: make(map[string]*backgroundProcessSession),
			order:    make([]string, 0),
		}
		s.projects[projectName] = state
	}
	state.observer = observer
	s.mu.Unlock()
	s.emit(projectName)
}

func (s *backgroundProcessStore) activeSnapshots(projectName string, ownerTaskID string) []BackgroundProcessSnapshot {
	projectName = resolveBackgroundProjectName(projectName)
	ownerTaskID = strings.TrimSpace(ownerTaskID)
	s.mu.RLock()
	state := s.projects[projectName]
	if state == nil {
		s.mu.RUnlock()
		return []BackgroundProcessSnapshot{}
	}
	ids := append([]string(nil), state.order...)
	sessions := make([]*backgroundProcessSession, 0, len(ids))
	for _, id := range ids {
		if session := state.sessions[id]; session != nil {
			if ownerTaskID != "" && session.ownerTaskID != ownerTaskID {
				continue
			}
			sessions = append(sessions, session)
		}
	}
	s.mu.RUnlock()

	out := make([]BackgroundProcessSnapshot, 0, len(sessions))
	for i := len(sessions) - 1; i >= 0; i-- {
		snapshot := sessions[i].snapshotView()
		if snapshot.Running {
			out = append(out, snapshot)
		}
	}
	return out
}

func (s *backgroundProcessStore) emit(projectName string) {
	projectName = resolveBackgroundProjectName(projectName)
	if projectName == "" {
		return
	}
	s.mu.RLock()
	state := s.projects[projectName]
	var observer func([]BackgroundProcessSnapshot)
	if state != nil {
		observer = state.observer
	}
	s.mu.RUnlock()
	if observer == nil {
		return
	}
	snapshots := s.activeSnapshots(projectName, "")
	go observer(snapshots)
}

func (session *backgroundProcessSession) setTimeout(timeoutSec int) {
	if timeoutSec <= 0 {
		return
	}
	session.mu.Lock()
	if session.timeout != nil {
		session.timeout.Stop()
	}
	session.timeout = time.AfterFunc(time.Duration(timeoutSec)*time.Second, func() {
		_, _ = TerminateBackgroundProcess(session.projectName, session.snapshot.SessionID, "", "timeout", fmt.Sprintf("timed out after %ds", timeoutSec), "KILL")
	})
	session.mu.Unlock()
}

func (session *backgroundProcessSession) consumeHostIO(stdoutPipe io.ReadCloser, stderrPipe io.ReadCloser) {
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&backgroundOutputWriter{session: session, stream: "stdout"}, stdoutPipe)
		_ = stdoutPipe.Close()
		close(stdoutDone)
	}()
	go func() {
		_, _ = io.Copy(&backgroundOutputWriter{session: session, stream: "stderr"}, stderrPipe)
		_ = stderrPipe.Close()
		close(stderrDone)
	}()

	waitErr := session.hostCmd.Wait()
	<-stdoutDone
	<-stderrDone
	session.finish(waitErr, hostExitCode(waitErr))
}

func (session *backgroundProcessSession) consumeSandboxIO() {
	copyErr := func() error {
		defer session.sandboxHandle.Resp.Close()
		_, err := stdcopy.StdCopy(
			&backgroundOutputWriter{session: session, stream: "stdout"},
			&backgroundOutputWriter{session: session, stream: "stderr"},
			session.sandboxHandle.Resp.Reader,
		)
		return err
	}()
	exitCode, inspectErr := session.inspectSandboxExitCode()
	if inspectErr != nil && copyErr == nil {
		copyErr = inspectErr
	}
	session.finish(copyErr, exitCode)
}

func (session *backgroundProcessSession) inspectSandboxExitCode() (int, error) {
	if session.sandbox == nil || session.sandboxHandle == nil {
		return 0, nil
	}
	var lastErr error
	for i := 0; i < 10; i++ {
		inspect, err := session.sandbox.InspectBackgroundCommand(session.sandboxHandle.ExecID)
		if err == nil {
			return inspect.ExitCode, nil
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	return 0, lastErr
}

func (session *backgroundProcessSession) finish(waitErr error, exitCode int) {
	session.mu.Lock()
	if session.timeout != nil {
		session.timeout.Stop()
		session.timeout = nil
	}
	if session.input != nil {
		_ = session.input.CloseInput()
	}
	session.snapshot.ExitCode = exitCode
	session.snapshot.Running = false
	session.snapshot.EndedAt = time.Now().Format("2006-01-02 15:04:05")
	session.snapshot.endedAtTime = time.Now()
	if session.snapshot.TerminatedBy != "" {
		session.snapshot.Status = "terminated"
		if session.snapshot.TerminationReason == "" {
			session.snapshot.TerminationReason = fmt.Sprintf("terminated by %s", session.snapshot.TerminatedBy)
		}
	} else if waitErr != nil && !isIgnorableWaitError(waitErr) {
		session.snapshot.Status = "failed"
		if session.snapshot.Error == "" {
			session.snapshot.Error = normalizeProcessWaitError(waitErr, exitCode)
		}
	} else if exitCode != 0 {
		session.snapshot.Status = "failed"
		if session.snapshot.Error == "" {
			session.snapshot.Error = fmt.Sprintf("process exited with code %d", exitCode)
		}
	} else {
		session.snapshot.Status = "completed"
	}
	if waitErr != nil && session.snapshot.TerminatedBy == "" && session.snapshot.Error == "" && !isIgnorableCopyError(waitErr) {
		session.snapshot.Error = waitErr.Error()
	}
	session.markUpdatedLocked()
	session.mu.Unlock()
	backgroundProcesses.emit(session.projectName)
}

func (session *backgroundProcessSession) snapshotView() BackgroundProcessSnapshot {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.snapshotViewLocked()
}

func (session *backgroundProcessSession) snapshotViewLocked() BackgroundProcessSnapshot {
	out := session.snapshot
	if !out.startedAtTime.IsZero() {
		if out.Running {
			out.RunningForSec = int64(time.Since(out.startedAtTime).Seconds())
		} else if !out.endedAtTime.IsZero() {
			out.RunningForSec = int64(out.endedAtTime.Sub(out.startedAtTime).Seconds())
		}
	}
	out.OutputPreview = buildOutputPreview(session.stdout.Tail(backgroundProcessPreviewBytes), session.stderr.Tail(backgroundProcessPreviewBytes))
	return out
}

func (session *backgroundProcessSession) markUpdatedLocked() {
	session.version++
	session.snapshot.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	close(session.waitCh)
	session.waitCh = make(chan struct{})
}

func (b *rollingTextBuffer) Append(p []byte) {
	if len(p) == 0 {
		return
	}
	b.data = append(b.data, p...)
	b.total += int64(len(p))
	if b.maxBytes > 0 && len(b.data) > b.maxBytes {
		trim := len(b.data) - b.maxBytes
		b.data = append([]byte(nil), b.data[trim:]...)
		b.start += int64(trim)
		return
	}
	b.data = append([]byte(nil), b.data...)
}

func (b *rollingTextBuffer) ReadFrom(offset int64, limit int) (string, int64, int64, bool) {
	truncated := false
	if offset < b.start {
		offset = b.start
		truncated = true
	}
	if offset > b.total {
		offset = b.total
	}
	rel := int(offset - b.start)
	if rel < 0 {
		rel = 0
	}
	chunk := b.data[rel:]
	if limit > 0 && len(chunk) > limit {
		chunk = chunk[:limit]
	}
	next := offset + int64(len(chunk))
	return string(chunk), next, b.total, truncated
}

func (b *rollingTextBuffer) Tail(limit int) string {
	if limit <= 0 || len(b.data) <= limit {
		return string(b.data)
	}
	return string(b.data[len(b.data)-limit:])
}

func newRollingTextBuffer(maxBytes int) rollingTextBuffer {
	return rollingTextBuffer{
		maxBytes: maxBytes,
		data:     make([]byte, 0, maxBytes),
	}
}

func (h *hostProcessInput) WriteString(input string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.writer == nil {
		return fmt.Errorf("stdin is closed")
	}
	_, err := io.WriteString(h.writer, input)
	return err
}

func (h *hostProcessInput) CloseInput() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.writer == nil {
		return nil
	}
	err := h.writer.Close()
	h.writer = nil
	return err
}

func (h *sandboxProcessInput) WriteString(input string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.handle == nil || h.handle.Resp == nil || h.handle.Resp.Conn == nil {
		return fmt.Errorf("stdin is closed")
	}
	_, err := io.WriteString(h.handle.Resp.Conn, input)
	return err
}

func (h *sandboxProcessInput) CloseInput() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.handle == nil || h.handle.Resp == nil {
		return nil
	}
	return h.handle.Resp.CloseWrite()
}

type backgroundOutputWriter struct {
	session *backgroundProcessSession
	stream  string
}

func (w *backgroundOutputWriter) Write(p []byte) (int, error) {
	if w.session == nil || len(p) == 0 {
		return len(p), nil
	}
	w.session.mu.Lock()
	switch w.stream {
	case "stdout":
		w.session.appendStdoutLocked(p)
	default:
		w.session.stderr.Append(p)
	}
	w.session.markUpdatedLocked()
	w.session.mu.Unlock()
	backgroundProcesses.emit(w.session.projectName)
	return len(p), nil
}

func (session *backgroundProcessSession) appendStdoutLocked(p []byte) {
	if !session.sandboxPIDPending {
		session.stdout.Append(p)
		return
	}
	session.sandboxPIDLine = append(session.sandboxPIDLine, p...)
	for {
		idx := bytesIndexByte(session.sandboxPIDLine, '\n')
		if idx < 0 {
			return
		}
		line := strings.TrimSpace(string(session.sandboxPIDLine[:idx]))
		rest := append([]byte(nil), session.sandboxPIDLine[idx+1:]...)
		session.sandboxPIDLine = nil
		if strings.HasPrefix(line, "__AIX_BG_PID__") {
			if pid, err := parsePositiveInt(strings.TrimSpace(strings.TrimPrefix(line, "__AIX_BG_PID__"))); err == nil && pid > 0 {
				session.snapshot.PID = pid
				session.snapshot.Status = "running"
			}
			session.sandboxPIDPending = false
			if len(rest) > 0 {
				session.stdout.Append(rest)
			}
			return
		}
		session.sandboxPIDPending = false
		session.stdout.Append([]byte(line))
		session.stdout.Append([]byte("\n"))
		if len(rest) > 0 {
			session.stdout.Append(rest)
		}
		return
	}
}

func buildOutputPreview(stdout string, stderr string) string {
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

func newBackgroundProcessSessionID(source string) string {
	raw := strings.ReplaceAll(uuid.New().String(), "-", "")
	if len(raw) > backgroundProcessIDHexChars {
		raw = raw[:backgroundProcessIDHexChars]
	}
	prefix := "bg"
	switch strings.TrimSpace(source) {
	case "sandbox":
		prefix = "sbx"
	case "host":
		prefix = "hst"
	}
	return prefix + "-" + raw
}

func summarizeBackgroundCommand(command string) string {
	command = strings.Join(strings.Fields(strings.TrimSpace(command)), " ")
	if command == "" {
		return ""
	}
	if len(command) <= backgroundProcessCommandPreview {
		return command
	}
	return command[:backgroundProcessCommandPreview-3] + "..."
}

func hostExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

func normalizeProcessWaitError(err error, exitCode int) string {
	if err == nil {
		return ""
	}
	if exitCode >= 0 {
		return fmt.Sprintf("process exited with code %d", exitCode)
	}
	return err.Error()
}

func isIgnorableWaitError(err error) bool {
	if err == nil {
		return true
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		_ = exitErr
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return msg == "" || strings.Contains(msg, "file already closed")
}

func isIgnorableCopyError(err error) bool {
	if err == nil {
		return true
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return true
	}
	return strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "file already closed") ||
		strings.Contains(msg, "closed pipe") ||
		strings.Contains(msg, "eof")
}

func isIgnorableSignalError(err error) bool {
	if err == nil {
		return true
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "process already finished") ||
		strings.Contains(msg, "no such process") ||
		strings.Contains(msg, "os: process already finished")
}

func parsePositiveInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty int")
	}
	value := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid int")
		}
		value = value*10 + int(ch-'0')
	}
	return value, nil
}

func bytesIndexByte(data []byte, target byte) int {
	for i, b := range data {
		if b == target {
			return i
		}
	}
	return -1
}
