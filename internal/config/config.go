package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config хранит глобальные настройки для панели
type Config struct {
	// remnawave
	RemnaPanelURL       string // Страница панели
	RemnaSecretUrlToken string // Секретный токен для подключения
	RemnaLogin          string // логин
	RemnaPass           string // пароль
	RemnaKey            string // Ключ для разработчика
	RemnaSquadUUID      string // ID клана

	// telegram
	TelegramToken   string
	TelegramSupport string // Поддержка телеграмм при ошибках сервиса

	//database
	DatabaseURL string
}

func New() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Println("не удалось загрузить .env")
	}
	return &Config{
		RemnaPanelURL:       os.Getenv("REMNA_BASE_PANEL"),
		RemnaSecretUrlToken: os.Getenv("REMNA_SECRET_TOKEN"),
		RemnaLogin:          os.Getenv("REMNA_LOGIN"),
		RemnaPass:           os.Getenv("REMNA_PASS"),
		RemnaKey:            os.Getenv("REMNA_TOKEN"),
		RemnaSquadUUID:      os.Getenv("REMNA_SQUAD_UUID"),
		TelegramToken:       os.Getenv("TELEGRAM_TOKEN"),
		TelegramSupport:     os.Getenv("TELEGRAM_SUPPORT"),
	}, nil
}
