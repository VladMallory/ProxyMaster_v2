// Package config обращается к env, получает
// данные и возвращает в виде структуры
package config

import (
	"log"
	"os"

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

	// telegram
	TelegramToken   string
	TelegramSupport string // Поддержка телеграмм при ошибках сервиса.

	// database
	DatabaseURL string

	// platega
	PlategaAPIKey string
}

// New создает новый экземпляр конфигурации env.
func New() (*Config, error) {
	// Загружаем переменные окружения из файла .env.
	if err := godotenv.Load(); err != nil {
		log.Println("не удалось загрузить .env")
	}

	return &Config{
		RemnaPanelURL:       os.Getenv("REMNA_BASE_PANEL"),
		RemnaSecretURLToken: os.Getenv("REMNA_SECRET_TOKEN"),
		RemnaLogin:          os.Getenv("REMNA_LOGIN"),
		RemnaPass:           os.Getenv("REMNA_PASS"),
		RemnaKey:            os.Getenv("REMNA_TOKEN"),
		RemnaSquadUUID:      os.Getenv("REMNA_SQUAD_UUID"),
		TelegramToken:       os.Getenv("TELEGRAM_TOKEN"),
		TelegramSupport:     os.Getenv("TELEGRAM_SUPPORT"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		PlategaAPIKey:       os.Getenv("PLATEGA_API_KEY"),
	}, nil
}
