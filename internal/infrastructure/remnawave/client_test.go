// nolint
package remnawave

import (
	"ProxyMaster_v2/internal/config"
	"ProxyMaster_v2/pkg/logger"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRemnaClient_GetUUIDByUsername(t *testing.T) {
	tests := []struct {
		name         string
		username     string
		responseCode int                    // код который мы ожидаем
		responseBody map[string]interface{} // JSON формат ответа
		wantErr      error                  // ожидаемая ошибка (nil если успех)
		wantUUID     string                 // если успех, какой UUID ожидаем
	}{
		// Успешный сценарий: сервер возвращает 200 OK с корректными данными
		{
			name:         "success",
			username:     "Alice",
			responseCode: http.StatusOK,
			responseBody: map[string]interface{}{
				"response": map[string]interface{}{
					"uuid":     "11111111-2222-3333-4444-555555555555",
					"username": "Alice",
				},
			},
			wantErr:  nil,
			wantUUID: "11111111-2222-3333-4444-555555555555",
		},
		// Пользователь не найден: сервер возвращает 404 Not Found
		{
			name:         "not found",
			username:     "unknown",
			responseCode: http.StatusNotFound,
			responseBody: nil, // При 404 тело ответа может быть пустым
			wantErr:      ErrNotFound,
			wantUUID:     "",
		},
		// Ошибка сервера: сервер возвращает 500 Internal Server Error
		{
			name:         "internal server error",
			username:     "bob",
			responseCode: http.StatusInternalServerError,
			responseBody: nil,
			wantErr:      ErrInternalServerError,
			wantUUID:     "",
		},
		// Пустой UUID в ответе: сервер возвращает 200 OK, но UUID пустой
		// Это нарушает контракт API, метод должен вернуть ошибку
		{
			name:         "empty uuid in response",
			username:     "charlie",
			responseCode: http.StatusOK,
			responseBody: map[string]interface{}{
				"response": map[string]interface{}{
					"uuid":     "",
					"username": "charlie",
				},
			},
			wantErr:  ErrUUIDorUsernameIsNill,
			wantUUID: "",
		},
		// Пустой username в ответе: аналогично пустому UUID
		{
			name:         "empty username in response",
			username:     "David",
			responseCode: http.StatusOK,
			responseBody: map[string]interface{}{
				"response": map[string]interface{}{
					"uuid":     "22222222-3333-4444-5555-666666666666",
					"username": "",
				},
			},
			wantErr:  ErrUUIDorUsernameIsNill,
			wantUUID: "",
		},
		// Некорректная структура JSON: отсутствует поле "response"
		// Метод должен корректно обработать эту ситуацию и вернуть ошибку
		{
			name:         "missing response field",
			username:     "eve",
			responseCode: http.StatusOK,
			responseBody: map[string]interface{}{
				"wrong": "structure", // Неправильная структура JSON
			},
			wantErr:  ErrUUIDorUsernameIsNill,
			wantUUID: "",
		},
		// Некорректный JSON: сервер возвращает 200 OK, но тело не является валидным JSON
		// Это проверяет обработку ошибок парсинга JSON
		{
			name:         "invalid json response",
			username:     "frank",
			responseCode: http.StatusOK,
			responseBody: nil, // Будем отправлять не-JSON данные
			wantErr:      ErrFailedToUnmarshal,
			wantUUID:     "",
		},
		// Bad Request (400): сервер возвращает 400 Bad Request
		// Согласно логике метода, все статусы кроме 200 и 404 приводят к ErrInternalServerError
		{
			name:         "bad request",
			username:     "grace",
			responseCode: http.StatusBadRequest,
			responseBody: nil,
			wantErr:      ErrInternalServerError,
			wantUUID:     "",
		},
	}

	for _, tc := range tests {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			// NewServer создание фейкового сервера
			// Создает тестовый сервер
			srv := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Делаем строку для запроса
					expectedPath := "/api/users/by-username/" + tcc.username

					// Сравниваем то что получилось в запросе
					// и то что ожидаем
					if !strings.HasSuffix(r.URL.Path, expectedPath) {
						t.Errorf(
							"unexpected request path: got %s, want %s",
							r.URL.Path,
							expectedPath,
						)
					}

					// r.URL.RawQuery токен, например
					// https://api.example.com/users?token=abc123&limit=10
					//               ↑
					//               └── r.URL.Path = "/users"
					//                    ↑
					//                    └── r.URL.RawQuery = "token=abc123&limit=10"

					if !strings.Contains(r.URL.RawQuery, "testtoken") {
						t.Errorf("missing secret token in query")
					}

					//
					authHeader := r.Header.Get("Authorization")
					if authHeader != "Bearer testkey" {
						t.Errorf("missing or incorrect Authorization header: got %s", authHeader)
					}
					w.WriteHeader(tcc.responseCode)
					if tcc.responseBody != nil {
						w.Header().Set("Content-Type", "application/json")
						if err := json.NewEncoder(w).Encode(tcc.responseBody); err != nil {
							t.Fatalf("failed to encode response: %v", err)
						}
					} else {
						_, _ = w.Write([]byte("not a json"))
					}
				}),
			)
			defer srv.Close()
			cfg := &config.Config{
				RemnaPanelURL:       srv.URL,
				RemnaSecretURLToken: "testtoken",
				RemnaKey:            "testkey",
			}
			logClient, _ := logger.NewSlog("debug")
			client := NewRemnaClient(cfg, logClient)
			gotUUID, err := client.GetUUIDByUsername(context.Background(), tcc.username)
			if tcc.wantErr != nil {
				// Ожидаем ошибку
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if !errors.Is(err, tcc.wantErr) {
					t.Fatalf("got error %v, want %v", err, tcc.wantErr)
				}
			} else {
				// Ожидаем успех
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if gotUUID != tcc.wantUUID {
					t.Fatalf("got UUID %q, want %q", gotUUID, tcc.wantUUID)
				}
			}
		})
	}
}
