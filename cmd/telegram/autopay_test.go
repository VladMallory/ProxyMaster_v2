package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	paymentsvc "github.com/VladMallory/ProxyMaster_v2/internal/payment/service"
)

type stubPaymentService struct {
	id string
}

func (s *stubPaymentService) CreateInvoice(
	context.Context,
	string,
	int,
) (string, string, error) {
	return s.id, "", nil
}

func (s *stubPaymentService) CheckStatus(context.Context, string) (bool, error) {
	return false, nil
}

func fallbackFunc(svc *stubPaymentService) func() paymentsvc.PaymentService {
	return func() paymentsvc.PaymentService { return svc }
}

func TestWithAutopay_DisabledReturnsFallback(t *testing.T) {
	t.Setenv("YOOKASSA_AUTOPAY", "false")

	fallback := &stubPaymentService{id: "fallback"}

	require.Same(t, fallback, withAutopay(context.Background(), nil, fallbackFunc(fallback)))
}

func TestWithAutopay_BrokenConfigReturnsFallback(t *testing.T) {
	t.Setenv("YOOKASSA_AUTOPAY", "true")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("YOUKASSA_SHOP_ID", "shop")
	t.Setenv("YOUKASSA_SECRET_KEY", "secret")
	t.Setenv("REMNA_PANEL", "https://panel.example?key=abc")
	t.Setenv("REMNA_TOKEN", "token")
	t.Setenv("PRICE_PER_MONTH", "100")

	fallback := &stubPaymentService{id: "fallback"}

	require.Same(t, fallback, withAutopay(context.Background(), nil, fallbackFunc(fallback)))
}
