package onboarding

import (
	"testing"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
	"gorm.io/datatypes"
)

func TestProvisionUserVocabularyAndDefaults(t *testing.T) {
	testutil.SetupTestDB(t)

	rows := []db.InitVocabulary{
		{EN: "hello", RU: "привет", NL: "hallo", ES: "hola", DE: "hallo", FR: "salut"},
		{EN: "bye", RU: "", NL: "dag", ES: "adios", DE: "tschuss", FR: "au revoir"},
	}
	if err := db.DB.Create(&rows).Error; err != nil {
		t.Fatalf("failed to seed init rows: %v", err)
	}
	if _, err := Begin(5001); err != nil {
		t.Fatalf("failed to create onboarding state: %v", err)
	}

	inserted, err := ProvisionUserVocabularyAndDefaults(5001, "en", "ru")
	if err != nil {
		t.Fatalf("provision failed: %v", err)
	}
	if inserted != 1 {
		t.Fatalf("expected 1 inserted pair, got %d", inserted)
	}

	var pairCount int64
	if err := db.DB.Model(&db.WordPair{}).Where("user_id = ?", 5001).Count(&pairCount).Error; err != nil {
		t.Fatalf("failed to count pairs: %v", err)
	}
	if pairCount != 1 {
		t.Fatalf("expected 1 pair, got %d", pairCount)
	}

	var settings db.UserSettings
	if err := db.DB.Where("user_id = ?", 5001).First(&settings).Error; err != nil {
		t.Fatalf("failed to load settings: %v", err)
	}
	if settings.PairsToSend != 5 || !settings.ReminderMorning || !settings.ReminderAfternoon || !settings.ReminderEvening {
		t.Fatalf("unexpected settings defaults: %+v", settings)
	}
}

func TestResetUserDataTxPreservesGameStatistics(t *testing.T) {
	testutil.SetupTestDB(t)

	userID := int64(7001)
	now := time.Now().UTC()
	if err := db.DB.Create(&db.WordPair{UserID: userID, Word1: "a", Word2: "b", SrsState: "new", SrsDueAt: now}).Error; err != nil {
		t.Fatalf("failed to seed pair: %v", err)
	}
	if err := db.DB.Create(&db.UserSettings{UserID: userID, PairsToSend: 3}).Error; err != nil {
		t.Fatalf("failed to seed settings: %v", err)
	}
	if err := db.DB.Create(&db.TrainingSession{UserID: userID, ChatID: userID, PairIDs: datatypes.JSON("[]"), LastActivityAt: now, ExpiresAt: now.Add(time.Hour)}).Error; err != nil {
		t.Fatalf("failed to seed training session: %v", err)
	}
	if err := db.DB.Create(&db.GameSession{UserID: userID, ChatID: userID, PairIDs: datatypes.JSON("[]"), LastActivityAt: now, ExpiresAt: now.Add(time.Hour)}).Error; err != nil {
		t.Fatalf("failed to seed game session: %v", err)
	}
	if err := db.DB.Create(&db.GameSessionStatistics{UserID: userID, SessionDate: now, StartedAt: now}).Error; err != nil {
		t.Fatalf("failed to seed game stats: %v", err)
	}

	if err := ResetUserDataTx(userID); err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	assertZeroRows(t, &db.WordPair{}, userID)
	assertZeroRows(t, &db.UserSettings{}, userID)
	assertZeroRows(t, &db.TrainingSession{}, userID)
	assertZeroRows(t, &db.GameSession{}, userID)

	var statsCount int64
	if err := db.DB.Model(&db.GameSessionStatistics{}).Where("user_id = ?", userID).Count(&statsCount).Error; err != nil {
		t.Fatalf("failed to count stats: %v", err)
	}
	if statsCount != 1 {
		t.Fatalf("expected game stats to remain, got %d", statsCount)
	}
}

func assertZeroRows(t *testing.T, model any, userID int64) {
	t.Helper()
	var count int64
	if err := db.DB.Model(model).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count rows for %T: %v", model, err)
	}
	if count != 0 {
		t.Fatalf("expected no rows for %T, got %d", model, count)
	}
}
