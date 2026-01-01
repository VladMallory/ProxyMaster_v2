// Package platega описывает взаимодействие с платежной системой Platega.
package platega

import "net/http"

// PaymentMethod методы оплаты, которые принимает platega.
type PaymentMethod int

// Список методов оплаты.
const (
	SBPQR                 PaymentMethod = 2
	RussianCards          PaymentMethod = 10
	CardEcuaring          PaymentMethod = 11
	InternationalEcuaring PaymentMethod = 12
	Crypto                PaymentMethod = 13
)

// Currency валюты которые принимает platega.
type Currency string

// Тип валюты который принимает platega.
const (
	RUB  Currency = "RUB"
	USDT Currency = "USDT"
)

// CreateTransactionRequest запрос на создание транзакции.
type CreateTransactionRequest struct {
	PaymentMethod  int            `json:"paymentMethod"`
	PaymentDetails PaymentDetails `json:"paymentDetails"`
	Description    string         `json:"description"`
	ReturnURL      string         `json:"return"`
	FailedURL      string         `json:"failedUrl"`
	Payload        string         `json:"payload"`
}

// PaymentDetails детали оплаты
type PaymentDetails struct {
	Amount   int    `json:"amount,string"` // мб можно и флоат64?
	Currency string `json:"currency"`
}

// Client что нужно для работы с platega
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// CreateTransactionResponse то что возвращает platega при создании транзакции.
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
