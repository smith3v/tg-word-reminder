package testutil

import (
	"testing"

	"github.com/smith3v/tg-word-reminder/pkg/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func SetupTestDB(t *testing.T) {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	if err := gdb.AutoMigrate(&db.WordPair{}, &db.UserSettings{}, &db.GameSessionStatistics{}, &db.TrainingSession{}, &db.GameSession{}); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	db.DB = gdb

	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("failed to access underlying DB: %v", err)
	}

	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("failed to close database: %v", err)
		}
		db.DB = nil
	})
}
