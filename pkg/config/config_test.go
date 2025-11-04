package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigSuccess(t *testing.T) {
	original := AppConfig
	t.Cleanup(func() {
		AppConfig = original
	})

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	content := `{
		"database": {
			"host": "localhost",
			"user": "test-user",
			"password": "test-pass",
			"dbname": "testdb",
			"port": 5433,
			"sslmode": "disable"
		},
		"telegram": {
			"token": "test-token"
		}
	}`

	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config fixture: %v", err)
	}

	if err := LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if AppConfig.Database.Host != "localhost" {
		t.Errorf("expected host to be localhost, got %q", AppConfig.Database.Host)
	}
	if AppConfig.Database.Port != 5433 {
		t.Errorf("expected port to be 5433, got %d", AppConfig.Database.Port)
	}
	if AppConfig.Telegram.Token != "test-token" {
		t.Errorf("expected token to be test-token, got %q", AppConfig.Telegram.Token)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	original := AppConfig
	t.Cleanup(func() {
		AppConfig = original
	})

	if err := LoadConfig(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected an error when loading a missing config file")
	}
}
