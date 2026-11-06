// deadcode yookassa: весь файл, временная интеграция автоплатежа через ЮKassa.
// Удаляется целиком вместе с internal/payment/autopay/.

package main

import (
	"context"
	"database/sql"
	"log"

	"github.com/VladMallory/ProxyMaster_v2/internal/config"
	telegramhandler "github.com/VladMallory/ProxyMaster_v2/internal/payment/adapter/inbound/telegram"
	"github.com/VladMallory/ProxyMaster_v2/internal/payment/autopay"
	paymentsvc "github.com/VladMallory/ProxyMaster_v2/internal/payment/service"
)

func init() {
	paymentServiceFactory = func(
		ctx context.Context,
		cfg config.Config,
		db *sql.DB,
	) paymentsvc.PaymentService {
		return withAutopay(ctx, db, func() paymentsvc.PaymentService {
			return newDefaultPaymentService(cfg)
		})
	}
}

func withAutopay(
	ctx context.Context,
	db *sql.DB,
	fallback func() paymentsvc.PaymentService,
) paymentsvc.PaymentService {
	if !autopay.Enabled() {
		return fallback()
	}

	cfg, err := autopay.LoadConfig()
	if err != nil {
		log.Printf("автоплатёж выключен: %v", err)

		return fallback()
	}

	router, err := autopay.NewRouter(ctx, cfg, db)
	if err != nil {
		log.Printf("автоплатёж выключен: %v", err)

		return fallback()
	}

	return router
}

// newAutopayButton кнопка отключения автоплатежа для меню и экрана тарифов.
func newAutopayButton(
	ctx context.Context,
	db *sql.DB,
) (*autopay.Contributor, telegramhandler.AutopayButton) {
	if !autopay.Enabled() {
		return nil, nil
	}

	c, err := autopay.NewContributor(ctx, db)
	if err != nil {
		log.Printf("кнопка автоплатежа выключена: %v", err)

		return nil, nil
	}

	return c, c
}
