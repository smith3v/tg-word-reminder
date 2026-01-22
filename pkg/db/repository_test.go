package db

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateReminderSlots(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	if err := gdb.AutoMigrate(&WordPair{}, &UserSettings{}, &GameSession{}); err != nil {
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

	cases := []struct {
		userID          int64
		remindersPerDay int
		wantMorning     bool
		wantAfternoon   bool
		wantEvening     bool
	}{
		{userID: 1, remindersPerDay: 0, wantMorning: false, wantAfternoon: false, wantEvening: false},
		{userID: 2, remindersPerDay: 1, wantMorning: false, wantAfternoon: false, wantEvening: true},
		{userID: 3, remindersPerDay: 2, wantMorning: true, wantAfternoon: false, wantEvening: true},
		{userID: 4, remindersPerDay: 3, wantMorning: true, wantAfternoon: true, wantEvening: true},
	}

	for _, tc := range cases {
		if err := DB.Create(&UserSettings{
			UserID:          tc.userID,
			RemindersPerDay: tc.remindersPerDay,
		}).Error; err != nil {
			t.Fatalf("failed to create user settings: %v", err)
		}
		if err := DB.Model(&UserSettings{}).
			Where("user_id = ?", tc.userID).
			Update("reminders_per_day", tc.remindersPerDay).Error; err != nil {
			t.Fatalf("failed to set reminders_per_day for user %d: %v", tc.userID, err)
		}
	}

	if err := DB.Create(&UserSettings{
		UserID:          10,
		RemindersPerDay: 1,
		ReminderMorning: true,
	}).Error; err != nil {
		t.Fatalf("failed to create preconfigured user: %v", err)
	}
	if err := DB.Model(&UserSettings{}).
		Where("user_id = ?", 10).
		Update("reminders_per_day", 1).Error; err != nil {
		t.Fatalf("failed to set reminders_per_day for preconfigured user: %v", err)
	}

	if err := migrateReminderSlots(DB); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	for _, tc := range cases {
		var settings UserSettings
		if err := DB.Where("user_id = ?", tc.userID).First(&settings).Error; err != nil {
			t.Fatalf("failed to load user settings for user %d: %v", tc.userID, err)
		}
		if settings.ReminderMorning != tc.wantMorning {
			t.Fatalf("user %d reminder_morning: got %v want %v", tc.userID, settings.ReminderMorning, tc.wantMorning)
		}
		if settings.ReminderAfternoon != tc.wantAfternoon {
			t.Fatalf("user %d reminder_afternoon: got %v want %v", tc.userID, settings.ReminderAfternoon, tc.wantAfternoon)
		}
		if settings.ReminderEvening != tc.wantEvening {
			t.Fatalf("user %d reminder_evening: got %v want %v", tc.userID, settings.ReminderEvening, tc.wantEvening)
		}
	}

	var preconfigured UserSettings
	if err := DB.Where("user_id = ?", 10).First(&preconfigured).Error; err != nil {
		t.Fatalf("failed to load preconfigured user: %v", err)
	}
	if !preconfigured.ReminderMorning || preconfigured.ReminderEvening {
		t.Fatalf("preconfigured user should not be overwritten, got morning=%v evening=%v", preconfigured.ReminderMorning, preconfigured.ReminderEvening)
	}
}

func TestMigrateNewRanks(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	if err := gdb.AutoMigrate(&WordPair{}, &UserSettings{}, &GameSession{}); err != nil {
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

	pair := WordPair{
		UserID:   12,
		Word1:    "alpha",
		Word2:    "beta",
		SrsState: "new",
		SrsDueAt: time.Now().UTC(),
	}
	if err := DB.Create(&pair).Error; err != nil {
		t.Fatalf("failed to create word pair: %v", err)
	}

	if err := migrateNewRanks(DB); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	var updated WordPair
	if err := DB.First(&updated, pair.ID).Error; err != nil {
		t.Fatalf("failed to reload word pair: %v", err)
	}
	if updated.SrsNewRank == 0 {
		t.Fatalf("expected srs_new_rank to be set, got %+v", updated)
	}
}
