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
	"time"

	youkassa2 "github.com/VladMallory/ProxyMaster_v2/cmd/tools/rest-api-test/youkassa"
	"github.com/VladMallory/ProxyMaster_v2/internal/config"
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

// payHandler создает платеж и отправляет пользователя на страницу оплаты.
func payHandler(remnawaveClient *remnawave.RemnaClient, yooKassaClient *youkassa2.YooKassaClient) http.HandlerFunc {
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

		_, ok := subscriptionAmountToDays(amount)
		if !ok {
			fmt.Println("недопустимая сумма оплаты:", amount)
			http.Error(w, "можно оплатить только 200, 400, 600, 800 или 1000 рублей", http.StatusBadRequest)

			return
		}

		// Проверяем пользователя до создания платежа, чтобы не принимать оплату для несуществующего клиента.
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

		payment, err := yooKassaClient.CreatePayment(username, amount)
		if err != nil {
			fmt.Println("ошибка создания платежа:", err)
			http.Error(w, "ошибка в создании платежа", http.StatusInternalServerError)

			return
		}

		fmt.Println("платеж создан:", payment.ID, "username:", username, "amount:", amount)
		fmt.Println("запускаю проверку платежа:", payment.ID)

		go func() {
			watchPayment(context.WithoutCancel(r.Context()), remnawaveClient, yooKassaClient, payment.ID)
		}()

		http.Redirect(w, r, payment.Confirmation.ConfirmationURL, http.StatusSeeOther)
	}
}

// isPaid проверяет, что ЮKassa подтвердила успешную оплату.
func isPaid(payment *youkassa2.YooKassaPaymentStatusResponse) bool {
	return payment.Status == "succeeded" && payment.Paid
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

// handleSuccessfulPayment выполняет действия после подтвержденной оплаты.
func handleSuccessfulPayment(ctx context.Context, remnawaveClient *remnawave.RemnaClient, payment *youkassa2.YooKassaPaymentStatusResponse) error {
	username := payment.Metadata.Username
	if username == "" {
		return fmt.Errorf("пустой username в metadata платежа")
	}

	uuidUser, err := remnawaveClient.GetUUIDByUsername(ctx, username)
	if err != nil {
		return err
	}

	days, err := strconv.Atoi(payment.Metadata.Days)
	if err != nil || days == 0 {
		return fmt.Errorf("пустое количество дней в metadata платежа")
	}

	err = remnawaveClient.ExtendClientSubscription(uuidUser, username, days)
	if err != nil {
		return err
	}

	fmt.Println("подписка продлена для пользователя:", username)

	return nil
}

func payPageHandler(w http.ResponseWriter, r *http.Request) {
	err := sitev01.PayPage().Render(r.Context(), w)
	if err != nil {
		http.Error(w, "ошибка отображения страницы", http.StatusInternalServerError)
	}
}

// yooKassaWebhookHandler принимает webhook от ЮKassa и фиксирует успешную оплату.
func yooKassaWebhookHandler(remnawaveClient *remnawave.RemnaClient, yooKassaClient *youkassa2.YooKassaClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var webhook YooKassaWebhook

		// ЮKassa отправляет JSON; если не смогли прочитать, отвечаем OK, чтобы она не дергала нас бесконечно.
		err := json.NewDecoder(r.Body).Decode(&webhook)
		if err != nil {
			w.WriteHeader(http.StatusOK)

			return
		}

		if webhook.Event != "payment.succeeded" || webhook.Object.ID == "" {
			w.WriteHeader(http.StatusOK)

			return
		}

		// Даже webhook перепроверяем через API ЮKassa, чтобы не доверять входящему запросу вслепую.
		payment, err := yooKassaClient.GetPayment(r.Context(), webhook.Object.ID)
		if err == nil && isPaid(payment) {
			err = handleSuccessfulPayment(r.Context(), remnawaveClient, payment)
			if err != nil {
				fmt.Println("ошибка обработки успешной оплаты:", err)
			}
		}

		w.WriteHeader(http.StatusOK)
	}
}

// watchPayment проверяет платеж каждые 10 секунд в течение 1 часа.
func watchPayment(ctx context.Context, remnawaveClient *remnawave.RemnaClient, yooKassaClient *youkassa2.YooKassaClient, paymentID string) {
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()

	fmt.Println("проверка платежа запущена:", paymentID)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		payment, err := yooKassaClient.GetPayment(ctx, paymentID)
		if err != nil {
			fmt.Println("ошибка проверки платежа:", paymentID, err)
		}

		if err == nil {
			fmt.Println("статус платежа:", payment.ID, payment.Status, "paid:", payment.Paid)
		}

		if err == nil && isPaid(payment) {
			err = handleSuccessfulPayment(ctx, remnawaveClient, payment)
			if err != nil {
				fmt.Println("ошибка обработки успешной оплаты:", err)
			}

			return
		}

		select {
		case <-ctx.Done():
			fmt.Println("проверка платежа остановлена:", paymentID)

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

	yooKassaClient := &youkassa2.YooKassaClient{
		ShopID:     cfg.YouKassaShopID,
		SecretKey:  cfg.YouKassaSecretKey,
		HTTPClient: http.DefaultClient,
	}

	loggerClient, err := logger.New("info")
	if err != nil {
		return
	}

	remnawaveLogger := loggerClient.Named("remnawave")
	remnaCfg := remnawave.RemnaConfig{
		PanelURL:       cfg.RemnaPanelURL,
		SecretURLToken: cfg.RemnaSecretURLToken,
		APIKey:         cfg.RemnaKey,
		SquadUUID:      cfg.RemnaSquadUUID,
		TrafficLimitGB: cfg.TrafficLimit,
		DeviceLimit:    cfg.DeviceLimit,
	}

	remnawaveClient := remnawave.NewRemnaClient(remnaCfg, remnawaveLogger)

	http.HandleFunc("GET /", payPageHandler)
	http.HandleFunc("GET /pay", payPageHandler)
	http.HandleFunc("POST /", payHandler(remnawaveClient, yooKassaClient))
	http.HandleFunc("POST /pay", payHandler(remnawaveClient, yooKassaClient))
	http.HandleFunc("POST /pay/webhook", yooKassaWebhookHandler(remnawaveClient, yooKassaClient))

	http.HandleFunc("GET /styles.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, "./frontend/site_v01/styles.css")
	})

	http.Handle("GET /assets/", http.StripPrefix(
		"/assets/", http.FileServer(http.Dir("./frontend/site_v01/assets")),
	))

	http.ListenAndServe("127.0.0.1:8080", nil)
}
