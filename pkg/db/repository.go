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
	gormLogger, gormErr := newGormLogger(config.AppConfig.Logging.GormLevel)
	if gormErr != nil {
		logger.Error("invalid gorm log level", "value", config.AppConfig.Logging.GormLevel, "error", gormErr)
	}
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormLogger})
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return err
	}
	if err := DB.AutoMigrate(&WordPair{}, &UserSettings{}, &GameSession{}, &TrainingSession{}, &GameSessionState{}); err != nil {
		logger.Error("failed to auto-migrate database", "error", err)
		return err
	}
	if err := migrateReminderSlots(DB); err != nil {
		logger.Error("failed to migrate reminder slots", "error", err)
		return err
	}
	if err := migrateNewRanks(DB); err != nil {
		logger.Error("failed to migrate new ranks", "error", err)
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

func migrateNewRanks(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasColumn(&WordPair{}, "srs_new_rank") {
		return nil
	}

	switch db.Dialector.Name() {
	case "sqlite":
		return db.Exec(`
UPDATE word_pairs
SET srs_new_rank = abs(random()) % 1000000000 + 1
WHERE srs_new_rank = 0
`).Error
	case "postgres":
		return db.Exec(`
UPDATE word_pairs
SET srs_new_rank = floor(random() * 1000000000)::int + 1
WHERE srs_new_rank = 0
`).Error
	default:
		return db.Exec(`
UPDATE word_pairs
SET srs_new_rank = 1
WHERE srs_new_rank = 0
`).Error
	}
}
