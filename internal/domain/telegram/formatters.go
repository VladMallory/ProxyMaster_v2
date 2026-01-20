package telegram

import (
	"fmt"
	"time"
)

// formatRussianDate форматирует дату по-русски (день, месяц в родительном падеже, год)
// Например: "2026 20 января"
func formatRussianDate(t time.Time) string {
	return fmt.Sprintf("%d %d %s", t.Year(), t.Day(), russianMonthGenitive(t.Month()))
}

// formatDevicePaymentDate форматирует дату платежа для доп. устройства
// Если год совпадает с текущим, год не выводится
// Например: "20 января" или "20 января 2027"
func formatDevicePaymentDate(t time.Time, now time.Time) string {
	if t.Year() == now.Year() {
		return fmt.Sprintf("%d %s", t.Day(), russianMonthGenitive(t.Month()))
	}
	return fmt.Sprintf("%d %s %d", t.Day(), russianMonthGenitive(t.Month()), t.Year())
}

// russianMonthGenitive возвращает название месяца в родительном падеже по-русски
func russianMonthGenitive(m time.Month) string {
	switch m {
	case time.January:
		return "января"
	case time.February:
		return "февраля"
	case time.March:
		return "марта"
	case time.April:
		return "апреля"
	case time.May:
		return "мая"
	case time.June:
		return "июня"
	case time.July:
		return "июля"
	case time.August:
		return "августа"
	case time.September:
		return "сентября"
	case time.October:
		return "октября"
	case time.November:
		return "ноября"
	case time.December:
		return "декабря"
	default:
		return ""
	}
}

// buildStartText строит приветственное сообщение с информацией о подписке
func buildStartText(firstName string, subscriptionLine string) string {
	name := firstName
	if name == "" {
		name = "друг"
	}

	return fmt.Sprintf(
		"🌟 Добро пожаловать, %s!\n<blockquote>%s\n</blockquote>\n🚀 Если вам не понятно как подключиться, обратитесь в поддержку, мы отправим инструкцию и поможем\n\n1️⃣ Скачайте приложение по кнопке <u>Скачать приложение</u>. Выберите ваше устройство, iOS или Android и т.д.\n2️⃣ После установки нажмите <u>Подключить (Happ)</u>, он импортирует подписку в Happ",
		name,
		subscriptionLine,
	)
}
