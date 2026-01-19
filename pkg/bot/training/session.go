package training

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
)

type Session struct {
	chatID            int64
	userID            int64
	queue             []db.WordPair
	currentPair       *db.WordPair
	currentToken      string
	currentMessageID  int
	currentPromptText string
}

func (s *Session) CurrentPair() *db.WordPair {
	if s == nil {
		return nil
	}
	return s.currentPair
}

func (s *Session) CurrentToken() string {
	if s == nil {
		return ""
	}
	return s.currentToken
}

type SessionSnapshot struct {
	Pair       db.WordPair
	Token      string
	MessageID  int
	PromptText string
	HasPrompt  bool
	HasMessage bool
}

type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	now      func() time.Time
}

func NewSessionManager(now func() time.Time) *SessionManager {
	if now == nil {
		now = time.Now
	}
	return &SessionManager{
		sessions: make(map[string]*Session),
		now:      now,
	}
}

var DefaultManager = NewSessionManager(nil)

func ResetDefaultManager(now func() time.Time) {
	DefaultManager = NewSessionManager(now)
}

func (m *SessionManager) StartOrRestart(chatID, userID int64, pairs []db.WordPair) *Session {
	session := &Session{
		chatID: chatID,
		userID: userID,
		queue:  append([]db.WordPair(nil), pairs...),
	}
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	m.sessions[key] = session
	m.nextPromptLocked(session)
	m.mu.Unlock()
	return session
}

func (m *SessionManager) GetSession(chatID, userID int64) *Session {
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[key]
}

func (m *SessionManager) Snapshot(chatID, userID int64) (SessionSnapshot, bool) {
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	defer m.mu.Unlock()
	session := m.sessions[key]
	if session == nil || session.currentPair == nil {
		return SessionSnapshot{}, false
	}
	return SessionSnapshot{
		Pair:       *session.currentPair,
		Token:      session.currentToken,
		MessageID:  session.currentMessageID,
		PromptText: session.currentPromptText,
		HasPrompt:  session.currentPromptText != "",
		HasMessage: session.currentMessageID != 0,
	}, true
}

func (m *SessionManager) SetCurrentMessageID(session *Session, messageID int) {
	if session == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	session.currentMessageID = messageID
}

func (m *SessionManager) SetCurrentPromptText(session *Session, text string) {
	if session == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	session.currentPromptText = text
}

func (m *SessionManager) Advance(chatID, userID int64) (*db.WordPair, string) {
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	defer m.mu.Unlock()
	session := m.sessions[key]
	if session == nil {
		return nil, ""
	}
	if !m.nextPromptLocked(session) {
		delete(m.sessions, key)
		return nil, ""
	}
	return session.currentPair, session.currentToken
}

func getSessionKey(chatID, userID int64) string {
	return fmt.Sprintf("%d:%d", chatID, userID)
}

func (m *SessionManager) nextPromptLocked(session *Session) bool {
	if session == nil || len(session.queue) == 0 {
		session.currentPair = nil
		return false
	}
	pair := session.queue[0]
	session.queue = session.queue[1:]
	session.currentPair = &pair
	session.currentToken = m.nextTokenLocked()
	session.currentMessageID = 0
	session.currentPromptText = ""
	return true
}

func (m *SessionManager) nextTokenLocked() string {
	return fmt.Sprintf("%x", rand.Int63())
}
