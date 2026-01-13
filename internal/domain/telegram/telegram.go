// Package telegram содержит бизнес логику бота и интерфейсы
// для взаимодействия. Определяется как себя будут вести команды и что выполнять
//
//nolint:goconst
package telegram

import (
	"ProxyMaster_v2/internal/database"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/models"
	"ProxyMaster_v2/internal/service"
	"context"
	"errors"
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
	ShowView(chatID int64, messageID int, viewType domain.ViewType, data string) error
}

// ProcessCallback обрабатывает нажатия на инлайн-кнопки (которые под сообщениями).
// sender: кто будет отправлять ответ (наш telegram клиент)
// chatID: ID чата, куда отправлять ответ
// messageID: ID сообщения, которое нужно отредактировать
// data: скрытые данные, зашитые в кнопку (например, "btn_balance")
func ProcessCallback(sender MessageSender,
	chatID int64,
	messageID int,
	data, firstName string,
	remnawaveClient domain.RemnawaveClient,
	subscriptionService domain.SubscriptionService,
	paymentGateway domain.PaymentGateway,
	userRepo *database.UserStorage,
) error {

	buildProfileData := func(userID string) (string, error) {
		user, err := userRepo.GetUserByID(userID)
		if err != nil {
			return "", err
		}

		extraCount, err := userRepo.CountActiveDeviceAddons(userID)
		if err != nil {
			return "", err
		}

		if user.ExtraDevicesCount != extraCount {
			_, err = userRepo.UpdateUser(userID, models.UpdateUserTGDTO{
				ExtraDevicesCount: &extraCount,
			})
			if err != nil {
				return "", err
			}
		}

		return user.ID + "|" + strconv.Itoa(user.Balance) + "|" + strconv.Itoa(extraCount), nil
	}

	if amountStr, ok := strings.CutPrefix(data, "btn_pay_sbp_"); ok {
		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			return sender.SendMessage(chatID, "Ошибка обработки суммы")
		}

		orderID := fmt.Sprintf("tg_%d_%d_%d", chatID, amount, time.Now().UnixNano())

		url, id, err := paymentGateway.CreateTransaction(context.Background(), float64(amount), orderID)
		if err != nil {
			log.Printf("Ошибка создания транзакции: %v", err)
			return sender.SendMessage(chatID, "Ошибка создания транзакции")
		}

		return sender.ShowView(chatID, messageID, domain.ViewTypeCheckPayment, url+"|"+id)
	}

	if transactionID, ok := strings.CutPrefix(data, "btn_check_payment_"); ok {
		status, err := paymentGateway.CheckStatus(context.Background(), transactionID)
		if err != nil {
			log.Printf("Ошибка проверки статуса транзакции: %v", err)
			return sender.SendMessage(chatID, "Ошибка проверки статуса платежа")
		}

		switch status {
		case domain.PaymentStatusSuccess:
			return handleSuccessfulPayment(sender, chatID, messageID, transactionID, paymentGateway, userRepo)
		case domain.PaymentStatusPending:
			started := tryStartPaymentStatusWatcher(sender, chatID, messageID, transactionID, paymentGateway, userRepo)
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
	case "btn_tariffs":
		return sender.ShowView(chatID, messageID, domain.ViewTypeTariffs, "")
	case "btn_sub_tariff_1":
		return handleSubscriptionFromBalance(sender, subscriptionService, chatID, messageID, 1)
	case "btn_sub_tariff_2":
		return handleSubscriptionFromBalance(sender, subscriptionService, chatID, messageID, 2)
	case "btn_sub_tariff_3":
		return handleSubscriptionFromBalance(sender, subscriptionService, chatID, messageID, 3)
	case "btn_balance":
		return sender.ShowView(chatID, messageID, domain.ViewTypeTopup, "")
	case "btn_topup_100":
		return sender.ShowView(chatID, messageID, domain.ViewTypePayment, "100")
	case "btn_topup_200":
		return sender.ShowView(chatID, messageID, domain.ViewTypePayment, "200")
	case "btn_topup_300":
		return sender.ShowView(chatID, messageID, domain.ViewTypePayment, "300")
	case "download_app":
		return sender.ShowView(chatID, messageID, domain.ViewTypeDownloadApp, "")
	case "btn_ios_menu":
		return sender.ShowView(chatID, messageID, domain.ViewTypeIosRegion, "")
	case "btn_back_download_app":
		return sender.ShowView(chatID, messageID, domain.ViewTypeDownloadApp, "")
	case "btn_unlimits":
		// Показываем меню лимитов устройств
		return sender.ShowView(chatID, messageID, domain.ViewTypeDeviceLimits, "")
	case "btn_traffic_limits":
		// Вызываем меню лимитов трафика
		return sender.ShowView(chatID, messageID, domain.ViewTypeTrafficLimits, "")
	case "btn_add_50gb":

	case "btn_add_100gb":
		return nil
	case "btn_reset_traffic":
		remnawaveClient.SetTraffic()
		return nil
	case "btn_profile":
		userID := strconv.FormatInt(chatID, 10)

		user, err := userRepo.GetUserByID(userID)
		if err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				user, err = userRepo.CreateUser(models.CreateUserTGDTO{
					ID:      userID,
					Balance: 0,
					Trial:   false,
				})
				if err != nil {
					log.Printf("Ошибка создания пользователя в DB: %v", err)
					return sender.SendMessage(chatID, "Ошибка создания пользователя")
				}
			} else {
				log.Printf("Ошибка получения пользователя: %v", err)
				return sender.SendMessage(chatID, "Ошибка получения данных пользователя")
			}
		}

		_ = user

		profileData, err := buildProfileData(userID)
		if err != nil {
			return sender.SendMessage(chatID, "Ошибка получения данных профиля")
		}

		return sender.ShowView(chatID, messageID, "profile", profileData)
	case "btn_add_device":
		userID := strconv.FormatInt(chatID, 10)

		if err := subscriptionService.AddPaidDevice(userID); err != nil {
			if errors.Is(err, domain.ErrInsufficientFunds) {
				return sender.SendMessage(chatID, "❌ Недостаточно средств. Нужно 50₽.")
			}
			if errors.Is(err, domain.ErrMaxDevices) {
				return sender.SendMessage(chatID, "❌ Достигнут лимит устройств.")
			}
			return sender.SendMessage(chatID, "❌ Ошибка добавления устройства.")
		}

		profileData, err := buildProfileData(userID)
		if err != nil {
			return sender.SendMessage(chatID, "Ошибка получения данных профиля")
		}

		return sender.ShowView(chatID, messageID, domain.ViewTypeProfile, profileData)
	case "btn_reset_devices":
		userID := strconv.FormatInt(chatID, 10)

		if err := subscriptionService.ResetPaidDevices(userID); err != nil {
			return sender.SendMessage(chatID, "❌ Ошибка сброса услуги.")
		}

		profileData, err := buildProfileData(userID)
		if err != nil {
			return sender.SendMessage(chatID, "Ошибка получения данных профиля")
		}

		return sender.ShowView(chatID, messageID, domain.ViewTypeProfile, profileData)
	case "btn_connect":
		username := strconv.FormatInt(chatID, 10)
		url, err := service.GetURLSubscription(remnawaveClient, username)
		if err != nil {
			//qury53: добавь пж обработку ошибки здесь
			//qury53: я просто хз нужно что-то пользователю отправлять

		}

		if url == "" {
			return sender.ShowView(chatID, messageID, domain.ViewTypeConnect, "Не удалось получить ссылку на подключение. Убедитесь, что подписка активна, или обратитесь в поддержку.")
		}
		return sender.ShowView(chatID, messageID, domain.ViewTypeConnect, url)
	case "btn_info":
		// Информация о боте
		return sender.SendMessage(chatID, "В разработке")
	case "btn_back":
		text := buildMainViewText(chatID, firstName, remnawaveClient, userRepo)
		return sender.ShowView(chatID, messageID, domain.ViewTypeMain, text)
	default:
		return sender.SendMessage(chatID, "Неизвестная команда")
	}
}

var activePaymentStatusWatchers sync.Map

func tryStartPaymentStatusWatcher(sender MessageSender, chatID int64, messageID int, transactionID string, paymentGateway domain.PaymentGateway, userRepo *database.UserStorage) bool {
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
				log.Printf("Ошибка проверки статуса транзакции: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}

			switch status {
			case domain.PaymentStatusSuccess:
				if err := handleSuccessfulPayment(sender, chatID, messageID, transactionID, paymentGateway, userRepo); err != nil {
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

func handleSuccessfulPayment(sender MessageSender, chatID int64, messageID int, transactionID string, paymentGateway domain.PaymentGateway, userRepo *database.UserStorage) error {
	info, err := paymentGateway.GetTransactionInfo(context.Background(), transactionID)
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

	return sender.ShowView(chatID, messageID, domain.ViewTypeSubscriptionResult, fmt.Sprintf("✅ Оплата прошла успешно! Ваш баланс пополнен на %d RUB.", amount))
}

func handleSubscriptionFromBalance(sender MessageSender, subscriptionService domain.SubscriptionService, chatID int64, messageID int, months int) error {
	result, err := subscriptionService.ActivateSubscription(chatID, months)
	if err != nil {
		if errors.Is(err, domain.ErrInsufficientFunds) {
			return sender.ShowView(chatID, messageID, domain.ViewTypeSubscriptionResult, "❌ Недостаточно средств на балансе")
		}
		return sender.ShowView(chatID, messageID, domain.ViewTypeSubscriptionResult, "Произошла ошибка при оформлении подписки")
	}

	return sender.ShowView(chatID, messageID, domain.ViewTypeSubscriptionResult, "✅ "+result)
}

// ProcessCommand обрабатывает текстовые команды (например, /start).
// Эта функция — "мозг" обработки текста.
func ProcessCommand(sender MessageSender, chatID int64, command string, firstName string, remnawaveClient domain.RemnawaveClient, userRepo *database.UserStorage) error {
	switch command {
	case "/start":
		userID := strconv.FormatInt(chatID, 10)

		_, err := userRepo.GetUserByID(userID)
		if err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				_, err = userRepo.CreateUser(models.CreateUserTGDTO{
					ID:      userID,
					Balance: 0,
					Trial:   false,
				})
				if err != nil {
					log.Printf("Ошибка создания пользователя в DB: %v", err)
					return sender.SendMessage(chatID, "Ошибка создания пользователя")
				}
			} else {
				log.Printf("Ошибка получения пользователя: %v", err)
				return sender.SendMessage(chatID, "Ошибка получения данных пользователя")
			}
		}

		username := strconv.Itoa(int(chatID))
		if err := remnawaveClient.CreateUser(username, 5); err != nil {
			log.Println("Ошибка создания пользователя")
		}

		text := buildMainViewText(chatID, firstName, remnawaveClient, userRepo)
		return sender.ShowView(chatID, 0, domain.ViewTypeMain, text)
	default:
		return sender.SendMessage(chatID, "Неизвестная команда. Пожалуйста, используйте /start.")
	}
}

func buildMainViewText(chatID int64, firstName string, remnawaveClient domain.RemnawaveClient, userRepo *database.UserStorage) string {
	username := strconv.FormatInt(chatID, 10)

	user, err := userRepo.GetUserByID(username)
	if err != nil {
		// Если пользователя нету, создаем его в базе
		if errors.Is(err, domain.ErrUserNotFound) {
			created, createErr := userRepo.CreateUser(models.CreateUserTGDTO{
				ID:      username,
				Balance: 0,
				Trial:   false,
			})
			if createErr == nil {
				return buildStartText(firstName, created.Balance, buildSubscriptionLine(username, remnawaveClient))
			} else {
				return buildStartText(firstName, 0, buildSubscriptionLine(username, remnawaveClient))
			}
		}
	}

	return buildStartText(firstName, user.Balance, buildSubscriptionLine(username, remnawaveClient))
}

func buildStartText(firstName string, balance int, subscriptionLine string) string {
	name := strings.TrimSpace(firstName)
	if name == "" {
		name = "друг"
	}

	return fmt.Sprintf(
		"🌟 Добро пожаловать, %s!\n<blockquote>—💰 Ваш баланс: %.2f₽\n%s\n</blockquote>\n🚀 Если вам не понятно как подключиться, обратитесь в поддержку, мы отправим инструкцию и поможем\n\n1️⃣ Скачайте приложение по кнопке <u>Скачать приложение</u>. Выберите ваше устройство, iOS или Android и т.д.\n2️⃣ После установки нажмите <u>Подключить (Happ)</u>, он импортирует подписку в Happ",
		name,
		float64(balance),
		subscriptionLine,
	)
}

func buildSubscriptionLine(username string, remnawaveClient domain.RemnawaveClient) string {
	uuid, err := remnawaveClient.GetUUIDByUsername(username)
	if err != nil {
		return "—❌ Подписка не активна"
	}

	info, err := remnawaveClient.GetUserInfo(uuid)
	if err != nil {
		return "—❌ Подписка не активна"
	}

	if strings.EqualFold(info.Response.Status, "ACTIVE") && info.Response.ExpireAt.After(time.Now()) {
		return "—✅ Подписка активна до " + formatRussianDate(info.Response.ExpireAt)
	}

	return "—❌ Подписка не активна"
}

func formatRussianDate(t time.Time) string {
	return fmt.Sprintf("%d %d %s", t.Year(), t.Day(), russianMonthGenitive(t.Month()))
}

func russianMonthGenitive(m time.Month) string {
	switch m {
	case time.January:
		return "января"
	case time.February:
		return "февраля"
	case time.March:
		return "марта"
	case time.April:
		return "апреля"
	case time.May:
		return "мая"
	case time.June:
		return "июня"
	case time.July:
		return "июля"
	case time.August:
		return "августа"
	case time.September:
		return "сентября"
	case time.October:
		return "октября"
	case time.November:
		return "ноября"
	case time.December:
		return "декабря"
	default:
		return ""
	}
}
