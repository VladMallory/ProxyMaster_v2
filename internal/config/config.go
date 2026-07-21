// Package config обращается к env, получает
// данные и возвращает в виде структуры
package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	RemnaPanelURL       string
	RemnaSecretURLToken string
	RemnaLogin          string
	RemnaPass           string
	RemnaKey            string
	RemnaSquadUUID      string
	RemnaDefaultGb      string

	TelegramToken   string
	TelegramSupport string
	TelegramAdminID string

	DatabaseURL       string
	PaymentProvider   string
	PlategaMerchantID string
	PlategaAPIKey     string
	PlategaReturnURL  string

	YouKassaShopID    string
	YouKassaSecretKey string
	YouKassaReturnURL string

	PricePerMonth     string
	ExtraDevicePrice  int
	ResetTrafficPrice int
	DeviceLimit       int
	MaxDeviceLimit    int
	TrafficLimit      int64

	LoggerLevel string

	SOCKS5Host     string
	SOCKS5Port     string
	SOCKS5Username string
	SOCKS5Password string
}

func New() (*Config, error) {
	_ = godotenv.Load()

	if err := injectVaultSecrets(); err != nil {
		return nil, fmt.Errorf("vault: %w", err)
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL не установлен")
	}

	deviceLimitStr := os.Getenv("DEVICE_LIMIT")
	deviceLimit, err := strconv.Atoi(deviceLimitStr)
	if err != nil || deviceLimit < 1 {
		log.Fatalln("укажите лимит устройств (DEVICE_LIMIT)")
	}

	maxDeviceLimitStr := os.Getenv("MAX_DEVICE_LIMIT")
	maxDeviceLimit, err := strconv.Atoi(maxDeviceLimitStr)
	if err != nil || maxDeviceLimit < 1 {
		log.Fatalln("укажите максимальный лимит устройств (MAX_DEVICE_LIMIT)")
	}

	trafficLimitStr := os.Getenv("TRAFFIC_LIMIT")
	trafficLimit, err := strconv.ParseInt(trafficLimitStr, 10, 64)
	if err != nil || trafficLimit <= 1 {
		log.Fatalln("укажите лимит трафика (TRAFFIC_LIMIT)")
	}

	extraDevicePriceStr := os.Getenv("EXTRA_DEVICE_PRICE")
	extraDevicePrice, err := strconv.Atoi(extraDevicePriceStr)
	if err != nil || extraDevicePrice < 1 {
		log.Fatalln("укажите стоимость доп. устройства (EXTRA_DEVICE_PRICE)")
	}

	resetTrafficPriceStr := os.Getenv("RESET_TRAFFIC_PRICE")
	resetTrafficPrice, err := strconv.Atoi(resetTrafficPriceStr)
	if err != nil || resetTrafficPrice < 1 {
		log.Fatalln("укажите стоимость сброса трафика (RESET_TRAFFIC_PRICE)")
	}

	return &Config{
		RemnaPanelURL:       os.Getenv("REMNA_BASE_PANEL"),
		RemnaSecretURLToken: os.Getenv("REMNA_SECRET_TOKEN"),
		RemnaLogin:          os.Getenv("REMNA_LOGIN"),
		RemnaPass:           os.Getenv("REMNA_PASS"),
		RemnaKey:            os.Getenv("REMNA_TOKEN"),
		RemnaSquadUUID:      os.Getenv("REMNA_SQUAD_UUID"),
		RemnaDefaultGb:      os.Getenv("REMNA_DEFOULT_GB"),
		TelegramToken:       os.Getenv("TELEGRAM_TOKEN"),
		TelegramSupport:     os.Getenv("TELEGRAM_SUPPORT"),
		TelegramAdminID:     os.Getenv("TELEGRAM_ADMIN_ID"),
		DatabaseURL:         databaseURL,
		PaymentProvider:     os.Getenv("PAYMENT_PROVIDER"),
		PlategaMerchantID:   os.Getenv("PLATEGA_MERCHANT_ID"),
		PlategaAPIKey:       os.Getenv("PLATEGA_API_KEY"),
		PlategaReturnURL:    os.Getenv("PLATEGA_RETURN_URL"),
		LoggerLevel:         os.Getenv("LOGGER_LEVEL"),
		YouKassaShopID:      os.Getenv("YOUKASSA_SHOP_ID"),
		YouKassaSecretKey:   os.Getenv("YOUKASSA_SECRET_KEY"),
		YouKassaReturnURL:   os.Getenv("YOUKASSA_RETURN_URL"),
		PricePerMonth:       os.Getenv("PRICE_PER_MONTH"),
		ExtraDevicePrice:    extraDevicePrice,
		ResetTrafficPrice:   resetTrafficPrice,
		DeviceLimit:         deviceLimit,
		MaxDeviceLimit:      maxDeviceLimit,
		TrafficLimit:        trafficLimit,
		SOCKS5Host:          os.Getenv("SOCKS5_HOST"),
		SOCKS5Port:          os.Getenv("SOCKS5_PORT"),
		SOCKS5Username:      os.Getenv("SOCKS5_USER"),
		SOCKS5Password:      os.Getenv("SOCKS5_PASS"),
	}, nil
}

// nolint: cyclop
func injectVaultSecrets() error {
	if os.Getenv("VAULT") != "enable" {
		return nil
	}

	addr := os.Getenv("VAULT_ADDRESS")
	if addr == "" {
		return errors.New("VAULT=enable, но VAULT_ADDRESS не указан, http://vault:8200")
	}

	roleID := os.Getenv("VAULT_ROLE_ID")
	secretID := os.Getenv("VAULT_SECRET_ID")
	secretIDFile := os.Getenv("VAULT_SECRET_ID_FILE")
	path := os.Getenv("VAULT_SECRET_PATH")

	if path == "" {
		path = "secret/data/proxymaster"
	}

	if roleID != "" {
		if secretID == "" && secretIDFile != "" {
			secretID = readSecretIDFromFile(secretIDFile)
		}
		if secretID == "" {
			return errors.New(
				"VAULT=enable и VAULT_ROLE_ID указан, но не задан VAULT_SECRET_ID (или VAULT_SECRET_ID_FILE)",
			)
		}
		secrets, err := LoadFromVaultAppRole(addr, roleID, secretID, path)
		if err != nil {
			return err
		}
		for k, v := range secrets {
			os.Setenv(k, v)
		}

		return nil
	}

	token := os.Getenv("VAULT_TOKEN")
	if token != "" {
		secrets, err := LoadFromVault(VaultConfig{
			Address:    addr,
			Token:      token,
			SecretPath: path,
		})
		if err != nil {
			return err
		}
		for k, v := range secrets {
			os.Setenv(k, v)
		}

		return nil
	}

	return nil
}
