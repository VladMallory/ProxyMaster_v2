// payment/domain/payment.go
package paymentdomain

import "time"

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

type Out struct {
	Status string `json:"status"`
}
