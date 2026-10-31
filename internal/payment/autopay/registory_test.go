package autopay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeSaver struct {
	saved []record
	err   error
}

func (f *fakeSaver) Save(_ context.Context, payment record) error {
	f.saved = append(f.saved, payment)

	return f.err
}

func testYooClient(t *testing.T, handler http.HandlerFunc) *yooClient {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return &yooClient{
		shopID:     "shop",
		secretKey:  "secret",
		returnURL:  "https://bot.example/return",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
	}
}

type createRequest struct {
	method      string
	path        string
	idempotence string
	user        string
	pass        string
	body        yooCreatePayment
	decodeErr   error
}

func TestCreateInvoice_SendsSavedCardRequest(t *testing.T) {
	t.Parallel()

	requests := make(chan createRequest, 1)

	client := testYooClient(t, func(w http.ResponseWriter, r *http.Request) {
		var captured createRequest

		captured.method = r.Method
		captured.path = r.URL.Path
		captured.idempotence = r.Header.Get("Idempotence-Key")
		captured.user, captured.pass, _ = r.BasicAuth()
		captured.decodeErr = json.NewDecoder(r.Body).Decode(&captured.body)

		requests <- captured

		_, _ = w.Write([]byte(
			`{"id":"pay-1","confirmation":{"confirmation_url":"https://pay.example/1"}}`,
		))
	})

	svc := &bindingPayment{
		client:        client,
		store:         &fakeSaver{},
		pricePerMonth: 100,
	}

	id, payURL, err := svc.CreateInvoice(context.Background(), "42", 20000)
	require.NoError(t, err)
	require.Equal(t, "pay-1", id)
	require.Equal(t, "https://pay.example/1", payURL)

	captured := <-requests

	require.NoError(t, captured.decodeErr)
	require.Equal(t, http.MethodPost, captured.method)
	require.Equal(t, "/v3/payments", captured.path)
	require.NotEmpty(t, captured.idempotence)
	require.Equal(t, "shop", captured.user)
	require.Equal(t, "secret", captured.pass)
	require.True(t, captured.body.SavePaymentMethod)
	require.Equal(t, "42", captured.body.Metadata["userID"])
	require.Equal(t, "200.00", captured.body.Amount.Value)
	require.Equal(t, "RUB", captured.body.Amount.Currency)
	require.Equal(t, "https://bot.example/return", captured.body.Confirmation.ReturnURL)
}

func TestCheckStatus_NotPaid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{
			name:   "ожидание не считается оплатой",
			status: http.StatusOK,
			body:   `{"id":"p","status":"pending"}`,
		},
		{
			name:    "ошибка ЮKassa не считается оплатой",
			status:  http.StatusInternalServerError,
			body:    `boom`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := testYooClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})

			saver := &fakeSaver{}
			svc := &bindingPayment{
				client:        client,
				store:         saver,
				pricePerMonth: 100,
			}

			ok, err := svc.CheckStatus(context.Background(), "p")
			if tt.wantErr {
				require.Error(t, err)
				require.False(t, ok)
				require.Empty(t, saver.saved)

				return
			}

			require.NoError(t, err)
			require.False(t, ok)
			require.Empty(t, saver.saved)
		})
	}
}

type checkStatusCase struct {
	name string
	body string
	want []record
}

var checkStatusSavesCardCases = []checkStatusCase{
	{
		name: "успех сохраняет карту и срок",
		body: `{"id":"p","status":"succeeded",
			"amount":{"value":"200.00","currency":"RUB"},
			"metadata":{"userID":"42"},
			"payment_method":{"id":"pm-1","saved":true}}`,
		want: []record{{
			Username:      "42",
			MethodID:      "pm-1",
			Months:        2,
			AmountKopecks: 20000,
		}},
	},
	{
		name: "карта не сохранена — привязки нет",
		body: `{"id":"p","status":"succeeded",
			"amount":{"value":"200.00","currency":"RUB"},
			"metadata":{"userID":"42"},
			"payment_method":{"id":"pm-1","saved":false}}`,
	},
	{
		name: "нет userID — привязки нет",
		body: `{"id":"p","status":"succeeded",
			"amount":{"value":"200.00","currency":"RUB"},
			"metadata":{},
			"payment_method":{"id":"pm-1","saved":true}}`,
	},
	{
		name: "сумма не кратна месяцу — привязки нет",
		body: `{"id":"p","status":"succeeded",
			"amount":{"value":"250.00","currency":"RUB"},
			"metadata":{"userID":"42"},
			"payment_method":{"id":"pm-1","saved":true}}`,
	},
}

func TestCheckStatus_SavesCardOnlyForFullMonth(t *testing.T) {
	t.Parallel()

	for _, tt := range checkStatusSavesCardCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := testYooClient(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			})

			saver := &fakeSaver{}
			svc := &bindingPayment{
				client:        client,
				store:         saver,
				pricePerMonth: 100,
			}

			ok, err := svc.CheckStatus(context.Background(), "p")
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, tt.want, saver.saved)
		})
	}
}

func TestParseRubles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  int
	}{
		{value: "200.00", want: 20000},
		{value: "99.90", want: 9990},
		{value: "100", want: 10000},
		{value: "0.05", want: 5},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()

			got, err := parseRubles(tt.value)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
