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
	if err := DB.AutoMigrate(&WordPair{}, &UserSettings{}); err != nil {
		logger.Error("failed to auto-migrate database", "error", err)
		return err
	}
	return nil
}
