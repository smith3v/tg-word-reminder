package handlers

import (
	"context"
	"testing"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"github.com/smith3v/tg-word-reminder/pkg/internal/testutil"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestActivityTrackerTouchDedup(t *testing.T) {
	tracker := NewActivityTracker()
	tracker.Touch(42)
	tracker.Touch(42)
	tracker.Touch(0)

	if got := len(tracker.pending); got != 1 {
		t.Fatalf("expected 1 pending entry, got %d", got)
	}
}

func TestActivityTrackerFlushClearsOnSuccess(t *testing.T) {
	testutil.SetupTestDB(t)

	settings := db.UserSettings{
		UserID:                 7,
		TrainingPaused:         true,
		MissedTrainingSessions: 3,
	}
	if err := db.DB.Create(&settings).Error; err != nil {
		t.Fatalf("failed to seed user settings: %v", err)
	}

	tracker := NewActivityTracker()
	tracker.Touch(settings.UserID)

	if err := tracker.Flush(context.Background()); err != nil {
		t.Fatalf("expected flush to succeed: %v", err)
	}

	var updated db.UserSettings
	if err := db.DB.Where("user_id = ?", settings.UserID).First(&updated).Error; err != nil {
		t.Fatalf("failed to reload user settings: %v", err)
	}
	if updated.TrainingPaused || updated.MissedTrainingSessions != 0 {
		t.Fatalf("expected user to be unpaused with zero misses, got paused=%v missed=%d", updated.TrainingPaused, updated.MissedTrainingSessions)
	}
	if got := len(tracker.pending); got != 0 {
		t.Fatalf("expected pending entries to be cleared, got %d", got)
	}
}

func TestActivityTrackerFlushKeepsOnError(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	previous := db.DB
	db.DB = gdb

	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("failed to access underlying db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
		db.DB = previous
	})

	tracker := NewActivityTracker()
	tracker.Touch(11)

	if err := tracker.Flush(context.Background()); err == nil {
		t.Fatalf("expected flush to fail without schema")
	}
	if got := len(tracker.pending); got != 1 {
		t.Fatalf("expected pending entry to remain, got %d", got)
	}
}
