package logger

import (
	"log/slog"
	"os"
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

func SetLogLevel(level LogLevel) {
	currentLevel = level
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
