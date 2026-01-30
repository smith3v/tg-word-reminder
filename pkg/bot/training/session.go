package training

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
	"gorm.io/datatypes"
)

type Session struct {
	chatID            int64
	userID            int64
	queue             []db.WordPair
	pairIDs           []uint
	currentPair       *db.WordPair
	currentToken      string
	currentMessageID  int
	currentPromptText string
	lastActivityAt    time.Time
	currentIndex      int
	totalPairs        int
	reviewedCount     int
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

const (
	SessionInactivityTimeout = 24 * time.Hour
	SessionSweeperInterval   = 10 * time.Minute
)

func StartTrainingSweeper(ctx context.Context) {
	DefaultManager.StartSweeper(ctx)
}

func (m *SessionManager) StartOrRestart(chatID, userID int64, pairs []db.WordPair) *Session {
	now := m.now()
	pairIDs := make([]uint, 0, len(pairs))
	for _, pair := range pairs {
		if pair.ID != 0 {
			pairIDs = append(pairIDs, pair.ID)
		}
	}
	session := &Session{
		chatID:         chatID,
		userID:         userID,
		queue:          append([]db.WordPair(nil), pairs...),
		pairIDs:        pairIDs,
		lastActivityAt: now,
		currentIndex:   -1,
		totalPairs:     len(pairs),
	}
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	m.sessions[key] = session
	m.nextPromptLocked(session)
	dbSession, err := buildTrainingSession(session)
	m.mu.Unlock()
	if err != nil {
		logger.Error("failed to build training session", "user_id", userID, "error", err)
		return session
	}
	if err := UpsertTrainingSession(dbSession); err != nil {
		logger.Error("failed to persist training session", "user_id", userID, "error", err)
	}
	return session
}

func (m *SessionManager) StartFromPersisted(row *db.TrainingSession, pairs []db.WordPair) (*Session, error) {
	if row == nil {
		return nil, errors.New("nil training session row")
	}
	var pairIDs []uint
	if err := json.Unmarshal(row.PairIDs, &pairIDs); err != nil {
		return nil, err
	}
	ordered := make([]db.WordPair, 0, len(pairIDs))
	indexed := make(map[uint]db.WordPair, len(pairs))
	for _, pair := range pairs {
		if pair.ID != 0 {
			indexed[pair.ID] = pair
		}
	}
	for _, id := range pairIDs {
		if pair, ok := indexed[id]; ok {
			ordered = append(ordered, pair)
		}
	}
	if row.CurrentIndex < 0 || row.CurrentIndex >= len(ordered) {
		return nil, errors.New("current index out of range")
	}

	session := &Session{
		chatID:            row.ChatID,
		userID:            row.UserID,
		queue:             append([]db.WordPair(nil), ordered[row.CurrentIndex+1:]...),
		pairIDs:           pairIDs,
		currentPair:       &ordered[row.CurrentIndex],
		currentToken:      row.CurrentToken,
		currentMessageID:  row.CurrentMessageID,
		currentPromptText: row.CurrentPromptText,
		lastActivityAt:    row.LastActivityAt,
		currentIndex:      row.CurrentIndex,
		totalPairs:        len(ordered),
		reviewedCount:     row.CurrentIndex,
	}
	key := getSessionKey(row.ChatID, row.UserID)
	m.mu.Lock()
	m.sessions[key] = session
	m.mu.Unlock()
	return session, nil
}

func (m *SessionManager) MarkReviewed(chatID, userID int64) (int, int) {
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	session := m.sessions[key]
	if session == nil {
		m.mu.Unlock()
		return 0, 0
	}
	session.lastActivityAt = m.now()
	if session.reviewedCount < session.totalPairs {
		session.reviewedCount++
	}
	dbSession, err := buildTrainingSession(session)
	reviewedCount := session.reviewedCount
	totalPairs := session.totalPairs
	m.mu.Unlock()
	if err != nil {
		logger.Error("failed to build training session", "user_id", userID, "error", err)
		return reviewedCount, totalPairs
	}
	if err := UpsertTrainingSession(dbSession); err != nil {
		logger.Error("failed to persist training session", "user_id", userID, "error", err)
	}
	return reviewedCount, totalPairs
}

func (m *SessionManager) GetSession(chatID, userID int64) *Session {
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[key]
}

func (m *SessionManager) End(chatID, userID int64) {
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	delete(m.sessions, key)
	m.mu.Unlock()
	if err := DeleteTrainingSession(chatID, userID); err != nil {
		logger.Error("failed to delete training session", "user_id", userID, "error", err)
	}
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
	session.currentMessageID = messageID
	dbSession, err := buildTrainingSession(session)
	m.mu.Unlock()
	if err != nil {
		logger.Error("failed to build training session", "user_id", session.userID, "error", err)
		return
	}
	if err := UpsertTrainingSession(dbSession); err != nil {
		logger.Error("failed to persist training session", "user_id", session.userID, "error", err)
	}
}

func (m *SessionManager) SetCurrentPromptText(session *Session, text string) {
	if session == nil {
		return
	}
	m.mu.Lock()
	session.currentPromptText = text
	dbSession, err := buildTrainingSession(session)
	m.mu.Unlock()
	if err != nil {
		logger.Error("failed to build training session", "user_id", session.userID, "error", err)
		return
	}
	if err := UpsertTrainingSession(dbSession); err != nil {
		logger.Error("failed to persist training session", "user_id", session.userID, "error", err)
	}
}

func (m *SessionManager) Touch(chatID, userID int64) {
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	defer m.mu.Unlock()
	session := m.sessions[key]
	if session == nil {
		return
	}
	session.lastActivityAt = m.now()
}

func (m *SessionManager) Advance(chatID, userID int64) (*db.WordPair, string) {
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	session := m.sessions[key]
	if session == nil {
		m.mu.Unlock()
		return nil, ""
	}
	session.lastActivityAt = m.now()
	if !m.nextPromptLocked(session) {
		delete(m.sessions, key)
		m.mu.Unlock()
		if err := DeleteTrainingSession(chatID, userID); err != nil {
			logger.Error("failed to delete training session", "user_id", userID, "error", err)
		}
		return nil, ""
	}
	dbSession, err := buildTrainingSession(session)
	if err != nil {
		logger.Error("failed to build training session", "user_id", userID, "error", err)
		m.mu.Unlock()
		return session.currentPair, session.currentToken
	}
	m.mu.Unlock()
	if err := UpsertTrainingSession(dbSession); err != nil {
		logger.Error("failed to persist training session", "user_id", userID, "error", err)
	}
	return session.currentPair, session.currentToken
}

func (m *SessionManager) StartSweeper(ctx context.Context) {
	ticker := time.NewTicker(SessionSweeperInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.SweepInactive(m.now())
		}
	}
}

func (m *SessionManager) SweepInactive(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, session := range m.sessions {
		if session == nil {
			delete(m.sessions, key)
			continue
		}
		if now.Sub(session.lastActivityAt) > SessionInactivityTimeout {
			delete(m.sessions, key)
		}
	}
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
	session.currentIndex++
	return true
}

func (m *SessionManager) nextTokenLocked() string {
	return fmt.Sprintf("%x", rand.Int63())
}

func buildTrainingSession(session *Session) (*db.TrainingSession, error) {
	if session == nil {
		return nil, errors.New("nil session")
	}
	raw, err := json.Marshal(session.pairIDs)
	if err != nil {
		return nil, err
	}
	return &db.TrainingSession{
		ChatID:            session.chatID,
		UserID:            session.userID,
		PairIDs:           datatypes.JSON(raw),
		CurrentIndex:      session.currentIndex,
		CurrentToken:      session.currentToken,
		CurrentMessageID:  session.currentMessageID,
		CurrentPromptText: session.currentPromptText,
		LastActivityAt:    session.lastActivityAt,
	}, nil
}
