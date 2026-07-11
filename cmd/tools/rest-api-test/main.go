// nolint
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	"github.com/VladMallory/ProxyMaster_v2/internal/features/billing/domain"
	billingSvc "github.com/VladMallory/ProxyMaster_v2/internal/features/billing/service"
	"github.com/VladMallory/ProxyMaster_v2/internal/integrations/payments/platega"
	"github.com/VladMallory/ProxyMaster_v2/internal/integrations/payments/youkassa"
	"github.com/VladMallory/ProxyMaster_v2/internal/integrations/remnawave"
	"github.com/VladMallory/ProxyMaster_v2/internal/platform/logger"
	sitev01 "github.com/VladMallory/ProxyMaster_v2/web/site"
)

type YooKassaWebhook struct {
	Event  string `json:"event"`
	Object struct {
		ID string `json:"id"`
	} `json:"object"`
}

type paymentMetadata struct {
	username string
	days     int
}

var pendingPayments sync.Map

// payHandler создает платеж через выбранный gateway и отправляет пользователя на страницу оплаты.
func payHandler(gateway billingSvc.PaymentGateway, remnawaveClient *remnawave.RemnaClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := strings.TrimSpace(r.FormValue("username"))
		amountRaw := r.FormValue("amount")

		if username == "" {
			fmt.Println("пустой username при создании платежа")
			http.Error(w, "укажите username", http.StatusBadRequest)

			return
		}

		amount, err := strconv.ParseInt(amountRaw, 10, 64)
		if err != nil {
			fmt.Println("ошибка суммы оплаты:", err)
			http.Error(w, "сумма должна быть числом", http.StatusBadRequest)

			return
		}

		days, ok := subscriptionAmountToDays(amount)
		if !ok {
			fmt.Println("недопустимая сумма оплаты:", amount)
			http.Error(w, "можно оплатить только 200, 400, 600, 800 или 1000 рублей", http.StatusBadRequest)

			return
		}

		// Проверяем пользователя до создания платежа.
		_, err = remnawaveClient.GetUUIDByUsername(r.Context(), username)
		if err != nil {
			fmt.Println("ошибка проверки пользователя перед оплатой:", username, err)

			if errors.Is(err, remnawave.ErrNotFound) {
				http.Error(w, "такой пользователь не найден в панели", http.StatusBadRequest)

				return
			}

			http.Error(w, "ошибка проверки пользователя в панели", http.StatusInternalServerError)

			return
		}

		// Уникальный ID заказа внутри нашего сайта.
		orderID := fmt.Sprintf("site_%s_%d_%d", username, amount, time.Now().UnixNano())

		paymentURL, transactionID, err := gateway.CreateTransaction(r.Context(), float64(amount), orderID)
		if err != nil {
			fmt.Println("ошибка создания платежа:", err)
			http.Error(w, "ошибка в создании платежа", http.StatusInternalServerError)

			return
		}

		fmt.Println("платеж создан:", transactionID, "username:", username, "amount:", amount)

		// Сохраняем метаданные локально, чтобы при успешной оплате знать, кому продлевать.
		pendingPayments.Store(transactionID, paymentMetadata{username: username, days: days})

		fmt.Println("запускаю проверку платежа:", transactionID)

		go watchPayment(context.WithoutCancel(r.Context()), remnawaveClient, gateway, transactionID)

		http.Redirect(w, r, paymentURL, http.StatusSeeOther)
	}
}

// isPaid проверяет, что платёжная система подтвердила успешную оплату.
func isPaid(status domain.PaymentStatus) bool {
	return status == domain.PaymentStatusSuccess
}

func subscriptionAmountToDays(amount int64) (int, bool) {
	switch amount {
	case 200:
		return 30, true
	case 400:
		return 60, true
	case 600:
		return 90, true
	case 800:
		return 120, true
	case 1000:
		return 150, true
	default:
		return 0, false
	}
}

// handleSuccessfulPayment читает сохранённые метаданные платежа и продлевает подписку.
func handleSuccessfulPayment(ctx context.Context, remnawaveClient *remnawave.RemnaClient, transactionID string) error {
	v, ok := pendingPayments.Load(transactionID)
	if !ok {
		return fmt.Errorf("платёж %s не найден в ожидающих", transactionID)
	}

	pendingPayments.Delete(transactionID)
	meta := v.(paymentMetadata)

	uuidUser, err := remnawaveClient.GetUUIDByUsername(ctx, meta.username)
	if err != nil {
		return err
	}

	err = remnawaveClient.ExtendClientSubscription(uuidUser, meta.username, meta.days)
	if err != nil {
		return err
	}

	fmt.Println("подписка продлена для пользователя:", meta.username)

	return nil
}

func payPageHandler(w http.ResponseWriter, r *http.Request) {
	err := sitev01.PayPage().Render(r.Context(), w)
	if err != nil {
		http.Error(w, "ошибка отображения страницы", http.StatusInternalServerError)
	}
}

// yooKassaWebhookHandler принимает webhook от ЮKassa и фиксирует успешную оплату.
func yooKassaWebhookHandler(gateway billingSvc.PaymentGateway, remnawaveClient *remnawave.RemnaClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var webhook YooKassaWebhook

		err := json.NewDecoder(r.Body).Decode(&webhook)
		if err != nil {
			w.WriteHeader(http.StatusOK)

			return
		}

		if webhook.Event != "payment.succeeded" || webhook.Object.ID == "" {
			w.WriteHeader(http.StatusOK)

			return
		}

		// Перепроверяем статус через API, а не доверяем вебхуку вслепую.
		status, err := gateway.CheckStatus(r.Context(), webhook.Object.ID)
		if err == nil && isPaid(status) {
			err = handleSuccessfulPayment(r.Context(), remnawaveClient, webhook.Object.ID)
			if err != nil {
				fmt.Println("ошибка обработки успешной оплаты:", err)
			}
		}

		w.WriteHeader(http.StatusOK)
	}
}

// watchPayment проверяет платеж каждые 10 секунд в течение 1 часа.
func watchPayment(ctx context.Context, remnawaveClient *remnawave.RemnaClient, gateway billingSvc.PaymentGateway, transactionID string) {
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()

	fmt.Println("проверка платежа запущена:", transactionID)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		status, err := gateway.CheckStatus(ctx, transactionID)
		if err != nil {
			fmt.Println("ошибка проверки платежа:", transactionID, err)
		} else {
			fmt.Println("статус платежа:", transactionID, status)
		}

		if err == nil && isPaid(status) {
			err = handleSuccessfulPayment(ctx, remnawaveClient, transactionID)
			if err != nil {
				fmt.Println("ошибка обработки успешной оплаты:", err)
			}

			return
		}

		if err == nil && status == domain.PaymentStatusFailed {
			fmt.Println("платёж отклонён:", transactionID)
			pendingPayments.Delete(transactionID)

			return
		}

		select {
		case <-ctx.Done():
			fmt.Println("проверка платежа остановлена:", transactionID)
			pendingPayments.Delete(transactionID)

			return
		case <-ticker.C:
		}
	}
}

func main() {
	cfg, err := config.New()
	if err != nil {
		return
	}

	loggerClient, err := logger.New("info")
	if err != nil {
		return
	}

	remnawaveLogger := loggerClient.Named("remnawave")
	remnaCfg := remnawave.RemnaConfig{
		PanelURL:           cfg.RemnaPanelURL,
		SecretURLToken:     cfg.RemnaSecretURLToken,
		APIKey:             cfg.RemnaKey,
		SquadUUID:          cfg.RemnaSquadUUID,
		TrafficLimitGB:     cfg.TrafficLimit,
		DefaultDeviceLimit: cfg.DeviceLimit,
	}

	remnawaveClient := remnawave.NewRemnaClient(remnaCfg, remnawaveLogger)

	// Выбираем платёжный gateway в зависимости от PAYMENT_PROVIDER.
	var gateway billingSvc.PaymentGateway

	switch cfg.PaymentProvider {
	case "yookassa":
		gateway = youkassa.NewClient(
			cfg.YouKassaShopID,
			cfg.YouKassaSecretKey,
			cfg.YouKassaReturnURL,
			loggerClient.Named("youkassa"),
		)
	case "platega":
		gateway = platega.NewClient(
			cfg.PlategaMerchantID,
			cfg.PlategaAPIKey,
			cfg.PlategaReturnURL,
			loggerClient.Named("platega"),
		)
	default:
		fmt.Println("неизвестный PAYMENT_PROVIDER:", cfg.PaymentProvider)

		return
	}

	http.HandleFunc("GET /", payPageHandler)
	http.HandleFunc("GET /pay", payPageHandler)
	http.HandleFunc("POST /", payHandler(gateway, remnawaveClient))
	http.HandleFunc("POST /pay", payHandler(gateway, remnawaveClient))

	// Вебхук есть только у ЮKassa.
	if cfg.PaymentProvider == "yookassa" {
		http.HandleFunc("POST /pay/webhook", yooKassaWebhookHandler(gateway, remnawaveClient))
	}

	http.HandleFunc("GET /styles.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, "./web/site/styles.css")
	})

	http.Handle("GET /assets/", http.StripPrefix(
		"/assets/", http.FileServer(http.Dir("./web/site/assets")),
	))

	http.ListenAndServe(":8080", nil)
}
