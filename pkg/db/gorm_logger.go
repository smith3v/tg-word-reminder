package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/smith3v/tg-word-reminder/pkg/logger"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	defaultSlowThreshold = 200 * time.Millisecond
	defaultGormLogLevel  = gormlogger.Warn
)

type gormSlogLogger struct {
	slowThreshold             time.Duration
	ignoreRecordNotFoundError bool
	logLevel                  gormlogger.LogLevel
}

func newGormLogger(levelValue string) (gormlogger.Interface, error) {
	level := defaultGormLogLevel
	var levelErr error
	if strings.TrimSpace(levelValue) != "" {
		level, levelErr = parseGormLogLevel(levelValue)
	}
	return &gormSlogLogger{
		slowThreshold:             defaultSlowThreshold,
		ignoreRecordNotFoundError: true,
		logLevel:                  level,
	}, levelErr
}

func (l *gormSlogLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	clone := *l
	clone.logLevel = level
	return &clone
}

func (l *gormSlogLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if !l.enabled(gormlogger.Info) {
		return
	}
	logger.Logger.Log(ctx, slog.LevelInfo, fmt.Sprintf(msg, data...))
}

func (l *gormSlogLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if !l.enabled(gormlogger.Warn) {
		return
	}
	logger.Logger.Log(ctx, slog.LevelWarn, fmt.Sprintf(msg, data...))
}

func (l *gormSlogLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if !l.enabled(gormlogger.Error) {
		return
	}
	logger.Logger.Log(ctx, slog.LevelError, fmt.Sprintf(msg, data...))
}

func (l *gormSlogLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.logLevel == gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	if err != nil {
		if l.ignoreRecordNotFoundError && errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		if l.enabled(gormlogger.Error) {
			logger.Logger.Log(
				ctx,
				slog.LevelError,
				"gorm query error",
				"elapsed",
				elapsed,
				"rows",
				rows,
				"sql",
				sql,
				"error",
				err,
			)
		}
		return
	}

	if l.slowThreshold > 0 && elapsed > l.slowThreshold {
		if l.enabled(gormlogger.Warn) {
			logger.Logger.Log(
				ctx,
				slog.LevelWarn,
				"gorm slow query",
				"elapsed",
				elapsed,
				"rows",
				rows,
				"sql",
				sql,
				"threshold",
				l.slowThreshold,
			)
		}
		return
	}

	if l.enabled(gormlogger.Info) {
		logger.Logger.Log(
			ctx,
			slog.LevelInfo,
			"gorm query",
			"elapsed",
			elapsed,
			"rows",
			rows,
			"sql",
			sql,
		)
	}
}

func (l *gormSlogLogger) enabled(level gormlogger.LogLevel) bool {
	if l.logLevel == gormlogger.Silent || l.logLevel < level {
		return false
	}
	switch level {
	case gormlogger.Info, gormlogger.Warn:
		return logger.Enabled(logger.INFO)
	case gormlogger.Error:
		return logger.Enabled(logger.ERROR)
	default:
		return false
	}
}

func parseGormLogLevel(value string) (gormlogger.LogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "silent":
		return gormlogger.Silent, nil
	case "error":
		return gormlogger.Error, nil
	case "warn":
		return gormlogger.Warn, nil
	case "info":
		return gormlogger.Info, nil
	default:
		return defaultGormLogLevel, fmt.Errorf("invalid gorm log level %q", value)
	}
}
