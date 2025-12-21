package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// хранит глобальные настройки для панели
type Config struct {
	RemnaPanelURL string // страница панели
	RemnaLogin    string // логин
	RemnaPass     string // пароль
	RemnawaveKey  string // ключ для разработчика, чтобы подключиться
}

func New() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Println("Не удалось загрузить .env")
	}
	return &Config{
		RemnaPanelURL: os.Getenv("REMNA_PANEL"),
		RemnaLogin:    os.Getenv("REMNA_LOGIN"),
		RemnaPass:     os.Getenv("REMNA_PASS"),
		RemnawaveKey:  os.Getenv("REMNA_TOKEN"),
	}, nil
}
