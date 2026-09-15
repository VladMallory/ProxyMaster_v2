// payment/domain/payment.go
package paymentdomain

import (
	"time"

	"gopkg.in/telebot.v4"
)

type Tariff struct {
	Months   int
	PriceRub int
}

type Invoice struct {
	ID        string
	UserID    string
	Amount    int
	Months    int
	Status    string
	CreatedAt time.Time
}

type stored struct {
	msg  telebot.Editable
	name string
}

type Out struct {
	Status string `json:"status"`
}
