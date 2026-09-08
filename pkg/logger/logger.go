package logger

import (
	"log/slog"
	"os"
)

var Logger = slog.New(
	slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}),
)

func Info(message string, args ...any) {
	Logger.Info(message, args...)
}

func Warn(message string, args ...any) {
	Logger.Warn(message, args...)
}

func Error(message string, args ...any) {
	Logger.Error(message, args...)
}
