package config

import (
	"errors"
	"io/fs"
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

	DatabaseURL string `env:"DATABASE_URL" envDefault:"postgres://app:change_me@localhost:5432/app?sslmode=disable"`

	// === SETTINGS ===
	PricePerMonth       int    `env:"PRICE_PER_MONTH"`
	DeviceLimit         int    `env:"DEVICE_LIMIT"`
	TrafficLimit        int64  `env:"TRAFFIC_LIMIT"`
	MaxDeviceLimit      int    `env:"MAX_DEVICE_LIMIT"    envDefault:"5"`
	ExtraDevicePriceRub int    `env:"EXTRA_DEVICE_PRICE"  envDefault:"50"`
	ResetTrafficPrice   string `env:"RESET_TRAFFIC_PRICE"`
	TrialDays           int    `env:"TRIAL_DAYS"          envDefault:"30"`

	LoggerLevel string `env:"LOGGER_LEVEL" envDefault:"info"`
	Encoding    string `env:"ENCODING"     envDefault:"console"`
}

func Load() Config {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatalln(err)
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatalln(err)
	}

	baseURL, apiKey := parseRemna(cfg.RemnaPanel)

	cfg.RemnawaveBaseURL = baseURL
	cfg.RemnawaveAPIKey = apiKey

	return cfg
}

func parseRemna(raw string) (baseURL, secretToken string) {
	u, err := url.Parse(raw)
	if err != nil {
		log.Fatalln(err)
	}

	if u.Scheme == "" || u.Host == "" {
		log.Fatalln("Scheme и Host не указаны")
	}

	if u.RawQuery == "" {
		log.Fatalln(err)
	}

	base := url.URL{Scheme: u.Scheme, Host: u.Host}

	return base.String(), u.RawQuery
}
