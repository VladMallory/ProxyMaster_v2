// remnawave.go - реализует взаимодействие с api remnawave
package remnawave

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

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

	// 5. Проверяем статус код ответа
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("некорректный статус ответа: %d", resp.StatusCode)
	}

	// Тут должна быть логика чтения body и анмаршалинга JSON
	// Для примера вернем заглушку
	return "Service Data for " + serviceID, nil
}
