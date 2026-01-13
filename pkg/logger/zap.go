// Package logger реализует логирование с использованием zap
package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// zapLogger реализация интерфейса Logger через библиотеку zap.
type zapLogger struct {
	logger *zap.Logger
}

// NewZap создаем экземпляр zap логгер.
func NewZap(level string) (Logger, error) {
	// Конфигурация по умолчанию (Prod)
	cfg := zap.NewProductionConfig()

	// Установка уровня логирования если не задано
	if level != "" {
		l, err := zap.ParseAtomicLevel(level)

		if err != nil {
			return nil, fmt.Errorf("ошибка при парсинге уровня логирования: %w", err)
		}

		cfg.Level = l
	}

	// Настройка формата времени
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	// Настройка формата длительности (например, "1.5s" вместо числа наносекунд)
	cfg.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder

	// AddCallerSkil(1) нужен, чтобы в логах указывалось место вызова методов
	// интерфейса, а не методов обертки zapLogger.
	//
	// Если ставить 1, то он ссылкой ведет прям туда где была ошибка (к примеру main:15)
	// А если 0, то на самого себя. (к примеру logger:15)
	l, err := cfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		return nil, fmt.Errorf("ошибка при билде логгера: %w", err)
	}

	return &zapLogger{
		logger: l,
	}, nil
}

// Debug логирует сообщение с уровнем Debug.
func (l *zapLogger) Debug(msg string, fields ...Field) {
	l.logger.Debug(msg, l.toZapFields(fields)...)
}

// Info логирует сообщение с уровнем Info.
func (l *zapLogger) Info(msg string, fields ...Field) {
	l.logger.Info(msg, l.toZapFields(fields)...)
}

// Warn логирует сообщение с уровнем Warn.
func (l *zapLogger) Warn(msg string, fields ...Field) {
	l.logger.Warn(msg, l.toZapFields(fields)...)
}

// Error логирует сообщение с уровнем Error.
func (l *zapLogger) Error(msg string, fields ...Field) {
	l.logger.Error(msg, l.toZapFields(fields)...)
}

// With добавляет поля к логгеру.
func (l *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{logger: l.logger.With(l.toZapFields(fields)...)}
}

// Named возвращает новый логгер с добавленным именем (категорией).
// Это позволяет разделять логи по модулям (например, "remnawave", "telegram").
func (l *zapLogger) Named(name string) Logger {
	return &zapLogger{
		logger: l.logger.Named(name),
	}
}

// Sync сбрасывает буфер логгера.
func (l *zapLogger) Sync() error {
	if err := l.logger.Sync(); err != nil {
		return fmt.Errorf("ошибка при синхронизации логгера: %w", err)
	}

	return nil
}

// toZapFields конвертирует внутренние поля в поля zap.
func (l *zapLogger) toZapFields(fields []Field) []zap.Field {
	zf := make([]zap.Field, len(fields))
	for i, f := range fields {
		zf[i] = zap.Any(f.Key, f.Value)
	}

	return zf
}
