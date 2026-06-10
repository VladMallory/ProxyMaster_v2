package telegram

import (
	"context"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	billingDomain "github.com/VladMallory/ProxyMaster_v2/internal/features/billing/domain"
	billingSvc "github.com/VladMallory/ProxyMaster_v2/internal/features/billing/service"
	deviceTelegram "github.com/VladMallory/ProxyMaster_v2/internal/features/subscription/device/transport/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/integrations/remnawave"
)

// MessageSender интерфейс для отправки сообщений через Telegram.
type MessageSender interface {
	SendMessage(chatID int64, text string) error
	ShowView(chatID int64, messageID int, viewType domain.ViewType, data string) error
}

func HandleSuccessfulPayment(
	sender MessageSender,
	chatID int64,
	messageID int,
	transactionID string,
	bSvc *billingSvc.Service,
	subscriptionService domain.SubscriptionService,
	deviceService domain.DeviceService,
	remnawaveClient remnawave.RemnawaveClient,
	cfg *config.Config,
) error {
	info, err := bSvc.GetPaymentInfo(context.Background(), transactionID)
	if err != nil {
		log.Printf("Ошибка получения информации о транзакции: %v", err)

		return sender.SendMessage(
			chatID,
			"Платеж прошел, но возникла ошибка при получении данных. Обратитесь в поддержку.",
		)
	}

	amount := int(info.GetAmount())
	if purpose, _, ok := bSvc.ConsumePaymentPurpose(transactionID); ok {
		switch purpose {
		case billingSvc.PurposeAddDevice:
			return deviceTelegram.HandleSuccessfulAddDevicePayment(
				sender, chatID, messageID, deviceService,
			)
		case billingSvc.PurposePrepayDevices:
			return deviceTelegram.HandleSuccessfulPrepayDevicesPayment(
				sender, chatID, messageID, deviceService,
			)
		case billingSvc.PurposeResetTraffic:
			return HandleSuccessfulResetTrafficPayment(
				sender, chatID, messageID, remnawaveClient,
			)
		}
	}

	pricePerMonth, err := strconv.Atoi(cfg.PricePerMonth)
	if err != nil {
		log.Printf("Ошибка преобразования цены: %v", err)

		return err
	}
	months := amount / pricePerMonth
	if months <= 0 {
		return nil
	}

	go func() {
		time.Sleep(10 * time.Second)

		if err := HandleSubscriptionFromBalance(
			sender, subscriptionService, chatID, messageID, months,
		); err != nil {
			log.Printf("Ошибка автопродления подписки: %v", err)
		}
	}()

	return nil
}

func HandleSubscriptionFromBalance(
	sender MessageSender,
	subscriptionService domain.SubscriptionService,
	chatID int64,
	messageID int,
	months int,
) error {
	userID := strconv.FormatInt(chatID, 10)

	result, err := subscriptionService.ActivateSubscription(userID, months)
	if err != nil {
		errorMsg := "Произошла ошибка при оформлении подписки"
		if errors.Is(err, billingDomain.ErrInsufficientFunds) {
			errorMsg = "❌ Недостаточно средств на балансе"
		}
		log.Printf("Ошибка активации подписки для %d: %v", chatID, err)

		return sender.ShowView(chatID, messageID, domain.ViewTypeSubscriptionResult, errorMsg)
	}

	return sender.ShowView(chatID, messageID, domain.ViewTypeSubscriptionResult, "✅ "+result)
}

func HandleSuccessfulResetTrafficPayment(
	sender MessageSender,
	chatID int64,
	messageID int,
	remnawaveClient remnawave.RemnawaveClient,
) error {
	userID := strconv.FormatInt(chatID, 10)

	if err := remnawaveClient.BetterResetTraffic(context.Background(), userID); err != nil {
		log.Printf("Ошибка сброса трафика у пользователя %s: %v", userID, err)

		return sender.SendMessage(chatID, "Не удалось сбросить трафик")
	}

	return sender.ShowView(chatID, messageID, domain.ViewTypeSubscriptionResult, "✅ Трафик успешно сброшен.")
}
