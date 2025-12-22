package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// хранит глобальные настройки для панели
type Config struct {
	RemnaPanelURL       string // страница панели
	RemnasecretUrlToken string // секретный токен для подключения
	RemnaLogin          string // логин
	RemnaPass           string // пароль
	RemnawaveKey        string // ключ для разработчика
}

func New() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Println("Не удалось загрузить .env")
	}
	return &Config{
		RemnaPanelURL:       os.Getenv("REMNA_BASE_PANEL"),
		RemnasecretUrlToken: os.Getenv("REMNA_SECRET_TOKEN"),
		RemnaLogin:          os.Getenv("REMNA_LOGIN"),
		RemnaPass:           os.Getenv("REMNA_PASS"),
		RemnawaveKey:        os.Getenv("REMNA_TOKEN"),
	}, nil
}
