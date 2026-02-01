package db

import (
	"testing"
	"time"

	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCleanupExpiredSessions(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file:session_cleanup?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	if err := gdb.AutoMigrate(&TrainingSession{}, &GameSession{}); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}
	DB = gdb

	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("failed to access underlying DB: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("failed to close database: %v", err)
		}
		DB = nil
	})

	now := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	raw := datatypes.JSON([]byte("[]"))

	expiredTraining := TrainingSession{
		ChatID:         1,
		UserID:         1,
		PairIDs:        raw,
		LastActivityAt: now.Add(-48 * time.Hour),
		ExpiresAt:      now.Add(-24 * time.Hour),
	}
	activeTraining := TrainingSession{
		ChatID:         2,
		UserID:         2,
		PairIDs:        raw,
		LastActivityAt: now,
		ExpiresAt:      now.Add(24 * time.Hour),
	}
	expiredGame := GameSession{
		ChatID:         3,
		UserID:         3,
		PairIDs:        raw,
		LastActivityAt: now.Add(-48 * time.Hour),
		ExpiresAt:      now.Add(-24 * time.Hour),
	}
	activeGame := GameSession{
		ChatID:         4,
		UserID:         4,
		PairIDs:        raw,
		LastActivityAt: now,
		ExpiresAt:      now.Add(24 * time.Hour),
	}

	if err := gdb.Create(&expiredTraining).Error; err != nil {
		t.Fatalf("failed to seed expired training: %v", err)
	}
	if err := gdb.Create(&activeTraining).Error; err != nil {
		t.Fatalf("failed to seed active training: %v", err)
	}
	if err := gdb.Create(&expiredGame).Error; err != nil {
		t.Fatalf("failed to seed expired game: %v", err)
	}
	if err := gdb.Create(&activeGame).Error; err != nil {
		t.Fatalf("failed to seed active game: %v", err)
	}

	deleted, err := CleanupExpiredSessions(now)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deleted rows, got %d", deleted)
	}

	var trainingCount int64
	if err := gdb.Model(&TrainingSession{}).Count(&trainingCount).Error; err != nil {
		t.Fatalf("failed to count training sessions: %v", err)
	}
	if trainingCount != 1 {
		t.Fatalf("expected 1 training session remaining, got %d", trainingCount)
	}

	var gameCount int64
	if err := gdb.Model(&GameSession{}).Count(&gameCount).Error; err != nil {
		t.Fatalf("failed to count game sessions: %v", err)
	}
	if gameCount != 1 {
		t.Fatalf("expected 1 game session remaining, got %d", gameCount)
	}
}
