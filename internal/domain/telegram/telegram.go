// Package telegram содержит бизнес логику для обработки команд и обратных вызовов в Telegram-боте.
// Он определяет, как бот реагирует на действия пользователя, управляет состоянием и взаимодействует
// с другими частями системы, такими как база данных и внешние API.
//
// nolint
package telegram

import (
	"ProxyMaster_v2/internal/database"
	"ProxyMaster_v2/internal/domain"
	"ProxyMaster_v2/internal/models"
	"ProxyMaster_v2/internal/service"
	"ProxyMaster_v2/pkg/logger"
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

		nextChargeAt, err := userRepo.GetNextDeviceAddonChargeAt(userID)
		if err != nil {
			return "", fmt.Errorf("ошибка получения даты следующего списания: %w", err)
		}

		nextPayment := "—"
		if nextChargeAt != nil {
			nextPayment = formatDevicePaymentDate(*nextChargeAt, time.Now())
		}

		return user.ID + "|" + strconv.Itoa(user.Balance) + "|" + strconv.Itoa(extraCount) + "|" + nextPayment, nil
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

		orderID := fmt.Sprintf("tg_%d_%d_%d", chatID, amount, time.Now().UnixNano())

		url, id, err := paymentGateway.CreateTransaction(
			context.Background(),
			float64(amount),
			orderID,
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
			paymentGateway,
			subscriptionService,
			userRepo,
			remnawaveClient,
		)

		return nil
	}

	if transactionID, ok := strings.CutPrefix(data, "btn_check_payment_"); ok {
		status, err := paymentGateway.CheckStatus(context.Background(), transactionID)
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
		case domain.PaymentStatusSuccess:
			// handleSuccessfulPayment уже содержит логику отправки сообщений и обработки ошибок
			return handleSuccessfulPayment(
				sender,
				chatID,
				messageID,
				transactionID,
				paymentGateway,
				subscriptionService,
				userRepo,
				remnawaveClient,
			)
		case domain.PaymentStatusPending:
			started := tryStartPaymentStatusWatcher(
				sender,
				chatID,
				messageID,
				transactionID,
				paymentGateway,
				subscriptionService,
				userRepo,
				remnawaveClient,
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
	case "btn_sub_tariff_1":
		return handleSubscriptionFromBalance(sender, subscriptionService, chatID, messageID, 1)
	case "btn_sub_tariff_2":
		return handleSubscriptionFromBalance(sender, subscriptionService, chatID, messageID, 2)
	case "btn_sub_tariff_3":
		return handleSubscriptionFromBalance(sender, subscriptionService, chatID, messageID, 3)
	case "btn_balance":
		return showView(domain.ViewTypeTopUp, "", "ошибка отображения меню пополнения")
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
	case "btn_add_50gb":
		userID := strconv.Itoa(int(chatID))
		if err := remnawaveClient.AddTraffic(userID, 50); err != nil {
			//заменить логгер
			log.Printf("не удалось добавить трафик у пользователя %s, %v", userID, err)
			return sender.SendMessage(chatID, "Не удалось добавить трафик")
		}

		profileData, err := buildProfileData(userID)
		if err != nil {
			return sender.SendMessage(chatID, "Ошибка получения данных профиля")
		}

		return sender.ShowView(chatID, messageID, domain.ViewTypeProfile, profileData)

	case "btn_add_100gb":
		userID := strconv.Itoa(int(chatID))
		if err := remnawaveClient.AddTraffic(userID, 100); err != nil {
			//заменить логгер
			log.Printf("не удалось добавить трафик у пользователя %s, %v", userID, err)
			return sender.SendMessage(chatID, "Не удалось добавить трафик")
		}

		profileData, err := buildProfileData(userID)
		if err != nil {
			return sender.SendMessage(chatID, "Ошибка получения данных профиля")
		}

		return sender.ShowView(chatID, messageID, domain.ViewTypeProfile, profileData)

	case "btn_reset_traffic":
		userID := strconv.Itoa(int(chatID))
		if err := remnawaveClient.BetterResetTraffic(context.Background(), userID); err != nil {
			//заменить логгер
			log.Printf("не удалось сбросить трафик у пользователя %s, %v", userID, err)
			return sender.SendMessage(chatID, "Не удалось сбросить трафик")
		}

		profileData, err := buildProfileData(userID)
		if err != nil {
			return sender.SendMessage(chatID, "Ошибка получения данных профиля")
		}

		return sender.ShowView(chatID, messageID, domain.ViewTypeProfile, profileData)

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
		uuid, err := remnawaveClient.GetUUIDByUsername(userID)
		if err != nil {
			log.Printf("Ошибка получения UUID пользователя %s: %v", userID, err)
			if sendErr := sender.SendMessage(chatID, "Ошибка получения данных пользователя"); sendErr != nil {
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
			if sendErr := sender.SendMessage(chatID, "Ошибка получения данных пользователя"); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение об ошибке получения данных пользователя: %w",
					sendErr,
				)
			}
			return nil
		}

		if userInfo.Response.HWIDDeviceLimit >= 5 {
			if sendErr := sender.SendMessage(chatID, "❌ Достигнут лимит устройств."); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение о превышении лимита устройств: %w",
					sendErr,
				)
			}
			return nil
		}

		const addDevicePriceRUB = 50
		orderID := fmt.Sprintf("tg_add_device_%d_%d", chatID, time.Now().UnixNano())
		url, id, err := paymentGateway.CreateTransaction(
			context.Background(),
			float64(addDevicePriceRUB),
			orderID,
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

		paymentPurposeByTransaction.Store(id, paymentPurposeData{
			purpose: paymentPurposeAddDevice,
			amount:  addDevicePriceRUB,
		})

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
			paymentGateway,
			subscriptionService,
			userRepo,
			remnawaveClient,
		)

		return nil

	case "btn_prepay_devices":
		userID := strconv.FormatInt(chatID, 10)
		activeAddons, err := userRepo.CountActiveDeviceAddons(userID)
		if err != nil {
			log.Printf("Ошибка подсчета доп. устройств для %s: %v", userID, err)
			if sendErr := sender.SendMessage(chatID, "Ошибка получения данных пользователя"); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение об ошибке получения данных пользователя: %w",
					sendErr,
				)
			}
			return nil
		}
		if activeAddons <= 0 {
			if sendErr := sender.SendMessage(chatID, "У вас нет активных доп. устройств для продления."); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение о отсутствии доп. устройств: %w",
					sendErr,
				)
			}
			return nil
		}

		amount := activeAddons * 50
		orderID := fmt.Sprintf("tg_prepay_devices_%d_%d", chatID, time.Now().UnixNano())
		url, id, err := paymentGateway.CreateTransaction(
			context.Background(),
			float64(amount),
			orderID,
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

		paymentPurposeByTransaction.Store(id, paymentPurposeData{
			purpose: paymentPurposePrepayDevices,
			amount:  amount,
		})

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
			paymentGateway,
			subscriptionService,
			userRepo,
			remnawaveClient,
		)

		return nil

	case "btn_reset_traffic_payment":
		userID := strconv.FormatInt(chatID, 10)
		_, err := remnawaveClient.GetUUIDByUsername(userID)
		if err != nil {
			log.Printf("Ошибка получения UUID пользователя %s: %v", userID, err)
			if sendErr := sender.SendMessage(chatID, "Ошибка получения данных пользователя"); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение об ошибке получения пользователя: %w",
					sendErr,
				)
			}
			return nil
		}

		const resetTrafficPriceRUB = 50
		orderID := fmt.Sprintf("tg_reset_traffic_%d_%d", chatID, time.Now().UnixNano())
		url, id, err := paymentGateway.CreateTransaction(
			context.Background(),
			float64(resetTrafficPriceRUB),
			orderID,
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

		paymentPurposeByTransaction.Store(id, paymentPurposeData{
			purpose: paymentPurposeResetTraffic,
			amount:  resetTrafficPriceRUB,
		})

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
			paymentGateway,
			subscriptionService,
			userRepo,
			remnawaveClient,
		)

		return nil

	case "btn_reset_devices":
		userID := strconv.FormatInt(chatID, 10)

		if err := subscriptionService.ResetPaidDevices(userID); err != nil {
			log.Printf("Ошибка сброса платных устройств для %s: %v", userID, err)
			if sendErr := sender.SendMessage(chatID, "❌ Ошибка сброса услуги."); sendErr != nil {
				return fmt.Errorf(
					"не удалось отправить сообщение об ошибке сброса устройств: %w",
					sendErr,
				)
			}
			return nil
		}

		profileData, err := buildProfileData(userID)
		if err != nil {
			log.Printf("Ошибка сборки данных профиля после сброса устройств: %v", err)
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
		return showView(
			domain.ViewTypeProfile,
			profileData,
			"ошибка отображения профиля после сброса устройств",
		)

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

type paymentPurpose string

const (
	paymentPurposeAddDevice     paymentPurpose = "add_device"
	paymentPurposePrepayDevices paymentPurpose = "prepay_devices"
	paymentPurposeResetTraffic  paymentPurpose = "reset_traffic"
)

type paymentPurposeData struct {
	purpose paymentPurpose
	amount  int
}

var paymentPurposeByTransaction sync.Map
var activePaymentStatusWatchers sync.Map

// startAutoPaymentCheck запускает автоматическую проверку платежа.
// Начинает проверку через 15 секунд после создания, затем проверяет каждые 5 секунд в течение 2 минут.
// Эта функция нужна для того, чтобы клиентам не приходилось вручную нажимать "Проверить платеж".
func startAutoPaymentCheck(
	sender MessageSender,
	chatID int64,
	messageID int,
	transactionID string,
	paymentGateway domain.PaymentGateway,
	subscriptionService domain.SubscriptionService,
	userRepo *database.UserStorage,
	remnawaveClient domain.RemnawaveClient,
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

		// Проверяем каждые 5 секунд в течение 2 минут
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
					remnawaveClient,
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

func tryStartPaymentStatusWatcher(
	sender MessageSender,
	chatID int64,
	messageID int,
	transactionID string,
	paymentGateway domain.PaymentGateway,
	subscriptionService domain.SubscriptionService,
	userRepo *database.UserStorage,
	remnawaveClient domain.RemnawaveClient,
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
					remnawaveClient,
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

func handleSuccessfulPayment(
	sender MessageSender,
	chatID int64,
	messageID int,
	transactionID string,
	paymentGateway domain.PaymentGateway,
	subscriptionService domain.SubscriptionService,
	userRepo *database.UserStorage,
	remnawaveClient domain.RemnawaveClient,
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
			case paymentPurposeResetTraffic:
				return handleSuccessfulResetTrafficPayment(
					sender,
					chatID,
					messageID,
					amount,
					remnawaveClient,
					userRepo,
				)
			}
		}
	}

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

func handleSuccessfulResetTrafficPayment(
	sender MessageSender,
	chatID int64,
	messageID int,
	amount int,
	remnawaveClient domain.RemnawaveClient,
	userRepo *database.UserStorage,
) error {
	userID := strconv.FormatInt(chatID, 10)

	if err := remnawaveClient.BetterResetTraffic(context.Background(), userID); err != nil {
		log.Printf("Ошибка сброса трафика у пользователя %s: %v", userID, err)
		if sendErr := sender.SendMessage(chatID, "Не удалось сбросить трафик"); sendErr != nil {
			return fmt.Errorf(
				"не удалось отправить сообщение об ошибке сброса трафика: %w",
				sendErr,
			)
		}
		return nil
	}

	successMsg := "✅ Трафик успешно сброшен."
	if err := sender.ShowView(
		chatID,
		messageID,
		domain.ViewTypeSubscriptionResult,
		successMsg,
	); err != nil {
		return fmt.Errorf("ошибка отображения сообщения об успешном сбросе трафика: %w", err)
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

// ProcessCommand обрабатывает текстовые команды (например, /start).
// Эта функция — "мозг" обработки текста.
func ProcessCommand(
	sender MessageSender,
	chatID int64,
	command string,
	firstName string,
	remnawaveClient domain.RemnawaveClient,
	userRepo *database.UserStorage,
	logger logger.Logger,
) error {

	//проверка на админа
	isAdmin, err := isAdmin(sender, chatID, command, firstName, remnawaveClient, userRepo, logger)

	if err != nil {
		logger.Error("Ошибка проверка на администратора")
		return nil
	}
	//если админская команда, пропускаем дальнейшую обработку
	if isAdmin {
		return nil
	}

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
	remnawaveClient domain.RemnawaveClient,
	userRepo *database.UserStorage,
) string {
	username := strconv.FormatInt(chatID, 10)

	_, err := userRepo.GetUserByID(username)
	if err != nil {
		// Если пользователя нету, создаем его в базе
		if errors.Is(err, domain.ErrUserNotFound) {
			_, createErr := userRepo.CreateUser(models.CreateUserTGDTO{
				ID:      username,
				Balance: 0,
				Trial:   false,
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
		"🌟 Добро пожаловать, %s!\n<blockquote>%s\n</blockquote>\n🚀 Если вам не понятно как подключиться, обратитесь в поддержку, мы отправим инструкцию и поможем\n\n1️⃣ Скачайте приложение по кнопке <u>Скачать приложение</u>. Выберите ваше устройство, iOS или Android и т.д.\n2️⃣ После установки нажмите <u>Подключить (Happ)</u>, он импортирует подписку в Happ",
		name,
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
