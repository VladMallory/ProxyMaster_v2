package telegram

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	billingDomain "github.com/VladMallory/ProxyMaster_v2/internal/features/billing/domain"
	subsDevice "github.com/VladMallory/ProxyMaster_v2/internal/features/subscription/device"
)

// MessageSender интерфейс для отправки сообщений через Telegram.
type MessageSender interface {
	SendMessage(chatID int64, text string) error
	ShowView(chatID int64, messageID int, viewType domain.ViewType, data string) error
}

func HandleSuccessfulAddDevicePayment(
	sender MessageSender,
	chatID int64,
	messageID int,
	deviceService domain.DeviceService,
) error {
	userID := strconv.FormatInt(chatID, 10)

	if err := deviceService.AddPaidDevice(userID); err != nil {
		errorMsg := "❌ Ошибка добавления устройства."
		if errors.Is(err, billingDomain.ErrInsufficientFunds) {
			errorMsg = "❌ Недостаточно средств. Нужно 50₽."
		} else if errors.Is(err, subsDevice.ErrMaxDevices) {
			errorMsg = "❌ Достигнут лимит устройств."
		}
		log.Printf("Ошибка добавления платного устройства для %s: %v", userID, err)

		return sender.ShowView(chatID, messageID, domain.ViewTypeSubscriptionResult, errorMsg)
	}

	return sender.ShowView(chatID, messageID, domain.ViewTypeSubscriptionResult, "✅ Устройство добавлено.")
}

func HandleSuccessfulPrepayDevicesPayment(
	sender MessageSender,
	chatID int64,
	messageID int,
	deviceService domain.DeviceService,
) error {
	userID := strconv.FormatInt(chatID, 10)

	count, err := deviceService.PrepayPaidDevices(userID)
	if err != nil {
		errorMsg := "❌ Ошибка продления доп. устройств."
		if errors.Is(err, billingDomain.ErrInsufficientFunds) {
			errorMsg = "❌ Недостаточно средств для продления доп. устройств."
		} else if errors.Is(err, subsDevice.ErrNoActiveDeviceAddons) {
			errorMsg = "У вас нет активных доп. устройств для продления."
		}
		log.Printf("Ошибка предоплаты доп. устройств для %s: %v", userID, err)

		return sender.ShowView(chatID, messageID, domain.ViewTypeSubscriptionResult, errorMsg)
	}

	successMsg := fmt.Sprintf("✅ Доп. устройства продлены на 1 месяц. Количество: %d.", count)

	return sender.ShowView(chatID, messageID, domain.ViewTypeSubscriptionResult, successMsg)
}
