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
		},
		"feedback": {
			"enabled": true,
			"admin_ids": [123456789, 987654321],
			"timeout_minutes": 5
		},
		"onboarding": {
			"init_vocabulary_path": "/app/vocabularies/multilang.csv"
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
	if !AppConfig.Feedback.Enabled {
		t.Errorf("expected feedback enabled to be true")
	}
	if len(AppConfig.Feedback.AdminIDs) != 2 {
		t.Errorf("expected 2 admin IDs, got %d", len(AppConfig.Feedback.AdminIDs))
	}
	if AppConfig.Feedback.TimeoutMinutes != 5 {
		t.Errorf("expected feedback timeout minutes to be 5, got %d", AppConfig.Feedback.TimeoutMinutes)
	}
	if AppConfig.Onboarding.InitVocabularyPath != "/app/vocabularies/multilang.csv" {
		t.Errorf("expected onboarding path to be set, got %q", AppConfig.Onboarding.InitVocabularyPath)
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
