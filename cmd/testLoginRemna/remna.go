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
	// Важно: используем чистый домен
	baseURL := "https://panel.moment-was-da.ru"

	fmt.Printf("Пробуем войти на %s с логином %s...\n", baseURL, login)

	// Формируем JSON тело
	authBody := map[string]string{
		"username": login,
		"password": pass,
	}
	jsonData, _ := json.Marshal(authBody)

	// Создаем запрос на Login с секретным параметром
	// ВАЖНО: Мы добавляем секретный ключ eyMjBapF=OXaAOjtG в URL, чтобы пройти защиту Nginx
	req, err := http.NewRequest("POST", baseURL+"/api/auth/login?eyMjBapF=OXaAOjtG", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Добавляем полный набор заголовков, чтобы притвориться браузером
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", baseURL+"/")
	req.Header.Set("Origin", baseURL)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

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
