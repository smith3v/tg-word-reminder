package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLogLevelFiltering(t *testing.T) {
	originalLogger := Logger
	t.Cleanup(func() {
		Logger = originalLogger
		SetLogLevel(INFO)
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
