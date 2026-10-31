package autopay

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"
)

func OpenDB(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()

		return nil, err
	}

	return db, nil
}

func RunWorker(ctx context.Context, cfg Config, db *sql.DB) error {
	st := newStore(db)
	if err := st.Migrate(ctx); err != nil {
		return err
	}

	w := newWorker(
		st,
		newYooClient(cfg.ShopID, cfg.SecretKey, cfg.ReturnURL),
		newRemnaClient(cfg.PanelBaseURL, cfg.PanelAPIKey, cfg.PanelToken),
		newNotifier(cfg.TelegramToken),
	)

	w.Run(ctx, autopayInterval)

	return nil
}

func Bind(
	ctx context.Context,
	cfg Config,
	db *sql.DB,
	username string,
	months int,
) error {
	if username == "" {
		return errors.New("нужен -user")
	}
	if months <= 0 {
		return errors.New("-months должен быть больше нуля")
	}

	st := newStore(db)
	if err := st.Migrate(ctx); err != nil {
		return err
	}

	amountKopecks := cfg.PricePerMonth * months * 100

	methodID, err := bindCard(ctx, cfg, username, amountKopecks)
	if err != nil {
		return err
	}

	remna := newRemnaClient(cfg.PanelBaseURL, cfg.PanelAPIKey, cfg.PanelToken)

	if err := remna.ExtendMonths(ctx, username, months); err != nil {
		return err
	}

	if err := st.Save(ctx, record{
		Username:      username,
		MethodID:      methodID,
		Months:        months,
		AmountKopecks: amountKopecks,
	}); err != nil {
		return err
	}

	log.Println("Автоплатёж включён")

	return nil
}

func bindCard(
	ctx context.Context,
	cfg Config,
	username string,
	amountKopecks int,
) (string, error) {
	yk := newYooClient(cfg.ShopID, cfg.SecretKey, cfg.ReturnURL)

	paymentID, payURL, err := yk.CreateBindingPayment(
		ctx,
		username,
		amountKopecks,
	)
	if err != nil {
		return "", err
	}

	log.Printf("Ссылка на оплату: %s", payURL)
	log.Println("Жду оплату...")

	if err = waitPayment(ctx, yk, paymentID, 15*time.Minute); err != nil {
		return "", err
	}

	methodID, saved, err := yk.SavedMethod(ctx, paymentID)
	if err != nil {
		return "", err
	}
	if !saved {
		return "", errors.New("ЮKassa не сохранила способ оплаты")
	}

	return methodID, nil
}

func waitPayment(
	ctx context.Context,
	yk *yooClient,
	paymentID string,
	timeout time.Duration,
) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return errors.New("время ожидания оплаты истекло")
		case <-ticker.C:
			status, err := yk.Status(ctx, paymentID)
			if err != nil {
				continue
			}

			switch status {
			case "succeeded":
				return nil
			case "canceled":
				return errors.New("платёж отменён")
			}
		}
	}
}
