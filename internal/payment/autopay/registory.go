package autopay

import (
	"context"
	"database/sql"
	"log/slog"
	"strconv"
	"strings"

	paymentsvc "github.com/VladMallory/ProxyMaster_v2/internal/payment/service"
)

var _ paymentsvc.PaymentService = (*bindingPayment)(nil)

// NewRouter собирает платёжный сервис на ЮKassa. Когда фича включена, он
// подменяет провайдера целиком: весь поток оплат идёт через ЮKassa, чтобы
// карта сохранилась для автоплатежа. Угадывать «подписка или нет» по сумме
// нельзя — тарифы живут в main, и любая их правка тихо ломала бы маршрут.
func NewRouter(
	ctx context.Context,
	cfg Config,
	db *sql.DB,
) (paymentsvc.PaymentService, error) {
	st := newStore(db)
	if err := st.Migrate(ctx); err != nil {
		return nil, err
	}

	return &bindingPayment{
		client:        newYooClient(cfg.ShopID, cfg.SecretKey, cfg.ReturnURL),
		store:         st,
		pricePerMonth: cfg.PricePerMonth,
	}, nil
}

type bindingPayment struct {
	client        *yooClient
	store         saver
	pricePerMonth int
}

type saver interface {
	Save(ctx context.Context, payment record) error
}

func (b *bindingPayment) CreateInvoice(
	ctx context.Context,
	userID string,
	amount int,
) (string, string, error) {
	return b.client.CreateBindingPayment(ctx, userID, amount)
}

func (b *bindingPayment) CheckStatus(
	ctx context.Context,
	invoiceID string,
) (bool, error) {
	payment, err := b.client.Payment(ctx, invoiceID)
	if err != nil {
		return false, err
	}

	if payment.Status != "succeeded" {
		return false, nil
	}

	b.remember(ctx, payment)

	return true, nil
}

func (b *bindingPayment) remember(ctx context.Context, payment yooPayment) {
	if !payment.PaymentMethod.Saved || payment.PaymentMethod.ID == "" {
		slog.Warn(
			"autopay: ЮKassa не сохранила способ оплаты",
			"payment", payment.ID,
		)

		return
	}

	userID := payment.Metadata["userID"]
	if userID == "" {
		slog.Warn("autopay: в платеже нет userID", "payment", payment.ID)

		return
	}

	amount, err := parseRubles(payment.Amount.Value)
	if err != nil {
		slog.Warn(
			"autopay: не разобрать сумму платежа",
			"payment", payment.ID,
			"err", err,
		)

		return
	}

	monthKopecks := b.pricePerMonth * 100
	if monthKopecks <= 0 || amount%monthKopecks != 0 {
		slog.Warn(
			"autopay: сумма не кратна цене месяца, карта не сохранена",
			"payment", payment.ID,
			"amount", payment.Amount.Value,
		)

		return
	}

	months := amount / monthKopecks

	if err := b.store.Save(ctx, record{
		Username:      userID,
		MethodID:      payment.PaymentMethod.ID,
		Months:        months,
		AmountKopecks: amount,
	}); err != nil {
		slog.Error(
			"autopay: не удалось сохранить способ оплаты",
			"payment", payment.ID,
			"err", err,
		)
	}
}

func parseRubles(value string) (int, error) {
	rubPart, kopPart, found := strings.Cut(value, ".")

	rub, err := strconv.Atoi(rubPart)
	if err != nil {
		return 0, err
	}

	if !found {
		return rub * 100, nil
	}

	if len(kopPart) > 2 {
		kopPart = kopPart[:2]
	}
	for len(kopPart) < 2 {
		kopPart += "0"
	}

	kop, err := strconv.Atoi(kopPart)
	if err != nil {
		return 0, err
	}

	return rub*100 + kop, nil
}
