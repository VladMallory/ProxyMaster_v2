// Package platega
package platega

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func NewClient(apiKey string) *Client {
	return &Client{
		baseURL: "https://app.platega.io",
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateTransaction - создает новую транзакцию в Platega
//
//			paymentMethod - метод оплаты, типа PaymentMethod(RUB, USDT, etc...)
//			amount        - цена услуги int
//			currency      - валюта, типа  Currency
//		 description      - "Оплата мешков картошки клиенту №293" string
//
//	   payload - инфа которую можно дополнительно оставить (как я понял) ни на что не влияет,   можно "" string
func (c *Client) CreateTransaction(ctx context.Context, paymentMethod PaymentMethod, amount int, currency Currency, description string, payload string) (URL string, err error) {
	merchantID := os.Getenv("PLATEGA_MERCHANT_ID")
	if merchantID == "" {
		log.Fatal("Не установленно MERCHANT_ID в .env")
	}

	plategaAPIKey := os.Getenv("PLATEGA_API_KEY")
	if plategaAPIKey == "" {
		log.Fatal("Не установленно PLATEGA_API_KEY в .env")
	}

	// сборка. URL
	plategaBaseURL := os.Getenv("PLATEGA_BASE_URL")
	if plategaBaseURL == "" {
		log.Fatal("Не установленно PLATEGA_BASE_URL в .env")
	}
	plategaTotalURL := plategaBaseURL + "/transaction/process"

	// сборка реквеста
	reqBody := CreateTransactionRequest{
		PaymentMethod: int(paymentMethod),
		PaymentDetails: PaymentDetails{
			Amount:   amount,
			Currency: string(currency),
		},
		Description: description,
		ReturnURL:   "https://google.com/success", // todo хз че тут
		FailedURL:   "https://google.com/fail",    // todo тут вроде пойдет, но тоже хз
		Payload:     payload,
	}

	// маршалинг реквеста
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Fatal(fmt.Errorf("platega.CreateTransaction: MarshalingError: %v", err))
	}

	// запрос к апи platega
	req, err := http.NewRequest("POST", plategaTotalURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatal(fmt.Errorf("platega.CreateTransaction: NewRequestError: %v", err))
	}
	req.Header.Set("X-MerchantId", merchantID)
	req.Header.Set("X-Secret", plategaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Fatal(fmt.Errorf("platega.CreateTransaction: GetResponseError: %v", err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("platega.CreateTransaction: ReadingResponseBodyError: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("platega.CreateTransaction: StatusCode: %v\nError: %s", resp.StatusCode, string(respBody))
	}

	var CreateTransactionResponse CreateTransactionResponse
	err = json.Unmarshal(respBody, &CreateTransactionResponse)
	if err != nil {
		log.Printf("platega.CreateTransaction: UnmarshalingResponseError: %v", err)
	}

	URL = CreateTransactionResponse.Redirect
	ID := CreateTransactionResponse.TransactionID

	fmt.Printf("\nURL для оплаты: %v\n", URL)
	fmt.Printf("\nID оплаты: %v\n", ID)

	return URL, nil
}
