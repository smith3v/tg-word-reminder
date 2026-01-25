package logger

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogLevelFiltering(t *testing.T) {
	originalLogger := Logger
	originalLevel := currentLevel
	t.Cleanup(func() {
		Logger = originalLogger
		SetLogLevel(originalLevel)
	})

	var buf bytes.Buffer
	Logger = slog.New(slog.NewTextHandler(&buf, nil))

	SetLogLevel(INFO)
	Debug("debug message should be filtered")
	Info("info message should appear")

	output := buf.String()
	if strings.Contains(output, "debug message should be filtered") {
		t.Fatalf("debug message was logged at INFO level:\n%s", output)
	}
	if !strings.Contains(output, "info message should appear") {
		t.Fatalf("info message was not logged:\n%s", output)
	}
}

func TestConfigureWritesToFileAndRespectsLevel(t *testing.T) {
	originalLogger := Logger
	originalLevel := currentLevel
	t.Cleanup(func() {
		Logger = originalLogger
		SetLogLevel(originalLevel)
	})

	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "nested", "tg-word-reminder.log")
	if err := Configure(Options{Level: "error", File: logPath}); err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}

	Info("info message should be filtered")
	Error("error message should appear")

	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	output := string(contents)
	if strings.Contains(output, "info message should be filtered") {
		t.Fatalf("info message was logged at ERROR level:\n%s", output)
	}
	if !strings.Contains(output, "error message should appear") {
		t.Fatalf("error message was not logged:\n%s", output)
	}
}

func TestConfigureInvalidLevelDefaultsToInfo(t *testing.T) {
	originalLogger := Logger
	originalLevel := currentLevel
	t.Cleanup(func() {
		Logger = originalLogger
		SetLogLevel(originalLevel)
	})

	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "tg-word-reminder.log")
	if err := Configure(Options{Level: "verbose", File: logPath}); err == nil {
		t.Fatalf("expected error for invalid log level")
	}

	Debug("debug message should be filtered")
	Info("info message should appear")

	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	output := string(contents)
	if strings.Contains(output, "debug message should be filtered") {
		t.Fatalf("debug message was logged at default INFO level:\n%s", output)
	}
	if !strings.Contains(output, "info message should appear") {
		t.Fatalf("info message was not logged:\n%s", output)
	}
}

func TestEnabledReportsLevel(t *testing.T) {
	originalLogger := Logger
	originalLevel := currentLevel
	t.Cleanup(func() {
		Logger = originalLogger
		SetLogLevel(originalLevel)
	})

	SetLogLevel(DEBUG)
	if !Enabled(DEBUG) || !Enabled(INFO) || !Enabled(ERROR) {
		t.Fatalf("expected DEBUG to allow all levels")
	}

	SetLogLevel(INFO)
	if Enabled(DEBUG) || !Enabled(INFO) || !Enabled(ERROR) {
		t.Fatalf("expected INFO to allow info and error only")
	}

	SetLogLevel(ERROR)
	if Enabled(DEBUG) || Enabled(INFO) || !Enabled(ERROR) {
		t.Fatalf("expected ERROR to allow error only")
	}
}
