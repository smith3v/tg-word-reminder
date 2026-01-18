// pkg/db/repository.go
package db

import (
	"strconv"

	"github.com/smith3v/tg-word-reminder/pkg/config"
	"github.com/smith3v/tg-word-reminder/pkg/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Export DB variable
var DB *gorm.DB

func InitDB(cfg config.DatabaseConfig) error {
	var err error
	dsn := "host=" + cfg.Host +
		" user=" + cfg.User +
		" password=" + cfg.Password +
		" dbname=" + cfg.DBName +
		" port=" + strconv.Itoa(cfg.Port) +
		" sslmode=" + cfg.SSLMode
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return err
	}
	if err := DB.AutoMigrate(&WordPair{}, &UserSettings{}, &GameSession{}); err != nil {
		logger.Error("failed to auto-migrate database", "error", err)
		return err
	}
	if err := migrateReminderSlots(DB); err != nil {
		logger.Error("failed to migrate reminder slots", "error", err)
		return err
	}
	return nil
}

func migrateReminderSlots(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasColumn(&UserSettings{}, "reminders_per_day") {
		return nil
	}
	query := `
UPDATE user_settings
SET
  reminder_morning = CASE
    WHEN reminders_per_day = 2 THEN TRUE
    WHEN reminders_per_day > 2 THEN TRUE
    ELSE reminder_morning
  END,
  reminder_afternoon = CASE
    WHEN reminders_per_day > 2 THEN TRUE
    ELSE reminder_afternoon
  END,
  reminder_evening = CASE
    WHEN reminders_per_day >= 1 THEN TRUE
    ELSE reminder_evening
  END
WHERE reminders_per_day IS NOT NULL
  AND reminder_morning = FALSE
  AND reminder_afternoon = FALSE
  AND reminder_evening = FALSE
`
	return db.Exec(query).Error
}
