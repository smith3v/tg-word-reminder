package training

import (
	"testing"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
)

func TestSweepInactiveRemovesExpiredSessions(t *testing.T) {
	now := time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC)
	current := now
	manager := NewSessionManager(func() time.Time { return current })

	session := manager.StartOrRestart(1, 1, []db.WordPair{{UserID: 1, Word1: "a", Word2: "b"}})
	if session == nil {
		t.Fatalf("expected session to be created")
	}

	current = now.Add(SessionInactivityTimeout - time.Minute)
	manager.SweepInactive(current)
	if manager.GetSession(1, 1) == nil {
		t.Fatalf("expected session to remain active")
	}

	current = now.Add(SessionInactivityTimeout + time.Minute)
	manager.SweepInactive(current)
	if manager.GetSession(1, 1) != nil {
		t.Fatalf("expected session to be swept")
	}
}
