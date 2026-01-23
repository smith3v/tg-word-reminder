package feedback

import (
	"context"
	"sync"
	"time"
)

type PendingFeedback struct {
	ChatID    int64
	ExpiresAt time.Time
}

type Manager struct {
	mu      sync.Mutex
	pending map[int64]PendingFeedback
	now     func() time.Time
}

func NewManager(now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	return &Manager{
		pending: make(map[int64]PendingFeedback),
		now:     now,
	}
}

var DefaultManager = NewManager(nil)

func ResetDefaultManager(now func() time.Time) {
	DefaultManager = NewManager(now)
}

func (m *Manager) Start(userID, chatID int64, now time.Time, timeout time.Duration) {
	if m == nil || userID == 0 || chatID == 0 {
		return
	}
	if now.IsZero() {
		now = m.now()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pending[userID] = PendingFeedback{
		ChatID:    chatID,
		ExpiresAt: now.Add(timeout),
	}
}

func (m *Manager) Consume(userID, chatID int64, now time.Time) bool {
	if m == nil || userID == 0 || chatID == 0 {
		return false
	}
	if now.IsZero() {
		now = m.now()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.pending[userID]
	if !ok {
		return false
	}
	if entry.ChatID != chatID {
		return false
	}
	if entry.ExpiresAt.IsZero() || !now.Before(entry.ExpiresAt) {
		delete(m.pending, userID)
		return false
	}
	delete(m.pending, userID)
	return true
}

func (m *Manager) SweepExpired(now time.Time) {
	if m == nil {
		return
	}
	if now.IsZero() {
		now = m.now()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for userID, entry := range m.pending {
		if entry.ExpiresAt.IsZero() || !now.Before(entry.ExpiresAt) {
			delete(m.pending, userID)
		}
	}
}

func (m *Manager) StartSweeper(ctx context.Context) {
	if m == nil || ctx == nil {
		return
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.SweepExpired(m.now())
		}
	}
}
