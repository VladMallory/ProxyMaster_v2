package logger

import (
	"ProxyMaster_v2/internal/domain"
	"log/slog"
	"os"
)

// slogLogger реализация интерфейса Logger через стандартный log/slog.
type slogLogger struct {
	logger *slog.Logger
}

// NewSlog создаем экземпляр slog логгер.
func NewSlog(level domain.LevelLogger) (Logger, error) {
	var l slog.Level

	switch level {
	case domain.LevelDebug:
		l = slog.LevelDebug
	case domain.LevelWarn:
		l = slog.LevelWarn
	case domain.LevelError:
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	// Создаем JSON хендлер, который пишет в stdout
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: l,
	})

	return &slogLogger{
		logger: slog.New(handler),
	}, nil
}

// Debug выводит сообщение в лог с уровнем Debug.
func (l *slogLogger) Debug(msg string, fields ...Field) {
	l.logger.Debug(msg, l.args(fields)...)
}

// Info выводит сообщение в лог с уровнем Info.
func (l *slogLogger) Info(msg string, fields ...Field) {
	l.logger.Info(msg, l.args(fields)...)
}

// Warn выводит сообщение в лог с уровнем Warn.
func (l *slogLogger) Warn(msg string, fields ...Field) {
	l.logger.Warn(msg, l.args(fields)...)
}

// Error выводит сообщение в лог с уровнем Error.
func (l *slogLogger) Error(msg string, fields ...Field) {
	l.logger.Error(msg, l.args(fields)...)
}

// With выводит сообщение в лог с уровнем With.
func (l *slogLogger) With(fields ...Field) Logger {
	return &slogLogger{logger: l.logger.With(l.args(fields)...)}
}

// Named то как будет называться сервис который логируем.
func (l *slogLogger) Named(name string) Logger {
	// В slog нет прямого аналога Named, но мы можем просто добавить поле logger
	return l.With(Field{Key: "logger", Value: name})
}

// Sync синхронизирует логгер с базой данных.
func (l *slogLogger) Sync() error {
	return nil // slog пишет сразу, sync не нужен
}

// args преобразует []Field в []any для slog
func (l *slogLogger) args(fields []Field) []any {
	args := make([]any, 0, len(fields)*2)
	for _, f := range fields {
		args = append(args, f.Key, f.Value)
	}

	return args
}
