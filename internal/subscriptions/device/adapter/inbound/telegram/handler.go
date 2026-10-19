// Package devicetg — inbound-адаптер device к телеграму.
package devicetg

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/platform/traceid"
	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
	"go.uber.org/zap"
	"gopkg.in/telebot.v4"
)

// deviceUC бизнес-логика устройств, которая нужна этому экрану.
// ISP: узкий порт — ровно два метода, ничего лишнего.
type deviceUC interface {
	ListDevices(ctx context.Context, username string) (device.Overview, error)
	RemoveDevice(ctx context.Context, username string, deviceID int64) error
}

// SeatStarter запускает оплату места: продление существующего или покупку
// нового (+1 к лимиту).
//
// DIP: интерфейс объявлен на стороне потребителя (здесь), реализует его
// адаптер devicepay.Starter.
type SeatStarter interface {
	StartDeviceExtend(
		ctx context.Context,
		username string,
		deviceID int64,
		msg telebot.Editable,
	) (payURL string, err error)
	StartDevicePurchase(
		ctx context.Context,
		username string,
		msg telebot.Editable,
	) (payURL string, err error)
}

type Handler struct {
	devices deviceUC
	starter SeatStarter
	keys    *Keyboard
	log     *zap.Logger
}

func NewHandler(devices deviceUC, starter SeatStarter, log *zap.Logger) *Handler {
	return &Handler{devices: devices, starter: starter, keys: NewKeyboard(), log: log}
}

// handleList экран Ваши устройства.
func (h *Handler) handleList(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	username := senderUsername(c)
	ctx := traceid.WithUsername(context.Background(), username)

	ov, err := h.devices.ListDevices(ctx, username)
	if err != nil {
		h.log.Error("devices list failed",
			zap.String("username", username), zap.Error(err))

		return c.Edit("Не смог получить устройства, попробуйте позже.", h.keys.Back())
	}

	return c.Edit(renderList(ov), h.keys.List(ov), telebot.ModeHTML)
}

// handleDelete отключает устройство по id строки из callback-data, потом
// перерендер списка. Место остаётся оплаченным — просто становится свободным.
func (h *Handler) handleDelete(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	deviceID, err := parseDeviceID(c.Data())
	if err != nil {
		return c.Send("Неверное устройство, нажмите /start")
	}

	username := senderUsername(c)
	ctx := traceid.WithUsername(context.Background(), username)

	if err := h.devices.RemoveDevice(ctx, username, deviceID); err != nil {
		h.log.Error("devices remove failed",
			zap.String("username", username), zap.Int64("device_id", deviceID), zap.Error(err))

		return c.Send("Не смог отключить, попробуйте позже.")
	}

	return h.render(ctx, c, username, "Отключил, но список обновить не смог. Нажмите /start.")
}

// handleExtendAsk показывает выбор «какое устройство продлить».
// В callback-data уходят id строк, а не hwid: hwid может не влезть в 64 байта.
func (h *Handler) handleExtendAsk(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	username := senderUsername(c)
	ctx := traceid.WithUsername(context.Background(), username)

	ov, err := h.devices.ListDevices(ctx, username)
	if err != nil {
		h.log.Error("devices list failed",
			zap.String("username", username), zap.Error(err))

		return c.Send("Не смог получить устройства, попробуйте позже.")
	}

	if len(ov.Devices) == 0 {
		// Продлевать нечего: возвращаем экран списка, где есть кнопка
		// «➕ Увеличить лимит» — так юзер не попадёт в тупик.
		return c.Edit(renderList(ov), h.keys.List(ov), telebot.ModeHTML)
	}

	text := "🔄 <b>Какое устройство хотите продлить?</b>\n\n" +
		"Продление считается от текущего срока: остаток не сгорает."

	return c.Edit(text, h.keys.Extend(ov.Devices), telebot.ModeHTML)
}

// handleExtend запускает оплату продления выбранного места и показывает ссылку.
func (h *Handler) handleExtend(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	deviceID, err := parseDeviceID(c.Data())
	if err != nil {
		return c.Send("Неверное устройство, нажмите /start")
	}

	username := senderUsername(c)
	ctx := traceid.WithUsername(context.Background(), username)

	payURL, err := h.starter.StartDeviceExtend(ctx, username, deviceID, c.Message())
	if err != nil {
		h.log.Error("start device extend failed",
			zap.String("username", username),
			zap.Int64("device_id", deviceID), zap.Error(err))

		return c.Send("Не смог создать оплату, попробуйте позже.")
	}

	menu := &telebot.ReplyMarkup{}
	btnPay := menu.URL("🔗 Оплатить продление", payURL)
	btnBack := menu.Data("🏠 Главное меню", "users_back")
	menu.Inline(menu.Row(btnPay), menu.Row(btnBack))

	return c.Edit("Нажмите чтобы оплатить:", menu)
}

// handleBuy (callback data "devices_buy") запускает покупку нового места.
// В отличие от продления deviceID не нужен: место создаст Grant после оплаты.
func (h *Handler) handleBuy(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	username := senderUsername(c)
	ctx := traceid.WithUsername(context.Background(), username)

	// Защита от «покупки в пустоту»: пустой список после ленивого импорта
	// значит, что панель юзера не знает лимит записать некуда. Просим
	// сначала подключиться.
	ov, err := h.devices.ListDevices(ctx, username)
	if err != nil {
		h.log.Error("devices list before purchase failed",
			zap.String("username", username), zap.Error(err))

		return c.Send("Не смог получить устройства, попробуйте позже.")
	}

	if len(ov.Devices) == 0 {
		return c.Edit("Сначала подключите подписку, а потом увеличивайте лимит.", h.keys.Back())
	}

	payURL, err := h.starter.StartDevicePurchase(ctx, username, c.Message())
	if err != nil {
		h.log.Error("start device purchase failed",
			zap.String("username", username), zap.Error(err))

		return c.Send("Не смог создать оплату, попробуйте позже.")
	}

	menu := &telebot.ReplyMarkup{}
	btnPay := menu.URL("🔗 Оплатить устройство", payURL)
	btnBack := menu.Data("🏠 Главное меню", "users_back")
	menu.Inline(menu.Row(btnPay), menu.Row(btnBack))

	return c.Edit("Нажмите чтобы оплатить:", menu)
}

// render перечитывает Overview и правит сообщение; при ошибке шлёт fallback.
func (h *Handler) render(
	ctx context.Context,
	c telebot.Context,
	username, fallback string,
) error {
	ov, err := h.devices.ListDevices(ctx, username)
	if err != nil {
		h.log.Error("devices list after action failed",
			zap.String("username", username), zap.Error(err))

		return c.Edit(fallback, h.keys.Back())
	}

	return c.Edit(renderList(ov), h.keys.List(ov), telebot.ModeHTML)
}

// renderList текст экрана. Список строится из БД: занятые и свободные места.
func renderList(ov device.Overview) string {
	now := time.Now()

	// Считаем занятые, чтобы заголовок показывал реальную загрузку мест.
	occupied := 0
	for _, d := range ov.Devices {
		if d.HWID != "" {
			occupied++
		}
	}

	// Если устройств нет
	if len(ov.Devices) == 0 {
		return fmt.Sprintf(
			"📱 <b>Ваши устройства</b> (0/%d)\n\n"+
				"Подключите подписку по кнопке 🚀 Подключиться или увеличьте лимит кнопкой ➕ Увеличить лимит.",
			ov.Limit,
		)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📱 <b>Ваши устройства</b> (занято %d из %d)\n\n", occupied, ov.Limit)

	for i, d := range ov.Devices {
		name := d.Label
		if d.HWID == "" {
			name = "Свободное место"
		}

		fmt.Fprintf(
			&b,
			"%d. %s — до %s",
			i+1,
			html.EscapeString(name),
			d.ExpiresAt.Format("02.01.2006"),
		)

		// Просроченное (но ещё не отозванное) подсвечиваем: его нужно продлить.
		if !d.ActiveAt(now) {
			b.WriteString(" ⚠️ <i>истёк</i>")
		}

		b.WriteString("\n")

		if d.HWID != "" {
			fmt.Fprintf(&b, "<code>%s</code>\n", html.EscapeString(d.HWID))
		}

		b.WriteString("\n")
	}

	return b.String()
}

// senderUsername получаем username клиента.
func senderUsername(c telebot.Context) string {
	return strconv.FormatInt(c.Sender().ID, 10)
}

// parseDeviceID разбирает id места из callback data.
func parseDeviceID(data string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(data), 10, 64)
}
