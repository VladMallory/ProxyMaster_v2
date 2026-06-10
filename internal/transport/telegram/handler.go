// Package telegram содержит бизнес логику для обработки команд и обратных вызовов в Telegram-боте.
// Он определяет, как бот реагирует на действия пользователя, управляет состоянием и взаимодействует
// с другими частями системы, такими как база данных и внешние API.
//
// nolint
package telegram

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	payment "github.com/VladMallory/ProxyMaster_v2/internal/features/billing/domain"
	billingSvc "github.com/VladMallory/ProxyMaster_v2/internal/features/billing/service"
	userstg "github.com/VladMallory/ProxyMaster_v2/internal/features/subscription/users/transport/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/domain"
	"github.com/VladMallory/ProxyMaster_v2/internal/integrations/remnawave"
	"github.com/VladMallory/ProxyMaster_v2/internal/platform/db"

	"go.uber.org/zap"
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
	remnawaveClient remnawave.RemnawaveClient,
	subscriptionService domain.SubscriptionService,
	deviceService domain.DeviceService,
	bSvc *billingSvc.Service,
	userRepo *db.UserStorage,
	cfg *config.Config,
) error {
	buildProfileData := func(userID string) (string, error) {
		// Получаем пользователя из БД (для ID)
		user, err := userRepo.GetUserByID(userID)
		if err != nil {
			return "", fmt.Errorf("ошибка получения пользователя: %w", err)
		}

		// Получаем реальный лимит устройств с RemnaWave
		uuid, err := remnawaveClient.GetUUIDByUsername(context.Background(), userID)
		if err != nil {
			return "", fmt.Errorf("ошибка получения UUID пользователя: %w", err)
		}

		userInfo, err := remnawaveClient.GetUserInfo(uuid)
		if err != nil {
			return "", fmt.Errorf("ошибка получения информации о пользователе: %w", err)
		}

		// Получаем дату следующего списания доп. устройств
		nextChargeAt, err := userRepo.GetNextDeviceAddonChargeAt(userID)
		if err != nil {
			return "", fmt.Errorf("ошибка получения даты следующего списания: %w", err)
		}

		nextPayment := "—"
		if nextChargeAt != nil {
			nextPayment = formatDevicePaymentDate(*nextChargeAt, time.Now())
		}

		// Формируем данные профиля
		// userID | balance (0, так как баланс удален) | deviceLimit (из RemnaWave) | nextPayment
		return user.ID + "|" + strconv.Itoa(
			0,
		) + "|" + strconv.Itoa(
			userInfo.Response.HWIDDeviceLimit,
		) + "|" + nextPayment, nil
	}

	if amountStr, ok := strings.CutPrefix(data, "btn_topUp_"); ok {
		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			log.Printf("Ошибка конвертации суммы платежа: %v", err)

			if sendErr := sender.SendMessage(chatID, "Ошибка обработки суммы"); sendErr != nil {
				return fmt.Errorf("ошибка обработки суммы: %w", sendErr)
			}

			return nil // Мы обработали ошибку, отправив сообщение
		}

		url, id, err := bSvc.CreatePayment(
			context.Background(),
			strconv.FormatInt(chatID, 10),
			amount,
			billingSvc.PurposeSubscription,
		)
		if err != nil {
			log.Printf("Ошибка создания транзакции: %v", err)
			if sendErr := sender.SendMessage(chatID, "Ошибка создания транзакции"); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение об ошибке создания транзакции: %w",
					sendErr,
				)
			}

			return nil
		}

		if err := sender.ShowView(
			chatID,
			messageID,
			domain.ViewTypeCheckPayment,
			url+"|"+id,
		); err != nil {
			return fmt.Errorf("ошибка отображения QR-кода для оплаты: %w", err)
		}

		// Запускаем автоматическую проверку через 15 секунд после создания платежа
		// Это помогает клиентам, которые не догадываются нажать кнопку "Проверить платеж"
		startAutoPaymentCheck(
			sender,
			chatID,
			messageID,
			id,
			bSvc,
			subscriptionService,
			deviceService,
			userRepo,
			remnawaveClient,
			firstName,
			cfg,
		)

		return nil
	}

	if transactionID, ok := strings.CutPrefix(data, "btn_check_payment_"); ok {
		// Останавливаем таймер сообщения при ручной проверке платежа
		stopMessageTimer(chatID, messageID)

		status, err := bSvc.CheckPayment(context.Background(), transactionID)
		if err != nil {
			log.Printf("Ошибка проверки статуса транзакции: %v", err)
			if sendErr := sender.SendMessage(
				chatID,
				"Ошибка проверки статуса платежа",
			); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение об ошибке проверки статуса: %w",
					sendErr,
				)
			}
			return nil
		}

		switch status {
		case payment.PaymentStatusSuccess:
			stopMessageTimer(chatID, messageID)

			return userstg.HandleSuccessfulPayment(
				sender,
				chatID,
				messageID,
				transactionID,
				bSvc,
				subscriptionService,
				deviceService,
				remnawaveClient,
				cfg,
			)
		case payment.PaymentStatusPending:
			started := tryStartPaymentStatusWatcher(
				sender,
				chatID,
				messageID,
				transactionID,
				bSvc,
				subscriptionService,
				deviceService,
				remnawaveClient,
				cfg,
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
					return fmt.Errorf(
						"ошибка отправки сообщения о запущенной автопроверке: %w",
						err,
					)
				}
			}
		default:
			// Останавливаем таймер сообщения при неудачной оплате
			stopMessageTimer(chatID, messageID)

			if err := sender.SendMessage(chatID, "❌ Оплата не прошла или отменена."); err != nil {
				return fmt.Errorf("ошибка отправки сообщения о неудачной оплате: %w", err)
			}
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
	case "btn_balance":
		return showView(domain.ViewTypeTopUp, "", "ошибка отображения меню пополнения")
	case "download_app":
		return showView(domain.ViewTypeDownloadApp, "", "ошибка отображения меню загрузки")
	case "btn_ios_menu":
		return showView(domain.ViewTypeIosRegion, "", "ошибка отображения меню iOS")
	case "btn_unlimits":
		userID := strconv.FormatInt(chatID, 10)

		devices, err := remnawaveClient.GetUserDevice(context.Background(), userID)
		if err != nil {
			log.Printf("Ошибка получения устройств пользователя %s: %v", userID, err)

			return showView(
				domain.ViewTypeDeviceLimits,
				"❌ Не удалось получить список устройств.\nПопробуйте позже.",
				"ошибка отображения лимитов устройств",
			)
		}

		var builder strings.Builder

		if len(devices) == 0 {
			builder.WriteString("К подписке пока не привязано ни одного устройства.")
		} else {
			builder.WriteString("📋 <b>Привязанные устройства:</b>\n\n")

			for i, device := range devices {
				builder.WriteString(fmt.Sprintf("— <b>Устройство %d</b>\n", i+1))
				builder.WriteString(fmt.Sprintf("Платформа: <code>%s</code>\n", valueOrDash(device.Platform)))
				builder.WriteString(fmt.Sprintf("Версия ОС: <code>%s</code>\n", valueOrDash(device.OSVersion)))
				builder.WriteString(fmt.Sprintf("Модель: <code>%s</code>\n\n", valueOrDash(device.DeviceModel)))
			}
		}

		return showView(
			domain.ViewTypeDeviceLimits,
			builder.String(),
			"ошибка отображения лимитов устройств",
		)
	case "btn_traffic_limits":
		return showView(domain.ViewTypeTrafficLimits, "", "ошибка отображения лимитов трафика")
	case "btn_profile":
		userID := strconv.FormatInt(chatID, 10)

		_, err := userRepo.GetUserByID(userID)
		if err != nil {
			if !errors.Is(err, domain.ErrUserNotFound) {
				log.Printf("Ошибка получения пользователя: %v", err)
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
			// Если пользователь не найден, создаем нового
			if _, err = userRepo.CreateUser(domain.CreateUserDTO{
				ID:    userID,
				Trial: false,
			}); err != nil {
				log.Printf("Ошибка создания пользователя в DB: %v", err)
				errorMsg := "ошибка создания пользователя"
				if errors.Is(err, domain.ErrDuplicateKey) {
					errorMsg = "пользователь с таким ID уже существует"
				} else if errors.Is(err, domain.ErrDatabaseConnection) {
					errorMsg = "временные проблемы с базой данных"
				}
				if sendErr := sender.SendMessage(chatID, errorMsg); sendErr != nil {
					return fmt.Errorf(
						"не удалось отправить сообщение об ошибке создания пользователя: %w",
						sendErr,
					)
				}
				return nil
			}
		}

		profileData, err := buildProfileData(userID)
		if err != nil {
			log.Printf("Ошибка сборки данных профиля: %v", err)
			if sendErr := sender.SendMessage(
				chatID,
				"Ошибка получения данных профиля",
			); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение об ошибке сборки профиля: %w",
					sendErr,
				)
			}
			return nil
		}
		return showView(domain.ViewTypeProfile, profileData, "ошибка отображения профиля")

	case "btn_add_device":
		userID := strconv.FormatInt(chatID, 10)
		uuid, err := remnawaveClient.GetUUIDByUsername(context.Background(), userID)
		if err != nil {
			log.Printf("Ошибка получения UUID пользователя %s: %v", userID, err)
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

		userInfo, err := remnawaveClient.GetUserInfo(uuid)
		if err != nil {
			log.Printf("Ошибка получения информации о пользователе %s: %v", userID, err)
			if sendErr := sender.SendMessage(
				chatID,
				"Ошибка получения данных пользователя",
			); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение об ошибке получения данных пользователя: %w",
					sendErr,
				)
			}
			return nil
		}

		if userInfo.Response.HWIDDeviceLimit >= cfg.MaxDeviceLimit {
			if sendErr := sender.SendMessage(
				chatID,
				"❌ Достигнут лимит устройств.",
			); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение о превышении лимита устройств: %w",
					sendErr,
				)
			}
			return nil
		}

		const addDevicePriceRUB = 50
		url, id, err := bSvc.CreatePayment(
			context.Background(),
			strconv.FormatInt(chatID, 10),
			addDevicePriceRUB,
			billingSvc.PurposeAddDevice,
		)
		if err != nil {
			log.Printf("Ошибка создания транзакции на добавление устройства: %v", err)
			if sendErr := sender.SendMessage(chatID, "Ошибка создания транзакции"); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение об ошибке создания транзакции: %w",
					sendErr,
				)
			}
			return nil
		}

		if err := sender.ShowView(
			chatID,
			messageID,
			domain.ViewTypeCheckPayment,
			url+"|"+id,
		); err != nil {
			return fmt.Errorf("ошибка отображения ссылки на оплату устройства: %w", err)
		}

		startAutoPaymentCheck(
			sender,
			chatID,
			messageID,
			id,
			bSvc,
			subscriptionService,
			deviceService,
			userRepo,
			remnawaveClient,
			firstName,
			cfg,
		)

		return nil

	case "btn_prepay_devices":
		userID := strconv.FormatInt(chatID, 10)
		activeAddons, err := userRepo.CountActiveDeviceAddons(userID)
		if err != nil {
			log.Printf("Ошибка подсчета доп. устройств для %s: %v", userID, err)
			if sendErr := sender.SendMessage(
				chatID,
				"Ошибка получения данных пользователя",
			); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение об ошибке получения данных пользователя: %w",
					sendErr,
				)
			}
			return nil
		}
		if activeAddons <= 0 {
			if sendErr := sender.SendMessage(
				chatID,
				"У вас нет активных доп. устройств для продления.",
			); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение о отсутствии доп. устройств: %w",
					sendErr,
				)
			}
			return nil
		}

		// Цена за устройство
		amount := activeAddons * 50
		url, id, err := bSvc.CreatePayment(
			context.Background(),
			strconv.FormatInt(chatID, 10),
			amount,
			billingSvc.PurposePrepayDevices,
		)
		if err != nil {
			log.Printf("Ошибка создания транзакции на предоплату устройств: %v", err)
			if sendErr := sender.SendMessage(chatID, "Ошибка создания транзакции"); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение об ошибке создания транзакции: %w",
					sendErr,
				)
			}
			return nil
		}

		if err := sender.ShowView(
			chatID,
			messageID,
			domain.ViewTypeCheckPayment,
			url+"|"+id,
		); err != nil {
			return fmt.Errorf("ошибка отображения ссылки на оплату устройств: %w", err)
		}

		startAutoPaymentCheck(
			sender,
			chatID,
			messageID,
			id,
			bSvc,
			subscriptionService,
			deviceService,
			userRepo,
			remnawaveClient,
			firstName,
			cfg,
		)

		return nil

	case "btn_reset_traffic_payment":
		userID := strconv.FormatInt(chatID, 10)
		_, err := remnawaveClient.GetUUIDByUsername(context.Background(), userID)
		if err != nil {
			log.Printf("Ошибка получения UUID пользователя %s: %v", userID, err)
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

		const resetTrafficPriceRUB = 50
		url, id, err := bSvc.CreatePayment(
			context.Background(),
			strconv.FormatInt(chatID, 10),
			resetTrafficPriceRUB,
			billingSvc.PurposeResetTraffic,
		)
		if err != nil {
			log.Printf("Ошибка создания транзакции на сброс трафика: %v", err)
			if sendErr := sender.SendMessage(chatID, "Ошибка создания транзакции"); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение об ошибке создания транзакции: %w",
					sendErr,
				)
			}
			return nil
		}

		if err := sender.ShowView(
			chatID,
			messageID,
			domain.ViewTypeCheckPayment,
			url+"|"+id,
		); err != nil {
			return fmt.Errorf("ошибка отображения ссылки на оплату сброса трафика: %w", err)
		}

		startAutoPaymentCheck(
			sender,
			chatID,
			messageID,
			id,
			bSvc,
			subscriptionService,
			deviceService,
			userRepo,
			remnawaveClient,
			firstName,
			cfg,
		)

		return nil

	case "btn_reset_devices":
		userID := strconv.FormatInt(chatID, 10)

		err := remnawaveClient.DeleteDeviceHWID(context.Background(), userID)
		if err != nil {
			// Отправка пользователю что не вышло
			if errSend := sender.SendMessage(
				chatID,
				"Не удалось отключить устройства от подписки. Обратитесь к администратору",
			); errSend != nil {
				return fmt.Errorf("не удалось отправить пользователю сообщение об ошибке %w: ", err)
			}

			return fmt.Errorf("не удалось удалить устройства с подписки")
		}

		return showView(
			domain.ViewTypeSubscriptionResult,
			"Устройства успешно сброшены ✅",
			"Ошибка отображения главного меню после сброса устройств",
		)

	case "btn_connect_error":
		return showView(
			domain.ViewTypeConnect,
			"Не удалось получить ссылку на подключение. Убедитесь, что подписка активна, или обратитесь в поддержку.",
			"ошибка отображения сообщения о пустой ссылке",
		)

	case "btn_info":
		return showView(domain.ViewTypeServiceInfo, "", "ошибка отображения информации о сервисе")

	case "btn_privacy_policy":
		return showView(
			domain.ViewTypePrivacyPolicy,
			"",
			"ошибка отображения политики конфиденциальности",
		)
	case "btn_user_agreement":
		return showView(
			domain.ViewTypeUserAgreement,
			"",
			"ошибка отображения пользовательского соглашения",
		)

	case "btn_back":
		text := buildMainViewText(chatID, firstName, remnawaveClient, userRepo)
		return showView(domain.ViewTypeMain, text, "ошибка возврата в главное меню")

	default:
		if err := sender.SendMessage(chatID, "Неизвестная команда"); err != nil {
			return fmt.Errorf("ошибка отправки сообщения о неизвестной команде: %w", err)
		}

		return nil
	}
}

var (
	activePaymentStatusWatchers sync.Map
	messageTimers               sync.Map // chatID|messageID -> *time.Timer
)

// startMessageTimer запускает таймер для сообщения об оплате.
// Через указанное время сообщение автоматически меняется на главное меню.
// Это защита от ситуации, когда пользователь не оплатил в течение разумного времени.
func startMessageTimer(
	sender MessageSender,
	chatID int64,
	messageID int,
	duration time.Duration,
	firstName string,
	remnawaveClient remnawave.RemnawaveClient,
	userRepo *db.UserStorage,
) {
	key := fmt.Sprintf("%d|%d", chatID, messageID)

	// Проверяем, не запущен ли уже таймер для этого сообщения
	if _, exists := messageTimers.Load(key); exists {
		log.Printf("[ТАЙМЕР] Таймер для сообщения %d в чате %d уже запущен", messageID, chatID)
		return
	}

	log.Printf(
		"[ТАЙМЕР] Запускаем таймер на %v для сообщения %d в чате %d",
		duration,
		messageID,
		chatID,
	)

	timer := time.AfterFunc(duration, func() {
		log.Printf(
			"[ТАЙМЕР] Таймер истек для сообщения %d в чате %d, показываем главное меню",
			messageID,
			chatID,
		)

		// Собираем текст для главного меню
		text := buildMainViewText(chatID, firstName, remnawaveClient, userRepo)

		// Показываем главное меню
		if err := sender.ShowView(chatID, messageID, domain.ViewTypeMain, text); err != nil {
			log.Printf("[ТАЙМЕР] Ошибка показа главного меню: %v", err)
		}

		// Удаляем таймер из мапы
		messageTimers.Delete(key)
	})

	// Сохраняем таймер в мапе
	messageTimers.Store(key, timer)
}

// stopMessageTimer останавливает таймер для сообщения об оплате.
// Вызывается при успешной оплате или когда пользователь вручную проверяет платеж.
func stopMessageTimer(chatID int64, messageID int) {
	key := fmt.Sprintf("%d|%d", chatID, messageID)

	if timer, exists := messageTimers.Load(key); exists {
		log.Printf("[ТАЙМЕР] Останавливаем таймер для сообщения %d в чате %d", messageID, chatID)

		if t, ok := timer.(*time.Timer); ok {
			t.Stop()
		}

		// Удаляем таймер из мапы
		messageTimers.Delete(key)
	}
}

// startAutoPaymentCheck запускает автоматическую проверку платежа.
// Начинает проверку через 15 секунд после создания, затем проверяет каждые 5 секунд в течение 20 минут.
// Эта функция нужна для того, чтобы клиентам не приходилось вручную нажимать "Проверить платеж".
// Также запускает таймер на 18 минут для автоматического возврата в главное меню.
func startAutoPaymentCheck(
	sender MessageSender,
	chatID int64,
	messageID int,
	transactionID string,
	bSvc *billingSvc.Service,
	subscriptionService domain.SubscriptionService,
	deviceService domain.DeviceService,
	userRepo *db.UserStorage,
	remnawaveClient remnawave.RemnawaveClient,
	firstName string,
	cfg *config.Config,
) {
	// 1. Запускаем таймер сообщения на 18 минут
	// Если пользователь не оплатит за это время, сообщение автоматически сменится на главное меню
	startMessageTimer(
		sender,
		chatID,
		messageID,
		18*time.Minute,
		firstName,
		remnawaveClient,
		userRepo,
	)

	// 2. Проверяем, не запущена ли уже проверка для этой транзакции
	_, alreadyRunning := activePaymentStatusWatchers.LoadOrStore(transactionID, struct{}{})
	if alreadyRunning {
		log.Printf(
			"[АВТОПРОВЕРКА] Проверка для транзакции %s уже запущена, пропускаем",
			transactionID,
		)
		return
	}

	go func() {
		// Удаляем транзакцию из активных при завершении горутины
		defer activePaymentStatusWatchers.Delete(transactionID)

		log.Printf(
			"[АВТОПРОВЕРКА] Запущена для транзакции %s, ожидание 15 секунд...",
			transactionID,
		)

		// Ждем 15 секунд перед первой проверкой (даем время на оплату)
		time.Sleep(15 * time.Second)

		deadline := time.Now().Add(20 * time.Minute)
		checkInterval := 30 * time.Second

		for time.Now().Before(deadline) {
			log.Printf("[АВТОПРОВЕРКА] Проверяем статус транзакции %s", transactionID)

			status, err := bSvc.CheckPayment(context.Background(), transactionID)
			if err != nil {
				log.Printf(
					"[АВТОПРОВЕРКА] Ошибка проверки статуса транзакции %s: %v",
					transactionID,
					err,
				)
				time.Sleep(checkInterval)
				continue
			}

			log.Printf("[АВТОПРОВЕРКА] Статус транзакции %s: %s", transactionID, status)

			switch status {
			case payment.PaymentStatusSuccess:
				log.Printf("[АВТОПРОВЕРКА] Платеж %s успешен, обрабатываем...", transactionID)

				stopMessageTimer(chatID, messageID)

				if err := userstg.HandleSuccessfulPayment(
					sender,
					chatID,
					messageID,
					transactionID,
					bSvc,
					subscriptionService,
					deviceService,
					remnawaveClient,
					cfg,
				); err != nil {
					log.Printf(
						"[АВТОПРОВЕРКА] Ошибка обработки успешного платежа %s: %v",
						transactionID,
						err,
					)
				} else {
					log.Printf("[АВТОПРОВЕРКА] Платеж %s успешно обработан!", transactionID)
				}
				return // Завершаем горутину после успешной обработки

			case payment.PaymentStatusFailed:
				log.Printf("[АВТОПРОВЕРКА] Платеж %s отменен или не прошел", transactionID)
				return // Завершаем горутину, платеж точно не пройдет

			case payment.PaymentStatusPending:
				// Платеж еще в ожидании, продолжаем проверять
				time.Sleep(checkInterval)
				continue
			}
		}

		log.Printf("[АВТОПРОВЕРКА] Время ожидания истекло для транзакции %s", transactionID)
	}()
}

func tryStartPaymentStatusWatcher(
	sender MessageSender,
	chatID int64,
	messageID int,
	transactionID string,
	bSvc *billingSvc.Service,
	subscriptionService domain.SubscriptionService,
	deviceService domain.DeviceService,
	remnawaveClient remnawave.RemnawaveClient,
	cfg *config.Config,
) bool {
	_, loaded := activePaymentStatusWatchers.LoadOrStore(transactionID, struct{}{})
	if loaded {
		return false
	}

	go func() {
		defer activePaymentStatusWatchers.Delete(transactionID)

		deadline := time.Now().Add(2 * time.Minute)
		for time.Now().Before(deadline) {
			status, err := bSvc.CheckPayment(context.Background(), transactionID)
			if err != nil {
				log.Printf("Ошибка проверки статуса транзакции (внутри горутины): %v", err)
				time.Sleep(2 * time.Second)
				continue
			}

			switch status {
			case payment.PaymentStatusSuccess:
				stopMessageTimer(chatID, messageID)

				if err := userstg.HandleSuccessfulPayment(
					sender,
					chatID,
					messageID,
					transactionID,
					bSvc,
					subscriptionService,
					deviceService,
					remnawaveClient,
					cfg,
				); err != nil {
					log.Printf("Ошибка обработки успешного платежа (внутри горутины): %v", err)
				}
				return // Завершаем горутину
			case payment.PaymentStatusPending:
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



// ProcessCommand обрабатывает текстовые команды (например, /start).
// Эта функция — "мозг" обработки текста.
func ProcessCommand(
	sender MessageSender,
	chatID int64,
	command string,
	firstName string,
	telegramUsername string,
	remnawaveClient remnawave.RemnawaveClient,
	userRepo *db.UserStorage,
	logger *zap.Logger,
	adminID int64,
) error {
	// проверка на админа
	isAdmin, err := IsAdmin(
		sender,
		chatID,
		command,
		adminID,
		userRepo,
		logger,
	)
	if err != nil {
		logger.Error("Ошибка проверка на администратора")
		return nil
	}
	// если админская команда, пропускаем дальнейшую обработку
	if isAdmin {
		return nil
	}

	switch command {
	case "/start":
		userID := strconv.FormatInt(chatID, 10)

		// Создаем в BD
		_, err := userRepo.GetUserByID(userID)
		if err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
				// Создаем пользователя, если не найден
				if _, err = userRepo.CreateUser(domain.CreateUserDTO{
					ID:    userID,
					Trial: false,
				}); err != nil {
					log.Printf("Ошибка создания пользователя в DB: %v", err)
					if sendErr := sender.SendMessage(
						chatID,
						"Ошибка создания пользователя",
					); sendErr != nil {
						return fmt.Errorf(
							"не удалось отправить сообщение об ошибке создания пользователя: %w",
							sendErr,
						)
					}
					return nil
				}
			} else {
				// Другая ошибка при поиске пользователя
				log.Printf("Ошибка получения пользователя: %v", err)
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
		}

		username := strconv.Itoa(int(chatID))
		dto := remnawave.CreateUserDTO{
			Username: username,
			Days:     5,
			FullName: firstName,
			Telegram: telegramUsername,
		}

		if err := remnawaveClient.CreateUser(dto); err != nil {
			// Ошибка некритична для пользователя, просто логируем
			log.Printf("Не удалось создать пользователя в remnawave: %v", err)
		}

		// В будущем активировать
		// _, err = subscriptionService.ActivateSubscription(chatID, 5)
		// if err != nil {
		// 	return err
		// }

		text := buildMainViewText(chatID, firstName, remnawaveClient, userRepo)
		if err := sender.ShowView(chatID, 0, domain.ViewTypeMain, text); err != nil {
			return fmt.Errorf("ошибка отображения главного меню: %w", err)
		}
		return nil

	default:
		if err := sender.SendMessage(
			chatID,
			"Неизвестная команда. Пожалуйста, используйте /start.",
		); err != nil {
			return fmt.Errorf("ошибка отправки сообщения о неизвестной команде: %w", err)
		}
		return nil
	}
}

func buildMainViewText(
	chatID int64,
	firstName string,
	remnawaveClient remnawave.RemnawaveClient,
	userRepo *db.UserStorage,
) string {
	username := strconv.FormatInt(chatID, 10)

	_, err := userRepo.GetUserByID(username)
	if err != nil {
		// Если пользователя нету, создаем его в базе
		if errors.Is(err, domain.ErrUserNotFound) {
			_, createErr := userRepo.CreateUser(domain.CreateUserDTO{
				ID:    username,
				Trial: false,
			})
			if createErr == nil {
				return buildStartText(firstName, buildSubscriptionLine(username, remnawaveClient))
			}
			// В случае ошибки создания, показываем с нулевым балансом
			log.Printf(
				"Не удалось создать пользователя %s при сборке текста: %v",
				username,
				createErr,
			)
			return buildStartText(firstName, buildSubscriptionLine(username, remnawaveClient))
		}
		// В случае другой ошибки, показываем с нулевым балансом
		log.Printf("Не удалось получить пользователя %s при сборке текста: %v", username, err)
		return buildStartText(firstName, buildSubscriptionLine(username, remnawaveClient))
	}

	return buildStartText(firstName, buildSubscriptionLine(username, remnawaveClient))
}

func buildStartText(firstName string, subscriptionLine string) string {
	name := strings.TrimSpace(firstName)
	if name == "" {
		name = "друг"
	}

	return fmt.Sprintf(
		"🌟 Добро пожаловать, %s!\n<blockquote>%s\n</blockquote>\n🚀 Если вам не понятно как подключиться, обратитесь в поддержку, мы отправим инструкцию и поможем\n\n1️⃣ Скачайте приложение по кнопке <u>Скачать приложение</u>. Выберите ваше устройство, iOS или Android и т.д.\n2️⃣ После установки нажмите <u>Подключить (Happ)</u>, откроется сайт, промотайте вниз и нажмите <u>Добавить подписку</u>, он импортирует подписку в Happ",
		name,
		subscriptionLine,
	)
}

func buildSubscriptionLine(username string, remnawaveClient remnawave.RemnawaveClient) string {
	ctx := context.Background()
	uuid, err := remnawaveClient.GetUUIDByUsername(ctx, username)
	if err != nil {
		return "—❌ Подписка не активна"
	}

	info, err := remnawaveClient.GetUserInfo(uuid)
	if err != nil {
		return "—❌ Подписка не активна"
	}

	deviceLimit := info.Response.HWIDDeviceLimit

	// Если от remnawave прилетело 0 (дефолтное значение)
	// то берем данные из env, преобразуем в int и подставляем его в /start
	if deviceLimit == 0 {
		envDeviceLimit := os.Getenv("DEVICE_LIMIT")
		if envDeviceLimit != "" {
			if parsed, err := strconv.Atoi(envDeviceLimit); err == nil && parsed > 0 {
				deviceLimit = parsed
			}
		}
	}

	if strings.EqualFold(info.Response.Status, "ACTIVE") &&
		info.Response.ExpireAt.After(time.Now()) {
		return "—✅ Подписка активна до " + formatRussianDate(info.Response.ExpireAt) + "\n—📱 Лимит устройств: " + strconv.Itoa(deviceLimit)
	}

	return "—❌ Подписка не активна\n—📱 Лимит устройств: " + strconv.Itoa(deviceLimit)
}

func valueOrDash(value *string) string {
	if value == nil || *value == "" {
		return "-"
	}

	return *value
}

func formatRussianDate(t time.Time) string {
	return fmt.Sprintf("%d %d %s", t.Year(), t.Day(), russianMonthGenitive(t.Month()))
}

func formatDevicePaymentDate(t time.Time, now time.Time) string {
	if t.Year() == now.Year() {
		return fmt.Sprintf("%d %s", t.Day(), russianMonthGenitive(t.Month()))
	}
	return fmt.Sprintf("%d %s %d", t.Day(), russianMonthGenitive(t.Month()), t.Year())
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
