package config

import (
	"log"
	"net/url"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

// Config env проекта.
type Config struct {
	// === REMNAWAVE ===
	RemnaPanel string `env:"REMNA_PANEL,required"`

	RemnawaveBaseURL   string
	RemnawaveAPIKey    string
	RemnawaveToken     string `env:"REMNA_TOKEN,required"`
	RemnawaveSquadUUID string `env:"REMNA_SQUAD_UUID,required"`

	// === TELEGRAM ===
	TelegramToken   string `env:"TELEGRAM_TOKEN,required"`
	TelegramSupport string `env:"TELEGRAM_SUPPORT,required"`
	TelegramAdminID string `env:"TELEGRAM_ADMIN_ID"`

	// === PAYMENT ===
	PaymentProvider string `env:"PAYMENT_PROVIDER,required"`
	// PLATEGA
	PlategaMerchantID string `env:"PLATEGA_MERCHANT_ID"`
	PlategaSecret     string `env:"PLATEGA_API_KEY"`
	PlategaReturnURL  string `env:"PLATEGA_RETURN_URL"`

	DatabaseURL string `env:"DATABASE_URL"`

	// === SETTINGS ===
	PricePerMonth     int    `env:"PRICE_PER_MONTH"`
	DeviceLimit       int    `env:"DEVICE_LIMIT"`
	TrafficLimit      string `env:"TRAFFIC_LIMIT"`
	MaxDeviceLimit    string `env:"MAX_DEVICE_LIMIT"`
	ExtraDevicePrice  string `env:"EXTRA_DEVICE_PRICE"`
	ResetTrafficPrice string `env:"RESET_TRAFFIC_PRICE"`
	TrialDays         int    `env:"TRIAL_DAYS"`

	LoggerLevel string `env:"LOGGER_LEVEL" envDefault:"info"`
	Encoding    string `env:"ENCODING"     envDefault:"console"`
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Fatalln(err)
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatalln(err)
	}

	baseURL, apiKey, err := parseRemna(cfg.RemnaPanel)
	if err != nil {
		return Config{}
	}

	cfg.RemnawaveBaseURL = baseURL
	cfg.RemnawaveAPIKey = apiKey

	return cfg
}

func parseRemna(raw string) (baseURL, secretToken string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}

	if u.Scheme == "" || u.Host == "" {
		return "", "", err
	}

	if u.RawQuery == "" {
		return "", "", err
	}

	base := url.URL{Scheme: u.Scheme, Host: u.Host}

	return base.String(), u.RawQuery, nil
}
