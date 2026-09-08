package userscase

import (
	"context"
	"fmt"
)

var tariffToDays = map[int]int{
	200:  30,
	400:  60,
	600:  90,
	800:  120,
	1000: 150,
}

type SubscriptionExtender struct {
	uc UserUseCase
}

func NewSubscriptionExtender(uc UserUseCase) *SubscriptionExtender {
	return &SubscriptionExtender{uc: uc}
}

// Этот метод удовлетворяет ТОМУ ЖЕ интерфейсу PaymentObserver, что и Notifier.
func (e *SubscriptionExtender) OnPaymentConfirmed(
	ctx context.Context,
	userID string,
	amount int,
) error {
	days, ok := tariffToDays[amount]
	if !ok {
		return fmt.Errorf("неизвестная сумма тарифа: %d", amount)
	}

	return e.uc.ExtendSubscription(ctx, userID, days)
}
