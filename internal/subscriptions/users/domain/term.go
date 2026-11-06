package subdomain

import (
	"fmt"
	"time"
)

// Term срок подписки: либо дни, либо календарные месяцы.
type Term struct {
	Days   int
	Months int
}

// Days срок в днях.
func Days(n int) Term {
	return Term{Days: n}
}

// Months срок в месяцах.
func Months(n int) Term {
	return Term{Months: n}
}

// Valid true если ровно одно из полей задано положительным числом.
func (t Term) Valid() bool {
	return (t.Days > 0) != (t.Months > 0)
}

// AddTo прибавляет срок к дате. Месяцы идут через календарь, дни ровно
// столько суток, сколько указано.
func (t Term) AddTo(base time.Time) time.Time {
	return base.AddDate(0, t.Months, t.Days)
}

// Label подпись срока для кнопок тарифов.
func (t Term) Label() string {
	switch {
	case t.Days > 0:
		return fmt.Sprintf("%d дн.", t.Days)
	case t.Months > 0:
		return fmt.Sprintf("%d мес.", t.Months)
	default:
		return ""
	}
}
