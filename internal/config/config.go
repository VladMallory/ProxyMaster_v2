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
	RemnawaveKey        string // Ключ для разработчика
	// telegram
	TelegramToken string
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
		RemnawaveKey:        os.Getenv("REMNA_TOKEN"),
		TelegramToken:       os.Getenv("TELEGRAM_TOKEN"),
	}, nil
}
