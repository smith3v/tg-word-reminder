package training

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const (
	OverdueCallbackPrefix   = "t:overdue:"
	OverduePromptExpiration = 24 * time.Hour
)

type OverduePrompt struct {
	token     string
	messageID int
	createdAt time.Time
}

type OverdueManager struct {
	mu      sync.Mutex
	prompts map[string]OverduePrompt
	now     func() time.Time
}

func NewOverdueManager(now func() time.Time) *OverdueManager {
	if now == nil {
		now = time.Now
	}
	return &OverdueManager{
		prompts: make(map[string]OverduePrompt),
		now:     now,
	}
}

var DefaultOverdue = NewOverdueManager(nil)

func ResetOverdueManager(now func() time.Time) {
	DefaultOverdue = NewOverdueManager(now)
}

func (m *OverdueManager) Start(chatID, userID int64) string {
	token := fmt.Sprintf("%x", rand.Int63())
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prompts[key] = OverduePrompt{
		token:     token,
		createdAt: m.now(),
	}
	return token
}

func (m *OverdueManager) BindMessage(chatID, userID int64, token string, messageID int) {
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	defer m.mu.Unlock()
	prompt := m.prompts[key]
	if prompt.token != token {
		return
	}
	prompt.messageID = messageID
	m.prompts[key] = prompt
}

func (m *OverdueManager) Validate(chatID, userID int64, token string, messageID int) bool {
	key := getSessionKey(chatID, userID)
	m.mu.Lock()
	defer m.mu.Unlock()
	prompt, ok := m.prompts[key]
	if !ok {
		return false
	}
	if prompt.token != token || prompt.messageID != messageID {
		return false
	}
	delete(m.prompts, key)
	return true
}

func (m *OverdueManager) SweepExpired(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, prompt := range m.prompts {
		if now.Sub(prompt.createdAt) > OverduePromptExpiration {
			delete(m.prompts, key)
		}
	}
}
