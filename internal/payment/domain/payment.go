// payment/domain/payment.go
package paymentdomain

import (
	"time"

	subdomain "github.com/VladMallory/ProxyMaster_v2/internal/subscriptions/users/domain"
)

type Tariff struct {
	Term     subdomain.Term
	PriceRub int
}

type Invoice struct {
	ID        string
	UserID    string
	Amount    int
	Days      int
	Status    string
	CreatedAt time.Time
}

type Out struct {
	Status string `json:"status"`
}
