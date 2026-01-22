package logger

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	ERROR
)

var (
	Logger       *slog.Logger
	currentLevel LogLevel = INFO // Default logging level
)

func init() {
	Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
}

type Options struct {
	Level string
	File  string
}

func Configure(opts Options) error {
	level := currentLevel
	var levelErr error
	if strings.TrimSpace(opts.Level) != "" {
		level, levelErr = ParseLogLevel(opts.Level)
	}

	writer := io.Writer(os.Stdout)
	var fileErr error
	if strings.TrimSpace(opts.File) != "" {
		dir := filepath.Dir(opts.File)
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			fileErr = mkErr
		} else {
			file, openErr := os.OpenFile(opts.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if openErr != nil {
				fileErr = openErr
			} else {
				writer = io.MultiWriter(os.Stdout, file)
			}
		}
	}

	currentLevel = level
	Logger = slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{Level: slogLevel(level)}))

	if levelErr != nil || fileErr != nil {
		return errors.Join(levelErr, fileErr)
	}
	return nil
}

func SetLogLevel(level LogLevel) {
	currentLevel = level
}

func ParseLogLevel(value string) (LogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return DEBUG, nil
	case "info":
		return INFO, nil
	case "error":
		return ERROR, nil
	default:
		return INFO, fmt.Errorf("invalid log level %q", value)
	}
}

func slogLevel(level LogLevel) slog.Level {
	switch level {
	case DEBUG:
		return slog.LevelDebug
	case ERROR:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func Debug(msg string, args ...any) {
	if currentLevel <= DEBUG {
		Logger.Debug(msg, args...)
	}
}

func Info(msg string, args ...any) {
	if currentLevel <= INFO {
		Logger.Info(msg, args...)
	}
}

func Error(msg string, args ...any) {
	if currentLevel <= ERROR {
		Logger.Error(msg, args...)
	}
}
