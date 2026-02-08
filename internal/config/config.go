// Package config обращается к env, получает
// данные и возвращает в виде структуры
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config хранит глобальные настройки для панели.
type Config struct {
	// remnawave
	RemnaPanelURL       string // Страница панели.
	RemnaSecretURLToken string // Секретный токен для подключения.
	RemnaLogin          string // логин
	RemnaPass           string // пароль
	RemnaKey            string // Ключ для разработчика.
	RemnaSquadUUID      string // ID squad.
	RemnaDefaultGb      string

	// telegram
	TelegramToken   string
	TelegramSupport string // Поддержка телеграмм при ошибках сервиса.
	TelegramAdminID string

	// database
	DatabaseURL string

	// platega
	PlategaAPIKey string

	// youkassa
	YouKassaShopID    string
	YouKassaSecretKey string
	YouKassaReturnURL string

	// Настройки
	PricePerMonth string // оплата за месяц
	DeviceLimit   int    // базовый лимит устройств

	// Logger
	LoggerLevel string
}

// New создает новый экземпляр конфигурации env.
func New() (*Config, error) {
	// Загружаем переменные окружения из файла .env.
	if err := godotenv.Load(); err != nil {
		log.Println("не удалось загрузить .env")
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL не установлен")
	}

	// Читаем базовый лимит устройств
	deviceLimitStr := os.Getenv("DEVICE_LIMIT")
	deviceLimit, err := strconv.Atoi(deviceLimitStr)

	if err != nil || deviceLimit < 1 {
		deviceLimit = 2 // Значение по умолчанию
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
		PlategaAPIKey:       os.Getenv("PLATEGA_API_KEY"),
		LoggerLevel:         os.Getenv("LOGGER_LEVEL"),
		YouKassaShopID:      os.Getenv("YOUKASSA_SHOP_ID"),
		YouKassaSecretKey:   os.Getenv("YOUKASSA_SECRET_KEY"),
		YouKassaReturnURL:   os.Getenv("YOUKASSA_RETURN_URL"),
		PricePerMonth:       os.Getenv("PRICE_PER_MONTH"),
		DeviceLimit:         deviceLimit,
	}, nil
}
