// remnawave.go - реализует взаимодействие с api remnawave
package remnawave

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// LoginRequest - данные для входа
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse - ответ сервера при входе
type LoginResponse struct {
	Response struct {
		AccessToken string `json:"accessToken"`
	} `json:"response"`
}

// Login выполняет авторизацию и сохраняет токен
func (c *Client) Login(ctx context.Context, username, password string) error {
	// подготавливаем данные для входа в панель
	reqBody := LoginRequest{
		Username: username,
		Password: password,
	}

	// превращаем верхнюю структуру из GOOO кода в json чтобы панель нас поняла
	// {"username":"admin","password":1234"}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("ошибка кодирования данных: %w", err)
	}

	// делаем итоговую ссылку куда идем логниться
	// Сохраняем адрес в requestURL поскольку remna
	// разрешает вход только по секретному адресу
	requestURL := fmt.Sprintf("%s/api/auth/login?%s", c.baseURL, c.secretUrlToken)

	// создаем запрос на отправку. Как письмо перед отправкой.
	// Кладем итоговый путь и json который делали как паспорт при
	// http.MethodPost - даем метом POST, отправляем данные на сервер
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}

	// сервер может принимать видео, текст и т.д.
	// тут мы говорим прям что даем json
	// Content-Type - указываем какой тип контента
	req.Header.Set("Content-Type", "application/json")

	// вот тут идет коннект с сервером
	// httpClient.Do() отправляет данные и в
	// него кладем что отправим на сервер
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка отправки запроса: %w", err)
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			fmt.Fprint(os.Stderr, "ошибка закрытия соединения: ", err)
		}
	}()

	//
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ошибка в чтении ответа", err)
	}

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
	c.apiKey = loginResp.Response.AccessToken

	// Логирование успеха как в remna.go
	fmt.Printf("Status: %s\n", resp.Status)
	// Выводим только часть токена или JSON для подтверждения, чтобы не засорять лог, но пользователь просил "как в remna.go"
	// В remna.go выводится все тело. Сделаем так же.
	fmt.Printf("Body: %s\n", string(bodyBytes))

	return nil
}

// Client - реализует интерфейс domain.RemnawaveClient
// общение с API
type Client struct {
	baseURL        string
	secretUrlToken string
	apiKey         string
	httpClient     *http.Client
}

// New - новый экземпляр клиента Remnawave который с ним общается
func New(url string, secretUrlToken string, key string) *Client {
	return &Client{
		baseURL:        url,
		secretUrlToken: secretUrlToken,
		apiKey:         key,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // таймаут для запроса
		},
	}
}

func (c *Client) GetServiceInfo(ctx context.Context, serviceID string) (string, error) {
	// 1. Формируем URL для запроса
	// Если serviceID пустой, запрашиваем список всех сервисов без слэша на конце
	requestURL := fmt.Sprintf("%s/api/services", c.baseURL)
	if serviceID != "" {
		requestURL = fmt.Sprintf("%s/%s", requestURL, serviceID)
	}

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
		// Если это HTML (например 404 от nginx), не нужно выводить все тело
		if resp.StatusCode == http.StatusNotFound {
			return "", fmt.Errorf("ошибка: ресурс не найден (404) по адресу %s", requestURL)
		}
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
