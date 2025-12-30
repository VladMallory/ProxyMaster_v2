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
func loadConfig() (*Config, error) {
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

// remnaClient отвечает за взаимодействие с внешним API.
// Он инкапсулирует HTTP-клиент и базовый URL.
type remnaClient struct {
	cfg        *Config
	httpClient *http.Client
}

// newRemnaClient - конструктор для создания клиента.
func newRemnaClient(cfg *Config) *remnaClient {
	return &remnaClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Хорошая практика: всегда задавать тайм-аут
		},
	}
}

// Login выполняет вход в систему и возвращает токен доступа.
func (c *remnaClient) Login() (string, error) {
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
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("ошибка закрытия тела ответа: %v\n", err)
		}
	}(resp.Body)

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

// getClients запрашивает список клиентов, используя полученный токен.
func (c *remnaClient) getClients(token string) error {
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
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("ошибка закрытия тела ответа: %v\n", err)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка чтения тела ответа: %w", err)
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

// extendClientSubscription увеличивает время подписки пользователя на указанное количество дней.
func (c *remnaClient) extendClientSubscription(userUUID string, username string, days int) error {
	//формирует url для запроса в api с секретным токеном для прохода через Nginx
	url := fmt.Sprintf("%s/api/users/bulk/extend-expiration-date?%s", c.cfg.BaseURL, c.cfg.SecretURLToken)

	payload := BulkExtendRequest{
		UUIDs: []string{userUUID},
		Days:  days,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга тела запроса: %w", err)
	}

	//создаем запрос
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", "Bearer "+c.cfg.APIToken)

	//делаем запрос и получаем ответ
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	//закрытие тела запроса, хз зачем, в go есть сборщик мусора, ну а хули пусть будет
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("ошибка закрытия тела ответа: %v\n", err)
		}
	}(response.Body)

	//если ок, то заебок
	if response.StatusCode == http.StatusOK {
		log.Printf("период подписки клиента: %s. UUID: %s увеличен на %d дней.\n", username, userUUID, days)
	} else {
		//ну плохо все
		body, err := io.ReadAll(response.Body)
		if err != nil {
			log.Println("не удалось преобразовать тело ответа")
			return fmt.Errorf("не удалось преобразовать тело ответа: %w", err)
		}
		log.Printf("не удалось увеличить период подписки клиента: %s. UUID: %s. Тел", username, userUUID, string(body))
		return fmt.Errorf("не удалось увеличить период подписки")
	}
	return nil
}

// ==========================================
// Main (Точка входа)
// ==========================================

func main() {
	// 1. Инициализация конфигурации
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Fatal: %v", err)
	}

	// 2. Инициализация клиента
	client := newRemnaClient(cfg)

	fmt.Println("------------------------------------------------")
	fmt.Println("Используем статический API токен из .env")
	fmt.Println("------------------------------------------------")

	// 3. Получение данных о клиентах (сразу, без логина)
	if err := client.getClients(cfg.APIToken); err != nil {
		log.Printf("Ошибка при получении клиентов: %v", err)
	}

	if err := client.extendClientSubscription("3380eb3a-d9bd-418a-bb2c-b42d5ec282b7", 500); err != nil {
		log.Fatal(err)
	}
}
