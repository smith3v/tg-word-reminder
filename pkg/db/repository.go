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
	if err := migrateGameSessionTables(DB); err != nil {
		logger.Error("failed to migrate game session tables", "error", err)
		return err
	}
	if err := DB.AutoMigrate(&WordPair{}, &UserSettings{}, &GameSessionStatistics{}, &TrainingSession{}, &GameSession{}); err != nil {
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

func migrateGameSessionTables(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	migrator := db.Migrator()
	renamedStats := false
	if migrator.HasTable("game_sessions") {
		hasSessionDate := migrator.HasColumn("game_sessions", "session_date")
		hasPairIDs := migrator.HasColumn("game_sessions", "pair_ids")
		if hasSessionDate && !hasPairIDs {
			if !migrator.HasTable("game_session_statistics") {
				if err := migrator.RenameTable("game_sessions", "game_session_statistics"); err != nil {
					return err
				}
				renamedStats = true
			}
		}
	}
	if migrator.HasTable("game_session_states") {
		if renamedStats || !migrator.HasTable("game_sessions") {
			if err := migrator.RenameTable("game_session_states", "game_sessions"); err != nil {
				return err
			}
		}
	}
	if db.Dialector.Name() == "postgres" {
		if err := migrateGameSessionSequences(db); err != nil {
			return err
		}
	}
	return nil
}

func migrateGameSessionSequences(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	return db.Exec(`
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_class WHERE relname = 'game_sessions_id_seq') THEN
    ALTER SEQUENCE game_sessions_id_seq RENAME TO game_session_statistics_id_seq;
  END IF;

  IF EXISTS (SELECT 1 FROM pg_class WHERE relname = 'game_session_statistics_id_seq') THEN
    ALTER TABLE game_session_statistics
      ALTER COLUMN id SET DEFAULT nextval('game_session_statistics_id_seq'::regclass);
    ALTER SEQUENCE game_session_statistics_id_seq
      OWNED BY game_session_statistics.id;
  END IF;

  IF EXISTS (SELECT 1 FROM pg_class WHERE relname = 'game_session_states_id_seq') THEN
    ALTER SEQUENCE game_session_states_id_seq RENAME TO game_sessions_id_seq;
  END IF;

  IF EXISTS (SELECT 1 FROM pg_class WHERE relname = 'game_sessions_id_seq') THEN
    ALTER TABLE game_sessions
      ALTER COLUMN id SET DEFAULT nextval('game_sessions_id_seq'::regclass);
    ALTER SEQUENCE game_sessions_id_seq
      OWNED BY game_sessions.id;
  END IF;
END $$;
`).Error
}
