package telegram

import (
	"ProxyMaster_v2/internal/database"
	"ProxyMaster_v2/internal/domain"
	"context"
	"log"
	"time"
)

// startAutoPaymentCheck запускает автоматическую проверку платежа.
// Начинает проверку через 15 секунд после создания, затем проверяет каждые 5 секунд в течение 20 минут.
// Эта функция нужна для того, чтобы клиентам не приходилось вручную нажимать "Проверить платеж".
func startAutoPaymentCheck(
	sender MessageSender,
	chatID int64,
	messageID int,
	transactionID string,
	paymentGateway domain.PaymentGateway,
	subscriptionService domain.SubscriptionService,
	userRepo *database.UserStorage,
) {
	// Проверяем, не запущена ли уже проверка для этой транзакции
	_, alreadyRunning := activePaymentStatusWatchers.LoadOrStore(transactionID, struct{}{})
	if alreadyRunning {
		log.Printf("[АВТОПРОВЕРКА] Проверка для транзакции %s уже запущена, пропускаем", transactionID)
		return
	}

	go func() {
		// Удаляем транзакцию из активных при завершении горутины
		defer activePaymentStatusWatchers.Delete(transactionID)

		log.Printf("[АВТОПРОВЕРКА] Запущена для транзакции %s, ожидание 15 секунд...", transactionID)

		// Ждем 15 секунд перед первой проверкой (даем время на оплату)
		time.Sleep(15 * time.Second)

		// Проверяем каждые 5 секунд в течение 20 минут
		deadline := time.Now().Add(20 * time.Minute)
		checkInterval := 5 * time.Second

		for time.Now().Before(deadline) {
			log.Printf("[АВТОПРОВЕРКА] Проверяем статус транзакции %s", transactionID)

			// Проверяем статус платежа
			status, err := paymentGateway.CheckStatus(context.Background(), transactionID)
			if err != nil {
				log.Printf("[АВТОПРОВЕРКА] Ошибка проверки статуса транзакции %s: %v", transactionID, err)
				time.Sleep(checkInterval)
				continue
			}

			log.Printf("[АВТОПРОВЕРКА] Статус транзакции %s: %s", transactionID, status)

			switch status {
			case domain.PaymentStatusSuccess:
				log.Printf("[АВТОПРОВЕРКА] Платеж %s успешен, обрабатываем...", transactionID)
				if err := handleSuccessfulPayment(
					sender,
					chatID,
					messageID,
					transactionID,
					paymentGateway,
					subscriptionService,
					userRepo,
				); err != nil {
					log.Printf("[АВТОПРОВЕРКА] Ошибка обработки успешного платежа %s: %v", transactionID, err)
				} else {
					log.Printf("[АВТОПРОВЕРКА] Платеж %s успешно обработан!", transactionID)
				}
				return // Завершаем горутину после успешной обработки

			case domain.PaymentStatusFailed:
				log.Printf("[АВТОПРОВЕРКА] Платеж %s отменен или не прошел", transactionID)
				return // Завершаем горутину, платеж точно не пройдет

			case domain.PaymentStatusPending:
				// Платеж еще в ожидании, продолжаем проверять
				time.Sleep(checkInterval)
				continue
			}
		}

		log.Printf("[АВТОПРОВЕРКА] Время ожидания истекло для транзакции %s", transactionID)
	}()
}

// tryStartPaymentStatusWatcher пытается запустить горутину для проверки статуса платежа.
// Возвращает true если горутина была запущена, false если она уже работала.
func tryStartPaymentStatusWatcher(
	sender MessageSender,
	chatID int64,
	messageID int,
	transactionID string,
	paymentGateway domain.PaymentGateway,
	subscriptionService domain.SubscriptionService,
	userRepo *database.UserStorage,
) bool {
	_, loaded := activePaymentStatusWatchers.LoadOrStore(transactionID, struct{}{})
	if loaded {
		return false
	}

	go func() {
		defer activePaymentStatusWatchers.Delete(transactionID)

		deadline := time.Now().Add(2 * time.Minute)
		for time.Now().Before(deadline) {
			status, err := paymentGateway.CheckStatus(context.Background(), transactionID)
			if err != nil {
				log.Printf("Ошибка проверки статуса транзакции (внутри горутины): %v", err)
				time.Sleep(2 * time.Second)
				continue
			}

			switch status {
			case domain.PaymentStatusSuccess:
				if err := handleSuccessfulPayment(
					sender,
					chatID,
					messageID,
					transactionID,
					paymentGateway,
					subscriptionService,
					userRepo,
				); err != nil {
					log.Printf("Ошибка обработки успешного платежа (внутри горутины): %v", err)
				}
				return // Завершаем горутину
			case domain.PaymentStatusPending:
				time.Sleep(2 * time.Second)
				continue
			default:
				if err := sender.SendMessage(
					chatID,
					"❌ Оплата не прошла или отменена.",
				); err != nil {
					log.Printf(
						"Ошибка отправки сообщения о неудачном платеже (внутри горутины): %v",
						err,
					)
				}
				return // Завершаем горутину
			}
		}
		// Отправляем сообщение по истечении времени
		if err := sender.SendMessage(
			chatID,
			"⏳ Автопроверка остановлена: время ожидания истекло. Нажмите «Проверить оплату» позже.",
		); err != nil {
			log.Printf(
				"Ошибка отправки сообщения об истечении времени ожидания (внутри горутины): %v",
				err,
			)
		}
	}()

	return true
}
