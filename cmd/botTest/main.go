package main

import (
	"fmt"
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/joho/godotenv"
)

type Config struct {
	Token string
}

func config() *Config {
	if err := godotenv.Load(); err != nil {
		log.Panic("не удалось загрузить .env")
	}

	return &Config{
		Token: os.Getenv("TELEGRAM_TOKEN"),
	}
}

func main() {
	cfg := config()
	fmt.Println(cfg.Token)

	// Подключаем к серверам телеграм
	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		log.Panic("не удалось подключиться к телеграм")
	}

	// Режим отладки
	// bot.Debug = true

	// Настраиваем то как мы хотим получать обновления
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	// Куда будет складывать все события, сообщения, нажатие кнопок и прочее
	updates, err := bot.GetUpdatesChan(updateConfig)
	if err != nil {
		log.Panic("не удалось получить канал обновлений")
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}
	}
}
