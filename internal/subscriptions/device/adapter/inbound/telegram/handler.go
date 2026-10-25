package telegramdevice

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
	"gopkg.in/telebot.v4"
)

// deviceTerms: сроки продления и покупки дополнительного места в месяцах.
var deviceTerms = []int{1, 2, 3}

type deviceUseCase interface {
	ListDevices(ctx context.Context, username string) ([]device.Device, error)
	GetDevice(ctx context.Context, username string, id int64) (device.Device, error)
	RemoveDevice(ctx context.Context, username string, deviceID int64) error
	CanBuyExtra(ctx context.Context, username string) error
	BuyExtraSeat(ctx context.Context, username string, paymentID string, months int) error
	ExtendSeat(
		ctx context.Context,
		username string,
		seatID int64,
		paymentID string,
		months int,
	) error
}

type checkout interface {
	CreatePayment(
		ctx context.Context,
		userID string,
		amountRub int,
		onPaid func(ctx context.Context, paymentID string) error,
	) (payURL string, err error)
}

type rememberer interface {
	Remember(userID string, msg telebot.Editable)
}

type HandlerTelegram struct {
	useCase    deviceUseCase
	checkout   checkout
	notifier   rememberer
	extraPrice int
}

func NewHandlerTelegram(
	useCase deviceUseCase,
	checkout checkout,
	notifier rememberer,
	extraPrice int,
) *HandlerTelegram {
	return &HandlerTelegram{
		useCase:    useCase,
		checkout:   checkout,
		notifier:   notifier,
		extraPrice: extraPrice,
	}
}

func (h *HandlerTelegram) handleList(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	username := strconv.FormatInt(c.Sender().ID, 10)

	devices, err := h.useCase.ListDevices(context.Background(), username)
	if err != nil {
		return err
	}

	return c.Edit(devicesText(devices), devicesMenu(devices), telebot.ModeHTML)
}

func (h *HandlerTelegram) handleOpen(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	username := strconv.FormatInt(c.Sender().ID, 10)

	deviceID, err := strconv.ParseInt(c.Data(), 10, 64)
	if err != nil {
		return h.editList(c, username)
	}

	d, err := h.useCase.GetDevice(context.Background(), username, deviceID)
	if err != nil {
		return h.editList(c, username)
	}

	return c.Edit(deviceCardText(d), deviceCardMenu(d), telebot.ModeHTML)
}

func (h *HandlerTelegram) handleDelete(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	username := strconv.FormatInt(c.Sender().ID, 10)

	deviceID, err := strconv.ParseInt(c.Data(), 10, 64)
	if err != nil {
		return c.Edit(
			"⚠️ <b>Не удалось отвязать устройство</b>\n\nОткройте список устройств заново.",
			backMenu(),
			telebot.ModeHTML,
		)
	}

	if err = h.useCase.RemoveDevice(context.Background(), username, deviceID); err != nil {
		return c.Edit(
			"⚠️ <b>Не удалось отвязать устройство</b>\n\nПопробуйте ещё раз чуть позже.",
			backMenu(),
			telebot.ModeHTML,
		)
	}

	return h.editList(c, username)
}

// handleExtend открывает выбор срока продления конкретного доп. места.
func (h *HandlerTelegram) handleExtend(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	username := strconv.FormatInt(c.Sender().ID, 10)

	deviceID, err := strconv.ParseInt(c.Data(), 10, 64)
	if err != nil {
		return h.editList(c, username)
	}

	d, err := h.useCase.GetDevice(context.Background(), username, deviceID)
	if err != nil || !d.Extra() || !d.ActiveAt(time.Now()) {
		return c.Edit(
			"⚠️ <b>Это место нельзя продлить</b>\n\nБазовые места продлеваются вместе с подпиской.",
			deviceCardMenu(d),
			telebot.ModeHTML,
		)
	}

	menu := h.termMenu("device_term", strconv.FormatInt(deviceID, 10), "device_open", c.Data())
	text := fmt.Sprintf(
		"💳 <b>Продление места</b>\n\nУстройство: <b>%s</b>\nВыберите срок:",
		html.EscapeString(devicesTitle(d)),
	)

	return c.Edit(text, menu, telebot.ModeHTML)
}

// handleAdd открывает выбор срока покупки нового доп. места.
func (h *HandlerTelegram) handleAdd(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	username := strconv.FormatInt(c.Sender().ID, 10)

	if err := h.useCase.CanBuyExtra(context.Background(), username); err != nil {
		return c.Edit(
			"⚠️ <b>Сейчас добавить устройство нельзя</b>\n\nПродлите подписку и проверьте лимит.",
			backMenu(),
			telebot.ModeHTML,
		)
	}

	menu := h.termMenu("device_add_term", "", "device_list", "")

	return c.Edit("➕ <b>Дополнительное устройство</b>\n\nВыберите срок:", menu, telebot.ModeHTML)
}

// handleTerm создаёт оплату продления конкретного места.
func (h *HandlerTelegram) handleTerm(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	parts := strings.Split(c.Data(), "|")
	if len(parts) != 2 {
		return h.editList(c, strconv.FormatInt(c.Sender().ID, 10))
	}

	seatID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return h.editList(c, strconv.FormatInt(c.Sender().ID, 10))
	}

	months, err := strconv.Atoi(parts[1])
	if err != nil || months <= 0 {
		return h.editList(c, strconv.FormatInt(c.Sender().ID, 10))
	}

	username := strconv.FormatInt(c.Sender().ID, 10)

	h.notifier.Remember(username, c.Message())

	onPaid := func(ctx context.Context, paymentID string) error {
		return h.useCase.ExtendSeat(ctx, username, seatID, paymentID, months)
	}

	return h.startPayment(c, username, months, onPaid)
}

// handleAddTerm создаёт оплату нового доп. места.
func (h *HandlerTelegram) handleAddTerm(c telebot.Context) error {
	if err := c.Respond(); err != nil {
		return err
	}

	months, err := strconv.Atoi(c.Data())
	if err != nil || months <= 0 {
		return h.editList(c, strconv.FormatInt(c.Sender().ID, 10))
	}

	username := strconv.FormatInt(c.Sender().ID, 10)

	h.notifier.Remember(username, c.Message())

	onPaid := func(ctx context.Context, paymentID string) error {
		return h.useCase.BuyExtraSeat(ctx, username, paymentID, months)
	}

	return h.startPayment(c, username, months, onPaid)
}

// startPayment создаёт платёж и отдаёт ссылку на оплату.
func (h *HandlerTelegram) startPayment(
	c telebot.Context,
	username string,
	months int,
	onPaid func(ctx context.Context, paymentID string) error,
) error {
	payURL, err := h.checkout.CreatePayment(
		context.Background(),
		username,
		h.extraPrice*months,
		onPaid,
	)
	if err != nil {
		return c.Send(err.Error())
	}

	menu := &telebot.ReplyMarkup{}
	btnPay := menu.URL("🔗 Оплатить", payURL)
	btnBack := menu.Data("🏠 В главное меню", "users_back")
	menu.Inline(menu.Row(btnPay), menu.Row(btnBack))

	return c.Edit("Нажмите чтобы оплатить:", menu)
}

// termMenu собирает меню выбора срока.
func (h *HandlerTelegram) termMenu(
	unique string,
	seatID string,
	backUnique string,
	backData string,
) *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	rows := make([]telebot.Row, 0, len(deviceTerms)+1)
	for _, months := range deviceTerms {
		text := fmt.Sprintf("%d мес. — %d₽", months, h.extraPrice*months)

		data := strconv.Itoa(months)
		if seatID != "" {
			data = seatID + "|" + strconv.Itoa(months)
		}

		rows = append(rows, menu.Row(menu.Data(text, unique, data)))
	}

	rows = append(rows, menu.Row(menu.Data("⬅️ Назад", backUnique, backData)))

	menu.Inline(rows...)

	return menu
}

// editList перерисовывает список устройств.
func (h *HandlerTelegram) editList(c telebot.Context, username string) error {
	devices, err := h.useCase.ListDevices(context.Background(), username)
	if err != nil {
		return err
	}

	return c.Edit(devicesText(devices), devicesMenu(devices), telebot.ModeHTML)
}

func devicesTitle(d device.Device) string {
	if d.Label != "" {
		return d.Label
	}

	return "Устройство"
}

func devicesText(devices []device.Device) string {
	var b strings.Builder

	now := time.Now()

	b.WriteString("📱 <b>Ваши устройства</b>\n\n")

	if len(devices) == 0 {
		b.WriteString("Пока нет ни одного места.\n\n")
		b.WriteString("<i>Подключите устройство, и оно займёт свободное место.</i>")

		return b.String()
	}

	var bound int

	for i, d := range devices {
		switch {
		case d.ExpiredAt(now):
			fmt.Fprintf(
				&b,
				"<blockquote>🔴 <b>%d. %s</b>\nСрок истёк</blockquote>\n\n",
				i+1,
				html.EscapeString(devicesTitle(d)),
			)
		case d.Bound():
			bound++

			fmt.Fprintf(
				&b,
				"<blockquote>🟢 <b>%d. %s</b>\nСрок до %s</blockquote>\n\n",
				i+1,
				html.EscapeString(devicesTitle(d)),
				d.ExpiresAt.Format("02.01.2006"),
			)
		default:
			fmt.Fprintf(
				&b,
				"<blockquote>⚪ <b>%d. Свободное место</b>\nСрок до %s</blockquote>\n\n",
				i+1,
				d.ExpiresAt.Format("02.01.2006"),
			)
		}
	}

	fmt.Fprintf(
		&b,
		"Занято: <b>%d</b> из <b>%d</b>",
		bound,
		device.CountActive(devices, now),
	)

	return b.String()
}

func deviceCardText(d device.Device) string {
	var b strings.Builder

	now := time.Now()

	b.WriteString("📱 <b>")
	b.WriteString(html.EscapeString(devicesTitle(d)))
	b.WriteString("</b>\n\n")

	switch {
	case d.ExpiredAt(now):
		b.WriteString("🔴 <b>Срок истёк</b>\n\n")
		if d.Extra() {
			b.WriteString(
				"<i>Место будет удалено. Добавьте новое, чтобы пользоваться устройством.</i>",
			)
		} else {
			b.WriteString("<i>Продлите подписку, чтобы вернуть доступ.</i>")
		}
	case d.Base():
		b.WriteString("✅ <b>Входит в подписку</b>\n")
		fmt.Fprintf(&b, "Срок до %s", d.ExpiresAt.Format("02.01.2006"))
	case d.Extra():
		b.WriteString("💳 <b>Дополнительное место</b>\n")
		fmt.Fprintf(&b, "Срок до %s", d.ExpiresAt.Format("02.01.2006"))
	default:
		fmt.Fprintf(&b, "Срок до %s", d.ExpiresAt.Format("02.01.2006"))
	}

	if d.Bound() {
		b.WriteString(
			"\n\n<i>«Удалить» отвяжет устройство, место останется оплаченным до срока.</i>",
		)
	}

	return b.String()
}
