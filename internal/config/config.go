package config

import (
	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

type Config struct {
	RemnawaveBaseURL   string `env:"REMNA_BASE_PANEL"`
	RemnawaveToken     string `env:"REMNA_TOKEN"`
	RemnawaveAPIKey    string `env:"REMNA_SECRET_TOKEN"`
	RemnawaveSquadUUID string `env:"REMNA_SQUAD_UUID"`

	TelegramToken   string `env:"TELEGRAM_TOKEN"`
	TelegramSupport string `env:"TELEGRAM_SUPPORT"`
	TelegramAdminID string `env:"TELEGRAM_ADMIN_ID"`

	PaymentProvider string `env:"PAYMENT_PROVIDER"`

	DatabaseURL string `env:"DATABASE_URL"`

	PricePerMonth     string `env:"PRICE_PER_MONTH"`
	DeviceLimit       string `env:"DEVICE_LIMIT"`
	TrafficLimit      string `env:"TRAFFIC_LIMIT"`
	MaxDeviceLimit    string `env:"MAX_DEVICE_LIMIT"`
	ExtraDevicePrice  string `env:"EXTRA_DEVICE_PRICE"`
	ResetTrafficPrice string `env:"RESET_TRAFFIC_PRICE"`

	LoggerLevel string `env:"LOGGER_LEVEL" default:"info"`
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
