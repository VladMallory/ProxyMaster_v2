// Файл: test.go
// Предназначение: Тестирование работы через официальную библиотеку remnawave-api-go.
// Использует кастомный Transport для внедрения секретного параметра URL.

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

// SecretTransport - middleware для http.Client
// Автоматически добавляет секретный query-параметр ко всем запросам.
type SecretTransport struct {
	Base       http.RoundTripper
	QueryParam string // Например: "eyMjBapF=OXaAOjtG"
	BaseURL    string // Базовый URL
}

func (t *SecretTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 1. ЧИНИМ URL (Библиотека почему-то шлет относительные пути)
	if req.URL.Scheme == "" && t.BaseURL != "" {
		if base, err := url.Parse(t.BaseURL); err == nil {
			// Создаем новый URL на основе базового, добавляя путь из запроса
			newURL := *base
			newURL.Path = strings.TrimRight(base.Path, "/") + req.URL.Path
			newURL.RawQuery = req.URL.RawQuery // сохраняем параметры если были
			req.URL = &newURL
		}
	}

	// 2. ДОБАВЛЯЕМ СЕКРЕТНЫЙ ПАРАМЕТР
	parts := strings.SplitN(t.QueryParam, "=", 2)
	if len(parts) == 2 {
		key, value := parts[0], parts[1]
		q := req.URL.Query()
		q.Set(key, value)
		req.URL.RawQuery = q.Encode()
	}

	// Выполняем запрос через базовый транспорт
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func main() {
	ctx := context.Background()
	// 1. Загрузка переменных окружения
	// Пытаемся загрузить .env файл, но не паникуем, если его нет (может быть в Docker или CI)
	if err := godotenv.Load(); err != nil {
		log.Println("Инфо: .env файл не найден, используем системные переменные окружения")
	}

	// 2. Конфигурация из переменных окружения
	// Загружаем данные из .env файла или системного окружения
	baseURL := os.Getenv("REMNA_BASE_PANEL")
	token := os.Getenv("REMNA_TOKEN")
	urlSecret := os.Getenv("REMNA_SECRET_TOKEN") // Используем правильное имя переменной из .env

	if baseURL == "" || token == "" || urlSecret == "" {
		log.Fatal("Ошибка: Не все переменные окружения заданы (REMNA_BASE_PANEL, REMNA_TOKEN, REMNA_SECRET_TOKEN)")
	}

	fmt.Println("=== Запуск теста через библиотеку remnawave-api-go ===")

	// 2. Настройка HTTP клиента с "секретным" транспортом
	// Это ключевой момент: мы учим библиотеку стучаться в "секретную дверь"
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &SecretTransport{
			QueryParam: urlSecret,
			BaseURL:    baseURL,
		},
	}

	// 3. Инициализация клиента библиотеки
	// Используем опцию WithClient для подмены стандартного http клиента
	// ВАЖНО: При использовании WithClient, ogen может игнорировать переданный URL в NewClient,
	// если клиент настроен определенным образом.
	// Но главная проблема была в том, что мы пытались чинить URL в транспорте, хотя библиотека
	// должна сама это делать.

	// Пробуем другой подход: если ogen генерирует относительные URL, значит он ожидает,
	// что базовый URL где-то сохранен.
	baseClient, err := remapi.NewClient(
		baseURL,
		remapi.StaticToken{Token: token},
		remapi.WithClient(httpClient),
	)
	// В библиотеке ogen, если передан http.Client, он используется "как есть".
	// Но если клиент не имеет настроенного транспорта с базовым URL (как это часто бывает в сгенерированных клиентах),
	// то он просто посылает запрос на путь.
	// ОДНАКО: NewClient принимает serverURL. Куда он девается?
	// Внутри NewClient обычно парсится URL и сохраняется.
	// При выполнении запроса, ogen склеивает serverURL и путь.
	// НО ПОЧЕМУ у нас приходит /api/users без хоста?

	// Возможно, библиотека `remnawave-api-go` версии v2 ведет себя специфично.
	// Попробуем грязный хак: вернем восстановление URL в транспорт, но сделаем это ПРАВИЛЬНО.
	// Проблема в том, что req.URL.Scheme пустой.
	// Значит, http.Client получил запрос с URL "/api/users".
	// Это значит, что библиотека сформировала такой запрос.

	// Попробуем добавить восстановление URL обратно в транспорт, но с полным парсингом.
	// И добавим импорт net/url обратно.
	if err != nil {
		log.Fatalf("Ошибка инициализации клиента: %v", err)
	}

	// Обертка для удобного доступа к методам (Users(), Nodes() и т.д.)
	client := remapi.NewClientExt(baseClient)

	// 4. Тест: Получение списка пользователей (проверка подключения)
	// В библиотеке нет метода GetAllUsers напрямую в Users(), нужно искать подходящий или использовать поиск
	// Попробуем получить конкретного пользователя по UUID (того, что создали ранее) или просто проверим доступ
	// Библиотека сгенерирована через ogen, методы могут отличаться.
	// Попробуем создать нового пользователя, так как это была задача.

	// Попробуем создать пользователя с уникальным именем, чтобы избежать коллизий
	newUsername := fmt.Sprintf("library_user_%d", time.Now().Unix())
	fmt.Printf("Попытка создать пользователя: %s\n", newUsername)

	// Создаем DTO запроса
	// Важно: используем remapi.Option для необязательных полей, если они так определены,
	// или просто заполняем структуру.
	// Ogen генерирует структуры, где обязательные поля - значения, а опциональные - generic Opt...

	// Вычисляем дату истечения
	expireAt := time.Now().Add(30 * 24 * time.Hour)

	createUserReq := &remapi.CreateUserRequestDto{
		Username: newUsername,
		ExpireAt: expireAt, // Поле, видимо, требует time.Time, а не OptDateTime
	}

	resp, err := client.Users().CreateUser(ctx, createUserReq)
	if err != nil {
		log.Printf("Ошибка при создании пользователя: %v", err)
		return
	}

	// Обработка ответа (он может быть разных типов)
	switch r := resp.(type) {
	case *remapi.UserResponse:
		fmt.Printf("✅ УСПЕХ! Пользователь создан.\n")
		fmt.Printf("UUID: %s\n", r.Response.UUID) // Исправлено: UUID вместо Uuid
		fmt.Printf("Username: %s\n", r.Response.Username)
	case *remapi.BadRequestError:
		fmt.Println("❌ Ошибка валидации (400):")
		for _, e := range r.Errors {
			fmt.Printf("- %s: %s\n", e.Code, e.Message)
		}
	default:
		fmt.Printf("Получен ответ неожиданного типа: %T\n", r)
	}
}
