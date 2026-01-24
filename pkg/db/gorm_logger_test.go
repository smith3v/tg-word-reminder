package db

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/logger"
)

func TestGormLoggerTraceLevels(t *testing.T) {
	originalLogger := logger.Logger
	t.Cleanup(func() {
		logger.Logger = originalLogger
		logger.SetLogLevel(logger.INFO)
	})

	var buf bytes.Buffer
	logger.Logger = slog.New(slog.NewTextHandler(&buf, nil))

	l := newGormLogger().(*gormSlogLogger)
	ctx := context.Background()

	logger.SetLogLevel(logger.INFO)
	l.slowThreshold = time.Nanosecond
	l.Trace(ctx, time.Now().Add(-time.Millisecond), func() (string, int64) {
		return "SELECT 1", 1
	}, nil)
	if !strings.Contains(buf.String(), "gorm slow query") {
		t.Fatalf("expected slow query warning, got: %s", buf.String())
	}

	buf.Reset()
	l.slowThreshold = time.Hour
	l.Trace(ctx, time.Now().Add(-time.Millisecond), func() (string, int64) {
		return "SELECT 2", 1
	}, nil)
	if !strings.Contains(buf.String(), "gorm query") {
		t.Fatalf("expected info query log, got: %s", buf.String())
	}

	buf.Reset()
	logger.SetLogLevel(logger.ERROR)
	l.Trace(ctx, time.Now().Add(-time.Millisecond), func() (string, int64) {
		return "SELECT 3", 1
	}, errors.New("boom"))
	if !strings.Contains(buf.String(), "gorm query error") {
		t.Fatalf("expected error log, got: %s", buf.String())
	}
}
