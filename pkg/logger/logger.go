package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New создаёт *zap.Logger, настроенный для продакшена.
// Логи в JSON с ISO8601 таймстемпом — Grafana парсит корректно.
func New(level string) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()

	if level != "" {
		lvl, err := zap.ParseAtomicLevel(level)
		if err != nil {
			return nil, err
		}
		cfg.Level = lvl
	}

	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.LevelKey = "level"
	cfg.EncoderConfig.CallerKey = "caller"
	cfg.EncoderConfig.MessageKey = "message"

	return cfg.Build()
}
