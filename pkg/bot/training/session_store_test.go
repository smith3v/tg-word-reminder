package training

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
	"gorm.io/datatypes"
)

func TestTrainingSessionStoreRoundTrip(t *testing.T) {
	testutil.SetupTestDB(t)

	now := time.Date(2025, 1, 3, 10, 0, 0, 0, time.UTC)
	pairIDs := []uint{11, 22, 33}
	raw, err := json.Marshal(pairIDs)
	if err != nil {
		t.Fatalf("failed to marshal pair IDs: %v", err)
	}

	session := &db.TrainingSession{
		ChatID:           100,
		UserID:           200,
		PairIDs:          datatypes.JSON(raw),
		CurrentIndex:     1,
		CurrentToken:     "tok",
		CurrentMessageID: 99,
		LastActivityAt:   now,
	}
	if err := UpsertTrainingSession(session); err != nil {
		t.Fatalf("failed to upsert session: %v", err)
	}

	loaded, err := LoadTrainingSession(100, 200, now)
	if err != nil {
		t.Fatalf("failed to load session: %v", err)
	}
	if loaded == nil {
		t.Fatalf("expected session to load")
	}
	if loaded.CurrentIndex != 1 || loaded.CurrentToken != "tok" || loaded.CurrentMessageID != 99 {
		t.Fatalf("unexpected fields: %+v", loaded)
	}

	var gotIDs []uint
	if err := json.Unmarshal(loaded.PairIDs, &gotIDs); err != nil {
		t.Fatalf("failed to unmarshal pair IDs: %v", err)
	}
	if len(gotIDs) != 3 || gotIDs[0] != 11 || gotIDs[1] != 22 || gotIDs[2] != 33 {
		t.Fatalf("unexpected pair IDs: %+v", gotIDs)
	}
	if !loaded.ExpiresAt.Equal(now.Add(trainingSessionTTL)) {
		t.Fatalf("expected expires_at to be last_activity + ttl, got %v", loaded.ExpiresAt)
	}
}

func TestTrainingSessionStoreExpired(t *testing.T) {
	testutil.SetupTestDB(t)

	now := time.Date(2025, 1, 3, 10, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Hour)
	raw, err := json.Marshal([]uint{1})
	if err != nil {
		t.Fatalf("failed to marshal pair IDs: %v", err)
	}

	session := db.TrainingSession{
		ChatID:           300,
		UserID:           400,
		PairIDs:          datatypes.JSON(raw),
		CurrentIndex:     0,
		CurrentToken:     "old",
		CurrentMessageID: 12,
		LastActivityAt:   expiredAt.Add(-trainingSessionTTL),
		ExpiresAt:        expiredAt,
	}
	if err := db.DB.Create(&session).Error; err != nil {
		t.Fatalf("failed to seed session: %v", err)
	}

	loaded, err := LoadTrainingSession(300, 400, now)
	if err != nil {
		t.Fatalf("failed to load session: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected expired session to be skipped")
	}
}

func TestTrainingSessionStoreDelete(t *testing.T) {
	testutil.SetupTestDB(t)

	raw, err := json.Marshal([]uint{5})
	if err != nil {
		t.Fatalf("failed to marshal pair IDs: %v", err)
	}
	session := db.TrainingSession{
		ChatID:           500,
		UserID:           600,
		PairIDs:          datatypes.JSON(raw),
		CurrentIndex:     0,
		CurrentToken:     "t",
		CurrentMessageID: 1,
		LastActivityAt:   time.Now().UTC(),
		ExpiresAt:        time.Now().UTC().Add(trainingSessionTTL),
	}
	if err := db.DB.Create(&session).Error; err != nil {
		t.Fatalf("failed to seed session: %v", err)
	}

	if err := DeleteTrainingSession(500, 600); err != nil {
		t.Fatalf("failed to delete session: %v", err)
	}

	var count int64
	if err := db.DB.Model(&db.TrainingSession{}).
		Where("chat_id = ? AND user_id = ?", 500, 600).
		Count(&count).Error; err != nil {
		t.Fatalf("failed to count sessions: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected session to be deleted, got %d", count)
	}
}
