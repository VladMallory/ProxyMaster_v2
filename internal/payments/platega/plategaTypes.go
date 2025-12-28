// platega/types.go
package platega

import "net/http"

// Методы оплаты которые принимает platega
type PaymentMethod int

const (
	SBPQR                 PaymentMethod = 2
	RussianCards          PaymentMethod = 10
	CardEcuaring          PaymentMethod = 11
	InternationalEcuaring PaymentMethod = 12
	Crypto                PaymentMethod = 13
)

// currency валюты которые принимает platega
type Currency string

const (
	RUB  Currency = "RUB"
	USDT Currency = "USDT"
)

// Запрос
type CreateTransactionRequest struct {
	PaymentMethod  int            `json:"paymentMethod"`
	PaymentDetails PaymentDetails `json:"paymentDetails"`
	Description    string         `json:"description"`
	ReturnURL      string         `json:"return"`
	FailedURL      string         `json:"failedUrl"`
	Payload        string         `json:"payload"`
}

type PaymentDetails struct {
	Amount   int    `json:"amount,string"` //мб можно и флоат64?
	Currency string `json:"currency"`
}

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Response
type CreateTransactionResponse struct {
	PaymentMethod  string  `json:"paymentMethod"`
	TransactionID  string  `json:"transactionId"`
	Redirect       string  `json:"redirect"`
	ReturnURL      string  `json:"return"`
	PaymentDetails string  `json:"paymentDetails"`
	Status         string  `json:"status"`
	ExpiresIn      string  `json:"expiresIn"`
	MerchantID     string  `json:"merchantId"`
	USDTPrice      float64 `json:"usdtRate"`
	CryptoAmount   float64 `json:"cryptoAmount"`
}
