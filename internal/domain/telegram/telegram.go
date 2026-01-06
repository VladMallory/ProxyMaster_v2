// Package telegram содержит бизнес логику бота и интерфейсы
// для взаимодействия. Определяется как себя будут вести команды и что выполнять
package telegram

import (
	"ProxyMaster_v2/internal/database"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/models"
	"ProxyMaster_v2/internal/payments/platega"
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MessageSender — интерфейс, который должен реализовать "отправитель" (в нашем случае Telegram клиент).
// Это позволяет бизнес-логике не зависеть от конкретной библиотеки (tgbotapi).
type MessageSender interface {
	// SendMessage отправляет обычное текстовое сообщение в чат
	SendMessage(chatID int64, text string) error
	// ShowView отправляет сообщение с нужной клавиатурой
	// viewType: "tariffs", "payment", "main"
	// messageID: ID сообщения для редактирования (0 — отправить новое)
	ShowView(chatID int64, messageID int, viewType, data string) error
}

// ProcessCallback обрабатывает нажатия на инлайн-кнопки (которые под сообщениями).
// sender: кто будет отправлять ответ (наш telegram клиент)
// chatID: ID чата, куда отправлять ответ
// messageID: ID сообщения, которое нужно отредактировать
// data: скрытые данные, зашитые в кнопку (например, "btn_balance")
func ProcessCallback(sender MessageSender, chatID int64, messageID int, data string, remnawaveClient domain.RemnawaveClient, plategaClient *platega.Client, userRepo *database.UserStorage) error {
	if amountStr, ok := strings.CutPrefix(data, "btn_pay_sbp_"); ok {
		amount, err := strconv.Atoi(amountStr)
		fmt.Println(amount)
		if err != nil {
			return sender.SendMessage(chatID, "Ошибка обработки суммы")
		}

		// Создание транзакции
		description := fmt.Sprintf("Оплата RUB для пользователя %d", chatID)
		// url, id, err := plategaClient.CreateTransaction(context.Background(), platega.SBPQR, amount, platega.RUB, description, strconv.FormatInt(chatID, 10))
		url, id, err := plategaClient.CreateTransaction(context.Background(), platega.SBPQR, 10, platega.RUB, description, strconv.FormatInt(chatID, 10))
		if err != nil {
			log.Printf("Ошибка создания транзакции: %v", err)
			return sender.SendMessage(chatID, "Ошибка создания транзакции")
		}

		return sender.ShowView(chatID, messageID, "check_payment", url+"|"+id)
	}

	if transactionID, ok := strings.CutPrefix(data, "btn_check_payment_"); ok {
		status, err := plategaClient.CheckStatus(context.Background(), transactionID)
		if err != nil {
			log.Printf("Ошибка проверки статуса транзакции: %v", err)
			return sender.SendMessage(chatID, "Ошибка проверки статуса платежа")
		}

		switch status {
		case domain.PaymentStatusSuccess:
			return handleSuccessfulPayment(sender, chatID, transactionID, plategaClient, userRepo)
		case domain.PaymentStatusPending:
			started := tryStartPaymentStatusWatcher(sender, chatID, transactionID, plategaClient, userRepo)
			if started {
				return sender.SendMessage(chatID, "⏳ Оплата еще не поступила. Я буду автоматически проверять статус каждые 2 секунды и сообщу, когда платеж подтвердится.")
			}
			return sender.SendMessage(chatID, "⏳ Автопроверка уже запущена. Я сообщу, когда платеж подтвердится.")
		default:
			return sender.SendMessage(chatID, "❌ Оплата не прошла или отменена.")
		}
	}

	if strings.HasPrefix(data, "btn_pay_crypto_") {
		return sender.SendMessage(chatID, "Криптовалюта пока не поддерживается")
	}

	switch data {
	case "btn_balance":
		// Показываем тарифы
		return sender.ShowView(chatID, messageID, "tariffs", "")
	case "btn_tariff_1":
		// Тариф 1 месяц - 100р
		return sender.ShowView(chatID, messageID, "payment", "100")
	case "btn_tariff_3":
		// Тариф 3 месяца - 270р
		return sender.ShowView(chatID, messageID, "payment", "270")
	case "btn_connect":
		// Заглушка для кнопки подключения
		return sender.SendMessage(chatID, "Функция подключения пока в разработке")
	case "btn_info":
		// Информация о боте
		return sender.SendMessage(chatID, "В разработке")
	case "btn_back":
		// Кнопка "Назад" возвращает в главное меню
		return sender.ShowView(chatID, messageID, "main", "")
	default:
		return sender.SendMessage(chatID, "Неизвестная команда")
	}
}

var activePaymentStatusWatchers sync.Map

func tryStartPaymentStatusWatcher(sender MessageSender, chatID int64, transactionID string, plategaClient *platega.Client, userRepo *database.UserStorage) bool {
	_, loaded := activePaymentStatusWatchers.LoadOrStore(transactionID, struct{}{})
	if loaded {
		return false
	}

	go func() {
		defer activePaymentStatusWatchers.Delete(transactionID)

		deadline := time.Now().Add(2 * time.Minute)
		for time.Now().Before(deadline) {
			status, err := plategaClient.CheckStatus(context.Background(), transactionID)
			if err != nil {
				log.Printf("Ошибка проверки статуса транзакции: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}

			switch status {
			case domain.PaymentStatusSuccess:
				if err := handleSuccessfulPayment(sender, chatID, transactionID, plategaClient, userRepo); err != nil {
					log.Printf("Ошибка обработки успешного платежа: %v", err)
				}
				return
			case domain.PaymentStatusPending:
				time.Sleep(2 * time.Second)
				continue
			default:
				_ = sender.SendMessage(chatID, "❌ Оплата не прошла или отменена.")
				return
			}
		}

		_ = sender.SendMessage(chatID, "⏳ Автопроверка остановлена: время ожидания истекло. Нажмите «Проверить оплату» позже.")
	}()

	return true
}

func handleSuccessfulPayment(sender MessageSender, chatID int64, transactionID string, plategaClient *platega.Client, userRepo *database.UserStorage) error {
	info, err := plategaClient.GetTransactionInfo(context.Background(), transactionID)
	if err != nil {
		log.Printf("Ошибка получения информации о транзакции: %v", err)
		return sender.SendMessage(chatID, "Платеж прошел, но возникла ошибка при получении данных. Обратитесь в поддержку.")
	}

	amount := int(info.GetAmount())

	user, err := userRepo.GetUserByID(strconv.FormatInt(chatID, 10))
	if err != nil {
		log.Printf("Ошибка получения пользователя: %v", err)
		return sender.SendMessage(chatID, "Ошибка получения данных пользователя")
	}

	newBalance := user.Balance + amount
	_, err = userRepo.UpdateUser(strconv.FormatInt(chatID, 10), models.UpdateUserTGDTO{
		Balance: &newBalance,
	})
	if err != nil {
		log.Printf("Ошибка обновления баланса: %v", err)
		return sender.SendMessage(chatID, "Платеж прошел, но не удалось обновить баланс. Обратитесь в поддержку.")
	}

	return sender.SendMessage(chatID, fmt.Sprintf("✅ Оплата прошла успешно! Ваш баланс пополнен на %d RUB.", amount))
}

// ProcessCommand обрабатывает текстовые команды (например, /start).
// Эта функция — "мозг" обработки текста.
func ProcessCommand(sender MessageSender, chatID int64, command string, remnawaveClient domain.RemnawaveClient) error {
	var replyText string
	switch command {
	case "/start":
		// ---Создаем подписку---
		username := strconv.Itoa(int(chatID))
		err := remnawaveClient.CreateUser(username, 5)
		if err != nil {
			log.Println("Ошибка создания пользователя")
		}

		replyText = "Добро пожаловать! Я помогу вам управлять подписками."
	default:
		replyText = "Неизвестная команда. Пожалуйста, используйте /start."
	}

	// Отправляем ответ
	return sender.SendMessage(chatID, replyText)
}
