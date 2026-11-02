package telegram

import "testing"

func TestIsLegacyPaymentCallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
		want bool
	}{
		{name: "кнопка баланса старого меню", data: "btn_balance", want: true},
		{name: "ещё более старый баланс", data: "topup_balance", want: true},
		{name: "старое меню продления", data: "payment_menu", want: true},
		{name: "старый тариф", data: "btn_topUp_300", want: true},
		{name: "новый callback оплаты", data: "\fpay_checkout", want: false},
		{name: "обычный текст", data: "hello", want: false},
		{name: "пустой callback", data: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isLegacyPaymentCallback(tt.data); got != tt.want {
				t.Fatalf("isLegacyPaymentCallback(%q) = %v, ожидалось %v", tt.data, got, tt.want)
			}
		})
	}
}
