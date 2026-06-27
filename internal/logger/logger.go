package logger

import (
	"log/slog"
	"os"
)

var log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelDebug,
}))

func Debug(msg string, args ...any) {
	log.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	log.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	log.Warn(msg, args...)
}

func Error(msg string, err error, args ...any) {
	log.Error(msg, append([]any{"error", err}, args...)...)
}

func Fatal(msg string, err error, args ...any) {
	log.Error(msg, append([]any{"error", err}, args...)...)
	os.Exit(1)
}
