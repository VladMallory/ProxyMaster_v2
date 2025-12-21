package main

import (
	"context"
	"fmt"
	"log"
	"os"

	remapi "github.com/Jolymmiles/remnawave-api-go/v2/api"
	"github.com/joho/godotenv"
)

func main() {
	// Загружаем .env, чтобы взять логин/пароль
	if err := godotenv.Load(); err != nil {
		log.Println("Не удалось загрузить .env, берем из окружения")
	}

	ctx := context.Background()
	login := os.Getenv("REMNA_LOGIN")
	_ = os.Getenv("REMNA_PASS")
	token := os.Getenv("REMNA_TOKEN")
	// Важно: используем чистый домен
	baseURL := "https://panel.moment-was-da.ru"

	fmt.Printf("Пробуем войти на %s с логином %s...\n", baseURL, login)

	// Create base client
	baseClient, err := remapi.NewClient(baseURL, remapi.StaticToken{Token: token})
	if err != nil {
		log.Fatal(err)
	}

	client := remapi.NewClientExt(baseClient)

	allUsers, err := client.Users().GetAllUsers(ctx, 10.0, 0.0)
	if err != nil {
		log.Fatalf("Ошибка получения списка пользователей: %v", err)
	}

	fmt.Println(allUsers)
}
