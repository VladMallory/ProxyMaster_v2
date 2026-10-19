package devicepay

import (
	"context"
	"errors"

	"gopkg.in/telebot.v4"
)

// extendMonths на сколько месяцев продлеваем место одним платежом.
// Пока фиксировано; когда появится выбор срока, вынесем в тариф.
const (
	extendMonths  = 1
	purchaseSeats = 1
)

type Payer interface {
	CreatePayment(
		ctx context.Context,
		userID string,
		amountRub int,
		onPaid func(ctx context.Context, paymentID string) error,
	) (string, error)
}

type Seats interface {
	Extend(ctx context.Context, paymentID, username string, deviceID int64, months int) error
	Grant(ctx context.Context, paymentID, username string, count, months int) error
}

type Rememberer interface {
	Remember(userID string, msg telebot.Editable)
}

type Starter struct {
	payer      Payer
	devices    Seats
	rememberer Rememberer
	price      int
}

func New(payer Payer, devices Seats, rememberer Rememberer, price int) *Starter {
	return &Starter{
		payer:      payer,
		devices:    devices,
		rememberer: rememberer,
		price:      price,
	}
}

// StartDeviceExtend создаёт счёт на продление места deviceID.
func (s *Starter) StartDeviceExtend(
	ctx context.Context,
	username string,
	deviceID int64,
	msg telebot.Editable,
) (string, error) {
	if username == "" || deviceID <= 0 {
		return "", errors.New("invalid extend arguments")
	}

	s.rememberer.Remember(username, msg)

	onPaid := func(ctx context.Context, paymentID string) error {
		return s.devices.Extend(ctx, paymentID, username, deviceID, extendMonths)
	}

	return s.payer.CreatePayment(ctx, username, s.price*extendMonths, onPaid)
}

// StartDevicePurchase создаёт счёт на покупку нового места (+1 к лимиту).
//
// Место покупается активным сразу: срок считается от «сейчас»
// (domain.NewExpiry), ждать конца текущего оплаченного периода не нужно.
func (s *Starter) StartDevicePurchase(
	ctx context.Context,
	username string,
	msg telebot.Editable,
) (string, error) {
	if username == "" {
		return "", errors.New("invalid purchase arguments")
	}

	// Запоминаем сообщение: после оплаты Notifier отредактирует именно его.
	s.rememberer.Remember(username, msg)

	onPaid := func(ctx context.Context, paymentID string) error {
		// paymentID уходит в Grant: повторный колбэк отсечётся
		// processed_payments, и «+1» не превратится в «+2».
		return s.devices.Grant(ctx, paymentID, username, purchaseSeats, extendMonths)
	}

	// Цена = цена места на extendMonths месяцев × количество мест.
	return s.payer.CreatePayment(ctx, username, s.price*extendMonths*purchaseSeats, onPaid)
}
