// Файл: test.go
// Предназначение: Демонстрация работы с API Remna.
// Выполняет авторизацию, получает токен доступа и запрашивает список клиентов.
// Написан в учебных целях с соблюдением базовых принципов структурирования.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

//модели все в models.go, нехуй их здесь оставлять

// LoadConfig загружает настройки из .env и переменных окружения.
// Возвращает структуру Config или ошибку.
func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("Инфо: .env файл не найден, пробуем брать из окружения ОС")
	}

	cfg := &Config{
		BaseURL:        os.Getenv("REMNA_BASE_PANEL"),
		Login:          os.Getenv("REMNA_LOGIN"),
		Pass:           os.Getenv("REMNA_PASS"),
		SecretURLToken: os.Getenv("REMNA_SECRET_TOKEN"),
		APIToken:       os.Getenv("REMNA_TOKEN"),
	}

	if cfg.BaseURL == "" || cfg.APIToken == "" {
		return nil, fmt.Errorf("ошибка конфигурации: не заданы REMNA_BASE_PANEL или REMNA_TOKEN")
	}

	return cfg, nil
}

// ==========================================
// Service (Слой бизнес-логики)
// ==========================================

// RemnaClient отвечает за взаимодействие с внешним API.
// Он инкапсулирует HTTP-клиент и базовый URL.
type RemnaClient struct {
	cfg        *Config
	httpClient *http.Client
}

// NewRemnaClient - конструктор для создания клиента.
func NewRemnaClient(cfg *Config) *RemnaClient {
	return &RemnaClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Хорошая практика: всегда задавать таймаут
		},
	}
}

// Login выполняет вход в систему и возвращает токен доступа.
func (c *RemnaClient) Login() (string, error) {
	// 1. Подготовка URL
	url := fmt.Sprintf("%s/api/auth/login?%s", c.cfg.BaseURL, c.cfg.SecretURLToken)
	fmt.Printf("[Login] Попытка входа: %s\n", url)

	// 2. Подготовка тела запроса
	reqBody := LoginRequest{
		Username: c.cfg.Login,
		Password: c.cfg.Pass,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("ошибка кодирования JSON: %w", err)
	}

	// 3. Создание HTTP запроса
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 4. Выполнение запроса
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	// 5. Чтение ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения тела ответа: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("неудачный вход. Status: %s, Body: %s", resp.Status, string(body))
	}

	// 6. Парсинг токена
	// Сначала попробуем распечатать сырой ответ, чтобы ты мог увидеть структуру (для отладки)
	fmt.Printf("[Login] Успешный ответ (Raw): %s\n", string(body))

	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return "", fmt.Errorf("ошибка парсинга JSON ответа: %w", err)
	}

	if loginResp.Response.AccessToken == "" {
		// Если токен пустой, возможно JSON поле называется по-другому
		return "", fmt.Errorf("токен не найден в ответе. Проверь структуру LoginResponse")
	}

	return loginResp.Response.AccessToken, nil
}

// GetClients запрашивает список клиентов, используя полученный токен.
func (c *RemnaClient) GetClients(token string) error {
	// ВАЖНО: Добавляем секретный токен в URL, чтобы пройти через Nginx
	// Пробуем эндпоинт /api/users (частый стандарт)
	endpoint := "/api/users"
	url := fmt.Sprintf("%s%s?%s", c.cfg.BaseURL, endpoint, c.cfg.SecretURLToken)

	fmt.Printf("[GetClients] Запрос клиентов: %s\n", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("ошибка создания запроса клиентов: %w", err)
	}

	// Добавляем токен авторизации
	// Обычно используется Bearer схема
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса клиентов: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("[GetClients] Status: %s\n", resp.Status)

	var usersResp UsersResponse
	if err := json.Unmarshal(body, &usersResp); err != nil {
		return fmt.Errorf("ошибка парсинга JSON пользователей: %w", err)
	}

	fmt.Println("\n=== Список клиентов ===")
	fmt.Printf("%-5s | %-20s | %-10s | %s\n", "ID", "Username", "Status", "Traffic (MB)")
	fmt.Println("------------------------------------------------------------")

	for _, u := range usersResp.Response.Users {
		trafficMB := float64(u.UserTraffic.UsedTrafficBytes) / 1024 / 1024
		fmt.Printf("%-5d | %-20s | %-10s | %.2f MB\n", u.ID, u.Username, u.Status, trafficMB)
	}
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("Всего клиентов: %d\n", usersResp.Response.Total)

	return nil
}

// метод увеличивает время подписки пользователя
func (c *RemnaClient) ExtendClientSubscription(userUUID string, days int) error {
	//формирует url для запроса в api
	url := fmt.Sprintf("%s/api/users/bulk/extend-expiration-date", c.cfg.BaseURL)

	payload := BulkExtendRequest{
		UUIDs: []string{userUUID},
		Days:  days,
	}
	json, _ := json.Marshal(payload)

	//создаем запрос
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(json))
	if err != nil {
		return err
	}
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", c.cfg.SecretURLToken)

	//делаем запрос и получаем ответ
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	//закрытие тела запроса, хз зачем, в гошке есть сборщик мусора, ну а хули пусть будет
	defer response.Body.Close()

	//если ок, то заебок
	if response.StatusCode == http.StatusOK {
		log.Printf("%s | Период подписки юзера с UUID: %s увеличен на %d дней.\n", time.Now(), userUUID, days)
	} else {
		//ну плохо все
		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Println("Не удалось преобразовать тело ответа")
			return fmt.Errorf("Не удалось преобразовать тело ответа.")
		}
		log.Printf(
			"%s | Не удалось увеличить период подписки юзера с UUID: %s. Тело ошибки: %s.\n", time.Now(), userUUID, string(body))
		return fmt.Errorf("Не удалось увеличить период подписки.")
	}
	return nil
}

// ==========================================
// Main (Точка входа)
// ==========================================

func main() {
	// 1. Инициализация конфигурации
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("Fatal: %v", err)
	}

	// 2. Инициализация клиента
	client := NewRemnaClient(cfg)

	fmt.Println("------------------------------------------------")
	fmt.Println("Используем статический API токен из .env")
	fmt.Println("------------------------------------------------")

	// 3. Получение данных о клиентах (сразу, без логина)
	if err := client.GetClients(cfg.APIToken); err != nil {
		log.Printf("Ошибка при получении клиентов: %v", err)
	}
}
