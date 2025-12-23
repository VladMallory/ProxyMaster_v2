// main.go
package main

import (
	"ProxyMaster_v2/cmd/testTransactions/platega"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	//
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	plategaClient := platega.NewClient(os.Getenv("PLATEGA_API_KEY"))

	// Создаем транзакцию через Platega
	URL, err := plategaClient.CreateTransaction(ctx, platega.SBPQR, 3939, platega.USDT, "Оплата мешков картошки клиенту №293", "Тест payload")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("URL для оплаты: %v", URL)

	/*
		// Получаем информацию о транзакции Platega
		plategaInfo, err := plategaClient.GetTransactionInfo(ctx, plategaID)
		if err != nil {
			log.Fatal(err)
		}
	*/
}
