package zaplogger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	LogLevel,
	Encoding string
}

func New(cfg Config) *zap.Logger {
	var zapConfig zap.Config

	switch cfg.Encoding {
	case "json":
		zapConfig = zap.NewProductionConfig()
		zapConfig.Encoding = "json"
		zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	case "console":
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.Encoding = "console"
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		zapConfig.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")
		zapConfig.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	default:
		panic("unsupported log encoding: %q")
	}

	zapConfig.OutputPaths = []string{"stdout"}
	zapConfig.ErrorOutputPaths = []string{"stderr"}

	if cfg.LogLevel != "" {
		level, err := zapcore.ParseLevel(cfg.LogLevel)
		if err != nil {
			panic("invalid log level")
		}

		zapConfig.Level = zap.NewAtomicLevelAt(level)
	}

	logger, err := zapConfig.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		panic("build logger")
	}

	return logger
}
