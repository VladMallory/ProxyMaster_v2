// cmd/testApiGo/test.go - Файл предназначен для демонстрации и тестирования работы с API Remnawave.
// Он выполняет базовые операции: создание пользователя и продление подписки, используя кастомный транспорт для авторизации.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	remapi "github.com/Jolymmiles/remnawave-api-go/v2/api"
	"github.com/joho/godotenv"
)

// SecretTransport реализует интерфейс http.RoundTripper.
// Он используется для перехвата HTTP-запросов перед отправкой, чтобы автоматически добавлять
// необходимые параметры авторизации (секретный ключ) и корректный базовый URL.
type SecretTransport struct {
	Base       http.RoundTripper // Базовый транспорт (обычно http.DefaultTransport)
	QueryParam string            // Секретный параметр запроса (например, secret=value)
	BaseURL    string            // Базовый URL API, к которому нужно обращаться
}

// RoundTrip выполняет HTTP-запрос.
// Эта функция вызывается для каждого запроса, проходящего через наш http.Client.
// Здесь мы модифицируем URL запроса, добавляя хост и секретные параметры.
func (t *SecretTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Если схема URL не указана (например, просто "/api/users"), мы добавляем BaseURL.
	if req.URL.Scheme == "" {
		// Парсим базовый URL из конфигурации
		if base, _ := url.Parse(t.BaseURL); base != nil {
			u := *base
			// Объединяем путь из базового URL и путь из текущего запроса
			u.Path, u.RawQuery = strings.TrimRight(u.Path, "/")+req.URL.Path, req.URL.RawQuery
			req.URL = &u
		}
	}

	// Разбираем секретный параметр (ожидается формат "key=value") и добавляем его в Query параметры
	if p := strings.Split(t.QueryParam, "="); len(p) == 2 {
		q := req.URL.Query()
		q.Set(p[0], p[1]) // Устанавливаем ключ и значение
		req.URL.RawQuery = q.Encode()
	}

	// Если базовый транспорт не задан, используем стандартный
	if t.Base == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	// Передаем запрос дальше оригинальному транспорту
	return t.Base.RoundTrip(req)
}

// Config хранит конфигурационные данные, необходимые для работы приложения.
// Эти данные загружаются из переменных окружения.
type Config struct {
	BaseURL string // Адрес панели Remnawave
	Token   string // Основной токен доступа
	Secret  string // Секретный токен для подписи запросов (если требуется)
}

// LoadConfig считывает настройки из .env файла и переменных окружения.
// Возвращает заполненную структуру Config или ошибку, если чего-то не хватает.
func LoadConfig() (*Config, error) {
	// Загружаем переменные из файла .env, если он есть (игнорируем ошибку, если файла нет)
	_ = godotenv.Load()

	// Карта обязательных переменных, которые мы ожидаем найти
	requiredVars := map[string]*string{
		"REMNA_BASE_PANEL":   new(string),
		"REMNA_TOKEN":        new(string),
		"REMNA_SECRET_TOKEN": new(string),
	}

	var missing []string
	// Проходим по всем ожидаемым ключам и пытаемся получить их значения
	for key, ptr := range requiredVars {
		val := os.Getenv(key)
		if val == "" {
			// Если переменной нет, запоминаем её как отсутствующую
			missing = append(missing, key)
		}
		*ptr = val
	}

	// Если есть пропущенные переменные, возвращаем ошибку со списком отсутствующих ключей
	if len(missing) > 0 {
		return nil, fmt.Errorf("отсутствуют обязательные переменные: %s", strings.Join(missing, ", "))
	}

	// Возвращаем успешно загруженную конфигурацию
	return &Config{
		BaseURL: *requiredVars["REMNA_BASE_PANEL"],
		Token:   *requiredVars["REMNA_TOKEN"],
		Secret:  *requiredVars["REMNA_SECRET_TOKEN"],
	}, nil
}

// main - главная функция, точка входа в приложение.
func main() {
	// 1. Загружаем конфигурацию
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatal("Ошибка конфигурации: ", err)
	}

	fmt.Println("=== Запуск теста через библиотеку remnawave-api-go ===")

	// 2. Инициализируем клиент API.
	// Используем кастомный SecretTransport для обработки авторизации и URL.
	cli, _ := remapi.NewClient(cfg.BaseURL, remapi.StaticToken{Token: cfg.Token}, remapi.WithClient(&http.Client{
		Transport: &SecretTransport{QueryParam: cfg.Secret, BaseURL: cfg.BaseURL},
		Timeout:   10 * time.Second, // Устанавливаем таймаут, чтобы не зависнуть навечно
	}))
	// Создаем расширенный клиент для доступа к методам API
	client := remapi.NewClientExt(cli)

	// 3. Генерируем уникальное имя пользователя для теста (чтобы не было конфликтов)
	username := fmt.Sprintf("library_user_%d", time.Now().Unix())
	fmt.Printf("Попытка создать пользователя: %s\n", username)

	// создание периода подписки
	expireAt := time.Now().Add(30 * 24 * time.Hour)

	//
	resp, err := client.Users().CreateUser(context.Background(), &remapi.CreateUserRequestDto{
		Username: username,
		ExpireAt: expireAt,
	})
	if err != nil {
		log.Fatalf("Ошибка создания: %v", err)
	}

	// Приводим ответ к ожидаемому типу UserResponse
	user, ok := resp.(*remapi.UserResponse)
	if !ok {
		log.Fatalf("Неожиданный ответ при создании: %T", resp)
	}
	fmt.Printf("✅ УСПЕХ! Пользователь создан.\nUUID: %s\nUsername: %s\n", user.Response.UUID, user.Response.Username)

	// 5. Тестируем обновление пользователя (продление подписки)
	fmt.Println("\n=== Тест: Продление подписки на 5 дней ===")
	newExpire := expireAt.Add(5 * 24 * time.Hour) // Добавляем еще 5 дней
	fmt.Printf("Новая дата истечения: %s\n", newExpire)

	_, err = client.Users().UpdateUser(context.Background(), &remapi.UpdateUserRequestDto{
		UUID:     remapi.NewOptUUID(user.Response.UUID), // Используем UUID созданного пользователя
		ExpireAt: remapi.NewOptDateTime(newExpire),      // Передаем новую дату
	})
	if err != nil {
		log.Fatalf("Ошибка обновления: %v", err)
	}
	fmt.Println("✅ УСПЕХ! Подписка продлена.")
}
