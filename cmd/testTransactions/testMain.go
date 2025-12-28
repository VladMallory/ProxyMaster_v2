// main.go
package main

import (
	"ProxyMaster_v2/internal/payments/platega"
	"context"
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
	_, err = plategaClient.CreateTransaction(ctx, platega.SBPQR, 3939, platega.USDT, "Оплата мешков картошки клиенту №293", "Тест payload")
	if err != nil {
		log.Fatal(err)
	}

}
