package telegramdevice

import (
	"strconv"
	"time"

	"github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/device"
	"gopkg.in/telebot.v4"
)

func devicesMenu(devices []device.Device) *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	rows := make([]telebot.Row, 0, len(devices)+2)

	for _, d := range devices {
		if !d.Bound() {
			continue
		}

		btnOpen := menu.Data(
			devicesTitle(d),
			"device_open",
			strconv.FormatInt(d.ID, 10),
		)
		rows = append(rows, menu.Row(btnOpen))
	}

	rows = append(rows, menu.Row(menu.Data("➕ Добавить устройство", "device_add")))
	rows = append(rows, menu.Row(menu.Data("🏠 В главное меню", "users_back")))

	menu.Inline(rows...)

	return menu
}

func deviceCardMenu(d device.Device) *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	rows := make([]telebot.Row, 0, 3)

	if d.Extra() && d.ActiveAt(time.Now()) {
		btnExtend := menu.Data(
			"🔄 Продлить",
			"device_extend",
			strconv.FormatInt(d.ID, 10),
		)
		rows = append(rows, menu.Row(btnExtend))
	}

	if d.Bound() {
		btnDelete := menu.Data(
			"🗑 Удалить",
			"devices_del",
			strconv.FormatInt(d.ID, 10),
		)
		rows = append(rows, menu.Row(btnDelete))
	}

	rows = append(rows, menu.Row(menu.Data("⬅️ Назад", "device_list")))

	menu.Inline(rows...)

	return menu
}

func backMenu() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}
	menu.Inline(
		menu.Row(menu.Data("⬅️ К устройствам", "device_list")),
		menu.Row(menu.Data("🏠 В главное меню", "users_back")),
	)

	return menu
}
