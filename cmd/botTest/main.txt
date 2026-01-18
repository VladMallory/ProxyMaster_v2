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
	fmt.Println("Начали")
	cfg := config()

	// Подключаем к серверам телеграм
	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		log.Panic("не удалось подключиться к телеграм")
	}

	// Настраиваем то как мы хотим получать обновления
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	// Куда будет складывать все события, сообщения, нажатие кнопок и прочее
	updates, err := bot.GetUpdatesChan(updateConfig)
	if err != nil {
		log.Panic("не удалось получить канал обновлений")
	}

	for update := range updates {
		if update.CallbackQuery != nil {
			fmt.Println(update.CallbackQuery.From.ID, update.CallbackQuery.Data)

			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
			fmt.Println(callback)
			bot.AnswerCallbackQuery(callback)

			// У каждого нажатия (на клавиатуру) есть свое событие
			// Тут мы зависываем это событие в chatID
			// Каждый раз оно уникальное, так как всегда разные нажатия
			chatID := update.CallbackQuery.Message.Chat.ID
			msg := tgbotapi.NewMessage(chatID, "")

			// Проверяем data
			switch update.CallbackQuery.Data {
			case "btn_unlimits":
				msg.Text = "Ваш профиль"
			case "btn_goods":
				msg.Text = "Ваши товары"
			default:
				msg.Text = "Команда не найдена. Для получения списка доступных команд напишите /help."
			}

			bot.Send(msg)
			continue
		}

		// Теперь обрабатываем обычные сообщения
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				msg.Text = "Привет! Я бот ProxyMaster."
				msg.ReplyMarkup = getMainMenuKeyboard()
			default:
				msg.Text = "Я не знаю такую команду. Напиши /help"
			}
		} else {
			msg.Text = "Я понимаю только команды. Попробуй /start"
		}

		// Отправляем сообщение только если текст не пустой
		if msg.Text != "" {
			if _, err := bot.Send(msg); err != nil {
				log.Printf("Ошибка отправки: %v", err) // Используем log.Printf, чтобы бот не падал
			}
		}
	}

	// Нажата кнопка (CallbackQuery) — пользователь нажал на встроенную кнопку
	// Редактирование сообщения — пользователь отредактировал уже отправленное сообщение
	// Удаление сообщения — сообщение было удалено
	// Изменение статуса в чате — кто-то вошел/вышел из чата
	// Голосование в опросе — пользователь проголосовал
	// Любое другое событие Telegram, которое не является обычным текстовым/медиа сообщение

}

func getMainMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			// Data - это скрытый код кнопки. Пользователь видит "👤 Профиль",
			// а бот получает строку "btn_unlimits".
			tgbotapi.NewInlineKeyboardButtonData("👤 Профиль", "btn_unlimits"),
			tgbotapi.NewInlineKeyboardButtonData("📦 Товары", "btn_goods"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🆘 Поддержка", "https://t.me/support_bot"),
			// URL-кнопка сразу открывает ссылку, боту ничего не приходит
		),
	)
}
