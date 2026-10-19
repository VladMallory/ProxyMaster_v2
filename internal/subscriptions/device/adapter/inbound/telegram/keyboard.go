package devicetg

import (
	"fmt"
	"strconv"

	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
	"gopkg.in/telebot.v4"
)

type Keyboard struct{}

func NewKeyboard() *Keyboard {
	return &Keyboard{}
}

// List кнопки главного экрана устройств: отключить занятое место, уйти в
// продление, увеличить лимит, вернуться в меню.
func (k *Keyboard) List(ov device.Overview) *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	rows := make([]telebot.Row, 0, len(ov.Devices)+3)
	for i, d := range ov.Devices {
		if d.HWID == "" {
			continue
		}

		btn := menu.Data(
			fmt.Sprintf("❌ Отключить %d", i+1),
			"devices_del",
			strconv.FormatInt(d.ID, 10),
		)
		rows = append(rows, menu.Row(btn))
	}

	rows = append(rows, menu.Row(menu.Data("🔄 Продлить устройство", "devices_extend_ask")))

	rows = append(rows, menu.Row(menu.Data("➕ Увеличить лимит", "devices_buy")))

	rows = append(rows, menu.Row(menu.Data("🏠 Главное меню", "users_back")))
	menu.Inline(rows...)

	return menu
}

// Extend кнопки выбора места для продления: по кнопке на устройство.
func (k *Keyboard) Extend(devices []device.Device) *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	rows := make([]telebot.Row, 0, len(devices)+1)
	for i, d := range devices {
		name := d.Label
		if d.HWID == "" {
			name = "Свободное место"
		}

		text := fmt.Sprintf("%d. %s — до %s", i+1, name, d.ExpiresAt.Format("02.01.2006"))
		rows = append(
			rows,
			menu.Row(menu.Data(text, "devices_extend", strconv.FormatInt(d.ID, 10))),
		)
	}

	rows = append(rows, menu.Row(menu.Data("🏠 Главное меню", "users_back")))
	menu.Inline(rows...)

	return menu
}

// Back один возврат, для ошибок когда список построить не из чего.
func (k *Keyboard) Back() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}
	menu.Inline(menu.Row(menu.Data("🏠 Главное меню", "users_back")))

	return menu
}
