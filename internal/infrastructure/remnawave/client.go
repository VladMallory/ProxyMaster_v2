// remnawave.go - реализует взаимодействие с api remnawave
package remnawave

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LoginRequest - данные для входа
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse - ответ сервера при входе
type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// Login выполняет авторизацию и сохраняет токен
func (c *Client) Login(ctx context.Context, username, password string) error {
	reqBody := LoginRequest{
		Username: username,
		Password: password,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("ошибка кодирования данных: %w", err)
	}

	// Предполагаем, что эндпоинт логина находится по пути /api/auth/login
	requestURL := fmt.Sprintf("%s/api/auth/login", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0") // Притворяемся браузером

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка отправки запроса: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("ошибка входа: %d, ответ: %s", resp.StatusCode, string(bodyBytes))
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(bodyBytes, &loginResp); err != nil {
		// Иногда токен приходит просто как текст, или JSON другой структуры.
		// Но пока пробуем стандартный Unmarshal
		return fmt.Errorf("ошибка разбора ответа: %w, тело: %s", err, string(bodyBytes))
	}

	// Сохраняем полученный токен
	c.apiKey = loginResp.AccessToken

	return nil
}

// Client - реализует интерфейс domain.RemnawaveClient
// общение с API
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New - новый экземпляр клиента Remnawave который с ним общается
func New(url string, key string) *Client {
	return &Client{
		baseURL: url,
		apiKey:  key,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // таймаут для запроса
		},
	}
}

func (c *Client) GetServiceInfo(ctx context.Context, serviceID string) (string, error) {
	// 1. Формируем URL для запроса
	requestURL := fmt.Sprintf("%s/api/services/%s", c.baseURL, serviceID)

	// 2. Создаем новый HTTP запрос с контекстом (важно для отмены запросов)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("не удалось создать запрос: %w", err)
	}

	// 3. Добавляем необходимые заголовки авторизации
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// 4. Выполняем запрос
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	// Важно: всегда закрываем тело ответа, чтобы не было утечек памяти
	defer resp.Body.Close()

	// TODO: верну попозже
	// 5. Проверяем статус код ответа
	// if resp.StatusCode != http.StatusOK {
	// 	return "", fmt.Errorf("некорректный статус ответа: %d", resp.StatusCode)
	// }
	//

	// 5 читаем тело ответа
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения ответа: %w", err)
	}
	bodyString := string(bodyBytes)

	// 6 проверяем статус код ответа
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("неккоректный статус ответа: %d, тело: %s", resp.StatusCode, bodyString)
	}

	// TODO: первый вариант
	// Тут должна быть логика чтения body и анмаршалинга JSON
	// Для примера вернем заглушку
	// return "Service Data for " + serviceID, nil
	//

	// проверяю какая вообще ошибка возвращения
	return bodyString, nil
}
