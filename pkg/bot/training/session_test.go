package training

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
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

func TestStartOrRestartPersistsSession(t *testing.T) {
	testutil.SetupTestDB(t)

	now := time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)
	manager := NewSessionManager(func() time.Time { return now })
	pairs := []db.WordPair{
		{ID: 11, UserID: 2, Word1: "a", Word2: "b"},
		{ID: 22, UserID: 2, Word1: "c", Word2: "d"},
	}

	session := manager.StartOrRestart(1, 2, pairs)
	if session == nil {
		t.Fatalf("expected session to be created")
	}

	var stored db.TrainingSession
	if err := db.DB.Where("chat_id = ? AND user_id = ?", 1, 2).First(&stored).Error; err != nil {
		t.Fatalf("failed to load training session: %v", err)
	}
	if stored.CurrentIndex != 0 {
		t.Fatalf("expected current_index 0, got %d", stored.CurrentIndex)
	}
	var gotIDs []uint
	if err := json.Unmarshal(stored.PairIDs, &gotIDs); err != nil {
		t.Fatalf("failed to unmarshal pair IDs: %v", err)
	}
	if len(gotIDs) != 2 || gotIDs[0] != 11 || gotIDs[1] != 22 {
		t.Fatalf("unexpected pair IDs: %+v", gotIDs)
	}
	if !stored.ExpiresAt.Equal(now.Add(trainingSessionTTL)) {
		t.Fatalf("expected expires_at %v, got %v", now.Add(trainingSessionTTL), stored.ExpiresAt)
	}
}

func TestAdvanceUpdatesPersistedSession(t *testing.T) {
	testutil.SetupTestDB(t)

	now := time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)
	manager := NewSessionManager(func() time.Time { return now })
	pairs := []db.WordPair{
		{ID: 11, UserID: 2, Word1: "a", Word2: "b"},
		{ID: 22, UserID: 2, Word1: "c", Word2: "d"},
	}

	session := manager.StartOrRestart(1, 2, pairs)
	if session == nil {
		t.Fatalf("expected session to be created")
	}
	firstToken := session.CurrentToken()
	_, nextToken := manager.Advance(1, 2)
	if nextToken == "" {
		t.Fatalf("expected next token")
	}
	if nextToken == firstToken {
		t.Fatalf("expected token to change on advance")
	}

	var stored db.TrainingSession
	if err := db.DB.Where("chat_id = ? AND user_id = ?", 1, 2).First(&stored).Error; err != nil {
		t.Fatalf("failed to load training session: %v", err)
	}
	if stored.CurrentIndex != 1 {
		t.Fatalf("expected current_index 1, got %d", stored.CurrentIndex)
	}
	if stored.CurrentToken != nextToken {
		t.Fatalf("expected token %q, got %q", nextToken, stored.CurrentToken)
	}
}

func TestEndDeletesPersistedSession(t *testing.T) {
	testutil.SetupTestDB(t)

	now := time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)
	manager := NewSessionManager(func() time.Time { return now })
	pairs := []db.WordPair{{ID: 11, UserID: 2, Word1: "a", Word2: "b"}}

	session := manager.StartOrRestart(1, 2, pairs)
	if session == nil {
		t.Fatalf("expected session to be created")
	}

	manager.End(1, 2)

	var count int64
	if err := db.DB.Model(&db.TrainingSession{}).
		Where("chat_id = ? AND user_id = ?", 1, 2).
		Count(&count).Error; err != nil {
		t.Fatalf("failed to count sessions: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected session to be deleted, got %d", count)
	}
}
