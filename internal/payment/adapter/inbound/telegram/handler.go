package telegramhandler

import (
	"context"
	"fmt"
	"strconv"

	paymentsvc "github.com/VladMallory/ProxyMaster_v2/internal/payment/service"
	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
	"gopkg.in/telebot.v4"
)

type subscriptionExtender interface {
	ExtendSubscription(ctx context.Context, username string, term subdomain.Term) error
}

// AutopayButton кнопка автоподключения.
// deadcode yookassa: интерфейс существует только ради временной кнопки автоплатежа.
type AutopayButton interface {
	Button(
		menu *telebot.ReplyMarkup,
		username string,
	) (telebot.Btn, bool)
}

type Handler struct {
	svc      *paymentsvc.Service
	notifier rememberer
	subs     subscriptionExtender
	// deadcode yookassa: временная кнопка отключения автоплатежа.
	autopayButton AutopayButton
}

type rememberer interface {
	Remember(userID string, msg telebot.Editable)
}

func NewHandler(
	svc *paymentsvc.Service,
	notifier rememberer,
	subs subscriptionExtender,
	// deadcode yookassa: параметр временной кнопки автоплатежа.
	autopayButton AutopayButton,
) *Handler {
	return &Handler{
		svc:           svc,
		notifier:      notifier,
		subs:          subs,
		autopayButton: autopayButton, // deadcode yookassa
	}
}

func (h *Handler) handleCheckout(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	menu := &telebot.ReplyMarkup{}
	tariffs := h.svc.Tariffs()

	rows := make([]telebot.Row, 0, 4)
	row := make([]telebot.Btn, 0, 2)

	for i, t := range tariffs {
		text := fmt.Sprintf("💰%d₽ - %s", t.PriceRub, t.Term.Label())
		btn := menu.Data(text, "pay_tariff", strconv.Itoa(i))
		row = append(row, btn)

		if len(row) == 2 || i == len(tariffs)-1 {
			rows = append(rows, menu.Row(row...))
			row = nil
		}
	}

	// deadcode yookassa: временная кнопка отключения автоплатежа.
	if h.autopayButton != nil {
		username := strconv.FormatInt(c.Sender().ID, 10)
		if btn, ok := h.autopayButton.Button(menu, username); ok {
			rows = append(rows, menu.Row(btn))
		}
	}
	// end deadcode yookassa

	rows = append(rows, menu.Row(menu.Data("🏠 В главное меню", "users_back")))
	menu.Inline(rows...)

	return c.Edit("Выберите срок подписки:", menu)
}

func (h *Handler) handleTariff(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	idx, err := strconv.Atoi(c.Data())
	if err != nil || idx < 0 || idx >= len(h.svc.Tariffs()) {
		return c.Send("Неверный тариф, нажмите /start")
	}

	tariff := h.svc.Tariffs()[idx]
	username := strconv.FormatInt(c.Sender().ID, 10)

	h.notifier.Remember(username, c.Message())

	onPaid := func(ctx context.Context, _ string) error {
		return h.subs.ExtendSubscription(ctx, username, tariff.Term)
	}

	payURL, err := h.svc.CreatePayment(context.Background(), username, tariff.PriceRub, onPaid)
	if err != nil {
		return c.Send(err.Error())
	}

	menu := &telebot.ReplyMarkup{}
	btnPay := menu.URL("🔗 Оплатить", payURL)
	btnBack := menu.Data("🏠 В главное меню", "users_back")
	menu.Inline(menu.Row(btnPay), menu.Row(btnBack))

	return c.Edit("Нажмите чтобы оплатить:", menu)
}
