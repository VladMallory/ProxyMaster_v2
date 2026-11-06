package autopay

import (
	"errors"
	"io/fs"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Autopay       bool
	DatabaseURL   string
	ShopID        string
	SecretKey     string
	ReturnURL     string
	PanelBaseURL  string
	PanelAPIKey   string
	PanelToken    string
	TelegramToken string
	PricePerMonth int
}

func Enabled() bool {
	return strings.EqualFold(os.Getenv("YOOKASSA_AUTOPAY"), "true")
}

func LoadConfig() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Config{}, err
	}

	price, err := strconv.Atoi(os.Getenv("PRICE_PER_MONTH"))
	if err != nil {
		return Config{}, errors.New("PRICE_PER_MONTH должен быть числом")
	}

	panelBase, panelKey, err := parsePanel(os.Getenv("REMNA_PANEL"))
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Autopay:       Enabled(),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		ShopID:        os.Getenv("YOUKASSA_SHOP_ID"),
		SecretKey:     os.Getenv("YOUKASSA_SECRET_KEY"),
		ReturnURL:     os.Getenv("YOUKASSA_RETURN_URL"),
		PanelBaseURL:  panelBase,
		PanelAPIKey:   panelKey,
		PanelToken:    os.Getenv("REMNA_TOKEN"),
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		PricePerMonth: price,
	}

	switch {
	case cfg.DatabaseURL == "":
		return Config{}, errors.New("DATABASE_URL пустой")
	case cfg.ShopID == "" || cfg.SecretKey == "":
		return Config{}, errors.New("YOUKASSA_SHOP_ID или YOUKASSA_SECRET_KEY пустые")
	case cfg.PanelToken == "":
		return Config{}, errors.New("REMNA_TOKEN пустой")
	case cfg.PricePerMonth <= 0:
		return Config{}, errors.New("PRICE_PER_MONTH должен быть больше нуля")
	}

	return cfg, nil
}

func parsePanel(raw string) (string, string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", err
	}

	if u.Scheme == "" || u.Host == "" || u.RawQuery == "" {
		return "", "", errors.New("REMNA_PANEL должен содержать схему, хост и ключ")
	}

	base := url.URL{Scheme: u.Scheme, Host: u.Host}

	return base.String(), u.RawQuery, nil
}
