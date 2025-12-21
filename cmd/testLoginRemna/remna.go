package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Загружаем .env, чтобы взять логин/пароль
	if err := godotenv.Load(); err != nil {
		log.Println("Не удалось загрузить .env, берем из окружения")
	}

	login := os.Getenv("REMNA_LOGIN")
	pass := os.Getenv("REMNA_PASS")
	secretToken := os.Getenv("REMNA_SECRET_TOKEN")

	// Важно: используем чистый домен
	baseURL := os.Getenv("REMNA_BASE_PANEL")

	fmt.Printf("Пробуем войти на %s с логином %s...\n", baseURL, login)

	// Формируем JSON тело
	authBody := map[string]string{
		"username": login,
		"password": pass,
	}
	jsonData, _ := json.Marshal(authBody)

	totalBaseUrl := baseURL + "/api/auth/login?" + secretToken

	// Создаем запрос на Login с секретным параметром
	req, err := http.NewRequest("POST", totalBaseUrl, bytes.NewBuffer(jsonData))

	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Body: %s\n", string(body))
}
