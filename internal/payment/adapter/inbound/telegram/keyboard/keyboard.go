package keyboard

import "gopkg.in/telebot.v4"

type Keyboard struct{}

func New() *Keyboard {
	return &Keyboard{}
}

// TariffMenu показывает варианты тарифов для продления/пополнения.
// Каждый тариф — своя Unique-кнопка, как у вас сделано в users для платформ.
func (k *Keyboard) TariffMenu() *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	btn200 := menu.Data("1 месяц — 200 ₽", "payment_tariff_200")
	btn400 := menu.Data("2 месяца — 400 ₽", "payment_tariff_400")
	btn600 := menu.Data("3 месяца — 600 ₽", "payment_tariff_600")
	btn800 := menu.Data("4 месяца — 800 ₽", "payment_tariff_800")
	btn1000 := menu.Data("5 месяцев — 1000 ₽", "payment_tariff_1000")
	btnBack := menu.Data("🏠 Главное меню", "users_back")

	menu.Inline(
		menu.Row(btn200, btn400),
		menu.Row(btn600, btn800),
		menu.Row(btn1000),
		menu.Row(btnBack),
	)

	return menu
}

// PayLink показывает кнопку-ссылку на страницу оплаты.
func (k *Keyboard) PayLink(payURL string) *telebot.ReplyMarkup {
	menu := &telebot.ReplyMarkup{}

	btnPay := menu.URL("💳 Продлить подписку", payURL)

	menu.Inline(menu.Row(btnPay))

	return menu
}
