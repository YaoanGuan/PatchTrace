package toolCalling

import (
	"AIxVuln/taskManager"
	"strings"
)

const (
	commandExecutionInitialWaitMs = 5000
	commandExecutionOutputLimit   = 16 * 1024
)

func buildDeferredBackgroundPayload(readResult taskManager.BackgroundProcessReadResult) map[string]interface{} {
	session := readResult.Session
	return map[string]interface{}{
		"session_timeout": true,
		"session_id":      session.SessionID,
		"source":          session.Source,
		"command_preview": session.CommandPreview,
	}
}

func buildDeferredHostCommandPayload(result taskManager.HostCommandResult) map[string]interface{} {
	return map[string]interface{}{
		"session_timeout": true,
		"request_id":      result.RequestID,
		"session_id":      result.SessionID,
		"source":          result.Source,
		"command_preview": summarizeCommandPreview(result.Command),
	}
}

func buildBackgroundCommandPayload(readResult taskManager.BackgroundProcessReadResult) map[string]interface{} {
	session := readResult.Session
	payload := map[string]interface{}{
		"status":             session.Status,
		"completed":          !session.Running,
		"background":         true,
		"session_id":         session.SessionID,
		"source":             session.Source,
		"command":            session.Command,
		"command_preview":    session.CommandPreview,
		"workdir":            session.Workdir,
		"pid":                session.PID,
		"started_at":         session.StartedAt,
		"updated_at":         session.UpdatedAt,
		"ended_at":           session.EndedAt,
		"running":            session.Running,
		"running_for_sec":    session.RunningForSec,
		"stdout":             readResult.Stdout,
		"stderr":             readResult.Stderr,
		"stdout_offset":      readResult.StdoutOffset,
		"stderr_offset":      readResult.StderrOffset,
		"stdout_total":       readResult.StdoutTotal,
		"stderr_total":       readResult.StderrTotal,
		"stdout_truncated":   readResult.StdoutTruncated,
		"stderr_truncated":   readResult.StderrTruncated,
		"wait_event":         readResult.WaitEvent,
		"has_new_output":     readResult.HasNewOutput,
		"waited_ms":          readResult.WaitedMs,
		"state_changed_only": readResult.StateChangedOnly,
		"exit_code":          session.ExitCode,
		"timed_out":          session.TimedOut,
		"terminated_by":      session.TerminatedBy,
		"termination_reason": session.TerminationReason,
		"output_preview":     session.OutputPreview,
	}
	if combined := combineCommandOutput(readResult.Stdout, readResult.Stderr, session.OutputPreview); combined != "" {
		payload["output"] = combined
	}
	if strings.TrimSpace(session.Error) != "" {
		payload["error"] = session.Error
	}
	return payload
}

func buildHostCommandPayload(result taskManager.HostCommandResult) map[string]interface{} {
	payload := map[string]interface{}{
		"request_id":         result.RequestID,
		"status":             result.Status,
		"approved":           result.Approved,
		"completed":          !result.Running,
		"background":         result.Background,
		"session_id":         result.SessionID,
		"source":             result.Source,
		"command":            result.Command,
		"command_preview":    summarizeCommandPreview(result.Command),
		"workdir":            result.Workdir,
		"pid":                result.PID,
		"started_at":         result.StartedAt,
		"updated_at":         result.UpdatedAt,
		"ended_at":           result.EndedAt,
		"running":            result.Running,
		"running_for_sec":    result.RunningForSec,
		"stdout":             result.Stdout,
		"stderr":             result.Stderr,
		"stdout_offset":      result.StdoutOffset,
		"stderr_offset":      result.StderrOffset,
		"stdout_total":       result.StdoutTotal,
		"stderr_total":       result.StderrTotal,
		"wait_event":         result.WaitEvent,
		"has_new_output":     result.HasNewOutput,
		"decision_note":      result.DecisionNote,
		"exit_code":          result.ExitCode,
		"timed_out":          result.TimedOut,
		"terminated_by":      result.TerminatedBy,
		"termination_reason": result.TerminationReason,
		"output_truncated":   result.OutputTruncated,
	}
	if combined := combineCommandOutput(result.Stdout, result.Stderr, result.Output); combined != "" {
		payload["output"] = combined
	}
	if strings.TrimSpace(result.Error) != "" {
		payload["error"] = result.Error
	}
	return payload
}

func isDeferredBackgroundSession(readResult taskManager.BackgroundProcessReadResult) bool {
	return readResult.Session.Running && strings.TrimSpace(readResult.WaitEvent) == "timeout"
}

func isDeferredHostCommand(result taskManager.HostCommandResult) bool {
	return result.Running && strings.TrimSpace(result.WaitEvent) == "timeout"
}

func backgroundCommandSucceeded(readResult taskManager.BackgroundProcessReadResult) bool {
	status := normalizeCommandStatus(readResult.Session.Status, readResult.Session.Running)
	switch status {
	case "starting", "running", "completed":
		return true
	default:
		return false
	}
}

func hostCommandSucceeded(result taskManager.HostCommandResult) bool {
	if !result.Approved {
		return false
	}
	status := normalizeCommandStatus(result.Status, result.Running)
	switch status {
	case "approved", "starting", "running", "completed":
		return true
	default:
		return false
	}
}

func normalizeCommandStatus(status string, running bool) string {
	status = strings.TrimSpace(status)
	if status != "" {
		return status
	}
	if running {
		return "running"
	}
	return "completed"
}

func combineCommandOutput(stdout string, stderr string, fallback string) string {
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	switch {
	case stdout != "" && stderr != "":
		return stdout + "\n[stderr]\n" + stderr
	case stdout != "":
		return stdout
	case stderr != "":
		return stderr
	default:
		return strings.TrimSpace(fallback)
	}
}

func summarizeCommandPreview(command string) string {
	command = strings.Join(strings.Fields(strings.TrimSpace(command)), " ")
	if len(command) <= 160 {
		return command
	}
	return command[:157] + "..."
}
