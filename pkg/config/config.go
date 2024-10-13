package config

import (
	"encoding/json"
	"os"

	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

type Config struct {
	Database DatabaseConfig `json:"database"`
	Telegram TelegramConfig `json:"telegram"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
	Port     int    `json:"port"`
	SSLMode  string `json:"sslmode"`
}

type TelegramConfig struct {
	Token string `json:"token"`
}

var AppConfig Config

func LoadConfig(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		logger.Error("failed to open config file", "error", err)
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&AppConfig); err != nil {
		logger.Error("failed to decode config file", "error", err)
		return err
	}

	return nil
}
