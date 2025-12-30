// package telegramBot обрабатывает клавиатуру от пользователя. В данном случае
// сколько на месяцев он хочет подписку.
package telegramBot

import (
	"ProxyMaster_v2/internal/domain"
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// CallbackHandler то какие сервисы используем
type CallbackHandler struct {
	// subService сервис подписки
	subService domain.SubscriptionService
}

// NewCallbackHandler конструктор
func NewCallbackHandler(subService domain.SubscriptionService) *CallbackHandler {
	fmt.Println("Создан экземпляр подписочного сервиса")

	return &CallbackHandler{
		subService: subService,
	}
}

// Handle обработка входящего callback
func (h *CallbackHandler) Handle(update tgbotapi.Update, bot *tgbotapi.BotAPI) error {
	data := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID

	// Отвечаем телеграму, что мы получили callback (чтобы часики пропали)
	callbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
	if _, err := bot.AnswerCallbackQuery(callbackCfg); err != nil {
		return fmt.Errorf("ошибка ответа на callback: %w", err)
	}

	// Логика обработки create_user_{months}
	if strings.HasPrefix(data, "create_user_") {
		monthsStr := strings.TrimPrefix(data, "create_user_")
		months, err := strconv.Atoi(monthsStr)
		if err != nil {
			return fmt.Errorf("неверный формат месяцев: %s", monthsStr)
		}

		// Вызываем сервис подписки
		resultMsg, err := h.subService.ActivateSubscription(int64(userID), months)
		if err != nil {
			log.Printf("Ошибка активации подписки: %v", err)
			msg := tgbotapi.NewMessage(int64(userID), "Произошла ошибка при обработке заказа.")
			_, _ = bot.Send(msg)
			return err
		}

		// Отправляем успешный ответ пользователю
		msg := tgbotapi.NewMessage(int64(userID), resultMsg)
		_, err = bot.Send(msg)
		return err
	}

	return nil
}
