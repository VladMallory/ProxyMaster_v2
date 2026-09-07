package telegram

import (
	"context"
	"strconv"

	"github.com/VladMallory/ProxyMaster_v2/internal/payment/adapter/inbound/telegram/keyboard"
	paymentservice "github.com/VladMallory/ProxyMaster_v2/internal/payment/service"

	"gopkg.in/telebot.v4"
)

type Handler struct {
	bot  *telebot.Bot
	svc  *paymentservice.PaymentService
	keys *keyboard.Keyboard
}

func NewHandler(bot *telebot.Bot, svc *paymentservice.PaymentService) *Handler {
	return &Handler{
		bot:  bot,
		svc:  svc,
		keys: keyboard.New(),
	}
}

// RegisterRoutes регистрирует все обработчики этого контекста.
// Unique-имена кнопок с префиксом payment_, чтобы не столкнуться с users_*.
func (h *Handler) RegisterRoutes() {
	h.bot.Handle(&telebot.Btn{Unique: "payment_menu"}, h.handleTariffMenu)

	h.bot.Handle(&telebot.Btn{Unique: "payment_tariff_200"}, h.makeTariffHandler(200))
	h.bot.Handle(&telebot.Btn{Unique: "payment_tariff_400"}, h.makeTariffHandler(400))
	h.bot.Handle(&telebot.Btn{Unique: "payment_tariff_600"}, h.makeTariffHandler(600))
	h.bot.Handle(&telebot.Btn{Unique: "payment_tariff_800"}, h.makeTariffHandler(800))
	h.bot.Handle(&telebot.Btn{Unique: "payment_tariff_1000"}, h.makeTariffHandler(1000))
}

// handleTariffMenu показывает меню выбора тарифа.
func (h *Handler) handleTariffMenu(c telebot.Context) error {
	return c.EditOrSend("Выберите тариф:", h.keys.TariffMenu())
}

func (h *Handler) makeTariffHandler(amount int) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		userID := strconv.FormatInt(c.Sender().ID, 10)

		payURL, err := h.svc.StartTopUp(context.Background(), userID, amount)
		if err != nil {
			return c.Respond()
		}

		return c.EditOrSend("Оплатите подписку:", h.keys.PayLink(payURL))
	}
}
