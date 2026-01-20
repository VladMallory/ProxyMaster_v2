package telegram

import (
	"ProxyMaster_v2/internal/database"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/models"
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"
)

// handleSuccessfulPayment обрабатывает успешный платеж в зависимости от его типа
func handleSuccessfulPayment(
	sender MessageSender,
	chatID int64,
	messageID int,
	transactionID string,
	paymentGateway domain.PaymentGateway,
	subscriptionService domain.SubscriptionService,
	userRepo *database.UserStorage,
) error {
	info, err := paymentGateway.GetTransactionInfo(context.Background(), transactionID)
	if err != nil {
		log.Printf("Ошибка получения информации о транзакции: %v", err)
		if sendErr := sender.SendMessage(
			chatID,
			"Платеж прошел, но возникла ошибка при получении данных. Обратитесь в поддержку.",
		); sendErr != nil {
			return fmt.Errorf(
				"не удалось отправить сообщение об ошибке получения информации о транзакции: %w",
				sendErr,
			)
		}
		return nil
	}

	amount := int(info.GetAmount())
	if value, ok := paymentPurposeByTransaction.Load(transactionID); ok {
		paymentPurposeByTransaction.Delete(transactionID)
		if data, ok := value.(paymentPurposeData); ok {
			switch data.purpose {
			case paymentPurposeAddDevice:
				return handleSuccessfulAddDevicePayment(
					sender,
					chatID,
					messageID,
					amount,
					subscriptionService,
					userRepo,
				)
			case paymentPurposePrepayDevices:
				return handleSuccessfulPrepayDevicesPayment(
					sender,
					chatID,
					messageID,
					amount,
					subscriptionService,
					userRepo,
				)
			}
		}
	}

	// Пополнение баланса по умолчанию
	userID := strconv.FormatInt(chatID, 10)
	user, err := userRepo.GetUserByID(userID)
	if err != nil {
		log.Printf("Ошибка получения пользователя для обновления баланса: %v", err)
		if sendErr := sender.SendMessage(
			chatID,
			"Ошибка получения данных пользователя",
		); sendErr != nil {
			return fmt.Errorf(
				"не удалось отправить сообщение об ошибке получения пользователя: %w",
				sendErr,
			)
		}
		return nil
	}

	newBalance := user.Balance + amount
	if _, err = userRepo.UpdateUser(userID, models.UpdateUserTGDTO{
		Balance: &newBalance,
	}); err != nil {
		log.Printf("Ошибка обновления баланса: %v", err)
		if sendErr := sender.SendMessage(
			chatID,
			"Платеж прошел, но не удалось обновить баланс. Обратитесь в поддержку.",
		); sendErr != nil {
			return fmt.Errorf(
				"не удалось отправить сообщение об ошибке обновления баланса: %w",
				sendErr,
			)
		}
		return nil
	}

	successMsg := fmt.Sprintf("✅ Оплата прошла успешно! Ваш баланс пополнен на %d RUB.", amount)
	if err := sender.ShowView(
		chatID,
		messageID,
		domain.ViewTypeSubscriptionResult,
		successMsg,
	); err != nil {
		return fmt.Errorf("ошибка отображения сообщения об успешной оплате: %w", err)
	}

	const pricePerMonthRUB = 100
	months := amount / pricePerMonthRUB
	if months <= 0 {
		return nil
	}

	go func() {
		time.Sleep(10 * time.Second)

		if err := handleSubscriptionFromBalance(
			sender,
			subscriptionService,
			chatID,
			messageID,
			months,
		); err != nil {
			log.Printf("Ошибка автопродления подписки: %v", err)
		}
	}()

	return nil
}

// handleSuccessfulAddDevicePayment обрабатывает успешный платеж за добавление устройства
func handleSuccessfulAddDevicePayment(
	sender MessageSender,
	chatID int64,
	messageID int,
	amount int,
	subscriptionService domain.SubscriptionService,
	userRepo *database.UserStorage,
) error {
	userID := strconv.FormatInt(chatID, 10)
	user, err := userRepo.GetUserByID(userID)
	if err != nil {
		log.Printf("Ошибка получения пользователя для добавления устройства: %v", err)
		if sendErr := sender.SendMessage(chatID, "Ошибка получения данных пользователя"); sendErr != nil {
			return fmt.Errorf(
				"не удалось отправить сообщение об ошибке получения пользователя: %w",
				sendErr,
			)
		}
		return nil
	}

	newBalance := user.Balance + amount
	if _, err = userRepo.UpdateUser(userID, models.UpdateUserTGDTO{
		Balance: &newBalance,
	}); err != nil {
		log.Printf("Ошибка обновления баланса для оплаты устройства: %v", err)
		if sendErr := sender.SendMessage(
			chatID,
			"Платеж прошел, но не удалось обновить баланс. Обратитесь в поддержку.",
		); sendErr != nil {
			return fmt.Errorf(
				"не удалось отправить сообщение об ошибке обновления баланса: %w",
				sendErr,
			)
		}
		return nil
	}

	if err := subscriptionService.AddPaidDevice(userID); err != nil {
		errorMsg := "❌ Ошибка добавления устройства."
		if errors.Is(err, domain.ErrInsufficientFunds) {
			errorMsg = "❌ Недостаточно средств. Нужно 50₽."
		} else if errors.Is(err, domain.ErrMaxDevices) {
			errorMsg = "❌ Достигнут лимит устройств."
		}
		log.Printf("Ошибка добавления платного устройства для %s: %v", userID, err)
		if sendErr := sender.ShowView(
			chatID,
			messageID,
			domain.ViewTypeSubscriptionResult,
			errorMsg,
		); sendErr != nil {
			return fmt.Errorf(
				"не удалось отправить сообщение об ошибке добавления устройства: %w",
				sendErr,
			)
		}
		return nil
	}

	successMsg := "✅ Устройство добавлено."
	if err := sender.ShowView(
		chatID,
		messageID,
		domain.ViewTypeSubscriptionResult,
		successMsg,
	); err != nil {
		return fmt.Errorf("ошибка отображения сообщения об успешном добавлении устройства: %w", err)
	}

	return nil
}

// handleSuccessfulPrepayDevicesPayment обрабатывает успешный платеж за продление доп. устройств
func handleSuccessfulPrepayDevicesPayment(
	sender MessageSender,
	chatID int64,
	messageID int,
	amount int,
	subscriptionService domain.SubscriptionService,
	userRepo *database.UserStorage,
) error {
	userID := strconv.FormatInt(chatID, 10)
	user, err := userRepo.GetUserByID(userID)
	if err != nil {
		log.Printf("Ошибка получения пользователя для продления устройств: %v", err)
		if sendErr := sender.SendMessage(chatID, "Ошибка получения данных пользователя"); sendErr != nil {
			return fmt.Errorf(
				"не удалось отправить сообщение об ошибке получения пользователя: %w",
				sendErr,
			)
		}
		return nil
	}

	newBalance := user.Balance + amount
	if _, err = userRepo.UpdateUser(userID, models.UpdateUserTGDTO{
		Balance: &newBalance,
	}); err != nil {
		log.Printf("Ошибка обновления баланса для продления устройств: %v", err)
		if sendErr := sender.SendMessage(
			chatID,
			"Платеж прошел, но не удалось обновить баланс. Обратитесь в поддержку.",
		); sendErr != nil {
			return fmt.Errorf(
				"не удалось отправить сообщение об ошибке обновления баланса: %w",
				sendErr,
			)
		}
		return nil
	}

	count, err := subscriptionService.PrepayPaidDevices(userID)
	if err != nil {
		errorMsg := "❌ Ошибка продления доп. устройств."
		if errors.Is(err, domain.ErrInsufficientFunds) {
			errorMsg = "❌ Недостаточно средств для продления доп. устройств."
		} else if errors.Is(err, domain.ErrNoActiveDeviceAddons) {
			errorMsg = "У вас нет активных доп. устройств для продления."
		}
		log.Printf("Ошибка предоплаты доп. устройств для %s: %v", userID, err)
		if sendErr := sender.ShowView(
			chatID,
			messageID,
			domain.ViewTypeSubscriptionResult,
			errorMsg,
		); sendErr != nil {
			return fmt.Errorf(
				"не удалось отправить сообщение об ошибке продления устройств: %w",
				sendErr,
			)
		}
		return nil
	}

	successMsg := fmt.Sprintf("✅ Доп. устройства продлены на 1 месяц. Количество: %d.", count)
	if err := sender.ShowView(
		chatID,
		messageID,
		domain.ViewTypeSubscriptionResult,
		successMsg,
	); err != nil {
		return fmt.Errorf("ошибка отображения сообщения о продлении устройств: %w", err)
	}

	return nil
}

// handleSubscriptionFromBalance активирует подписку со скидкой на баланс пользователя
func handleSubscriptionFromBalance(
	sender MessageSender,
	subscriptionService domain.SubscriptionService,
	chatID int64,
	messageID int,
	months int,
) error {
	result, err := subscriptionService.ActivateSubscription(chatID, months)
	if err != nil {
		errorMsg := "Произошла ошибка при оформлении подписки"
		if errors.Is(err, domain.ErrInsufficientFunds) {
			errorMsg = "❌ Недостаточно средств на балансе"
		}
		log.Printf("Ошибка активации подписки для %d: %v", chatID, err)
		if sendErr := sender.ShowView(
			chatID,
			messageID,
			domain.ViewTypeSubscriptionResult,
			errorMsg,
		); sendErr != nil {
			return fmt.Errorf("не удалось отправить сообщение об ошибке подписки: %w", sendErr)
		}
		return nil
	}

	successMsg := "✅ " + result
	if err := sender.ShowView(
		chatID,
		messageID,
		domain.ViewTypeSubscriptionResult,
		successMsg,
	); err != nil {
		return fmt.Errorf("ошибка отображения сообщения об успешной подписке: %w", err)
	}
	return nil
}
