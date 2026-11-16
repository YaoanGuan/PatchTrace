package toolCalling

import (
	"AIxVuln/taskManager"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const defaultPocKnowledgeSessionTTL = 30 * time.Minute

type pocKnowledgeResponder interface {
	answer(question string) (string, error)
}

type pocKnowledgeSession struct {
	id         string
	scope      string
	agent      pocKnowledgeResponder
	createdAt  time.Time
	lastUsedAt time.Time
	mu         sync.Mutex
}

type pocKnowledgeSessionStore struct {
	mu       sync.Mutex
	sessions map[string]*pocKnowledgeSession
	ttl      time.Duration
	now      func() time.Time
}

var newPocKnowledgeResponder = func(task *taskManager.Task) pocKnowledgeResponder {
	return newPocKnowledgeAgent(task)
}

var pocKnowledgeSessions = newPocKnowledgeSessionStore(defaultPocKnowledgeSessionTTL, time.Now)
var pocKnowledgeAnswerFunc = defaultPocKnowledgeAnswer

func newPocKnowledgeSessionStore(ttl time.Duration, now func() time.Time) *pocKnowledgeSessionStore {
	if ttl <= 0 {
		ttl = defaultPocKnowledgeSessionTTL
	}
	if now == nil {
		now = time.Now
	}
	return &pocKnowledgeSessionStore{
		sessions: make(map[string]*pocKnowledgeSession),
		ttl:      ttl,
		now:      now,
	}
}

func defaultPocKnowledgeAnswer(task *taskManager.Task, req pocKnowledgeRequest) (pocKnowledgeResult, error) {
	return pocKnowledgeSessions.answer(task, req)
}

func (s *pocKnowledgeSessionStore) answer(task *taskManager.Task, req pocKnowledgeRequest) (pocKnowledgeResult, error) {
	req.Question = strings.TrimSpace(req.Question)
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.Scope = strings.TrimSpace(req.Scope)
	switch {
	case req.Question == "" && !req.CloseSession:
		return pocKnowledgeResult{}, fmt.Errorf("question is required")
	case req.Question == "" && req.CloseSession && req.SessionID == "":
		return pocKnowledgeResult{}, fmt.Errorf("session_id is required when closing without a question")
	}

	session, created, err := s.getOrCreate(task, req.SessionID, req.Scope)
	if err != nil {
		return pocKnowledgeResult{}, err
	}

	result := pocKnowledgeResult{
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

func (s *pocKnowledgeSessionStore) getOrCreate(task *taskManager.Task, sessionID string, scopeOverride string) (*pocKnowledgeSession, bool, error) {
	now := s.now()
	scope := pocKnowledgeSessionScope(task, scopeOverride)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked(now)
	if sessionID != "" {
		session, ok := s.sessions[sessionID]
		if !ok || session.scope != scope {
			return nil, false, fmt.Errorf("poc knowledge session %q not found or expired; start a new AskPocKnowledgeTool session without session_id", sessionID)
		}
		session.lastUsedAt = now
		return session, false, nil
	}

	session := &pocKnowledgeSession{
		id:         newPocKnowledgeSessionID(),
		scope:      scope,
		agent:      newPocKnowledgeResponder(task),
		createdAt:  now,
		lastUsedAt: now,
	}
	s.sessions[session.id] = session
	return session, true, nil
}

func newPocKnowledgeSessionID() string {
	raw := strings.ReplaceAll(uuid.New().String(), "-", "")
	if len(raw) > 10 {
		raw = raw[:10]
	}
	return "pks-" + raw
}

func (s *pocKnowledgeSessionStore) close(task *taskManager.Task, sessionID string, scopeOverride string) (bool, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false, fmt.Errorf("session_id is required")
	}
	scope := pocKnowledgeSessionScope(task, scopeOverride)
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked(now)
	session, ok := s.sessions[sessionID]
	if !ok || session.scope != scope {
		return false, fmt.Errorf("poc knowledge session %q not found or expired; start a new AskPocKnowledgeTool session without session_id", sessionID)
	}
	delete(s.sessions, sessionID)
	return true, nil
}

func (s *pocKnowledgeSessionStore) touch(sessionID string) {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.sessions[sessionID]; ok {
		session.lastUsedAt = now
	}
}

func (s *pocKnowledgeSessionStore) remove(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, strings.TrimSpace(sessionID))
}

func (s *pocKnowledgeSessionStore) cleanupExpiredLocked(now time.Time) {
	if s.ttl <= 0 {
		return
	}
	for id, session := range s.sessions {
		if now.Sub(session.lastUsedAt) > s.ttl {
			delete(s.sessions, id)
		}
	}
}

func pocKnowledgeSessionScope(task *taskManager.Task, scopeOverride string) string {
	return helperLookupSessionScope(task, scopeOverride)
}
