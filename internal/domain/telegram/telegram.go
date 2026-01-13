// Package telegram содержит бизнес логику бота и интерфейсы
// для взаимодействия. Определяется как себя будут вести команды и что выполнять
//
//nolint:maintidx,goconst
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
// Sender: кто будет отправлять ответ (наш telegram клиент)
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
			return "", fmt.Errorf("ошибка получения пользователя: %w", err)
		}

		// Получаем 100% точный результат количества устройств
		extraCount, err := userRepo.CountActiveDeviceAddons(userID)
		if err != nil {
			return "", fmt.Errorf("ошибка подсчета активных дополнений устройств: %w", err)
		}

		// Проверяем
		if user.ExtraDevicesCount != extraCount {
			_, err = userRepo.UpdateUser(userID, models.UpdateUserTGDTO{
				ExtraDevicesCount: &extraCount,
			})
			if err != nil {
				return "", fmt.Errorf("ошибка обновления пользователя: %w", err)
			}
		}

		return user.ID + "|" + strconv.Itoa(user.Balance) + "|" + strconv.Itoa(extraCount), nil
	}

	if amountStr, ok := strings.CutPrefix(data, "btn_pay_sbp_"); ok {
		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			log.Printf("Ошибка конвертации суммы платежа: %v", err)

			if sendErr := sender.SendMessage(chatID, "Ошибка обработки суммы"); sendErr != nil {
				return fmt.Errorf("ошибка обработки суммы: %w", sendErr)
			}

			return nil // Мы обработали ошибку, отправив сообщение
		}

		orderID := fmt.Sprintf("tg_%d_%d_%d", chatID, amount, time.Now().UnixNano())

		url, id, err := paymentGateway.CreateTransaction(
			context.Background(),
			float64(amount),
			orderID,
		)
		if err != nil {
			log.Printf("Ошибка создания транзакции: %v", err)
			if sendErr := sender.SendMessage(chatID, "Ошибка создания транзакции"); sendErr != nil {
				return fmt.Errorf("не удалось отправить сообщение об ошибке создания транзакции: %w", sendErr)
			}

			return nil
		}

		if err := sender.ShowView(chatID, messageID, domain.ViewTypeCheckPayment, url+"|"+id); err != nil {
			return fmt.Errorf("ошибка отображения QR-кода для оплаты: %w", err)
		}

		return nil
	}

	if transactionID, ok := strings.CutPrefix(data, "btn_check_payment_"); ok {
		status, err := paymentGateway.CheckStatus(context.Background(), transactionID)
		if err != nil {
			log.Printf("Ошибка проверки статуса транзакции: %v", err)
			if sendErr := sender.SendMessage(chatID, "Ошибка проверки статуса платежа"); sendErr != nil {
				return fmt.Errorf("не удалось отправить сообщение об ошибке проверки статуса: %w", sendErr)
			}
			return nil
		}

		switch status {
		case domain.PaymentStatusSuccess:
			// handleSuccessfulPayment уже содержит логику отправки сообщений и обработки ошибок
			return handleSuccessfulPayment(
				sender,
				chatID,
				messageID,
				transactionID,
				paymentGateway,
				userRepo,
			)
		case domain.PaymentStatusPending:
			started := tryStartPaymentStatusWatcher(
				sender,
				chatID,
				messageID,
				transactionID,
				paymentGateway,
				userRepo,
			)
			if started {
				if err := sender.SendMessage(
					chatID,
					"⏳ Оплата еще не поступила. Я буду автоматически проверять статус каждые 2 секунды и сообщу, когда платеж подтвердится.",
				); err != nil {
					return fmt.Errorf("ошибка отправки сообщения об ожидании платежа: %w", err)
				}
			} else {
				if err := sender.SendMessage(
					chatID,
					"⏳ Автопроверка уже запущена. Я сообщу, когда платеж подтвердится.",
				); err != nil {
					return fmt.Errorf("ошибка отправки сообщения о запущенной автопроверке: %w", err)
				}
			}
		default:
			if err := sender.SendMessage(chatID, "❌ Оплата не прошла или отменена."); err != nil {
				return fmt.Errorf("ошибка отправки сообщения о неудачной оплате: %w", err)
			}
		}
		return nil
	}

	if strings.HasPrefix(data, "btn_pay_crypto_") {
		if err := sender.SendMessage(chatID, "Криптовалюта пока не поддерживается"); err != nil {
			return fmt.Errorf("ошибка отправки сообщения о неподдерживаемой криптовалюте: %w", err)
		}
		return nil
	}

	// Обертка для вызовов sender.ShowView для упрощения
	showView := func(viewType domain.ViewType, data string, errMsg string) error {
		if err := sender.ShowView(chatID, messageID, viewType, data); err != nil {
			return fmt.Errorf("%s: %w", errMsg, err)
		}
		return nil
	}

	switch data {
	case "btn_tariffs":
		return showView(domain.ViewTypeTariffs, "", "ошибка отображения тарифов")
	case "btn_sub_tariff_1":
		return handleSubscriptionFromBalance(sender, subscriptionService, chatID, messageID, 1)
	case "btn_sub_tariff_2":
		return handleSubscriptionFromBalance(sender, subscriptionService, chatID, messageID, 2)
	case "btn_sub_tariff_3":
		return handleSubscriptionFromBalance(sender, subscriptionService, chatID, messageID, 3)
	case "btn_balance":
		return showView(domain.ViewTypeTopUp, "", "ошибка отображения меню пополнения")
	case "btn_topUp_100":
		return showView(domain.ViewTypePayment, "100", "ошибка отображения вариантов оплаты")
	case "btn_topUp_200":
		return showView(domain.ViewTypePayment, "200", "ошибка отображения вариантов оплаты")
	case "btn_topUp_300":
		return showView(domain.ViewTypePayment, "300", "ошибка отображения вариантов оплаты")
	case "btn_topUp_500":
		return showView(domain.ViewTypePayment, "500", "ошибка отображения вариантов оплаты")
	case "btn_topUp_700":
		return showView(domain.ViewTypePayment, "700", "ошибка отображения вариантов оплаты")
	case "btn_topUp_1000":
		return showView(domain.ViewTypePayment, "1000", "ошибка отображения вариантов оплаты")
	case "download_app":
		return showView(domain.ViewTypeDownloadApp, "", "ошибка отображения меню загрузки")
	case "btn_ios_menu":
		return showView(domain.ViewTypeIosRegion, "", "ошибка отображения меню iOS")
	case "btn_back_download_app":
		return showView(domain.ViewTypeDownloadApp, "", "ошибка возврата к меню загрузки")
	case "btn_unlimits":
		return showView(domain.ViewTypeDeviceLimits, "", "ошибка отображения лимитов устройств")
	case "btn_traffic_limits":
		return showView(domain.ViewTypeTrafficLimits, "", "ошибка отображения лимитов трафика")
	case "btn_profile":
		userID := strconv.FormatInt(chatID, 10)

		_, err := userRepo.GetUserByID(userID)
		if err != nil {
			if !errors.Is(err, domain.ErrUserNotFound) {
				log.Printf("Ошибка получения пользователя: %v", err)
				if sendErr := sender.SendMessage(chatID, "Ошибка получения данных пользователя"); sendErr != nil {
					return fmt.Errorf("не удалось отправить сообщение об ошибке получения пользователя: %w", sendErr)
				}
				return nil
			}
			// Если пользователь не найден, создаем нового
			if _, err = userRepo.CreateUser(models.CreateUserTGDTO{
				ID:      userID,
				Balance: 0,
				Trial:   false,
			}); err != nil {
				log.Printf("Ошибка создания пользователя в DB: %v", err)
				errorMsg := "ошибка создания пользователя"
				if errors.Is(err, domain.ErrDuplicateKey) {
					errorMsg = "пользователь с таким ID уже существует"
				} else if errors.Is(err, domain.ErrDatabaseConnection) {
					errorMsg = "временные проблемы с базой данных"
				}
				if sendErr := sender.SendMessage(chatID, errorMsg); sendErr != nil {
					return fmt.Errorf("не удалось отправить сообщение об ошибке создания пользователя: %w", sendErr)
				}
				return nil
			}
		}

		profileData, err := buildProfileData(userID)
		if err != nil {
			log.Printf("Ошибка сборки данных профиля: %v", err)
			if sendErr := sender.SendMessage(chatID, "Ошибка получения данных профиля"); sendErr != nil {
				return fmt.Errorf("не удалось отправить сообщение об ошибке сборки профиля: %w", sendErr)
			}
			return nil
		}
		return showView(domain.ViewTypeProfile, profileData, "ошибка отображения профиля")

	case "btn_add_device":
		userID := strconv.FormatInt(chatID, 10)

		if err := subscriptionService.AddPaidDevice(userID); err != nil {
			errorMsg := "❌ Ошибка добавления устройства."
			if errors.Is(err, domain.ErrInsufficientFunds) {
				errorMsg = "❌ Недостаточно средств. Нужно 50₽."
			} else if errors.Is(err, domain.ErrMaxDevices) {
				errorMsg = "❌ Достигнут лимит устройств."
			}
			log.Printf("Ошибка добавления платного устройства для %s: %v", userID, err)
			if sendErr := sender.SendMessage(chatID, errorMsg); sendErr != nil {
				return fmt.Errorf("не удалось отправить сообщение об ошибке добавления устройства: %w", sendErr)
			}
			return nil
		}

		profileData, err := buildProfileData(userID)
		if err != nil {
			log.Printf("Ошибка сборки данных профиля после добавления устройства: %v", err)
			if sendErr := sender.SendMessage(chatID, "Ошибка получения данных профиля"); sendErr != nil {
				return fmt.Errorf("не удалось отправить сообщение об ошибке сборки профиля: %w", sendErr)
			}
			return nil
		}
		return showView(domain.ViewTypeProfile, profileData, "ошибка отображения профиля после добавления устройства")

	case "btn_reset_devices":
		userID := strconv.FormatInt(chatID, 10)

		if err := subscriptionService.ResetPaidDevices(userID); err != nil {
			log.Printf("Ошибка сброса платных устройств для %s: %v", userID, err)
			if sendErr := sender.SendMessage(chatID, "❌ Ошибка сброса услуги."); sendErr != nil {
				return fmt.Errorf("не удалось отправить сообщение об ошибке сброса устройств: %w", sendErr)
			}
			return nil
		}

		profileData, err := buildProfileData(userID)
		if err != nil {
			log.Printf("Ошибка сборки данных профиля после сброса устройств: %v", err)
			if sendErr := sender.SendMessage(chatID, "Ошибка получения данных профиля"); sendErr != nil {
				return fmt.Errorf("не удалось отправить сообщение об ошибке сборки профиля: %w", sendErr)
			}
			return nil
		}
		return showView(domain.ViewTypeProfile, profileData, "ошибка отображения профиля после сброса устройств")

	case "btn_connect":
		username := strconv.FormatInt(chatID, 10)
		url, err := service.GetURLSubscription(remnawaveClient, username)
		if err != nil {
			// qury53: добавь пж обработку ошибки здесь
			// qury53: я просто хз нужно что-то пользователю отправлять
			log.Printf("Ошибка получения URL подписки для %s: %v", username, err)
		}

		if url == "" {
			return showView(
				domain.ViewTypeConnect,
				"Не удалось получить ссылку на подключение. Убедитесь, что подписка активна, или обратитесь в поддержку.",
				"ошибка отображения сообщения о пустой ссылке",
			)
		}
		return showView(domain.ViewTypeConnect, url, "ошибка отображения ссылки на подключение")

	case "btn_info":
		if err := sender.SendMessage(chatID, "В разработке"); err != nil {
			return fmt.Errorf("ошибка отправки сообщения 'в разработке': %w", err)
		}
		return nil

	case "btn_back":
		text := buildMainViewText(chatID, firstName, remnawaveClient, userRepo)
		return showView(domain.ViewTypeMain, text, "ошибка возврата в главное меню")

	default:
		if err := sender.SendMessage(chatID, "Неизвестная команда"); err != nil {
			return fmt.Errorf("ошибка отправки сообщения о неизвестной команде: %w", err)
		}
		return nil
	}
	// Этот return не должен достигаться, но нужен для компилятора
	return nil
}

var activePaymentStatusWatchers sync.Map

func tryStartPaymentStatusWatcher(
	sender MessageSender,
	chatID int64,
	messageID int,
	transactionID string,
	paymentGateway domain.PaymentGateway,
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
					userRepo,
				); err != nil {
					log.Printf("Ошибка обработки успешного платежа (внутри горутины): %v", err)
				}
				return // Завершаем горутину
			case domain.PaymentStatusPending:
				time.Sleep(2 * time.Second)
				continue
			default:
				if err := sender.SendMessage(chatID, "❌ Оплата не прошла или отменена."); err != nil {
					log.Printf("Ошибка отправки сообщения о неудачном платеже (внутри горутины): %v", err)
				}
				return // Завершаем горутину
			}
		}
		// Отправляем сообщение по истечении времени
		if err := sender.SendMessage(
			chatID,
			"⏳ Автопроверка остановлена: время ожидания истекло. Нажмите «Проверить оплату» позже.",
		); err != nil {
			log.Printf("Ошибка отправки сообщения об истечении времени ожидания (внутри горутины): %v", err)
		}
	}()

	return true
}

func handleSuccessfulPayment(
	sender MessageSender,
	chatID int64,
	messageID int,
	transactionID string,
	paymentGateway domain.PaymentGateway,
	userRepo *database.UserStorage,
) error {
	info, err := paymentGateway.GetTransactionInfo(context.Background(), transactionID)
	if err != nil {
		log.Printf("Ошибка получения информации о транзакции: %v", err)
		if sendErr := sender.SendMessage(
			chatID,
			"Платеж прошел, но возникла ошибка при получении данных. Обратитесь в поддержку.",
		); sendErr != nil {
			return fmt.Errorf("не удалось отправить сообщение об ошибке получения информации о транзакции: %w", sendErr)
		}
		return nil
	}

	amount := int(info.GetAmount())

	userID := strconv.FormatInt(chatID, 10)
	user, err := userRepo.GetUserByID(userID)
	if err != nil {
		log.Printf("Ошибка получения пользователя для обновления баланса: %v", err)
		if sendErr := sender.SendMessage(chatID, "Ошибка получения данных пользователя"); sendErr != nil {
			return fmt.Errorf("не удалось отправить сообщение об ошибке получения пользователя: %w", sendErr)
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
			return fmt.Errorf("не удалось отправить сообщение об ошибке обновления баланса: %w", sendErr)
		}
		return nil
	}

	successMsg := fmt.Sprintf("✅ Оплата прошла успешно! Ваш баланс пополнен на %d RUB.", amount)
	if err := sender.ShowView(chatID, messageID, domain.ViewTypeSubscriptionResult, successMsg); err != nil {
		return fmt.Errorf("ошибка отображения сообщения об успешной оплате: %w", err)
	}
	return nil
}

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
		if sendErr := sender.ShowView(chatID, messageID, domain.ViewTypeSubscriptionResult, errorMsg); sendErr != nil {
			return fmt.Errorf("не удалось отправить сообщение об ошибке подписки: %w", sendErr)
		}
		return nil
	}

	successMsg := "✅ " + result
	if err := sender.ShowView(chatID, messageID, domain.ViewTypeSubscriptionResult, successMsg); err != nil {
		return fmt.Errorf("ошибка отображения сообщения об успешной подписке: %w", err)
	}
	return nil
}

// ProcessCommand обрабатывает текстовые команды (например, /start).
// Эта функция — "мозг" обработки текста.
func ProcessCommand(
	sender MessageSender,
	chatID int64,
	command string,
	firstName string,
	remnawaveClient domain.RemnawaveClient,
	userRepo *database.UserStorage,
) error {
	switch command {
	case "/start":
		userID := strconv.FormatInt(chatID, 10)

		_, err := userRepo.GetUserByID(userID)
		if err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				// Создаем пользователя, если не найден
				if _, err = userRepo.CreateUser(models.CreateUserTGDTO{
					ID:      userID,
					Balance: 0,
					Trial:   false,
				}); err != nil {
					log.Printf("Ошибка создания пользователя в DB: %v", err)
					if sendErr := sender.SendMessage(chatID, "Ошибка создания пользователя"); sendErr != nil {
						return fmt.Errorf("не удалось отправить сообщение об ошибке создания пользователя: %w", sendErr)
					}
					return nil
				}
			} else {
				// Другая ошибка при поиске пользователя
				log.Printf("Ошибка получения пользователя: %v", err)
				if sendErr := sender.SendMessage(chatID, "Ошибка получения данных пользователя"); sendErr != nil {
					return fmt.Errorf("не удалось отправить сообщение об ошибке получения пользователя: %w", sendErr)
				}
				return nil
			}
		}

		username := strconv.Itoa(int(chatID))
		if err := remnawaveClient.CreateUser(username, 5); err != nil {
			// Ошибка некритична для пользователя, просто логируем
			log.Printf("Не удалось создать пользователя в remnawave: %v", err)
		}

		text := buildMainViewText(chatID, firstName, remnawaveClient, userRepo)
		if err := sender.ShowView(chatID, 0, domain.ViewTypeMain, text); err != nil {
			return fmt.Errorf("ошибка отображения главного меню: %w", err)
		}
		return nil

	default:
		if err := sender.SendMessage(chatID, "Неизвестная команда. Пожалуйста, используйте /start."); err != nil {
			return fmt.Errorf("ошибка отправки сообщения о неизвестной команде: %w", err)
		}
		return nil
	}
}

func buildMainViewText(
	chatID int64,
	firstName string,
	remnawaveClient domain.RemnawaveClient,
	userRepo *database.UserStorage,
) string {
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
				return buildStartText(
					firstName,
					created.Balance,
					buildSubscriptionLine(username, remnawaveClient),
				)
			}
			// В случае ошибки создания, показываем с нулевым балансом
			log.Printf("Не удалось создать пользователя %s при сборке текста: %v", username, createErr)
			return buildStartText(
				firstName,
				0,
				buildSubscriptionLine(username, remnawaveClient),
			)
		}
		// В случае другой ошибки, показываем с нулевым балансом
		log.Printf("Не удалось получить пользователя %s при сборке текста: %v", username, err)
		return buildStartText(
			firstName,
			0,
			buildSubscriptionLine(username, remnawaveClient),
		)
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

	if strings.EqualFold(info.Response.Status, "ACTIVE") &&
		info.Response.ExpireAt.After(time.Now()) {
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
