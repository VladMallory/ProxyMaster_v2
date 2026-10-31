package autopay

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

const (
	autopayLeadTime = 24 * time.Hour
	autopayInterval = time.Hour
)

type payments interface {
	Charge(
		ctx context.Context,
		methodID string,
		amountKopecks int,
		idempotenceKey string,
	) (string, error)
	Status(ctx context.Context, paymentID string) (string, error)
}

type panel interface {
	ExpireAt(ctx context.Context, username string) (time.Time, error)
	ExtendMonths(ctx context.Context, username string, months int) error
}

type storage interface {
	List(ctx context.Context) ([]record, error)
	SetPending(
		ctx context.Context,
		username string,
		paymentID string,
		target time.Time,
	) error
	ClearPending(ctx context.Context, username string) error
	Delete(ctx context.Context, username string) error
}

type notifier interface {
	Paid(userID string, months, amountRub int)
	Failed(userID string)
}

type worker struct {
	store    storage
	payments payments
	panel    panel
	notifier notifier
	now      func() time.Time
}

func newWorker(
	st storage,
	p payments,
	pl panel,
	n notifier,
) *worker {
	return &worker{
		store:    st,
		payments: p,
		panel:    pl,
		notifier: n,
		now:      time.Now,
	}
}

func (w *worker) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = autopayInterval
	}

	w.runOnce(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *worker) runOnce(ctx context.Context) {
	payments, err := w.store.List(ctx)
	if err != nil {
		log.Printf("autopay: не удалось прочитать список: %v", err)

		return
	}

	for _, payment := range payments {
		if err := w.process(ctx, payment); err != nil {
			log.Printf(
				"autopay: пользователь %s пропущен: %v",
				payment.Username,
				err,
			)
		}
	}
}

func (w *worker) process(ctx context.Context, payment record) error {
	now := w.now()

	if payment.PendingPaymentID != "" {
		return w.settle(ctx, payment)
	}

	expireAt, err := w.panel.ExpireAt(ctx, payment.Username)
	if errors.Is(err, errUserNotFound) {
		return w.drop(ctx, payment.Username, "подписки нет в панели")
	}
	if err != nil {
		return fmt.Errorf("прочитать подписку: %w", err)
	}

	if !expireAt.After(now) {
		return w.drop(ctx, payment.Username, "подписка уже истекла")
	}

	if expireAt.After(now.Add(autopayLeadTime)) {
		return nil
	}

	target := expireAt.AddDate(0, payment.Months, 0)
	idempotenceKey := fmt.Sprintf(
		"autopay-%s-%d",
		payment.Username,
		expireAt.Unix(),
	)

	paymentID, err := w.payments.Charge(
		ctx,
		payment.MethodID,
		payment.AmountKopecks,
		idempotenceKey,
	)
	if err != nil {
		return fmt.Errorf("создать списание: %w", err)
	}

	if err := w.store.SetPending(
		ctx,
		payment.Username,
		paymentID,
		target,
	); err != nil {
		return fmt.Errorf("запомнить списание: %w", err)
	}

	payment.PendingPaymentID = paymentID
	payment.PendingTarget = target

	return w.settle(ctx, payment)
}

func (w *worker) settle(ctx context.Context, payment record) error {
	status, err := w.payments.Status(ctx, payment.PendingPaymentID)
	if err != nil {
		return fmt.Errorf("статус списания: %w", err)
	}

	switch status {
	case "succeeded":
		return w.complete(ctx, payment)
	case "canceled":
		return w.drop(ctx, payment.Username, "списание отменено")
	default:
		return nil
	}
}

func (w *worker) complete(ctx context.Context, payment record) error {
	expireAt, err := w.panel.ExpireAt(ctx, payment.Username)
	if errors.Is(err, errUserNotFound) {
		return w.drop(ctx, payment.Username, "подписки нет в панели")
	}
	if err != nil {
		return fmt.Errorf("прочитать подписку: %w", err)
	}

	if payment.PendingTarget.IsZero() {
		payment.PendingTarget = expireAt.AddDate(0, payment.Months, 0)
	}

	if expireAt.Before(payment.PendingTarget) {
		if err := w.panel.ExtendMonths(
			ctx,
			payment.Username,
			payment.Months,
		); err != nil {
			return fmt.Errorf("продлить подписку: %w", err)
		}
	}

	if err := w.store.ClearPending(ctx, payment.Username); err != nil {
		return fmt.Errorf("очистить списание: %w", err)
	}

	w.notifier.Paid(
		payment.Username,
		payment.Months,
		payment.AmountKopecks/100,
	)

	return nil
}

func (w *worker) drop(
	ctx context.Context,
	username, reason string,
) error {
	if err := w.store.Delete(ctx, username); err != nil {
		return fmt.Errorf("удалить автоплатёж: %w", err)
	}

	log.Printf("autopay: выключен для %s: %s", username, reason)

	w.notifier.Failed(username)

	return nil
}
