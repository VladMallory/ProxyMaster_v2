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
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

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
	if amountStr, ok := strings.CutPrefix(data, "btn_topUp_"); ok {
		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			log.Printf("Ошибка конвертации суммы платежа: %v", err)

			if sendErr := sender.SendMessage(chatID, "Ошибка обработки суммы"); sendErr != nil {
				return fmt.Errorf("ошибка обработки суммы: %w", sendErr)
			}

			return nil
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

		startAutoPaymentCheck(
			sender,
			chatID,
			messageID,
			id,
			paymentGateway,
			subscriptionService,
			userRepo,
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
			return handleSuccessfulPayment(
				sender,
				chatID,
				messageID,
				transactionID,
				paymentGateway,
				subscriptionService,
				userRepo,
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
		return sender.ShowView(chatID, messageID, domain.ViewTypeTrafficLimits, "")
	case "btn_add_50gb":
		userID := strconv.Itoa(int(chatID))
		if err := remnawaveClient.AddTraffic(userID, 50); err != nil {
			log.Printf("не удалось добавить трафик у пользователя %s, %v", userID, err)
			return sender.SendMessage(chatID, "Не удалось добавить трафик")
		}

		profileData, err := buildProfileData(userID, userRepo)
		if err != nil {
			return sender.SendMessage(chatID, "Ошибка получения данных профиля")
		}

		return sender.ShowView(chatID, messageID, domain.ViewTypeProfile, profileData)

	case "btn_add_100gb":
		userID := strconv.Itoa(int(chatID))
		if err := remnawaveClient.AddTraffic(userID, 100); err != nil {
			log.Printf("не удалось добавить трафик у пользователя %s, %v", userID, err)
			return sender.SendMessage(chatID, "Не удалось добавить трафик")
		}

		profileData, err := buildProfileData(userID, userRepo)
		if err != nil {
			return sender.SendMessage(chatID, "Ошибка получения данных профиля")
		}

		return sender.ShowView(chatID, messageID, domain.ViewTypeProfile, profileData)

	case "btn_reset_traffic":
		userID := strconv.Itoa(int(chatID))
		if err := remnawaveClient.SetTraffic(userID, 200); err != nil {
			log.Printf("не удалось сбросить трафик у пользователя %s, %v", userID, err)
			return sender.SendMessage(chatID, "Не удалось сбросить трафик")
		}

		profileData, err := buildProfileData(userID, userRepo)
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

		profileData, err := buildProfileData(userID, userRepo)
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

		profileData, err := buildProfileData(userID, userRepo)
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

// ProcessCommand обрабатывает текстовые команды (например, /start).
func ProcessCommand(
	sender MessageSender,
	chatID int64,
	command string,
	firstName string,
	remnawaveClient domain.RemnawaveClient,
	userRepo *database.UserStorage,
) error {
	const adminID = 873925520

	if strings.HasPrefix(command, "/distribution") {
		if chatID != adminID {
			if err := sender.SendMessage(chatID, "У вас нет прав для этой команды."); err != nil {
				return fmt.Errorf("ошибка отправки сообщения о нехватке прав: %w", err)
			}
			return nil
		}

		message := strings.TrimSpace(strings.TrimPrefix(command, "/distribution"))
		if message == "" {
			if err := sender.SendMessage(
				chatID,
				"Пожалуйста, введите сообщение для рассылки.",
			); err != nil {
				return fmt.Errorf("ошибка отправки сообщения с просьбой ввести текст: %w", err)
			}
			return nil
		}

		userIDs, err := userRepo.GetActiveUserIDs()
		if err != nil {
			log.Printf("Ошибка получения ID пользователей для рассылки: %v", err)
			if sendErr := sender.SendMessage(
				chatID,
				"Не удалось получить список пользователей для рассылки.",
			); sendErr != nil {
				return fmt.Errorf(
					"ошибка отправки сообщения о неудаче получения списка пользователей: %w",
					sendErr,
				)
			}
			return nil
		}

		go func() {
			for _, userIDStr := range userIDs {
				userID, err := strconv.ParseInt(userIDStr, 10, 64)
				if err != nil {
					log.Printf("Ошибка конвертации ID пользователя %s: %v", userIDStr, err)
					continue
				}
				if err := sender.SendMessage(userID, message); err != nil {
					log.Printf("Ошибка отправки сообщения пользователю %d: %v", userID, err)
				}
				time.Sleep(100 * time.Millisecond)
			}
		}()

		if err := sender.SendMessage(
			chatID,
			fmt.Sprintf("✅ Рассылка запущена для %d пользователей.", len(userIDs)),
		); err != nil {
			return fmt.Errorf("ошибка отправки подтверждения о запуске рассылки: %w", err)
		}

		return nil
	}

	switch command {
	case "/start":
		userID := strconv.FormatInt(chatID, 10)

		_, err := userRepo.GetUserByID(userID)
		if err != nil {
			if errors.Is(err, domain.ErrUserNotFound) {
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
