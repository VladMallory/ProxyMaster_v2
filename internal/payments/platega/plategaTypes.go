// Package platega описывает взаимодействие с платежной системой Platega.
package platega

import (
	"net/http"

	"github.com/VladMallory/ProxyMaster_v2/pkg/logger"
)

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

// CreateTransactionRequest запрос на создание транзакции с заданным методом оплаты (v1).
type CreateTransactionRequest struct {
	PaymentMethod  int            `json:"paymentMethod"`
	PaymentDetails PaymentDetails `json:"paymentDetails"`
	Description    string         `json:"description"`
	ReturnURL      string         `json:"return"`
	FailedURL      string         `json:"failedUrl"`
	Payload        string         `json:"payload,omitempty"`
}

// CreateTransactionV2Request запрос на создание транзакции без метода оплаты (v2).
// Пользователь сам выбирает способ на странице Platega.
type CreateTransactionV2Request struct {
	PaymentDetails PaymentDetails `json:"paymentDetails"`
	Description    string         `json:"description"`
	ReturnURL      string         `json:"return,omitempty"`
	FailedURL      string         `json:"failedUrl,omitempty"`
	Payload        string         `json:"payload,omitempty"`
}

// PaymentDetails детали оплаты.
type PaymentDetails struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// Client что нужно для работы с platega.
type Client struct {
	baseURL    string
	merchantID string
	apiKey     string
	returnURL  string
	httpClient *http.Client
	logger     logger.Logger
}

// CreateTransactionResponse то что возвращает platega при создании транзакции (v1).
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

// CreateTransactionV2Response то что возвращает platega при создании транзакции без метода (v2).
type CreateTransactionV2Response struct {
	TransactionID string  `json:"transactionId"`
	Status        string  `json:"status"`
	URL           string  `json:"url"`
	ExpiresIn     string  `json:"expiresIn"`
	Rate          float64 `json:"rate"`
}

// TransactionInfoResponse ответ с информацией о транзакции.
type TransactionInfoResponse struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	PaymentMethod  string `json:"paymentMethod"`
	PaymentDetails struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	} `json:"paymentDetails"`
	Description string `json:"description"`
	Payload     string `json:"payload"`
	ExternalID  string `json:"externalId"`
}
