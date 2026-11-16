package toolCalling

import (
	"AIxVuln/taskManager"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const defaultToolUsageGuideSessionTTL = 30 * time.Minute

type toolUsageGuideResult struct {
	Answer    string
	SessionID string
	Created   bool
	Closed    bool
}

type toolUsageGuideResponder interface {
	answer(question string) (string, error)
}

type toolUsageGuideSession struct {
	id         string
	scope      string
	agent      toolUsageGuideResponder
	createdAt  time.Time
	lastUsedAt time.Time
	mu         sync.Mutex
}

type toolUsageGuideSessionStore struct {
	mu       sync.Mutex
	sessions map[string]*toolUsageGuideSession
	ttl      time.Duration
	now      func() time.Time
}

var newToolUsageGuideResponder = func(task *taskManager.Task) toolUsageGuideResponder {
	return newToolUsageGuideAgent(task)
}

var toolUsageGuideSessions = newToolUsageGuideSessionStore(defaultToolUsageGuideSessionTTL, time.Now)

func newToolUsageGuideSessionStore(ttl time.Duration, now func() time.Time) *toolUsageGuideSessionStore {
	if ttl <= 0 {
		ttl = defaultToolUsageGuideSessionTTL
	}
	if now == nil {
		now = time.Now
	}
	return &toolUsageGuideSessionStore{
		sessions: make(map[string]*toolUsageGuideSession),
		ttl:      ttl,
		now:      now,
	}
}

func defaultToolUsageGuideAnswer(task *taskManager.Task, req toolUsageGuideRequest) (toolUsageGuideResult, error) {
	return toolUsageGuideSessions.answer(task, req)
}

func (s *toolUsageGuideSessionStore) answer(task *taskManager.Task, req toolUsageGuideRequest) (toolUsageGuideResult, error) {
	req.Question = strings.TrimSpace(req.Question)
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.Scope = strings.TrimSpace(req.Scope)
	switch {
	case req.Question == "" && !req.CloseSession:
		return toolUsageGuideResult{}, fmt.Errorf("question is required")
	case req.Question == "" && req.CloseSession && req.SessionID == "":
		return toolUsageGuideResult{}, fmt.Errorf("session_id is required when closing without a question")
	}

	session, created, err := s.getOrCreate(task, req.SessionID, req.Scope)
	if err != nil {
		return toolUsageGuideResult{}, err
	}

	result := toolUsageGuideResult{
		SessionID: session.id,
		Created:   created,
	}

	if req.Question != "" {
		session.mu.Lock()
		answer, answerErr := session.agent.answer(req.Question)
		session.mu.Unlock()
		if answerErr != nil {
			if created {
				s.remove(session.id)
			}
			return result, answerErr
		}
		result.Answer = strings.TrimSpace(answer)
		s.touch(session.id)
	}

	if req.CloseSession {
		closed, closeErr := s.close(task, session.id, req.Scope)
		if closeErr != nil {
			return result, closeErr
		}
		result.Closed = closed
	}

	return result, nil
}

func (s *toolUsageGuideSessionStore) getOrCreate(task *taskManager.Task, sessionID string, scopeOverride string) (*toolUsageGuideSession, bool, error) {
	now := s.now()
	scope := toolUsageGuideSessionScope(task, scopeOverride)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked(now)
	if sessionID != "" {
		session, ok := s.sessions[sessionID]
		if !ok || session.scope != scope {
			return nil, false, fmt.Errorf("tool usage guide session %q not found or expired; start a new AskToolUsageGuideTool session without session_id", sessionID)
		}
		session.lastUsedAt = now
		return session, false, nil
	}

	session := &toolUsageGuideSession{
		id:         newToolUsageGuideSessionID(),
		scope:      scope,
		agent:      newToolUsageGuideResponder(task),
		createdAt:  now,
		lastUsedAt: now,
	}
	s.sessions[session.id] = session
	return session, true, nil
}

func newToolUsageGuideSessionID() string {
	raw := strings.ReplaceAll(uuid.New().String(), "-", "")
	if len(raw) > 10 {
		raw = raw[:10]
	}
	return "tug-" + raw
}

func (s *toolUsageGuideSessionStore) close(task *taskManager.Task, sessionID string, scopeOverride string) (bool, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false, fmt.Errorf("session_id is required")
	}
	scope := toolUsageGuideSessionScope(task, scopeOverride)
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked(now)
	session, ok := s.sessions[sessionID]
	if !ok || session.scope != scope {
		return false, fmt.Errorf("tool usage guide session %q not found or expired; start a new AskToolUsageGuideTool session without session_id", sessionID)
	}
	delete(s.sessions, sessionID)
	return true, nil
}

func (s *toolUsageGuideSessionStore) touch(sessionID string) {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.sessions[sessionID]; ok {
		session.lastUsedAt = now
	}
}

func (s *toolUsageGuideSessionStore) remove(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, strings.TrimSpace(sessionID))
}

func (s *toolUsageGuideSessionStore) cleanupExpiredLocked(now time.Time) {
	if s.ttl <= 0 {
		return
	}
	for id, session := range s.sessions {
		if now.Sub(session.lastUsedAt) > s.ttl {
			delete(s.sessions, id)
		}
	}
}

func toolUsageGuideSessionScope(task *taskManager.Task, scopeOverride string) string {
	return helperLookupSessionScope(task, scopeOverride)
}
