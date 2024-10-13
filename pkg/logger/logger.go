package logger

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

func init() {
	Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
}

func Error(msg string, args ...any) {
	Logger.Error(msg, args...)
}

func Info(msg string, args ...any) {
	Logger.Info(msg, args...)
}
