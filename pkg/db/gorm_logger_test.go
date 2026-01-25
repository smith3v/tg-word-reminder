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
	gormlogger "gorm.io/gorm/logger"
)

func TestGormLoggerTraceLevels(t *testing.T) {
	originalLogger := logger.Logger
	t.Cleanup(func() {
		logger.Logger = originalLogger
		logger.SetLogLevel(logger.INFO)
	})

	var buf bytes.Buffer
	logger.Logger = slog.New(slog.NewTextHandler(&buf, nil))

	lg, err := newGormLogger("info")
	if err != nil {
		t.Fatalf("failed to create gorm logger: %v", err)
	}
	l := lg.(*gormSlogLogger)
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

func TestNewGormLoggerDefaultsToWarn(t *testing.T) {
	lg, err := newGormLogger("")
	if err != nil {
		t.Fatalf("unexpected error for default gorm logger: %v", err)
	}
	l := lg.(*gormSlogLogger)
	if l.logLevel != gormlogger.Warn {
		t.Fatalf("expected default gorm log level warn, got: %v", l.logLevel)
	}
}

func TestNewGormLoggerInvalidLevelDefaultsToWarn(t *testing.T) {
	lg, err := newGormLogger("nope")
	if err == nil {
		t.Fatalf("expected error for invalid gorm level")
	}
	l := lg.(*gormSlogLogger)
	if l.logLevel != gormlogger.Warn {
		t.Fatalf("expected default gorm log level warn for invalid input, got: %v", l.logLevel)
	}
}
